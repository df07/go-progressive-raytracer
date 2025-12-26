# Architecture Documentation

## Overview

This directory contains detailed documentation of the raytracer's architecture. Each document focuses on a specific subsystem with comprehensive details about implementation, usage patterns, and design decisions.

## Documents

### [Core Types and Interfaces](core-types.md)

Foundation types used throughout the codebase.

**Key Topics**:
- Vec3 for 3D vectors and RGB colors
- Vec2 for 2D coordinates
- Ray structure for intersection testing
- SurfaceInteraction for hit data
- Sampler interface for random number generation

**Read this if**: You need to understand basic types, color representation, or random sampling.

### [Material System](material-system.md)

Physically-based material interface and built-in material types.

**Key Topics**:
- Material interface (Scatter, EvaluateBRDF, PDF)
- ColorSource interface for spatially-varying properties
- Lambertian, Metal, Dielectric, Emissive materials
- TransportMode for BDPT correctness
- Material assignment patterns

**Read this if**: You're working with materials, implementing new material types, or debugging light transport.

### [Texture System](texture-system.md)

Image loading and texture mapping for spatially-varying material properties.

**Key Topics**:
- ColorSource interface and implementations
- Image texture loading (PNG/JPEG)
- Procedural textures (checkerboard, gradient, UV debug)
- UV coordinate systems
- Integration with materials and geometry

**Read this if**: You're adding textures, debugging UV mapping, or implementing custom texture types.

### [Geometry Primitives](geometry-primitives.md)

Geometric primitives and acceleration structures.

**Key Topics**:
- Shape interface
- Primitives: Sphere, Quad, Triangle, TriangleMesh, Disc, Cylinder, Cone, Box
- UV coordinate generation for all primitives
- BVH acceleration structure
- PLY mesh loader with UV support
- Intersection algorithms

**Read this if**: You're adding geometry, optimizing intersection tests, working with meshes, or debugging UV coordinates.

### [Scene System](scene-system.md)

Scene definition, registration, and asset loading patterns.

**Key Topics**:
- Scene structure and configuration
- Programmatic scene definition in Go
- Scene registration and discovery
- External asset loading (PLY files, image textures)
- Texture loading patterns
- Camera and sampling configuration
- Light setup

**Read this if**: You're creating scenes, loading assets, or integrating with the rendering system.

### [Integrator System](integrator-system.md)

Light transport algorithms that compute pixel colors by tracing paths through the scene.

**Key Topics**:
- Path Tracing algorithm (unidirectional)
- BDPT algorithm (bidirectional path tracing)
- Material evaluation timing and texture sampling
- MIS (Multiple Importance Sampling) weighting
- Connection strategies in BDPT
- Splat ray system for cross-tile contributions
- Direct vs indirect lighting
- Russian Roulette termination
- Performance characteristics and debugging

**Read this if**: You're debugging rendering inconsistencies between integrators, implementing new integrators, optimizing light transport, or need to understand when and how materials/textures are evaluated.

## Architecture Overview

### Dependency Hierarchy

The codebase follows strict hierarchical dependencies to avoid circular imports:

```
web → renderer → integrator → scene → lights → geometry → material → core
```

**Rule**: Higher layers can import lower layers, never vice versa.

### Key Design Principles

**Zero External Dependencies**: Uses only Go standard library

**Deterministic Parallelism**: Tile-specific random seeds ensure reproducible results

**Progressive Rendering**: Multi-pass tile-based rendering with immediate visual feedback

**Physically-Based**: Materials implement BRDF interface for accurate light transport

**Acceleration**: BVH provides logarithmic intersection scaling for complex scenes

## Quick Reference

### Creating a Sphere

```go
mat := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
sphere := geometry.NewSphere(core.NewVec3(0, 1, 0), 1.0, mat)
```

### Creating a Textured Material

```go
// Load image
imageData, _ := loaders.LoadImage("assets/brick.png")
texture := material.NewImageTexture(imageData.Width, imageData.Height, imageData.Pixels)

// Or use procedural texture
texture := material.NewCheckerboardTexture(256, 256, 32, color1, color2)

// Create material
mat := material.NewTexturedLambertian(texture)
```

### Loading a Mesh

```go
plyData, err := loaders.LoadPLY("assets/model.ply")
mesh := geometry.NewTriangleMesh(
    plyData.Vertices,
    plyData.Faces,
    material,
    &geometry.TriangleMeshOptions{
        Normals:   plyData.Normals,
        VertexUVs: plyData.TexCoords,
    },
)
```

### Building a Scene

```go
scene := &Scene{
    Camera:         geometry.NewCamera(cameraConfig),
    Shapes:         []geometry.Shape{sphere1, sphere2, mesh},
    Lights:         []lights.Light{},
    SamplingConfig: samplingConfig,
}

scene.AddSphereLight(position, radius, emission)
scene.Preprocess()
```

## Related Documentation

- **CLAUDE.md**: High-level project overview, build commands, CLI usage
- **Package Documentation**: GoDoc comments in source files for API details
- **Tests**: `*_test.go` files demonstrate usage patterns

## Contributing to Documentation

When adding features or modifying architecture:

1. Verify existing docs for accuracy
2. Update relevant architecture document
3. Add code examples demonstrating usage
4. Keep focus on "what exists" not "what's missing"
5. Be specific: file names, function signatures, concrete examples
