# Material System Architecture

## Overview
Materials implement a physically-based BRDF interface supporting both unidirectional path tracing and bidirectional path tracing (BDPT). Each material defines how light scatters at surfaces through three key methods: Scatter (random sampling), EvaluateBRDF (explicit evaluation), and PDF (probability density).

## Material Interface

All materials must implement (`pkg/material/interfaces.go`):

```go
type Material interface {
    // Generate random scattered direction
    Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool)

    // Evaluate BRDF for specific incoming/outgoing directions with transport mode
    EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3

    // Calculate PDF for specific incoming/outgoing directions
    // Returns (pdf, isDelta) where isDelta indicates if this is a delta function (specular)
    PDF(incomingDir, outgoingDir, normal core.Vec3) (pdf float64, isDelta bool)
}
```

### ScatterResult Structure
```go
type ScatterResult struct {
    Incoming    core.Ray   // The incoming ray
    Scattered   core.Ray   // The scattered ray (sampled direction)
    Attenuation core.Vec3  // BRDF value (color attenuation)
    PDF         float64    // Probability density (0 for specular/delta materials)
}
```

**Delta vs Non-Delta**:
- Non-delta (diffuse): PDF > 0, importance sampling used
- Delta (specular/glass): PDF = 0, perfect reflection/refraction only

### TransportMode
```go
type TransportMode int
const (
    Radiance   TransportMode = iota  // Camera vertices (divide by η² for refraction)
    Importance                       // Light vertices (no η² correction)
)
```

**Purpose**: Handles non-reciprocal light transport through refractive interfaces (critical for BDPT correctness).

## Built-in Material Types

### Lambertian (Diffuse) - `pkg/material/lambertian.go`

**Properties**:
- `Albedo core.Vec3` - Base color/reflectance

**Constructor**: `material.NewLambertian(albedo Vec3)`

**Behavior**:
- Perfectly diffuse surface (matte appearance)
- Cosine-weighted hemisphere sampling
- BRDF = albedo / π (energy conserving)
- PDF = cos(θ) / π
- Non-delta (PDF > 0)

**Example**:
```go
greenDiffuse := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))
```

### Metal (Specular Reflection) - `pkg/material/metal.go`

**Properties**:
- `Albedo core.Vec3` - Metal color
- `Fuzzness float64` - 0.0 = perfect mirror, 1.0 = very fuzzy (clamped to [0,1])

**Constructor**: `material.NewMetal(albedo Vec3, fuzzness float64)`

**Behavior**:
- Perfect or fuzzy reflection
- Fuzziness adds random perturbation to reflection direction
- BRDF = albedo (no π factor for specular)
- PDF = 0 (delta function)
- Absorbs rays that scatter below surface

**Example**:
```go
perfectMirror := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0)
brushedMetal := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)
```

### Dielectric (Glass/Transparent) - `pkg/material/dielectric.go`

**Properties**:
- `RefractiveIndex float64` - Index of refraction (1.5 for glass, 1.33 for water)

**Constructor**: `material.NewDielectric(refractiveIndex float64)`

**Behavior**:
- Both reflection and refraction (Fresnel equations)
- Schlick's approximation for reflectance
- Total internal reflection when applicable
- Attenuation = (1,1,1) for clear glass
- PDF = 0 (delta function)
- Handles entering vs exiting material via FrontFace flag
- **Transport mode critical**: Divides by η² for radiance transport through refractive boundaries

**Example**:
```go
glass := material.NewDielectric(1.5)
water := material.NewDielectric(1.33)
diamond := material.NewDielectric(2.42)
```

**Negative Radius Trick**: For hollow glass spheres, use negative radius for inner surface to flip normals.

### Emissive - `pkg/material/emissive.go`

**Purpose**: Materials that emit light (used by area lights).

**Properties**:
- `Emission core.Vec3` - Emitted radiance

**Constructor**: `material.NewEmissive(emission Vec3)`

**Emitter Interface**: Materials that emit implement:
```go
type Emitter interface {
    Emit(rayIn core.Ray, hit *SurfaceInteraction) core.Vec3
}
```

**Typical Usage**: Combined with light primitives (not used standalone as surface material).

### Layered Material - `pkg/material/layered.go`

**Purpose**: Coating system - applies one material over another.

**Constructor**: `material.NewLayered(coating, base Material)`

**Behavior**:
- Coating applied first (e.g., glass over colored base)
- Probabilistic layer selection based on Fresnel
- Supports complex effects like clear-coated paint

**Example**:
```go
// Glass coating over red diffuse
coatedRed := material.NewLayered(
    material.NewDielectric(1.5),
    material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1)),
)
```

## Material Property Specification

**Current State**: Materials have uniform properties across entire surface.

**Color Properties**:
- Lambertian: Single albedo color
- Metal: Single albedo color + scalar fuzzness
- Dielectric: Colorless (white attenuation)

**No Spatially-Varying Properties**: Materials cannot currently vary color or parameters based on position/UV.

## Render Pipeline Interaction

### When Materials are Queried

1. **Ray Intersection**: Geometry Hit() returns SurfaceInteraction with Material reference
2. **Path Tracing**: Integrator calls `material.Scatter()` to get random bounce direction
3. **BDPT**: Integrator calls `material.EvaluateBRDF()` to evaluate specific light paths
4. **MIS Weight Calculation**: Integrator calls `material.PDF()` for Multiple Importance Sampling

### Scatter Call Pattern
```go
// In integrator
hit, didHit := scene.BVH.Hit(ray, tMin, tMax)
if didHit {
    scatter, scattered := hit.Material.Scatter(ray, *hit, sampler)
    if scattered {
        // Continue path with scatter.Scattered ray
        // Attenuate by scatter.Attenuation
    }
}
```

### EvaluateBRDF Call Pattern
```go
// In BDPT when connecting paths
brdfValue := material.EvaluateBRDF(
    lightDir,      // Direction toward light
    cameraDir,     // Direction toward camera
    hit,           // Surface interaction
    Radiance,      // Transport mode
)
```

## Material Construction Patterns

**Direct Construction**: Most scenes use `New*()` constructors directly:
```go
mat := material.NewLambertian(core.NewVec3(r, g, b))
sphere := geometry.NewSphere(center, radius, mat)
```

**Material Reuse**: Same material can be shared by multiple objects:
```go
redDiffuse := material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1))
sphere1 := geometry.NewSphere(pos1, r1, redDiffuse)
sphere2 := geometry.NewSphere(pos2, r2, redDiffuse)
```

**Per-Object Materials**: Each geometry object stores its own material reference:
- Sphere: Single material for entire surface
- Quad: Single material for entire quad
- Triangle: Single material per triangle
- TriangleMesh: Can specify per-triangle materials via TriangleMeshOptions

## Material Assignment in Scenes

**Scene Definition** (`pkg/scene/*.go`): Scenes are built programmatically in Go code:

```go
// Example from default_scene.go
lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))
metalSilver := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0)
glass := material.NewDielectric(1.5)

sphere1 := geometry.NewSphere(core.NewVec3(-1, 0.5, -1), 0.5, metalSilver)
sphere2 := geometry.NewSphere(core.NewVec3(0, 0.5, -1), 0.5, glass)
sphere3 := geometry.NewSphere(core.NewVec3(1, 0.5, -1), 0.5, lambertianGreen)

scene.Shapes = append(scene.Shapes, sphere1, sphere2, sphere3)
```

**No Scene File Format**: Scenes are Go source files, not data files (except for PBRT loader support).

## Integration Strategy for Textures

To add texture mapping to materials, consider these approaches:

### Option 1: Textured Material Variants
Create new material types that wrap existing ones:
```go
type TexturedLambertian struct {
    AlbedoTexture Texture  // Instead of Vec3 albedo
}
```

**Pros**: Clean separation, preserves existing materials
**Cons**: Code duplication for each material type

### Option 2: Texture Interface in Albedo
Replace `Vec3` colors with a `ColorSource` interface:
```go
type ColorSource interface {
    Evaluate(uv Vec2) Vec3
}

type SolidColor struct { Color Vec3 }
type ImageTexture struct { Data [][]Vec3; Width, Height int }
```

**Pros**: Single material implementation handles both solid and textured
**Cons**: Requires refactoring all existing material types

### Option 3: Material Modifier Pattern
Wrap materials with texture evaluation:
```go
type TexturedMaterial struct {
    BaseMaterial Material
    AlbedoMap    Texture
}
```

**Pros**: Works with existing materials
**Cons**: Adds indirection, complicated BRDF evaluation

**Recommendation**: Option 2 provides the cleanest architecture for long-term extensibility.

## Key Constraints

- Materials cannot query world-space position (only local hit data)
- Materials are stateless (no random state between calls)
- All randomness must come from provided Sampler
- BRDF must be energy-conserving (reflectance ≤ 1.0)
- Delta materials must return PDF=0 and isDelta=true consistently
