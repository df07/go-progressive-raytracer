# Geometry Primitives

## Overview

The raytracer provides six geometric primitives (Sphere, Quad, Triangle, TriangleMesh, Disc, Cylinder, Cone) plus composite types (Box) and BVH acceleration. All primitives implement the Shape interface for ray intersection testing.

## Shape Interface

All geometry implements (`pkg/geometry/interfaces.go`):

```go
type Shape interface {
    Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool)
    BoundingBox() AABB
}
```

**Hit Method**: Tests ray intersection, returns SurfaceInteraction if hit occurs within [tMin, tMax] range

**BoundingBox Method**: Returns axis-aligned bounding box for acceleration structure construction

## Sphere

**File**: `pkg/geometry/sphere.go`

**Definition**:
```go
type Sphere struct {
    Center   core.Vec3
    Radius   float64
    Material material.Material
}
```

**Constructor**: `geometry.NewSphere(center Vec3, radius float64, mat Material)`

**Intersection Algorithm**: Quadratic equation solving for ray-sphere intersection

**Normal Computation**: (Point - Center) / Radius (outward pointing)

**Use Cases**: Simple primitives, glass orbs, planets, basic scene building blocks

## Quad

**File**: `pkg/geometry/quad.go`

**Definition**:
```go
type Quad struct {
    Corner   core.Vec3  // One corner of quad
    U        core.Vec3  // First edge vector
    V        core.Vec3  // Second edge vector
    Normal   core.Vec3  // Computed from U × V
    Material material.Material
}
```

**Constructor**: `geometry.NewQuad(corner, u, v Vec3, mat Material)`

**Intersection Algorithm**:
- Plane intersection via ray-plane equation
- Barycentric coordinate computation (alpha, beta) for bounds checking
- Hit accepted if 0 ≤ alpha ≤ 1 and 0 ≤ beta ≤ 1

**Corners**:
- (0,0) at Corner
- (1,0) at Corner+U
- (0,1) at Corner+V
- (1,1) at Corner+U+V

**Use Cases**: Walls, floors, area lights, rectangular surfaces

## Triangle

**File**: `pkg/geometry/triangle.go`

**Definition**:
```go
type Triangle struct {
    V0, V1, V2    core.Vec3         // Three vertices
    UV0, UV1, UV2 core.Vec2         // Per-vertex texture coordinates (optional)
    hasUVs        bool              // Whether per-vertex UVs are provided
    Material      material.Material
    normal        core.Vec3         // Cached face normal
    bbox          AABB              // Cached bounding box
}
```

**Constructors**:
- `geometry.NewTriangle(v0, v1, v2, mat)` - Auto-compute normal from cross product
- `geometry.NewTriangleWithNormal(v0, v1, v2, normal, mat)` - Use provided normal
- `geometry.NewTriangleWithUVs(v0, v1, v2, uv0, uv1, uv2, mat)` - With per-vertex UVs
- `geometry.NewTriangleWithNormalAndUVs(v0, v1, v2, uv0, uv1, uv2, normal, mat)` - Custom normal + UVs

**Intersection Algorithm**: Möller-Trumbore algorithm
- Computes barycentric coordinates (u, v) where w = 1 - u - v
- Single ray-triangle test without plane intersection
- Efficient and numerically stable

**Use Cases**: Building block for meshes, simple faceted geometry

## TriangleMesh

**File**: `pkg/geometry/triangle_mesh.go`

**Definition**:
```go
type TriangleMesh struct {
    triangles []Shape      // Individual triangle shapes
    bvh       *BVH         // Acceleration structure
    bbox      AABB         // Overall bounding box
    material  Material     // Default material
}
```

**Constructor**:
```go
NewTriangleMesh(
    vertices []core.Vec3,
    faces []int,                     // Triangle indices (3 per triangle)
    material material.Material,
    options *TriangleMeshOptions,    // Optional parameters
)
```

**TriangleMeshOptions**:
```go
type TriangleMeshOptions struct {
    Normals   []core.Vec3          // Per-triangle custom normals
    VertexUVs []core.Vec2          // Per-vertex texture coordinates
    Materials []material.Material  // Per-triangle materials
    Rotation  *core.Vec3           // Rotation to apply (Euler angles)
    Center    *core.Vec3           // Rotation center point
}
```

**VertexUVs Usage**: Must match vertex count if provided. UVs are interpolated using barycentric coordinates.

**Capabilities**:
- Automatic BVH construction for acceleration
- Per-triangle normal specification
- Per-triangle material assignment
- Mesh transformation (rotation about point)

**Intersection**: Delegates to BVH, which delegates to individual triangles

**Use Cases**: Complex meshes loaded from files, procedural geometry, high-poly models

## Other Primitives

### Disc (`pkg/geometry/disc.go`)

Circular disc defined by center, radius, normal, and material.

**Use Cases**: Circular area lights, lens elements, caps for cylinders

### Cylinder (`pkg/geometry/cylinder.go`)

Finite cylinder between two points with specified radius.

**Use Cases**: Columns, pipes, cylindrical objects

### Cone (`pkg/geometry/cone.go`)

Cone with apex, base center, height, and base radius.

**Use Cases**: Conical objects, specialized geometry

### Box (`pkg/geometry/box.go`)

Axis-aligned box composed of 6 quads.

**Constructor**: `geometry.NewBox(min, max Vec3, mat Material)`

**Use Cases**: Room geometry, simple box shapes, Cornell box walls

## BVH Acceleration

**File**: `pkg/geometry/bvh.go`

**Purpose**: Spatial acceleration structure for fast ray-object intersection in complex scenes

**Algorithm**: Median-split recursive partitioning
- Sorts objects by centroid along longest axis
- Splits at median to create balanced tree
- Recursion terminates when ≤ 4 objects remain (leaf node)

**Construction**: `NewBVH(shapes []Shape) *BVH`

**Performance**: Up to 39x speedup for complex meshes (dragon scene: 1.8M triangles)

**Transparency**: BVH delegates Hit() calls to leaf geometry. Intersection results come from primitives unchanged.

**Usage**: Automatically built for TriangleMesh. Can also wrap scene-level shape lists for top-level acceleration.

## Mesh Loading

### PLY Loader (`pkg/loaders/ply.go`)

Loads triangle meshes from PLY files.

**Function**: `LoadPLY(filename string) (*PLYData, error)`

**PLYData Structure**:
```go
type PLYData struct {
    Vertices  []core.Vec3  // Vertex positions
    Faces     []int        // Triangle indices (3 per triangle)
    Normals   []core.Vec3  // Per-vertex normals (if present)
    TexCoords []core.Vec2  // Per-vertex UVs (if present)
    Colors    []core.Vec3  // Per-vertex colors (if present)
}
```

**Supported Features**:
- Vertex positions (required)
- Face indices (required)
- Per-vertex normals (optional)
- Per-vertex texture coordinates (optional - u/v, s/t, or texture_u/texture_v)
- Per-vertex colors (optional)

**Usage Example**:
```go
plyData, err := loaders.LoadPLY("assets/dragon.ply")
if err != nil {
    // Handle error
}

options := &geometry.TriangleMeshOptions{
    Normals:   plyData.Normals,
    VertexUVs: plyData.TexCoords,  // Connect PLY UVs to mesh
}

mesh := geometry.NewTriangleMesh(
    plyData.Vertices,
    plyData.Faces,
    material,
    options,
)
```

## Intersection Data Flow

```
Ray → Scene.BVH.Hit() → TriangleMesh.bvh.Hit() → Triangle.Hit() → SurfaceInteraction
```

Each level:
1. Tests bounding box intersection
2. Delegates to children or leaf primitives
3. Returns closest hit within [tMin, tMax] range

Final SurfaceInteraction contains:
- Point (3D position)
- Normal (surface normal at hit)
- T (distance along ray)
- FrontFace (front/back face flag)
- Material (material reference from geometry)
- UV (texture coordinates computed by primitive)

## UV Coordinate Systems

All geometry primitives populate UV texture coordinates in their Hit() methods. UVs are in [0,1] range by convention.

### Sphere (`pkg/geometry/sphere.go`)

**UV Mapping**: Spherical coordinates (latitude/longitude)

```
U = φ / 2π    where φ = azimuthal angle [0, 2π]
V = θ / π     where θ = polar angle [0, π]
```

**Characteristics**:
- Top pole: V ≈ 0
- Bottom pole: V ≈ 1
- Seam discontinuity at φ = ±π (back of sphere)
- Wraps naturally around equator

### Quad (`pkg/geometry/quad.go`)

**UV Mapping**: Barycentric coordinates (alpha, beta) used directly as UV

**Corners**:
- Corner: (0,0)
- Corner+U: (1,0)
- Corner+V: (0,1)
- Corner+U+V: (1,1)

**Characteristics**: Natural planar mapping for rectangular surfaces

### Triangle (`pkg/geometry/triangle.go`)

**UV Mapping**: Two modes depending on constructor

**Mode 1** - Barycentric (default):
- `NewTriangle()` uses barycentric coordinates (u, v, w) as UV
- Simple but not useful for textured meshes

**Mode 2** - Per-vertex interpolation:
- `NewTriangleWithUVs()` interpolates per-vertex UVs using barycentric weights
- Formula: `UV = w*UV0 + u*UV1 + v*UV2` where `w = 1 - u - v`
- Used by TriangleMesh for UV-mapped models

### TriangleMesh (`pkg/geometry/triangle_mesh.go`)

**UV Mapping**: Per-vertex UVs via `TriangleMeshOptions.VertexUVs`

**Usage**:
```go
options := &geometry.TriangleMeshOptions{
    VertexUVs: plyData.TexCoords,  // From PLY file
}
```

**Characteristics**: Connects PLY loader's texture coordinates to triangle UVs for proper mesh texturing

### Cylinder (`pkg/geometry/cylinder.go`)

**UV Mapping**: Cylindrical coordinates

**Body Surface**:
- U = angle around axis [0, 1]
- V = height from base to top [0, 1]

**Caps**: Planar disc mapping (radial projection)

### Cone (`pkg/geometry/cone.go`)

**UV Mapping**: Conical unwrapping

**Body Surface**:
- U = angle around axis [0, 1]
- V = height from base to top [0, 1]

**Caps**: Planar disc mapping for base and frustum top

### Disc (`pkg/geometry/disc.go`)

**UV Mapping**: Planar projection centered on disc

**Mapping**:
- Projects hit point onto Right and Up vectors
- Maps disc radius to [0, 1] range
- Center of disc: (0.5, 0.5)

## Material Assignment Patterns

**Single Material**:
```go
mat := material.NewLambertian(color)
sphere := geometry.NewSphere(center, radius, mat)
```

**Textured Material**:
```go
texture := material.NewCheckerboardTexture(256, 256, 32, color1, color2)
texturedMat := material.NewTexturedLambertian(texture)
sphere := geometry.NewSphere(center, radius, texturedMat)
```

**Per-Triangle Materials**:
```go
materials := make([]material.Material, numTriangles)
for i := 0; i < numTriangles; i++ {
    materials[i] = selectMaterialForTriangle(i)
}

options := &geometry.TriangleMeshOptions{
    Materials: materials,
}

mesh := geometry.NewTriangleMesh(vertices, faces, defaultMat, options)
```

**Material Reuse**:
```go
redDiffuse := material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1))
sphere1 := geometry.NewSphere(pos1, r1, redDiffuse)
sphere2 := geometry.NewSphere(pos2, r2, redDiffuse)
```

## Performance Considerations

**BVH Essential**: For meshes with >100 triangles, BVH provides orders of magnitude speedup

**Bounding Boxes**: Tight bounding boxes improve BVH efficiency. All primitives compute minimal AABBs.

**Triangle Count**: TriangleMesh automatically builds BVH. Dragon scene (1.8M triangles) renders interactively with BVH.

**Memory**: Each Triangle stores 3 vertices + normal + bbox. Large meshes can consume significant memory. Shared vertex representation not currently used.
