# Geometry and UV Coordinates

## Overview
The raytracer supports six primitive types (Sphere, Quad, Triangle, TriangleMesh, Disc, Cylinder, Cone) plus BVH acceleration. Currently NONE of these primitives compute UV coordinates - this must be added for texture mapping. Each primitive type requires a different UV parameterization strategy.

## Available Geometry Primitives

### Sphere (`pkg/geometry/sphere.go`)

**Definition**:
- `Center core.Vec3` - Sphere center point
- `Radius float64` - Sphere radius
- `Material material.Material` - Surface material

**Constructor**: `geometry.NewSphere(center Vec3, radius float64, mat Material)`

**Intersection Result**:
- Point: Ray intersection point on sphere surface
- Normal: (Point - Center) / Radius (outward pointing)
- T: Ray parameter
- FrontFace: True for outside hits, false for inside
- Material: Assigned material
- **UV: NOT COMPUTED**

**UV Parameterization Strategy** (not implemented):
```
Compute from outward normal (x,y,z):
  phi = atan2(z, x)           // Azimuthal angle [-π, π]
  theta = asin(y / radius)    // Polar angle [-π/2, π/2]
  u = (phi + π) / (2π)        // Map to [0, 1]
  v = (theta + π/2) / π       // Map to [0, 1]
```

**Implementation Notes**:
- UV seam at phi=±π (back of sphere)
- Poles (theta=±π/2) map to v=0 or v=1 (entire u range)
- Suitable for spherical/equirectangular texture maps

### Quad (`pkg/geometry/quad.go`)

**Definition**:
- `Corner core.Vec3` - One corner of quad
- `U core.Vec3` - First edge vector (corner to adjacent corner)
- `V core.Vec3` - Second edge vector (corner to opposite corner)
- `Normal core.Vec3` - Computed from U × V
- `Material material.Material` - Surface material

**Constructor**: `geometry.NewQuad(corner, u, v Vec3, mat Material)`

**Intersection Result**:
- Uses plane equation and barycentric coordinates (alpha, beta)
- Alpha, beta computed in Hit() but not stored
- **UV: NOT COMPUTED**

**UV Parameterization Strategy** (not implemented):
```
Already computed in Hit() method:
  alpha = W.Dot(hitVector.Cross(V))   // Barycentric coordinate
  beta = W.Dot(U.Cross(hitVector))    // Barycentric coordinate

Simply store as UV:
  uv = Vec2{X: alpha, Y: beta}        // Already in [0,1] range
```

**Implementation Notes**:
- Hit() already computes barycentric coordinates for bounds checking
- Trivial to expose as UV - just need to store in SurfaceInteraction
- (0,0) at Corner, (1,0) at Corner+U, (0,1) at Corner+V, (1,1) at Corner+U+V

### Triangle (`pkg/geometry/triangle.go`)

**Definition**:
- `V0, V1, V2 core.Vec3` - Three vertices
- `Material material.Material` - Surface material
- `normal core.Vec3` - Cached face normal
- `bbox AABB` - Cached bounding box

**Constructors**:
- `geometry.NewTriangle(v0, v1, v2, mat)` - Auto-compute normal
- `geometry.NewTriangleWithNormal(v0, v1, v2, normal, mat)` - Custom normal

**Intersection Result**:
- Uses Möller-Trumbore algorithm (computes barycentric u, v)
- Barycentric coordinates (u, v) computed but not stored
- **UV: NOT COMPUTED**

**UV Parameterization Strategy** (not implemented):

**Option A - Barycentric UVs** (simple, no mesh data needed):
```
From Möller-Trumbore algorithm in Hit():
  u, v are barycentric coordinates (already computed)
  w = 1 - u - v (implicit third coordinate)

Return as UV:
  uv = Vec2{X: u, Y: v}
```

**Option B - Per-Vertex UV Interpolation** (requires mesh data):
```
Requires storing per-vertex UVs:
  UV0, UV1, UV2 core.Vec2  // Per-vertex texture coordinates

Interpolate using barycentric coordinates:
  uv = UV0*w + UV1*u + UV2*v
  where w = 1 - u - v
```

**Implementation Notes**:
- Möller-Trumbore already computes barycentric (u,v) - just needs to be saved
- Option A works immediately but doesn't match artist-specified UVs
- Option B requires extending Triangle struct with per-vertex UVs

### TriangleMesh (`pkg/geometry/triangle_mesh.go`)

**Definition**:
- `triangles []Shape` - Individual triangle shapes
- `bvh *BVH` - BVH acceleration structure
- `bbox AABB` - Overall bounding box
- `material Material` - Default material

**Constructor**:
```go
NewTriangleMesh(
    vertices []core.Vec3,
    faces []int,                      // Triangle indices (3 per triangle)
    material material.Material,
    options *TriangleMeshOptions,     // Optional parameters
)
```

**TriangleMeshOptions**:
```go
type TriangleMeshOptions struct {
    Normals   []core.Vec3          // Per-triangle custom normals
    Materials []material.Material  // Per-triangle materials
    Rotation  *core.Vec3           // Rotation to apply
    Center    *core.Vec3           // Rotation center
}
```

**Current State**:
- Supports per-triangle normals and materials
- **Does NOT support per-vertex UVs**
- Options struct would need UV field

**Intersection Result**:
- Delegates to individual triangle Hit()
- Returns triangle's SurfaceInteraction
- **UV: NOT COMPUTED**

**UV Support Strategy** (not implemented):

1. **Extend TriangleMeshOptions**:
```go
type TriangleMeshOptions struct {
    // ... existing fields ...
    VertexUVs []core.Vec2  // Per-vertex texture coordinates
}
```

2. **Store UVs in Triangles**:
   - Create triangles with per-vertex UVs (requires new Triangle variant)
   - Interpolate UVs using barycentric coordinates in Hit()

3. **PLY Loader Integration**:
   - PLYData already contains `TexCoords []core.Vec2`
   - Pass through to TriangleMeshOptions
   - Map vertex UVs to triangle corners

### Other Primitives

**Disc** (`pkg/geometry/disc.go`):
- Similar to Quad - could use polar or planar UVs
- Not commonly used for textured surfaces

**Cylinder** (`pkg/geometry/cylinder.go`):
- Cylindrical coordinates: u from angle, v from height
- Requires atan2 for u, linear interpolation for v

**Cone** (`pkg/geometry/cone.go`):
- Conical unwrapping or similar to cylinder
- Less common for texture mapping

**Box** (`pkg/geometry/box.go`):
- Composed of 6 quads
- Each face needs separate UV parameterization

## PLY Loader UV Support

### Current Capabilities (`pkg/loaders/ply.go`)

The PLY loader ALREADY supports reading texture coordinates:

**Detected Properties**:
- `u, v` - Standard UV names
- `s, t` - Alternative UV names
- `texture_u, texture_v` - Explicit texture coordinate names

**Storage**:
```go
type PLYData struct {
    Vertices  []core.Vec3  // Vertex positions
    Faces     []int        // Triangle indices
    Normals   []core.Vec3  // Per-vertex normals (if present)
    TexCoords []core.Vec2  // Per-vertex UVs (if present) ← ALREADY SUPPORTED
    Colors    []core.Vec3  // Per-vertex colors (if present)
    // ...
}
```

**Detection Logic**:
- Header parsing sets `HasTexCoords = true` if UV properties found
- Stores indices of UV properties in `TexCoordIndices [2]int`
- Populates `TexCoords []core.Vec2` during vertex reading

**Current Limitation**: TexCoords are loaded but NOT passed to geometry.

### Integration Path for PLY UVs

To use PLY texture coordinates:

1. **Extend TriangleMeshOptions** to accept `VertexUVs []core.Vec2`
2. **Pass PLYData.TexCoords** to mesh constructor
3. **Store per-vertex UVs** in Triangle structs
4. **Interpolate UVs** using barycentric coordinates in Triangle.Hit()

Example usage:
```go
plyData, _ := loaders.LoadPLY("model.ply")
options := &geometry.TriangleMeshOptions{
    Normals:   plyData.Normals,   // Already supported
    VertexUVs: plyData.TexCoords, // Need to add this
}
mesh := geometry.NewTriangleMesh(
    plyData.Vertices,
    plyData.Faces,
    material,
    options,
)
```

## BVH and Intersection Results

### BVH Structure (`pkg/geometry/bvh.go`)

**Purpose**: Spatial acceleration structure for fast ray-object intersection.

**Implementation**: Median-split algorithm, recursively partitions objects.

**Performance**: Up to 39x speedup for complex meshes.

**Impact on UVs**: None - BVH delegates to leaf geometry Hit() methods, which return SurfaceInteraction. UV computation happens at leaf level.

### Intersection Data Flow

```
Ray → BVH.Hit() → Triangle.Hit() → SurfaceInteraction
                                         ↓
                                    (should contain UV)
```

BVH is transparent to UV computation - just needs Triangle to populate UV field.

## Per-Vertex vs Per-Face Data

### Per-Vertex Data (Interpolated)

**Examples**: Position, Normal, UV, Color

**Storage**: One value per vertex in mesh

**Usage**: Interpolate using barycentric coordinates at hit point

**Advantages**:
- Smooth variation across surface
- Matches artist-created UV maps
- Standard for textured meshes

**Implementation**:
```go
// In Triangle with per-vertex UVs
uv := UV0.Multiply(w).Add(UV1.Multiply(u)).Add(UV2.Multiply(v))
```

### Per-Face Data (Constant)

**Examples**: Material assignment, flat shading

**Storage**: One value per triangle/face

**Usage**: Use value directly from face

**Advantages**:
- Lower memory usage
- Simpler implementation
- Good for faceted appearance

**Current Support**:
- Normals: Both per-vertex (interpolated) and per-face (NewTriangleWithNormal)
- Materials: Per-triangle via TriangleMeshOptions.Materials
- UVs: Not supported yet (would need per-vertex for standard texture mapping)

## UV Coordinate Generation Requirements

To implement texture mapping, each primitive needs:

### Sphere
- Compute (u,v) from spherical coordinates in Hit()
- Add UV field to returned SurfaceInteraction
- Handle pole and seam discontinuities

### Quad
- Expose existing barycentric (alpha, beta) as UV
- Minimal code change - already computed in Hit()

### Triangle
- **Simple**: Use barycentric (u,v) directly as UV
- **Full**: Store per-vertex UVs, interpolate with barycentric weights
- Modify Möller-Trumbore to save barycentric coordinates

### TriangleMesh
- Extend TriangleMeshOptions to accept per-vertex UVs
- Pass UVs to individual Triangle constructors
- Integrate with PLY loader's TexCoords data

### All Primitives
- Modify SurfaceInteraction to include `UV core.Vec2` field
- Populate UV in each Hit() implementation
- Document UV coordinate system for each primitive type

## Common UV Coordinate Systems

**Sphere**: Spherical/equirectangular (latitude/longitude)
- U wraps around equator (0→1 = -180°→+180°)
- V from bottom pole to top (0→1 = -90°→+90°)

**Quad/Plane**: Planar projection
- U,V aligned with edge vectors
- Natural [0,1] range

**Triangle Mesh**: Artist-defined unwrapping
- UVs specified per-vertex in modeling tool
- Arbitrary mapping from 3D surface to 2D texture space

**Procedural**: Generated from position or other attributes
- Can be computed on-the-fly without storage
- Examples: triplanar, world-space, noise-based
