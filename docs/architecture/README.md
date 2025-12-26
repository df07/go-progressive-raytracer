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

## Additional Architecture Documentation

### [Rendering Pipeline Architecture](rendering-pipeline.md)

High-level architecture of the progressive rendering pipeline.

**Key Topics**:
- Pipeline flow: Camera → Tiles → Workers → Integrator → Pixels
- Component responsibilities (Renderer, Integrator, Scene)
- Progressive rendering mechanics
- BDPT splat system
- Why bugs appear only in full renders vs tests

**Read this if**: You're debugging the rendering pipeline, understanding component interactions, or investigating splat-related issues.

### [Material and Texture System Data Flow](material-texture-data-flow.md)

Complete data flow from geometry intersection to texture sampling.

**Key Topics**:
- UV coordinate journey: Geometry → SurfaceInteraction → Material → ColorSource
- Component responsibilities
- Why preserving complete SurfaceInteraction is critical
- PT and BDPT data flow examples
- Debugging UV-related issues

**Read this if**: You're debugging texture issues, working with materials, or investigating UV coordinate problems.

## Implementation Guides

### [BDPT Implementation Guide](../implementation/bdpt-implementation-guide.md)

Code-level walkthrough of BDPT implementation with common pitfalls.

**Key Topics**:
- Code structure and file organization
- Path construction (camera and light paths)
- Connection strategies breakdown
- Vertex structure and data preservation
- Common pitfalls (UV loss, PDF errors, geometric terms)
- Verification tests and debugging

**Read this if**: You're modifying BDPT, debugging BDPT-specific issues, or need to understand connection strategies.

## Testing and Debugging Guides

### [Testing Strategy](../guides/testing-strategy.md)

Comprehensive guide to testing integrators and rendering systems.

**Key Topics**:
- Three-level testing strategy (unit, integration, visual)
- Luminance comparison patterns for PT vs BDPT
- Test scene selection and design
- Debugging workflow examples
- When bugs appear at different test levels

**Read this if**: You're testing integrators, debugging rendering inconsistencies, or writing new tests.

### [CLI Usage and Testing Workflow](../guides/cli-usage.md)

Complete guide to CLI flags, output behavior, and debugging workflows.

**Key Topics**:
- Complete CLI flag reference
- Output file locations and naming
- Quick test render recipes
- Integrator comparison workflow
- Profiling and performance analysis

**Read this if**: You're running renders for testing, comparing integrators, or need a debugging workflow.

### [Debugging Guide for Rendering Issues](../guides/debugging-rendering-issues.md)

Practical guide to diagnosing and fixing common rendering bugs.

**Key Topics**:
- Common bug classes (brightness, color bleeding, artifacts, crashes)
- Diagnostic workflows for each bug type
- Tools and techniques (luminance comparison, visual diff, debug rendering)
- PT vs BDPT comparison as debugging tool
- Step-by-step debugging examples

**Read this if**: You're debugging a rendering issue, investigating integrator differences, or need systematic debugging techniques.

### [Common Bug Patterns in Raytracers](../guides/common-bug-patterns.md)

Catalog of common bug patterns with recognition and fix strategies.

**Key Topics**:
- Data loss bugs (UV, normals not preserved)
- Coordinate system bugs (world vs local space)
- Integrator inconsistencies (PT vs BDPT divergence)
- Floating-point precision issues
- Parallel rendering bugs (race conditions)
- Scale-dependent bugs
- Real examples from this codebase

**Read this if**: You're debugging any rendering issue - this guide helps classify and fix common patterns quickly.

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
