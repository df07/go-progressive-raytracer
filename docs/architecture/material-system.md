# Material System

## Overview

Materials implement a physically-based BRDF interface supporting both unidirectional path tracing and bidirectional path tracing (BDPT). Each material defines how light scatters at surfaces through three key methods: Scatter (importance sampling), EvaluateBRDF (explicit evaluation), and PDF (probability density).

## Material Interface

All materials implement (`pkg/material/interfaces.go`):

```go
type Material interface {
    // Generate random scattered direction via importance sampling
    Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool)

    // Evaluate BRDF for specific incoming/outgoing directions
    EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3

    // Calculate PDF for specific incoming/outgoing directions
    // Returns (pdf, isDelta) where isDelta indicates specular reflection
    PDF(incomingDir, outgoingDir, normal core.Vec3) (pdf float64, isDelta bool)
}
```

### ScatterResult

```go
type ScatterResult struct {
    Incoming    core.Ray   // The incoming ray
    Scattered   core.Ray   // The scattered ray (sampled direction)
    Attenuation core.Vec3  // BRDF value (color attenuation)
    PDF         float64    // Probability density (0 for specular/delta materials)
}
```

**Delta vs Non-Delta Materials**:
- **Non-delta** (diffuse/glossy): PDF > 0, importance sampling distributes rays across hemisphere
- **Delta** (perfect specular/glass): PDF = 0, only single reflection/refraction direction

### TransportMode

```go
type TransportMode int
const (
    Radiance   TransportMode = iota  // Camera vertices
    Importance                       // Light vertices
)
```

**Purpose**: Handles non-reciprocal light transport through refractive interfaces. Critical for BDPT correctness when connecting camera and light paths through glass.

**Effect**: Divides BRDF by η² for radiance transport through refraction (see Dielectric material).

## ColorSource Interface

Materials support spatially-varying properties through the ColorSource abstraction (`pkg/material/color_source.go`):

```go
type ColorSource interface {
    Evaluate(uv core.Vec2, point core.Vec3) core.Vec3
}
```

**Implementations**:

**SolidColor**: Uniform color across surface (backward compatibility)
```go
solid := material.NewSolidColor(core.NewVec3(0.8, 0.2, 0.2))
```

**ImageTexture**: Samples from 2D image textures (`pkg/material/image_texture.go`)
- Nearest-neighbor filtering
- UV wrapping (repeat mode)
- V-flip for standard image coordinate systems

**Procedural Textures** (`pkg/material/procedural_textures.go`):
- `NewCheckerboardTexture()` - Checkerboard pattern
- `NewGradientTexture()` - Vertical color interpolation
- `NewUVDebugTexture()` - UV visualization (U→red, V→green)

## Built-in Materials

### Lambertian (`pkg/material/lambertian.go`)

Perfectly diffuse surface with matte appearance.

**Properties**:
- `Albedo ColorSource` - Base reflectance (solid or textured)

**Constructors**:
- `material.NewLambertian(albedo Vec3)` - Solid color (backward compatible)
- `material.NewTexturedLambertian(albedoTexture ColorSource)` - Textured material

**Behavior**:
- Cosine-weighted hemisphere sampling
- BRDF = albedo / π (energy conserving)
- PDF = cos(θ) / π
- Non-delta material
- Calls `Albedo.Evaluate(hit.UV, hit.Point)` to sample texture

**Examples**:
```go
// Solid color
greenDiffuse := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))

// Textured
texture := material.NewCheckerboardTexture(256, 256, 32, color1, color2)
texturedMat := material.NewTexturedLambertian(texture)
```

### Metal (`pkg/material/metal.go`)

Specular reflection with optional fuzziness.

**Properties**:
- `Albedo ColorSource` - Metal color (solid or textured)
- `Fuzzness float64` - 0.0 = perfect mirror, 1.0 = very fuzzy (clamped to [0,1])

**Constructors**:
- `material.NewMetal(albedo Vec3, fuzzness float64)` - Solid color (backward compatible)
- `material.NewTexturedMetal(albedoTexture ColorSource, fuzzness float64)` - Textured material

**Behavior**:
- Perfect or fuzzy reflection based on fuzziness parameter
- Fuzziness adds random perturbation to reflection direction
- BRDF = albedo (no π factor for specular)
- PDF = 0 (delta function)
- Absorbs rays that scatter below surface
- Calls `Albedo.Evaluate(hit.UV, hit.Point)` to sample texture

**Examples**:
```go
// Solid color
perfectMirror := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0)
brushedMetal := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)

// Textured
imageData, _ := loaders.LoadImage("metal_texture.png")
texture := material.NewImageTexture(imageData.Width, imageData.Height, imageData.Pixels)
texturedMetal := material.NewTexturedMetal(texture, 0.1)
```

### Dielectric (`pkg/material/dielectric.go`)

Glass and transparent materials with reflection and refraction.

**Properties**:
- `RefractiveIndex float64` - Index of refraction (1.5 for glass, 1.33 for water, 2.42 for diamond)

**Constructor**: `material.NewDielectric(refractiveIndex float64)`

**Behavior**:
- Both reflection and refraction governed by Fresnel equations
- Schlick's approximation for reflectance calculation
- Total internal reflection when applicable
- Attenuation = (1,1,1) for clear glass
- PDF = 0 (delta function)
- Handles entering vs exiting material via FrontFace flag
- **Transport mode critical**: Divides by η² for radiance transport

**Examples**:
```go
glass := material.NewDielectric(1.5)
water := material.NewDielectric(1.33)
diamond := material.NewDielectric(2.42)
```

**Hollow glass spheres**: Use negative radius for inner surface to flip normals.

### Emissive (`pkg/material/emissive.go`)

Light-emitting material for area lights.

**Properties**:
- `Emission core.Vec3` - Emitted radiance

**Constructor**: `material.NewEmissive(emission Vec3)`

**Emitter Interface**:
```go
type Emitter interface {
    Emit(rayIn core.Ray, hit *SurfaceInteraction) core.Vec3
}
```

**Usage**: Combined with light primitives (sphere lights, quad lights). Not typically used standalone as surface material.

### Layered (`pkg/material/layered.go`)

Coating system applying one material over another.

**Constructor**: `material.NewLayered(coating, base Material)`

**Behavior**:
- Coating material applied first (e.g., glass over colored base)
- Probabilistic layer selection based on Fresnel reflectance
- Enables effects like clear-coated paint or varnished wood

**Example**:
```go
// Glass coating over red diffuse base
coatedRed := material.NewLayered(
    material.NewDielectric(1.5),
    material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1)),
)
```

## Material Usage in Rendering

### Integration with Ray Tracing

**Step 1 - Ray Intersection**: Geometry Hit() method returns SurfaceInteraction with Material reference

**Step 2 - Path Tracing**: Integrator calls `material.Scatter()` to get random bounce direction

**Step 3 - BDPT**: Integrator calls `material.EvaluateBRDF()` to evaluate specific light paths

**Step 4 - MIS**: Integrator calls `material.PDF()` for Multiple Importance Sampling weight calculation

### Scatter Call Pattern

```go
// In path tracing integrator
hit, didHit := scene.BVH.Hit(ray, tMin, tMax)
if didHit {
    scatter, scattered := hit.Material.Scatter(ray, *hit, sampler)
    if scattered {
        // Continue path with scatter.Scattered ray
        // Attenuate throughput by scatter.Attenuation
    }
}
```

### EvaluateBRDF Call Pattern

```go
// In BDPT when connecting camera and light paths
brdfValue := material.EvaluateBRDF(
    lightDir,   // Direction toward light
    cameraDir,  // Direction toward camera
    hit,        // Surface interaction data
    Radiance,   // Transport mode
)
```

## Material Properties

**Spatially-Varying Support**: Materials can have varying properties via ColorSource interface

**Property Types**:
- Lambertian: Albedo (ColorSource) - supports solid colors and textures
- Metal: Albedo (ColorSource) + scalar fuzzness - supports solid colors and textures
- Dielectric: Scalar refractive index (colorless transmission) - uniform only

**Spatial Variation Methods**:
- Solid colors: Uniform properties across surface
- Image textures: Sample from 2D images using UV coordinates
- Procedural textures: Compute colors from UV coordinates or 3D position

## Material Assignment

Materials are assigned directly when creating geometry:

```go
// Solid color material
mat := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
sphere := geometry.NewSphere(center, radius, mat)

// Textured material
texture := material.NewImageTexture(width, height, pixels)
texturedMat := material.NewTexturedLambertian(texture)
quad := geometry.NewQuad(corner, u, v, texturedMat)
```

**Material Reuse**: Same material instance can be shared by multiple objects

**Per-Triangle Materials**: TriangleMesh supports different material per triangle via TriangleMeshOptions

## Design Constraints

- Materials are stateless (no persistent state between calls)
- All randomness must come from provided Sampler
- BRDF must be energy-conserving (total reflectance ≤ 1.0)
- Delta materials must consistently return PDF=0 and isDelta=true
- Materials cannot query world-space position (only local intersection data)
