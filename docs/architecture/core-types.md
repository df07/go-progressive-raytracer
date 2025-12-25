# Core Types and Interfaces

## Overview

The raytracer uses a minimal, self-contained type system with no external dependencies. Vec3 serves as both 3D vectors and RGB colors, intersection data is captured in SurfaceInteraction structs, and random sampling is abstracted through the Sampler interface.

## Fundamental Vector Types

### Vec3 (`pkg/core/vec3.go`)

**Purpose**: 3D vectors for positions, directions, normals, and RGB colors

**Fields**:
- `X, Y, Z float64`

**Constructor**: `core.NewVec3(x, y, z float64)`

**Vector Operations**:
- `Add(other Vec3)` - Vector addition
- `Subtract(other Vec3)` - Vector subtraction
- `Multiply(scalar float64)` - Scalar multiplication
- `Dot(other Vec3)` - Dot product
- `Cross(other Vec3)` - Cross product
- `Normalize()` - Return unit vector
- `Length()` - Vector magnitude

**Color Operations**:
- `Luminance()` - Rec. 709 perceptual luminance (0.2126R + 0.7152G + 0.0722B)
- `GammaCorrect(gamma)` - Apply gamma correction for display
- `Clamp(min, max)` - Clamp RGB components
- `MultiplyVec(other)` - Component-wise multiplication for color modulation

### Vec2 (`pkg/core/vec3.go`)

**Purpose**: 2D coordinates for texture UVs and sampler outputs

**Fields**:
- `X, Y float64`

**Constructor**: `core.NewVec2(x, y float64)`

**Operations**:
- `Add(other Vec2)` - Vector addition
- `Multiply(scalar float64)` - Scalar multiplication

**Primary Uses**:
- Texture coordinates in SurfaceInteraction (UV field)
- 2D sample points from `Sampler.Get2D()`
- All geometry primitives populate UV coordinates for texture mapping

### Ray (`pkg/core/vec3.go`)

**Purpose**: Ray for intersection testing

**Fields**:
- `Origin Vec3` - Ray starting point
- `Direction Vec3` - Ray direction (not necessarily normalized)

**Methods**:
- `At(t float64) Vec3` - Returns point at parameter t along ray
- `NewRayTo(origin, target Vec3)` - Create normalized ray from origin to target

## Color Representation

**Colors are Vec3**: RGB values stored as Vec3 with X=red, Y=green, Z=blue

**Range**: Values typically in [0,1] but can exceed 1.0 for HDR. No clamping during computation - only when writing output.

**No separate Color type**: All color operations use Vec3 methods directly

## Intersection Data

### SurfaceInteraction (`pkg/material/interfaces.go`)

Contains all information about a ray-surface intersection:

```go
type SurfaceInteraction struct {
    Point     core.Vec3  // 3D intersection point
    Normal    core.Vec3  // Surface normal (always faces ray)
    T         float64    // Ray parameter (distance from origin)
    FrontFace bool       // True if ray hit front face
    Material  Material   // Material at intersection point
    UV        core.Vec2  // Texture coordinates
}
```

**Key Method**:
- `SetFaceNormal(ray Ray, outwardNormal Vec3)` - Determines front/back face and sets normal to always point against ray direction

**Available Information**:
- 3D position (`Point`)
- Surface normal (`Normal`)
- Distance along ray (`T`)
- Front vs back face (`FrontFace`)
- Material reference (`Material`)
- UV texture coordinates (`UV`) - populated by all geometry primitives

## Sampler Interface

### Sampler (`pkg/core/sampling.go`)

Provides random number generation for rendering algorithms:

```go
type Sampler interface {
    Get1D() float64      // Returns random value in [0,1)
    Get2D() core.Vec2    // Returns 2D sample point
    Get3D() core.Vec3    // Returns 3D sample point
}
```

**Implementation**: RandomSampler wraps Go's `math/rand.Rand`

**Deterministic Rendering**: Each tile uses seed-based sampler for reproducible results across runs

**Usage**: Materials and integrators use samplers for all random decisions:
- Scatter direction sampling
- Russian roulette path termination
- Light source selection
- BRDF importance sampling

## Type Conventions

**Float Precision**: All floating point values are `float64` (never float32)

**Value Semantics**: Vec3, Vec2, and Ray are value types (passed by value, immutable operations return new instances)

**Zero Values**: `Vec3{X:0, Y:0, Z:0}` represents black color or zero vector

**Normalization**: Direction vectors are often normalized but not always - check specific usage context
