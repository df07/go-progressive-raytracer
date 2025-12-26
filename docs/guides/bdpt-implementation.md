# BDPT Implementation Guide

## Overview

BDPT implements bidirectional path tracing by generating camera and light paths separately, then connecting them via all possible strategies. The Vertex structure embeds complete SurfaceInteraction to preserve UV coordinates and material data. Common pitfalls include incomplete PDF calculations, missing MIS terms, and recreating SurfaceInteraction without copying all fields (especially UV).

## Code Structure

### File Organization

- `/pkg/integrator/bdpt.go` - Main BDPT implementation (path construction, connection strategies)
- `/pkg/integrator/bdpt_mis.go` - MIS weight calculation
- `/pkg/integrator/bdpt_debug_test.go` - Debug tests with controllable sampling
- `/pkg/integrator/bdpt_test.go` - Unit tests for path generation
- `/pkg/integrator/bdpt_light_test.go` - Light-specific strategy tests

### Key Types

**Vertex**:
```go
type Vertex struct {
    *material.SurfaceInteraction  // EMBEDDED - preserves UV, Material, Normal, Point

    Light      lights.Light  // Light at this vertex
    LightIndex int          // Index in scene's light array

    IncomingDirection core.Vec3  // Direction ray arrived from

    // MIS PDFs (area measure)
    AreaPdfForward float64  // PDF generating this vertex forward
    AreaPdfReverse float64  // PDF generating this vertex reverse

    // Classification
    IsLight         bool
    IsCamera        bool
    IsSpecular      bool  // Delta BRDF (cannot connect)
    IsInfiniteLight bool

    // Transport quantities
    Beta         core.Vec3  // Throughput from path start
    EmittedLight core.Vec3  // Emission at this vertex
}
```

**Critical**: SurfaceInteraction is EMBEDDED (not a pointer field). Access fields directly: `vertex.UV`, `vertex.Normal`, `vertex.Material`.

**Path**:
```go
type Path struct {
    Vertices []Vertex
    Length   int
}
```

## Path Construction

### Camera Path Generation (`generateCameraPath`, line 106)

**Flow**:
```
1. Create camera vertex (virtual vertex at camera position)
2. Calculate camera direction PDF
3. Extend path through scene using extendPath()
4. Return complete camera path
```

**Camera vertex**:
```go
cameraVertex := Vertex{
    SurfaceInteraction: &material.SurfaceInteraction{
        Point:  ray.Origin,
        Normal: ray.Direction.Multiply(-1),  // Points back along ray
    },
    IsCamera: true,
    Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
}
```

**Why camera vertex exists**: Provides consistent vertex structure for MIS calculation (camera is "vertex 0" of camera path).

### Light Path Generation (`generateLightPath`, line 139)

**Flow**:
```
1. Sample light from scene (light selection PDF)
2. Sample emission point and direction on light (area PDF, direction PDF)
3. Create light vertex with emission
4. Calculate initial throughput beta
5. Extend path through scene using extendPath()
6. Apply infinite light corrections if needed
7. Return complete light path
```

**Light vertex**:
```go
lightVertex := Vertex{
    SurfaceInteraction: &material.SurfaceInteraction{
        Point:  emissionSample.Point,
        Normal: emissionSample.Normal,
    },
    LightIndex:      lightIndex,
    AreaPdfForward:  emissionSample.AreaPDF * lightSelectionPdf,
    IsLight:         true,
    IsInfiniteLight: sampledLight.Type() == lights.LightTypeInfinite,
    Beta:            emissionSample.Emission,
    EmittedLight:    emissionSample.Emission,
}
```

**Initial throughput** (line 177):
```go
beta := emissionSample.Emission.Multiply(
    cosTheta / (lightSelectionPdf * emissionSample.AreaPDF * emissionSample.DirectionPDF)
)
```

**Why this formula**: Converts emission radiance to path throughput accounting for light sampling PDFs and geometric term.

### Path Extension (`extendPath`, line 206)

**Shared logic** for both camera and light paths after initial vertex.

**Per-bounce loop**:
```go
for bounces := 0; bounces < maxBounces; bounces++ {
    // 1. Intersect scene
    hit, isHit := scene.BVH.Hit(currentRay, 0.001, math.Inf(1))

    // 2. Handle miss (background)
    if !isHit {
        if isCameraPath {
            vertex := createBackgroundVertex(...)
            path.Vertices = append(path.Vertices, *vertex)
        }
        break
    }

    // 3. Create vertex with FULL SurfaceInteraction
    vertex := Vertex{
        SurfaceInteraction: hit,  // CRITICAL: Preserves UV, Material, etc.
        IncomingDirection:  currentRay.Direction.Multiply(-1),
        Beta:               beta,
    }

    // 4. Capture emission
    vertex.EmittedLight = getEmittedLight(currentRay, hit)
    vertex.IsLight = !vertex.EmittedLight.IsZero()

    // 5. Convert previous vertex's solid angle PDF to area PDF for this vertex
    vertex.AreaPdfForward = vertexPrev.convertSolidAngleToAreaPdf(&vertex, pdfFwd)

    // 6. Scatter material
    scatter, didScatter := hit.Material.Scatter(currentRay, *hit, sampler)
    if !didScatter {
        path.Vertices = append(path.Vertices, vertex)
        break
    }

    // 7. Update throughput
    cosTheta := scatter.Scattered.Direction.AbsDot(hit.Normal)
    if scatter.IsSpecular() {
        beta = beta.MultiplyVec(scatter.Attenuation)
    } else {
        beta = beta.MultiplyVec(scatter.Attenuation).Multiply(cosTheta / scatter.PDF)
    }

    // 8. Calculate reverse PDF and set in previous vertex
    pdfRev, isReverseDelta := hit.Material.PDF(
        scatter.Scattered.Direction,
        currentRay.Direction.Multiply(-1),
        hit.Normal,
    )
    vertexPrev.AreaPdfReverse = vertex.convertSolidAngleToAreaPdf(vertexPrev, pdfRev)

    // 9. Add vertex to path
    path.Vertices = append(path.Vertices, vertex)
    path.Length++

    // 10. Continue with scattered ray
    currentRay = scatter.Scattered
}
```

**PDF handling** (`convertSolidAngleToAreaPdf`):
- Materials return PDFs in solid angle measure
- BDPT needs PDFs in area measure for MIS
- Conversion: `areaPDF = solidAnglePDF × cosθ / distance²`

## Connection Strategies

### Strategy Selection (`evaluateBDPTStrategy`, line 294)

```go
for s := 0; s <= lightPath.Length; s++ {      // Light path vertices
    for t := 1; t <= cameraPath.Length; t++ {  // Camera path vertices
        light, splats, sample := evaluateBDPTStrategy(cameraPath, lightPath, s, t, ...)

        if !light.IsZero() || len(splats) > 0 {
            misWeight := calculateMISWeight(...)
            totalLight = totalLight.Add(light.Multiply(misWeight))
            // Splats also weighted by MIS
        }
    }
}
```

**Strategy dispatch**:
- `s=1, t=1`: Not implemented (captured by s=0, t=1)
- `s=0, t≥1`: Path tracing strategy
- `s≥1, t=1`: Light tracing strategy (produces splats)
- `s=1, t≥2`: Direct lighting strategy
- `s≥2, t≥2`: Connection strategy

### Path Tracing Strategy (`evaluatePathTracingStrategy`, line 317)

**When**: s=0 (no light path vertices), t=full camera path

**Logic**:
```go
lastVertex := &cameraPath.Vertices[t-1]
contribution := lastVertex.EmittedLight.MultiplyVec(lastVertex.Beta)
return contribution
```

**What this does**: Returns emission at end of camera path weighted by accumulated throughput (standard path tracing).

**No splats**: Path tracing never produces splats (camera ray determines pixel).

### Direct Lighting Strategy (`evaluateDirectLightingStrategy`, line 334)

**When**: s=1 (sample one light vertex), t≥2 (camera path endpoint)

**Logic**:
```go
// 1. Get camera path endpoint
cameraVertex := &cameraPath.Vertices[t-1]
if cameraVertex.IsSpecular { return zero }  // Cannot connect to delta BRDF

// 2. Sample light from camera vertex
lightSample := lights.SampleLight(scene.Lights, cameraVertex.Point, ...)

// 3. Evaluate BRDF at camera vertex for light direction
brdf := cameraVertex.Material.EvaluateBRDF(
    cameraVertex.IncomingDirection,
    lightSample.Direction,
    cameraVertex.SurfaceInteraction,  // CRITICAL: Pass full SurfaceInteraction (has UV!)
    material.TransportModeRadiance,
)

// 4. Check visibility with shadow ray
visible := !scene.BVH.Hit(shadowRay, 0.001, lightSample.Distance-0.001)

// 5. Calculate contribution
cosTheta := lightSample.Direction.AbsDot(cameraVertex.Normal)
contribution := cameraVertex.Beta.MultiplyVec(brdf).MultiplyVec(lightSample.Emission).Multiply(cosTheta / lightSample.PDF)

// 6. Create sample vertex for MIS
sampleVertex := createLightVertex(lightSample, ...)

return contribution, sampleVertex
```

**Why EvaluateBRDF here**: Need BRDF value for specific light direction (not scattered direction).

**CRITICAL PITFALL**: Must pass `cameraVertex.SurfaceInteraction` (includes UV) to `EvaluateBRDF()`. Creating new SurfaceInteraction loses UV → texture sampled at (0,0).

### Light Tracing Strategy (`evaluateLightTracingStrategy`, line ~400)

**When**: s≥2 (light path), t=1 (connect to camera)

**Logic**:
```go
// 1. Get light path endpoint
lightVertex := &lightPath.Vertices[s-1]
if lightVertex.IsSpecular { return nil }  // Cannot connect to delta BRDF

// 2. Sample camera from light vertex
cameraSample := camera.SampleCameraFromPoint(lightVertex.Point, sampler)

// 3. Evaluate BRDF at light vertex for camera direction
brdf := lightVertex.Material.EvaluateBRDF(
    lightVertex.IncomingDirection,
    cameraSample.Direction,
    lightVertex.SurfaceInteraction,  // CRITICAL: Full SurfaceInteraction!
    material.TransportModeImportance,  // Note: Importance (light path)
)

// 4. Check visibility
visible := !scene.BVH.Hit(shadowRay, ...)

// 5. Calculate contribution
contribution := lightVertex.Beta.MultiplyVec(brdf) × geometricTerm × cameraSample.Weight

// 6. Create splat ray (contribution goes to different pixel)
splatRay := SplatRay{
    Ray:   cameraSample.Ray,  // Ray from camera through pixel
    Color: contribution,       // MIS-weighted later
}

// 7. Create sample vertex for MIS
sampleVertex := createCameraVertex(cameraSample, ...)

return []SplatRay{splatRay}, sampleVertex
```

**Why splats**: Camera ray may go through pixel different from the one we're rendering. Contribution must be added to correct pixel later.

**Transport mode**: `TransportModeImportance` for light path (vs `TransportModeRadiance` for camera path). Handles non-symmetric BRDFs correctly.

### Connection Strategy (`evaluateConnectionStrategy`, line ~500)

**When**: s≥2, t≥2 (connect light vertex to camera vertex)

**Logic**:
```go
// 1. Get endpoints
cameraVertex := &cameraPath.Vertices[t-1]
lightVertex := &lightPath.Vertices[s-1]

// 2. Cannot connect through specular vertices
if cameraVertex.IsSpecular || lightVertex.IsSpecular { return zero }

// 3. Calculate connection direction
direction := lightVertex.Point.Sub(cameraVertex.Point)
distance := direction.Length()
direction = direction.Normalize()

// 4. Evaluate BRDF at both vertices
cameraBRDF := cameraVertex.Material.EvaluateBRDF(
    cameraVertex.IncomingDirection,
    direction,
    cameraVertex.SurfaceInteraction,  // FULL SurfaceInteraction
    material.TransportModeRadiance,
)

lightBRDF := lightVertex.Material.EvaluateBRDF(
    lightVertex.IncomingDirection,
    direction.Multiply(-1),
    lightVertex.SurfaceInteraction,  // FULL SurfaceInteraction
    material.TransportModeImportance,
)

// 5. Geometric term
cosCam := direction.AbsDot(cameraVertex.Normal)
cosLight := direction.AbsDot(lightVertex.Normal)
G := cosCam * cosLight / (distance * distance)

// 6. Check visibility
visible := !scene.BVH.Hit(connectionRay, ...)

// 7. Calculate contribution
contribution := lightVertex.Beta.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraVertex.Beta).Multiply(G)

return contribution
```

**Geometric term**: `G = cosθ_camera × cosθ_light / distance²` accounts for solid angle conversion and surface orientation.

**Two BRDF evaluations**: Need BRDF at both endpoints for connection direction.

## Common Pitfalls

### 1. Losing UV Coordinates

**WRONG**:
```go
// Creating new SurfaceInteraction without UV
vertex := Vertex{
    SurfaceInteraction: &material.SurfaceInteraction{
        Point:  hit.Point,
        Normal: hit.Normal,
        // Missing: UV, Material, etc.
    },
}

// Later in connection evaluation
brdf := vertex.Material.EvaluateBRDF(..., vertex.SurfaceInteraction, ...)
// SurfaceInteraction has UV=(0,0) → texture sampled at wrong location!
```

**CORRECT**:
```go
// Use hit SurfaceInteraction directly (preserves all fields)
vertex := Vertex{
    SurfaceInteraction: hit,  // Complete SurfaceInteraction
}

// Or embed and access fields directly
type Vertex struct {
    *material.SurfaceInteraction  // Embedded pointer
}
// Access: vertex.UV, vertex.Point, vertex.Normal
```

**Why this matters**: `EvaluateBRDF()` calls `texture.Evaluate(hit.UV, hit.Point)`. If UV is not set, texture samples at (0,0) instead of correct coordinates.

**Real bug**: BDPT texture sampling inconsistency (fixed in recent commit). BDPT created new SurfaceInteraction in connection evaluation, lost UV, always sampled checkerboard at (0,0) → solid blue instead of checkerboard.

### 2. Incorrect PDF Calculations

**WRONG**:
```go
// Missing area PDF conversion
vertex.AreaPdfForward = pdfFwd  // pdfFwd is solid angle PDF!
```

**CORRECT**:
```go
// Convert solid angle PDF to area PDF
vertex.AreaPdfForward = vertexPrev.convertSolidAngleToAreaPdf(&vertex, pdfFwd)

// Implementation:
func (v *Vertex) convertSolidAngleToAreaPdf(next *Vertex, pdfSolidAngle float64) float64 {
    direction := next.Point.Sub(v.Point)
    distanceSquared := direction.LengthSquared()
    direction = direction.Normalize()
    cosTheta := direction.AbsDot(next.Normal)
    return pdfSolidAngle * cosTheta / distanceSquared
}
```

**Why this matters**: MIS weights require consistent PDF measure. Mixing solid angle and area PDFs breaks balance → brightness differences.

### 3. Missing Geometric Term

**WRONG**:
```go
// Connection without geometric term
contribution := lightBeta.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraBeta)
```

**CORRECT**:
```go
// Include geometric term
cosCam := direction.AbsDot(cameraVertex.Normal)
cosLight := direction.AbsDot(lightVertex.Normal)
G := cosCam * cosLight / (distance * distance)
contribution := lightBeta.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraBeta).Multiply(G)
```

**Why this matters**: Geometric term accounts for solid angle subtended by surfaces. Missing it causes brightness errors proportional to distance² → scale-dependent bugs.

### 4. Forgetting MIS Weighting

**WRONG**:
```go
// Adding contribution without MIS weight
totalLight = totalLight.Add(contribution)
```

**CORRECT**:
```go
// Apply MIS weight to each strategy
misWeight := calculateMISWeight(cameraPath, lightPath, sample, s, t, scene)
totalLight = totalLight.Add(contribution.Multiply(misWeight))
```

**Why this matters**: Without MIS, strategies double-count paths → image too bright. MIS balances contributions to maintain unbiasedness and reduce variance.

### 5. Transport Mode Confusion

**WRONG**:
```go
// Using same transport mode for both paths
cameraBRDF := cameraVertex.Material.EvaluateBRDF(..., TransportModeRadiance)
lightBRDF := lightVertex.Material.EvaluateBRDF(..., TransportModeRadiance)
```

**CORRECT**:
```go
// Camera path uses Radiance mode
cameraBRDF := cameraVertex.Material.EvaluateBRDF(..., TransportModeRadiance)
// Light path uses Importance mode
lightBRDF := lightVertex.Material.EvaluateBRDF(..., TransportModeImportance)
```

**Why this matters**: Non-symmetric BRDFs (e.g., some layered materials) require different evaluation based on light direction. Swap test: `BRDF(in, out, Radiance)` may not equal `BRDF(out, in, Importance)`.

## Verifying BDPT Changes

### Must-Run Tests After BDPT Modifications

**1. Luminance comparison test**:
```bash
go test -v ./pkg/renderer -run TestIntegratorLuminanceComparison
```
All scenes should pass within tolerance. If any fail:
- Check which scene fails (simple vs complex)
- Compare saved images (output/debug_renders/)
- Isolate with simpler scene

**2. Manual PT vs BDPT comparison**:
```bash
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100
# Note luminosity (e.g., 0.1245)

./raytracer --scene=cornell --integrator=bdpt --max-samples=100
# Note luminosity (e.g., 0.1238)

# Should be within 10% for Cornell box
```

**3. Textured scene test**:
```bash
# Test texture sampling specifically
go test -v ./pkg/renderer -run "Textured.*Checkerboard"

# Or manual visual check
./raytracer --scene=cornell --integrator=bdpt --max-samples=50
# Textures should appear correctly, not solid color
```

**4. Scale independence test**:
```bash
# Run large-scale Cornell box test
go test -v ./pkg/renderer -run "Large.*Scale.*Cornell"

# Should match unit-scale result within tolerance
# Scale-dependent bugs indicate missing area terms in PDFs
```

### What to Test When Changing Specific Code

**Modifying `generateCameraPath` or `generateLightPath`**:
- Test: `go test ./pkg/integrator -run TestExtendPath`
- Test: `go test ./pkg/integrator -run TestBDPTPathGeneration`
- Verify: Path lengths correct, Beta values reasonable

**Modifying `evaluateConnectionStrategy`**:
- Test: All luminance comparison tests
- Verify: No brightness differences between PT and BDPT
- Check: Texture sampling still works (UV preserved)

**Modifying MIS calculation**:
- Test: `go test ./pkg/integrator -run MIS`
- Verify: MIS weights sum to 1.0 for each pixel
- Check: No fireflies (weights should never be >1)

**Modifying PDF calculations**:
- Test: Scale independence test
- Verify: Unit-scale and 278×-scale produce same result
- Check: Luminance matches PT within tolerance

## Debugging BDPT Issues

### Enable Verbose Logging

```go
bdpt := NewBDPTIntegrator(config)
bdpt.Verbose = true  // Enables logf() statements
```

**Common log points**:
- Path generation: Beta values, path lengths
- Connection evaluation: BRDF values, geometric terms
- MIS calculation: PDF ratios, final weights

### Add Assertions

```go
// After MIS calculation
if misWeight < 0 || misWeight > 1.0 {
    panic(fmt.Sprintf("Invalid MIS weight: %.6f", misWeight))
}

// After BRDF evaluation
if brdf.X < 0 || brdf.Y < 0 || brdf.Z < 0 {
    panic(fmt.Sprintf("Negative BRDF: %v", brdf))
}

// After PDF conversion
if vertex.AreaPdfForward < 0 {
    panic(fmt.Sprintf("Negative area PDF: %.6f", vertex.AreaPdfForward))
}
```

### Isolate Strategy

**Test single strategy**:
```go
// In RayColor(), limit to specific s,t
for s := 0; s <= lightPath.Length; s++ {
    for t := 1; t <= cameraPath.Length; t++ {
        if s != 1 || t != 2 { continue }  // Only test s=1,t=2 (direct lighting)

        light, splats, sample := evaluateBDPTStrategy(...)
        // ...
    }
}
```

**Compare to PT**:
- s=0 strategies should match PT exactly (pure camera paths)
- s≥1 strategies add light-side sampling (should increase, not decrease, quality)

### Inspect Vertex Data

```go
// After path generation
for i, v := range cameraPath.Vertices {
    fmt.Printf("Camera vertex %d: Point=%v UV=%v Beta=%v AreaPdfFwd=%.6f\n",
               i, v.Point, v.UV, v.Beta, v.AreaPdfForward)
}

for i, v := range lightPath.Vertices {
    fmt.Printf("Light vertex %d: Point=%v UV=%v Beta=%v AreaPdfFwd=%.6f\n",
               i, v.Point, v.UV, v.Beta, v.AreaPdfForward)
}
```

**What to check**:
- UV values: Should vary, not always (0,0)
- Beta values: Should decrease along path (accumulated attenuation)
- PDF values: Should be positive, reasonable magnitude (not 1e-100 or 1e100)

## Access Log
