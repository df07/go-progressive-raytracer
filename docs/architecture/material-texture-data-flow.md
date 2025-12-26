# Material and Texture System Data Flow

## Overview

UV coordinates flow from Geometry.Hit() → SurfaceInteraction → Material.Scatter/EvaluateBRDF() → ColorSource.Evaluate(). The SurfaceInteraction acts as a complete data carrier containing Point, Normal, UV, Material, and distance. Preserving the complete SurfaceInteraction is critical - partial reconstruction loses UV and breaks texture sampling in both PT and BDPT.

## Complete Data Flow

### Step 1: Ray-Geometry Intersection

**Entry point**: Scene.BVH.Hit() or Shape.Hit()

**What happens**:
```go
// In /pkg/geometry/sphere.go (example)
func (s *Sphere) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
    // 1. Ray-sphere intersection math
    t := solveQuadratic(...)
    if t < tMin || t > tMax { return nil, false }

    // 2. Calculate hit point
    point := ray.At(t)

    // 3. Calculate surface normal
    normal := point.Sub(s.Center).Normalize()

    // 4. Calculate UV coordinates (sphere-specific)
    uv := s.calculateUV(point, normal)

    // 5. Create complete SurfaceInteraction
    hit := &material.SurfaceInteraction{
        Point:    point,
        Normal:   normal,
        UV:       uv,
        Material: s.Material,
        T:        t,
        Ray:      ray,
    }

    return hit, true
}
```

**UV calculation** (sphere example, `/pkg/geometry/sphere.go:89-103`):
```go
func (s *Sphere) calculateUV(p, normal core.Vec3) core.Vec2 {
    // Spherical coordinates: theta ∈ [0, 2π], phi ∈ [0, π]
    theta := math.Atan2(normal.Z, normal.X)
    phi := math.Acos(core.Clamp(normal.Y, -1, 1))

    // Map to UV ∈ [0,1]
    u := (theta + math.Pi) / (2 * math.Pi)
    v := phi / math.Pi

    return core.Vec2{X: u, Y: v}
}
```

**Other geometries**:
- **Quad**: Barycentric coordinates based on quad parameters (`quad.go`)
- **Triangle**: Interpolate vertex UVs from mesh attributes (`triangle.go`)
- **Mesh**: Per-vertex UV from PLY file or procedural generation (`triangle_mesh.go`)

**Key point**: Each geometry type is responsible for generating correct UV coordinates. UV generation happens once per intersection.

### Step 2: SurfaceInteraction Transport

**Data carrier**: `material.SurfaceInteraction` (`/pkg/material/interfaces.go`)

```go
type SurfaceInteraction struct {
    Point    core.Vec3       // Hit point in world space
    Normal   core.Vec3       // Surface normal (unit vector)
    UV       core.Vec2       // Texture coordinates [0,1]
    Material Material        // Material at this point
    T        float64         // Ray parameter (distance)
    Ray      core.Ray        // Incoming ray
}
```

**Why all fields matter**:
- **Point**: Needed for world-space texture lookup (procedural textures)
- **Normal**: Required for BRDF evaluation (cosine terms)
- **UV**: Primary texture coordinate (lost → texture sampled at wrong location)
- **Material**: Interface for Scatter() and EvaluateBRDF()
- **T**: Distance (used in PDF calculations, geometric terms)
- **Ray**: Original ray (used for reflection/refraction)

**Transport through pipeline**:
```
Geometry.Hit() creates SurfaceInteraction
    ↓
Integrator receives *SurfaceInteraction
    ↓
Material.Scatter() receives SurfaceInteraction
    ↓ (Path Tracing)
Material.EvaluateBRDF() receives SurfaceInteraction
    ↓ (BDPT)
Vertex stores *SurfaceInteraction (embedded)
    ↓
Connection evaluation uses Vertex.SurfaceInteraction
    ↓
Material.EvaluateBRDF() receives SurfaceInteraction
```

### Step 3: Material Scatter (Path Construction)

**Entry point**: Material.Scatter() (`/pkg/material/lambertian.go` example)

**What happens**:
```go
// In Lambertian.Scatter()
func (l *Lambertian) Scatter(ray core.Ray, hit material.SurfaceInteraction, sampler core.Sampler) (material.ScatterResult, bool) {
    // 1. Sample texture at UV coordinates
    albedo := l.Albedo.Evaluate(hit.UV, hit.Point)
    //                            ↑        ↑
    //                        Texture    World
    //                        coords     coords

    // 2. Sample random direction (cosine-weighted)
    scatterDir := sampleCosineHemisphere(hit.Normal, sampler)

    // 3. Calculate PDF
    pdf := scatterDir.Dot(hit.Normal) / math.Pi

    // 4. Return scatter result
    return material.ScatterResult{
        Scattered:   core.NewRay(hit.Point, scatterDir),
        Attenuation: albedo,  // Sampled texture color
        PDF:         pdf,
    }, true
}
```

**Texture evaluation** (called by Scatter):
```go
// In ImageTexture.Evaluate() (/pkg/material/image_texture.go)
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    // 1. Wrap UV coordinates
    u := uv.X - math.Floor(uv.X)  // [0, 1]
    v := 1 - (uv.Y - math.Floor(uv.Y))  // Flip V, [0, 1]

    // 2. Map to pixel coordinates
    x := int(u * float64(t.Width))
    y := int(v * float64(t.Height))

    // 3. Clamp to image bounds
    x = clamp(x, 0, t.Width-1)
    y = clamp(y, 0, t.Height-1)

    // 4. Sample pixel color
    idx := (y*t.Width + x) * 3
    return core.Vec3{
        X: float64(t.Pixels[idx]) / 255.0,
        Y: float64(t.Pixels[idx+1]) / 255.0,
        Z: float64(t.Pixels[idx+2]) / 255.0,
    }
}
```

**Key point**: Scatter() calls `Evaluate(hit.UV, hit.Point)` exactly once. The returned albedo is stored in `ScatterResult.Attenuation`.

### Step 4: Material BRDF Evaluation (Direct Lighting)

**Entry point**: Material.EvaluateBRDF() (`/pkg/material/lambertian.go` example)

**What happens**:
```go
// In Lambertian.EvaluateBRDF()
func (l *Lambertian) EvaluateBRDF(
    incoming core.Vec3,
    outgoing core.Vec3,
    hit *material.SurfaceInteraction,
    mode material.TransportMode,
) core.Vec3 {
    // 1. Sample texture at SAME UV coordinates
    albedo := l.Albedo.Evaluate(hit.UV, hit.Point)
    //                            ↑        ↑
    //                        Same UV   Same point
    //                        as Scatter()

    // 2. Lambertian BRDF: albedo / π
    return albedo.Multiply(core.InvPi)
}
```

**Critical consistency requirement**:
- `Scatter()` samples texture at `(hit.UV, hit.Point)`
- `EvaluateBRDF()` samples texture at `(hit.UV, hit.Point)`
- **Must be same hit**, not reconstructed SurfaceInteraction
- Same UV → same texture sample → consistent BRDF

**Why EvaluateBRDF exists**:
- Scatter() returns BRDF/PDF for randomly sampled direction
- EvaluateBRDF() returns BRDF for specific direction (light sampling, connections)
- Both must evaluate texture identically

### Step 5: Data Flow in Path Tracing

**Simplified PT flow**:
```
1. Camera.GetRay() → ray

2. Scene.BVH.Hit(ray) → hit (SurfaceInteraction)
   Contains: Point, Normal, UV, Material

3. Material.Scatter(ray, hit, sampler) → scatter
   Internally: albedo = Albedo.Evaluate(hit.UV, hit.Point)
   Returns: scattered ray, attenuation (albedo), PDF

4. CalculateDirectLighting():
   Sample light → lightDirection
   Material.EvaluateBRDF(incomingDir, lightDirection, hit, Radiance)
   Internally: albedo = Albedo.Evaluate(hit.UV, hit.Point)
   Returns: BRDF value (albedo / π for Lambertian)

5. CalculateIndirectLighting():
   Recursively trace scattered ray
   Weight by scatter.Attenuation and MIS

6. Combine direct + indirect with MIS weighting
```

**UV journey in PT**:
```
Geometry generates UV
    ↓
SurfaceInteraction stores UV
    ↓
Scatter() reads UV → samples texture → returns attenuation
    ↓
EvaluateBRDF() reads UV → samples texture → returns BRDF
    ↓
Both sample same UV → consistent color
```

### Step 6: Data Flow in BDPT

**BDPT vertex storage** (`/pkg/integrator/bdpt.go:14-36`):
```go
type Vertex struct {
    *material.SurfaceInteraction  // EMBEDDED pointer

    // Additional BDPT data
    IncomingDirection core.Vec3
    AreaPdfForward   float64
    AreaPdfReverse   float64
    Beta             core.Vec3
    // ...
}
```

**Path construction**:
```
1. generateCameraPath():
   For each bounce:
       hit := Scene.BVH.Hit(ray)  // Get SurfaceInteraction
       vertex := Vertex{
           SurfaceInteraction: hit,  // Store COMPLETE hit
       }
       scatter := hit.Material.Scatter(ray, *hit, sampler)
       // Scatter() samples texture at hit.UV
       path.Vertices = append(path.Vertices, vertex)

2. generateLightPath():
   Same logic, stores complete SurfaceInteraction in each vertex
```

**Connection evaluation** (`evaluateConnectionStrategy`):
```go
// Get vertices from paths
cameraVertex := &cameraPath.Vertices[t-1]
lightVertex := &lightPath.Vertices[s-1]

// Evaluate BRDF at camera vertex
cameraBRDF := cameraVertex.Material.EvaluateBRDF(
    cameraVertex.IncomingDirection,
    connectionDirection,
    cameraVertex.SurfaceInteraction,  // Use stored SurfaceInteraction
    TransportModeRadiance,
)
// EvaluateBRDF() reads cameraVertex.UV (from embedded SurfaceInteraction)
// Samples texture at correct UV

// Evaluate BRDF at light vertex
lightBRDF := lightVertex.Material.EvaluateBRDF(
    lightVertex.IncomingDirection,
    -connectionDirection,
    lightVertex.SurfaceInteraction,  // Use stored SurfaceInteraction
    TransportModeImportance,
)
// EvaluateBRDF() reads lightVertex.UV
// Samples texture at correct UV
```

**UV journey in BDPT**:
```
Geometry generates UV
    ↓
SurfaceInteraction stores UV
    ↓
Vertex embeds SurfaceInteraction (preserves UV)
    ↓
Scatter() during path construction reads UV → samples texture
    ↓
Connection evaluation reads vertex.UV (embedded field)
    ↓
EvaluateBRDF() reads UV → samples texture
    ↓
Same UV throughout → consistent texture sampling
```

## Component Responsibilities

### Geometry Primitives (`/pkg/geometry/`)

**Responsibility**: Generate UV coordinates for intersection points

**UV generation strategies**:

**Sphere** (`sphere.go`):
- Spherical coordinates (theta, phi) → (u, v)
- Continuous mapping, no seams at poles

**Quad** (`quad.go`):
- Barycentric coordinates within quad
- Based on edge vectors U and V

**Triangle** (`triangle.go`):
- Barycentric interpolation of vertex UVs
- Requires UV attributes in mesh data

**Mesh** (`triangle_mesh.go`):
- Per-vertex UVs from PLY file or procedural
- Interpolated per-triangle using barycentric coords

**Other primitives** (Disc, Cylinder, Cone, Box):
- Primitive-specific parameterization
- See individual files in `/pkg/geometry/`

### Material Interface (`/pkg/material/interfaces.go`)

**Responsibility**: Evaluate BRDF using texture data

**Two evaluation modes**:

**Scatter()**:
- Called during path construction (each bounce)
- Samples random direction according to BRDF
- Returns attenuation (BRDF/PDF weighted)
- Calls `ColorSource.Evaluate(uv, point)` once

**EvaluateBRDF()**:
- Called for specific direction queries (light sampling, connections)
- Evaluates BRDF for given incoming/outgoing directions
- Calls `ColorSource.Evaluate(uv, point)` again
- Must produce same texture value as Scatter() for same hit

**TransportMode**:
- `TransportModeRadiance`: Camera path (light traveling toward camera)
- `TransportModeImportance`: Light path (importance traveling toward light)
- Affects non-symmetric BRDFs (most materials ignore this)

### ColorSource Interface (`/pkg/material/color_source.go`)

**Responsibility**: Provide spatially-varying color values

**Interface**:
```go
type ColorSource interface {
    // Evaluate returns color at given UV and world position
    Evaluate(uv core.Vec2, point core.Vec3) core.Vec3
}
```

**Implementations**:

**SolidColor**:
```go
func (s *SolidColor) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    return s.Color  // Constant, ignores UV and point
}
```

**ImageTexture**:
```go
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    // Maps UV to pixel coordinates
    // Samples image data
    return sampledColor
}
```

**CheckerboardTexture**:
```go
func (c *CheckerboardTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    // Checkerboard pattern based on UV
    checker := (int(u*scale) + int(v*scale)) % 2
    if checker == 0 {
        return c.Color1
    }
    return c.Color2
}
```

**Procedural textures**:
- Use `point` for world-space patterns (noise, gradients)
- Use `uv` for surface-space patterns (checkerboard, stripes)
- Both parameters available for flexible patterns

## Why Preserving Complete SurfaceInteraction Is Critical

### The Problem: Partial Reconstruction

**WRONG** (loses UV):
```go
// In BDPT connection evaluation - INCORRECT
newHit := material.SurfaceInteraction{
    Point:  vertex.Point,
    Normal: vertex.Normal,
    // Missing: UV, Material, T, Ray
}

brdf := vertex.Material.EvaluateBRDF(
    incomingDir,
    outgoingDir,
    &newHit,  // UV is zero-initialized!
    mode,
)
```

**Result**:
- `newHit.UV = (0, 0)` (zero value)
- `EvaluateBRDF()` calls `texture.Evaluate((0,0), point)`
- Texture always sampled at corner → wrong color
- **Bug manifestation**: Checkerboard appears as solid color in BDPT

### The Solution: Preserve Complete SurfaceInteraction

**CORRECT** (preserves all fields):
```go
// Store complete SurfaceInteraction in vertex
vertex := Vertex{
    SurfaceInteraction: hit,  // Complete hit with UV
}

// Later, in connection evaluation
brdf := vertex.Material.EvaluateBRDF(
    incomingDir,
    outgoingDir,
    vertex.SurfaceInteraction,  // Has correct UV
    mode,
)
```

**Result**:
- `vertex.UV` has correct value from geometry
- `EvaluateBRDF()` calls `texture.Evaluate(vertex.UV, vertex.Point)`
- Texture sampled at correct coordinates → correct color

### Why Each Field Matters

**Point**:
- World-space texture lookup (procedural textures)
- Geometric term calculations (distance between vertices)

**Normal**:
- BRDF evaluation (cosine terms)
- PDF calculations (cosine-weighted sampling)
- Reflection/refraction direction

**UV**:
- Primary texture coordinate
- Lost → texture sampled at (0,0) → wrong color

**Material**:
- Interface to Scatter(), EvaluateBRDF(), PDF()
- Lost → cannot evaluate BRDF

**T** (distance):
- PDF conversions (solid angle → area measure)
- Geometric term in connections

**Ray**:
- Reflection/refraction calculations
- Incoming direction for BRDF

**Rule**: Never create new SurfaceInteraction. Always preserve the one from Geometry.Hit().

## Examples of Correct Data Flow

### Path Tracing Example

```go
// 1. Intersection
hit, isHit := scene.BVH.Hit(ray, 0.001, 1000)
// hit.UV = (0.35, 0.67) from sphere geometry

// 2. Scatter
scatter, didScatter := hit.Material.Scatter(ray, *hit, sampler)
// Internally: albedo = texture.Evaluate(0.35, 0.67) → (0.8, 0.2, 0.2)
// Returns: attenuation = (0.8, 0.2, 0.2)

// 3. Direct lighting
lightSample := lights.SampleLight(...)
brdf := hit.Material.EvaluateBRDF(
    ray.Direction,
    lightSample.Direction,
    hit,  // Same hit, same UV
    TransportModeRadiance,
)
// Internally: albedo = texture.Evaluate(0.35, 0.67) → (0.8, 0.2, 0.2)
// Returns: BRDF = (0.8/π, 0.2/π, 0.2/π)

// Result: Consistent texture color in both Scatter and EvaluateBRDF
```

### BDPT Example

```go
// 1. Path construction
hit, isHit := scene.BVH.Hit(ray, 0.001, 1000)
// hit.UV = (0.35, 0.67)

vertex := Vertex{
    SurfaceInteraction: hit,  // UV preserved
}

scatter, _ := hit.Material.Scatter(ray, *hit, sampler)
// albedo = texture.Evaluate(0.35, 0.67) → (0.8, 0.2, 0.2)

path.Vertices = append(path.Vertices, vertex)

// 2. Connection evaluation (later)
cameraVertex := &cameraPath.Vertices[2]
// cameraVertex.UV = (0.35, 0.67) from embedded SurfaceInteraction

brdf := cameraVertex.Material.EvaluateBRDF(
    cameraVertex.IncomingDirection,
    connectionDirection,
    cameraVertex.SurfaceInteraction,  // UV = (0.35, 0.67)
    TransportModeRadiance,
)
// albedo = texture.Evaluate(0.35, 0.67) → (0.8, 0.2, 0.2)

// Result: Same texture color as during path construction
```

## Debugging UV Data Flow

### Check UV Generation

```go
// In geometry Hit() function
uv := calculateUV(point, normal)
fmt.Printf("Generated UV: (%.3f, %.3f) for point (%v)\n", uv.X, uv.Y, point)
```

**Expected**: UV values vary across surface, range [0, 1]

### Check UV Preservation

```go
// After creating vertex
fmt.Printf("Vertex UV: (%.3f, %.3f)\n", vertex.UV.X, vertex.UV.Y)
```

**Expected**: Same UV as generated by geometry

### Check Texture Sampling

```go
// In ColorSource.Evaluate()
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    fmt.Printf("Texture sample: UV=(%.3f,%.3f) → ", uv.X, uv.Y)
    color := // ... sample texture ...
    fmt.Printf("color=(%.3f,%.3f,%.3f)\n", color.X, color.Y, color.Z)
    return color
}
```

**Expected**:
- PT: Multiple UV values, varying colors
- BDPT: Multiple UV values, varying colors
- **BUG**: Always UV=(0,0), solid color

### Verify Consistency

```go
// Track samples per hit point
var uvSamples = make(map[core.Vec3][]core.Vec2)

// In Scatter()
uvSamples[hit.Point] = append(uvSamples[hit.Point], hit.UV)

// In EvaluateBRDF()
previousUV := uvSamples[hit.Point][len(uvSamples[hit.Point])-1]
if hit.UV != previousUV {
    panic(fmt.Sprintf("UV mismatch: Scatter UV=%v, EvaluateBRDF UV=%v", previousUV, hit.UV))
}
```

## Access Log
