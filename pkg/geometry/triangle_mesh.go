package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// TriangleMesh represents a collection of triangles with efficient ray intersection
// It uses an internal BVH (Bounding Volume Hierarchy) for fast intersection tests
type TriangleMesh struct {
	triangles []Shape           // Individual triangles as shapes
	bvh       *BVH              // BVH for fast intersection
	bbox      AABB              // Overall bounding box
	material  material.Material // Default material (can be overridden per triangle)
}

// TriangleMeshOptions contains optional parameters for triangle mesh creation
type TriangleMeshOptions struct {
	Normals   []core.Vec3         // Optional custom normals (one per triangle)
	Materials []material.Material // Optional per-triangle materials
	Rotation  *core.Vec3          // Optional rotation to apply to vertices
	Center    *core.Vec3          // Optional center point for rotation
	VertexUVs []core.Vec2         // Optional per-vertex texture coordinates
}

// NewTriangleMesh creates a new triangle mesh from vertices and face indices
// vertices: array of 3D points
// faces: array of triangle indices (each group of 3 indices forms a triangle)
// material: default material for all triangles
// options: optional parameters (can be nil for basic mesh)
func NewTriangleMesh(vertices []core.Vec3, faces []int, material material.Material, options *TriangleMeshOptions) *TriangleMesh {
	if len(faces)%3 != 0 {
		panic("Face indices must be a multiple of 3")
	}

	numTriangles := len(faces) / 3

	// Validate options if provided
	if options != nil {
		if options.Normals != nil && len(options.Normals) != numTriangles {
			panic("Number of normals must match number of triangles")
		}
		if options.Materials != nil && len(options.Materials) != numTriangles {
			panic("Number of materials must match number of triangles")
		}
		if options.VertexUVs != nil && len(options.VertexUVs) != len(vertices) {
			panic("Number of vertex UVs must match number of vertices")
		}
	}

	// Apply rotation if specified
	workingVertices := vertices
	if options != nil && options.Rotation != nil {
		workingVertices = make([]core.Vec3, len(vertices))
		for i, vertex := range vertices {
			// Translate to center, rotate, then translate back
			if options.Center != nil {
				vertex = vertex.Subtract(*options.Center)
			}
			vertex = rotateVertex(vertex, *options.Rotation)
			if options.Center != nil {
				vertex = vertex.Add(*options.Center)
			}
			workingVertices[i] = vertex
		}
	}

	triangles := make([]Shape, numTriangles)

	// Create individual triangles
	for i := 0; i < numTriangles; i++ {
		i0 := faces[i*3]
		i1 := faces[i*3+1]
		i2 := faces[i*3+2]

		// Bounds check
		if i0 >= len(workingVertices) || i1 >= len(workingVertices) || i2 >= len(workingVertices) ||
			i0 < 0 || i1 < 0 || i2 < 0 {
			panic("Face index out of bounds")
		}

		// Determine material for this triangle
		triangleMaterial := material
		if options != nil && options.Materials != nil {
			triangleMaterial = options.Materials[i]
		}

		// Get vertex positions
		v0 := workingVertices[i0]
		v1 := workingVertices[i1]
		v2 := workingVertices[i2]

		// Create triangle with appropriate constructor based on available data
		var triangle Shape
		hasUVs := options != nil && options.VertexUVs != nil
		hasNormals := options != nil && options.Normals != nil

		if hasUVs && hasNormals {
			// Both UVs and normals provided
			uv0 := options.VertexUVs[i0]
			uv1 := options.VertexUVs[i1]
			uv2 := options.VertexUVs[i2]
			triangle = NewTriangleWithNormalAndUVs(v0, v1, v2, uv0, uv1, uv2, options.Normals[i], triangleMaterial)
		} else if hasUVs {
			// Only UVs provided
			uv0 := options.VertexUVs[i0]
			uv1 := options.VertexUVs[i1]
			uv2 := options.VertexUVs[i2]
			triangle = NewTriangleWithUVs(v0, v1, v2, uv0, uv1, uv2, triangleMaterial)
		} else if hasNormals {
			// Only normals provided
			triangle = NewTriangleWithNormal(v0, v1, v2, options.Normals[i], triangleMaterial)
		} else {
			// Neither UVs nor normals provided
			triangle = NewTriangle(v0, v1, v2, triangleMaterial)
		}
		triangles[i] = triangle
	}

	// Build BVH for fast intersection
	bvh := NewBVH(triangles)

	// Calculate overall bounding box
	var bbox AABB
	if len(triangles) > 0 {
		bbox = triangles[0].BoundingBox()
		for i := 1; i < len(triangles); i++ {
			bbox = bbox.Union(triangles[i].BoundingBox())
		}
	}

	// Determine default material
	defaultMaterial := material
	if options != nil && options.Materials != nil && len(options.Materials) > 0 {
		defaultMaterial = options.Materials[0]
	}

	return &TriangleMesh{
		triangles: triangles,
		bvh:       bvh,
		bbox:      bbox,
		material:  defaultMaterial,
	}
}

// Hit tests if a ray intersects with any triangle in the mesh
func (tm *TriangleMesh) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
	// Use the BVH for fast intersection
	return tm.bvh.Hit(ray, tMin, tMax)
}

// BoundingBox returns the axis-aligned bounding box for the entire mesh
func (tm *TriangleMesh) BoundingBox() AABB {
	return tm.bbox
}

// GetTriangleCount returns the number of triangles in this mesh
func (tm *TriangleMesh) GetTriangleCount() int {
	return len(tm.triangles)
}

// GetTriangles returns the individual triangles (for debugging or special operations)
func (tm *TriangleMesh) GetTriangles() []Shape {
	return tm.triangles
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
