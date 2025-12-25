# Texture Mapping Documentation

## Overview
This directory contains comprehensive documentation for designing and implementing a texture mapping system for the Go Progressive Raytracer. The documentation is organized into four focused areas that cover all aspects needed to add spatially-varying material properties.

## Documentation Structure

### [Core Types and Interfaces](core-types.md)
**What it covers**:
- Fundamental types (Vec3, Vec2, Ray)
- Color representation (Vec3 as RGB)
- SurfaceInteraction structure (intersection data)
- UV coordinate types and current state
- Sampler interface for random number generation

**Key findings**:
- Vec2 type exists but UV coordinates are NOT currently computed
- Colors are Vec3 (no separate Color type)
- SurfaceInteraction contains Point, Normal, T, FrontFace, Material but no UV field
- PLY loader already supports reading texture coordinates

### [Material System Architecture](material-system.md)
**What it covers**:
- Material interface (Scatter, EvaluateBRDF, PDF methods)
- Built-in materials (Lambertian, Metal, Dielectric, Emissive, Layered)
- Material construction patterns
- Render pipeline interaction
- Integration strategies for textures

**Key findings**:
- Materials have uniform properties (no spatial variation)
- Materials store single color values (e.g., Albedo as Vec3)
- Three integration options: textured variants, ColorSource interface, or modifier pattern
- ColorSource interface approach recommended for long-term extensibility

### [Geometry and UV Coordinates](geometry-uv-coordinates.md)
**What it covers**:
- Available geometry primitives (Sphere, Quad, Triangle, TriangleMesh, etc.)
- Current UV coordinate state (not computed by any primitive)
- UV parameterization strategies for each primitive type
- PLY loader UV support (already loads TexCoords)
- Per-vertex vs per-face data handling

**Key findings**:
- NO primitives currently compute UV coordinates
- Quad already computes barycentric coords - trivial to expose as UV
- Triangle Möller-Trumbore computes barycentric - just needs storage
- PLY loader reads UVs but they're not passed to geometry
- Each primitive needs different UV strategy (spherical, planar, barycentric)

### [Scene and Material Assignment](scene-material-assignment.md)
**What it covers**:
- Scene definition pattern (programmatic Go files)
- Material assignment approaches
- External resource loading (PLY meshes)
- Asset path resolution
- Texture integration strategy

**Key findings**:
- Scenes are Go source files, not data files
- Materials assigned directly at geometry creation
- PLY loader pattern can be replicated for image loading
- Asset paths are relative to execution directory
- Error handling provides fallbacks for missing resources

## Current Texture Support Status

### What EXISTS
- Vec2 type for UV coordinates
- PLY loader reads texture coordinates from files
- Material system supports per-triangle materials
- Resource loading pattern (PLY meshes)

### What's MISSING
- UV computation in geometry Hit() methods
- UV field in SurfaceInteraction
- Image loading infrastructure
- Texture sampling interface
- Textured material types
- Integration of PLY TexCoords with geometry

## Implementation Roadmap

Based on the documentation, here's the recommended implementation order:

### Phase 1: UV Coordinate Infrastructure
1. Add `UV core.Vec2` field to SurfaceInteraction
2. Implement UV generation in Quad.Hit() (simplest - uses existing barycentric)
3. Implement UV generation in Sphere.Hit() (spherical coordinates)
4. Implement UV generation in Triangle.Hit() (barycentric)

### Phase 2: Triangle Mesh UV Support
1. Extend TriangleMeshOptions to include `VertexUVs []core.Vec2`
2. Modify Triangle to store per-vertex UVs
3. Interpolate UVs in Triangle.Hit() using barycentric weights
4. Connect PLY loader TexCoords to TriangleMesh constructor

### Phase 3: Texture Sampling
1. Create image loader (`pkg/loaders/image.go`) using Go's `image` package
2. Define Texture interface with `Sample(uv Vec2) Vec3` method
3. Implement ImageTexture with bilinear filtering
4. Implement SolidColorTexture for uniform colors

### Phase 4: Material Integration
1. Define ColorSource interface (replaces Vec3 colors)
2. Create SolidColor and ImageTexture implementations
3. Refactor Lambertian to use ColorSource instead of Vec3 albedo
4. Apply pattern to Metal and other materials
5. Update scene constructors to use new API

### Phase 5: Scene Integration
1. Add texture loading to example scenes
2. Create textured scene preset
3. Test with various primitives (sphere, quad, mesh)
4. Document texture coordinate systems for each primitive

## Design Decisions Summary

### UV Coordinate Systems
- **Sphere**: Spherical coordinates (latitude/longitude mapping)
- **Quad**: Barycentric coordinates (already computed in Hit())
- **Triangle**: Barycentric for simple case, per-vertex interpolation for artist UVs
- **TriangleMesh**: Per-vertex UVs interpolated with barycentric weights

### Material Architecture
**Recommended**: ColorSource interface approach
```go
type ColorSource interface {
    Evaluate(uv Vec2, point Vec3) Vec3
}

type Lambertian struct {
    Albedo ColorSource  // Was: core.Vec3
}
```

**Advantages**:
- Single material implementation handles both cases
- No code duplication
- Extensible to procedural textures
- Backward compatible (SolidColor implements ColorSource)

### Image Loading
**Use Go standard library**: `image/png` and `image/jpeg` packages
- No external dependencies (matches project philosophy)
- Conversion to `[]core.Vec3` for internal representation
- Relative paths from execution directory
- Fallback to solid colors on load failure

## Testing Strategy

### Primitive-Specific Tests
- Sphere UV continuity (check seam and poles)
- Quad UV bounds (verify [0,1] range)
- Triangle barycentric interpolation correctness
- Mesh UV interpolation across triangle boundaries

### Integration Tests
- Textured sphere with checkerboard pattern
- Textured quad with image
- Textured mesh loaded from PLY with UVs
- Mixed textured and solid materials in same scene

### Edge Cases
- Missing texture files (fallback behavior)
- PLY files without UVs (graceful degradation)
- UV coordinates outside [0,1] (wrapping or clamping)
- Texture filtering at boundaries

## Reference Implementation Examples

See documentation files for detailed examples:
- **UV Generation**: `geometry-uv-coordinates.md` - strategies for each primitive
- **Material Refactoring**: `material-system.md` - ColorSource integration options
- **Scene Setup**: `scene-material-assignment.md` - textured scene example
- **Resource Loading**: `scene-material-assignment.md` - PLY loader pattern

## Questions Answered

This documentation comprehensively answers the four original questions:

1. **Core Types**: Vec3 for colors/vectors, Vec2 for UVs, SurfaceInteraction for hit data, no UV computation yet
2. **Material System**: BRDF interface with Scatter/Evaluate/PDF, materials store uniform colors, three integration strategies
3. **Geometry and UVs**: Six primitive types, none compute UVs, PLY supports loading UVs, each needs specific parameterization
4. **Scene Assignment**: Programmatic Go scenes, direct material assignment, PLY loader pattern for resources, no scene files

## Next Steps

With this documentation, you can now:
1. Design the texture coordinate system (Phase 1)
2. Define the texture sampling interface (Phase 3)
3. Specify how textures integrate with materials (Phase 4)
4. Outline image loading requirements (Phase 3)
5. Define UV generation for each geometry type (Phase 1-2)

All technical details needed for implementation are covered in the four documentation files.
