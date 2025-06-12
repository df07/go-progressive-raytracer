package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TriangleMesh represents a collection of triangles with efficient ray intersection
// It uses an internal BVH (Bounding Volume Hierarchy) for fast intersection tests
type TriangleMesh struct {
	triangles []core.Shape  // Individual triangles as shapes
	bvh       *core.BVH     // BVH for fast intersection
	bbox      core.AABB     // Overall bounding box
	material  core.Material // Default material (can be overridden per triangle)
}

// NewTriangleMesh creates a new triangle mesh from vertices and face indices
// vertices: array of 3D points
// faces: array of triangle indices (each group of 3 indices forms a triangle)
// material: default material for all triangles
func NewTriangleMesh(vertices []core.Vec3, faces []int, material core.Material) *TriangleMesh {
	if len(faces)%3 != 0 {
		panic("Face indices must be a multiple of 3")
	}

	numTriangles := len(faces) / 3
	triangles := make([]core.Shape, numTriangles)

	// Create individual triangles
	for i := 0; i < numTriangles; i++ {
		i0 := faces[i*3]
		i1 := faces[i*3+1]
		i2 := faces[i*3+2]

		// Bounds check
		if i0 >= len(vertices) || i1 >= len(vertices) || i2 >= len(vertices) ||
			i0 < 0 || i1 < 0 || i2 < 0 {
			panic("Face index out of bounds")
		}

		triangle := NewTriangle(vertices[i0], vertices[i1], vertices[i2], material)
		triangles[i] = triangle
	}

	// Build BVH for fast intersection
	bvh := core.NewBVH(triangles)

	// Calculate overall bounding box
	var bbox core.AABB
	if len(triangles) > 0 {
		bbox = triangles[0].BoundingBox()
		for i := 1; i < len(triangles); i++ {
			bbox = bbox.Union(triangles[i].BoundingBox())
		}
	}

	return &TriangleMesh{
		triangles: triangles,
		bvh:       bvh,
		bbox:      bbox,
		material:  material,
	}
}

// NewTriangleMeshWithMaterials creates a triangle mesh where each triangle can have its own material
// vertices: array of 3D points
// faces: array of triangle indices (each group of 3 indices forms a triangle)
// materials: array of materials, one per triangle (must match number of triangles)
func NewTriangleMeshWithMaterials(vertices []core.Vec3, faces []int, materials []core.Material) *TriangleMesh {
	if len(faces)%3 != 0 {
		panic("Face indices must be a multiple of 3")
	}

	numTriangles := len(faces) / 3
	if len(materials) != numTriangles {
		panic("Number of materials must match number of triangles")
	}

	triangles := make([]core.Shape, numTriangles)

	// Create individual triangles
	for i := 0; i < numTriangles; i++ {
		i0 := faces[i*3]
		i1 := faces[i*3+1]
		i2 := faces[i*3+2]

		// Bounds check
		if i0 >= len(vertices) || i1 >= len(vertices) || i2 >= len(vertices) ||
			i0 < 0 || i1 < 0 || i2 < 0 {
			panic("Face index out of bounds")
		}

		triangle := NewTriangle(vertices[i0], vertices[i1], vertices[i2], materials[i])
		triangles[i] = triangle
	}

	// Build BVH for fast intersection
	bvh := core.NewBVH(triangles)

	// Calculate overall bounding box
	var bbox core.AABB
	if len(triangles) > 0 {
		bbox = triangles[0].BoundingBox()
		for i := 1; i < len(triangles); i++ {
			bbox = bbox.Union(triangles[i].BoundingBox())
		}
	}

	return &TriangleMesh{
		triangles: triangles,
		bvh:       bvh,
		bbox:      bbox,
		material:  materials[0], // Use first material as default
	}
}

// Hit tests if a ray intersects with any triangle in the mesh
func (tm *TriangleMesh) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	// Use the BVH for fast intersection
	return tm.bvh.Hit(ray, tMin, tMax)
}

// BoundingBox returns the axis-aligned bounding box for the entire mesh
func (tm *TriangleMesh) BoundingBox() core.AABB {
	return tm.bbox
}

// GetTriangleCount returns the number of triangles in this mesh
func (tm *TriangleMesh) GetTriangleCount() int {
	return len(tm.triangles)
}

// GetTriangles returns the individual triangles (for debugging or special operations)
func (tm *TriangleMesh) GetTriangles() []core.Shape {
	return tm.triangles
}

// NewTriangleMeshWithRotation creates a triangle mesh with rotation applied to all vertices
func NewTriangleMeshWithRotation(vertices []core.Vec3, faces []int, material core.Material, center, rotation core.Vec3) *TriangleMesh {
	// Apply rotation to vertices
	rotatedVertices := make([]core.Vec3, len(vertices))
	for i, vertex := range vertices {
		// Translate to origin, rotate, then translate back
		localVertex := vertex.Subtract(center)
		rotatedVertex := rotateVertex(localVertex, rotation)
		rotatedVertices[i] = rotatedVertex.Add(center)
	}

	return NewTriangleMesh(rotatedVertices, faces, material)
}

// rotateVertex applies rotation around X, Y, Z axes (in that order)
func rotateVertex(vertex, rotation core.Vec3) core.Vec3 {
	// Rotation around X axis
	if rotation.X != 0 {
		cos := math.Cos(rotation.X)
		sin := math.Sin(rotation.X)
		y := vertex.Y*cos - vertex.Z*sin
		z := vertex.Y*sin + vertex.Z*cos
		vertex = core.NewVec3(vertex.X, y, z)
	}

	// Rotation around Y axis
	if rotation.Y != 0 {
		cos := math.Cos(rotation.Y)
		sin := math.Sin(rotation.Y)
		x := vertex.X*cos + vertex.Z*sin
		z := -vertex.X*sin + vertex.Z*cos
		vertex = core.NewVec3(x, vertex.Y, z)
	}

	// Rotation around Z axis
	if rotation.Z != 0 {
		cos := math.Cos(rotation.Z)
		sin := math.Sin(rotation.Z)
		x := vertex.X*cos - vertex.Y*sin
		y := vertex.X*sin + vertex.Y*cos
		vertex = core.NewVec3(x, y, vertex.Z)
	}

	return vertex
}
