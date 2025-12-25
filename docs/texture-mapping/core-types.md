# Core Types and Interfaces

## Overview
The raytracer uses a simple, self-contained type system with no external dependencies. Colors are represented as Vec3, intersection data is stored in SurfaceInteraction structs, and 2D texture coordinates use Vec2. Currently UV coordinates are NOT computed by geometry primitives.

## Fundamental Types

### Vec3 (`pkg/core/vec3.go`)
- **Purpose**: 3D vectors for positions, directions, normals, and colors
- **Fields**: `X, Y, Z float64`
- **Key Operations**: Add, Subtract, Multiply, Dot, Cross, Normalize, Length
- **Color-Specific**:
  - `Luminance()` - Rec. 709 perceptual luminance (0.2126R + 0.7152G + 0.0722B)
  - `GammaCorrect(gamma)` - Apply gamma correction for display
  - `Clamp(min, max)` - Clamp RGB components
  - `MultiplyVec(other)` - Component-wise multiplication for color modulation

### Vec2 (`pkg/core/vec3.go`)
- **Purpose**: 2D coordinates for texture UVs, samples, etc.
- **Fields**: `X, Y float64`
- **Constructor**: `core.NewVec2(x, y float64)`
- **Usage**: Currently used for sampler Get2D() results, intended for texture coordinates

### Ray (`pkg/core/vec3.go`)
- **Fields**:
  - `Origin Vec3` - ray starting point
  - `Direction Vec3` - ray direction (not necessarily normalized)
- **Key Methods**:
  - `At(t float64) Vec3` - returns point at parameter t along ray
  - `NewRayTo(origin, target Vec3)` - create normalized ray pointing from origin to target

## Color Representation

**Colors are Vec3**: RGB values stored as Vec3 with X=red, Y=green, Z=blue. Values typically in [0,1] range but can exceed 1.0 for HDR.

**No separate Color type**: All color operations use Vec3 methods directly.

**HDR Support**: No clamping during computation - colors can have any positive value. Only clamped when writing to output image.

## Intersection Data Structure

### SurfaceInteraction (`pkg/material/interfaces.go`)
Contains all information about a ray-surface intersection:

```go
type SurfaceInteraction struct {
    Point     core.Vec3      // 3D intersection point
    Normal    core.Vec3      // Surface normal (always faces ray)
    T         float64        // Ray parameter (distance from origin)
    FrontFace bool           // True if ray hit front face
    Material  Material       // Material at intersection point
}
```

**Key Method**: `SetFaceNormal(ray Ray, outwardNormal Vec3)` - determines front/back face and sets normal to always point against the ray direction.

**Available Data**:
- 3D position (Point)
- Surface normal (Normal)
- Distance along ray (T)
- Front vs back face (FrontFace)
- Material reference (Material)

**NOT Available**:
- UV coordinates (not currently computed)
- Tangent/bitangent vectors
- Differential geometry (dP/du, dP/dv)
- Vertex attributes (per-vertex data)

## UV Coordinates: Current State

**Status**: Vec2 type exists but UV coordinates are NOT currently computed by any geometry primitive.

**What needs to be added for texture mapping**:
- Add `UV core.Vec2` field to SurfaceInteraction
- Implement UV calculation in each geometry primitive's Hit() method:
  - Sphere: Use spherical coordinates (phi, theta)
  - Quad: Use barycentric coordinates (alpha, beta from Hit calculation)
  - Triangle: Requires per-vertex UVs stored in mesh data
  - TriangleMesh: Requires UV data in PLYData and interpolation

**PLY Loader Support**: The PLY loader (`pkg/loaders/ply.go`) already supports reading texture coordinates:
- Detects `u/v`, `s/t`, or `texture_u/texture_v` properties
- Stores in `PLYData.TexCoords []core.Vec2`
- Per-vertex texture coordinates available but not currently used

## Sampler Interface

### Sampler (`pkg/core/sampling.go`)
Provides random number generation for rendering algorithms:

```go
type Sampler interface {
    Get1D() float64          // Returns random value in [0,1)
    Get2D() core.Vec2        // Returns 2D sample point
    Get3D() core.Vec3        // Returns 3D sample point
}
```

**Implementation**: RandomSampler wraps Go's `math/rand.Rand`

**Deterministic Rendering**: Tiles use seed-based samplers for reproducible results

**Usage**: Materials and integrators use samplers for all random decisions (scatter direction, Russian roulette, etc.)

## Type Conventions

**Float Precision**: All floating point values are `float64` (never float32)

**No Pointer Semantics**: Vec3, Vec2, Ray are value types (passed by value, immutable)

**Zero Values**: `Vec3{X:0, Y:0, Z:0}` represents black color or zero vector

**Normalization**: Direction vectors often normalized but not always - check specific usage

## Integration Points for Textures

To add texture mapping, you would need to:

1. Add `UV core.Vec2` field to SurfaceInteraction
2. Implement UV generation in each geometry primitive's Hit() method
3. Define texture sampling interface (e.g., `Texture` interface with `Sample(uv Vec2) Vec3`)
4. Replace material albedo colors with texture samplers
5. Load image data and implement bilinear/nearest filtering

The existing Vec2 type and PLY UV support provide the foundation - only the UV computation and texture sampling need implementation.
