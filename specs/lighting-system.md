# Lighting System Specification

## Overview

This specification describes the lighting system implementation for the progressive raytracer, including emissive materials, spherical lights, and PDF-based sampling with multiple importance sampling (MIS) support.

## Architecture

### Core Components

1. **Emissive Material** (`pkg/material/emissive.go`)
   - Implements the `Material` and `Emitter` interfaces
   - Does not scatter rays (absorbs all incoming light)
   - Emits light according to its emission color

2. **Spherical Light** (`pkg/geometry/sphere_light.go`)
   - Implements the `Light` interface for direct light sampling
   - Supports both uniform and cone sampling strategies
   - Automatically selects appropriate sampling method based on viewing distance

3. **Multiple Importance Sampling** (`pkg/core/sampling.go`)
   - Power heuristic and balance heuristic implementations
   - Utility functions for combining light and material PDFs
   - Sphere sampling PDF calculations

4. **Lighting Scene** (`pkg/scene/lighting.go`)
   - Manages multiple lights in a scene
   - Provides light sampling and PDF calculation methods
   - Example scene creation utilities

## Interface Definitions

### Light Interface
```go
type Light interface {
    Sample(point Vec3, random *rand.Rand) LightSample
    PDF(point Vec3, direction Vec3) float64
}
```

### Emitter Interface
```go
type Emitter interface {
    Emit() Vec3
}
```

### LightSample Structure
```go
type LightSample struct {
    Point     Vec3    // Point on the light source
    Normal    Vec3    // Normal at the light sample point
    Direction Vec3    // Direction from shading point to light
    Distance  float64 // Distance to light
    Emission  Vec3    // Emitted light
    PDF       float64 // Probability density of this sample
}
```

## Sampling Strategies

### Spherical Light Sampling

The spherical light implements two sampling strategies:

1. **Uniform Sampling**: Used when the shading point is inside the sphere
   - Samples uniformly across the entire sphere surface
   - PDF = `1 / (4π * radius²)`

2. **Cone Sampling**: Used when the shading point is outside the sphere
   - Samples only the visible hemisphere of the sphere
   - More efficient as it focuses on the visible portion
   - PDF = `1 / (2π * (1 - cos(θ_max)))`

### PDF Calculations

The PDF calculations are critical for proper energy balance:

- **Light PDF**: Probability of sampling a direction toward the light
- **Material PDF**: Probability of the material scattering in that direction
- **Combined PDF**: Uses multiple importance sampling to balance both strategies

## Multiple Importance Sampling

Two heuristics are implemented:

1. **Power Heuristic** (β = 2): `(n₁p₁)² / ((n₁p₁)² + (n₂p₂)²)`
2. **Balance Heuristic**: `(n₁p₁) / (n₁p₁ + n₂p₂)`

Where:
- `n₁, n₂` = number of samples from each strategy
- `p₁, p₂` = PDF values from each strategy

## Usage Example

```go
// Create a lighting scene
ls := scene.NewLightingScene()

// Add spherical lights
ls.AddSphereLight(
    core.NewVec3(0, 5, 0),    // center
    0.5,                      // radius
    core.NewVec3(10, 10, 10), // white emission
)

// Sample a light for direct lighting
if lightSample, hasLight := ls.SampleLight(shadingPoint, random); hasLight {
    // Calculate light contribution
    lightContribution := lightSample.Emission.MultiplyVec(materialBRDF)
    
    // Get material PDF for MIS
    materialPDF := material.PDF(incomingDirection, lightSample.Direction)
    
    // Combine PDFs using MIS
    misWeight := core.CombinePDFs(lightSample.PDF, materialPDF, true)
    
    // Apply MIS weight
    finalContribution := lightContribution.Multiply(misWeight / lightSample.PDF)
}
```

## Implementation Details

### Edge Cases Handled

1. **Point inside sphere**: Automatically switches to uniform sampling
2. **Zero radius spheres**: Handles numerical precision gracefully
3. **Grazing angles**: Proper normal and direction calculations
4. **Zero PDF cases**: Returns zero contribution safely

### Mathematical Correctness

- All PDF calculations are mathematically sound
- Energy conservation is maintained
- Proper normalization factors are applied
- Numerical stability is ensured with appropriate tolerances

### Testing Strategy

- Unit tests for all mathematical calculations
- Edge case testing for boundary conditions
- Interface compliance verification
- PDF correctness validation
- Multi-sample statistical testing

## Future Enhancements

1. **Additional Light Types**: Point lights, directional lights, area lights
2. **Light Importance Sampling**: Weight lights by power/visibility
3. **Environment Lighting**: HDR environment maps
4. **Shadow Rays**: Explicit visibility testing
5. **Photon Mapping**: Global illumination techniques

## Performance Considerations

- Cone sampling reduces unnecessary light samples
- PDF calculations are optimized for common cases
- Memory allocation is minimized during sampling
- Random number generation is efficient and deterministic

## Mathematical References

- Veach, Eric. "Robust Monte Carlo Methods for Light Transport Simulation" (1997)
- Pharr, Matt, et al. "Physically Based Rendering" (4th Edition)
- Jensen, Henrik Wann. "Realistic Image Synthesis Using Photon Mapping" (2001) 