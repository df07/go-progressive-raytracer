package geometry

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Triangle represents a single triangle defined by three vertices
type Triangle struct {
	V0, V1, V2 core.Vec3     // The three vertices
	Material   core.Material // Material of the triangle
	normal     core.Vec3     // Cached normal vector
	bbox       core.AABB     // Cached bounding box
}

// NewTriangle creates a new triangle from three vertices
func NewTriangle(v0, v1, v2 core.Vec3, material core.Material) *Triangle {
	t := &Triangle{
		V0:       v0,
		V1:       v1,
		V2:       v2,
		Material: material,
	}

	// Precompute normal and bounding box for efficiency
	t.computeNormal()
	t.computeBoundingBox()

	return t
}

// NewTriangleWithNormal creates a new triangle from three vertices with a custom normal
func NewTriangleWithNormal(v0, v1, v2 core.Vec3, normal core.Vec3, material core.Material) *Triangle {
	t := &Triangle{
		V0:       v0,
		V1:       v1,
		V2:       v2,
		Material: material,
		normal:   normal.Normalize(), // Ensure the normal is normalized
	}

	// Only compute bounding box, normal is provided
	t.computeBoundingBox()

	return t
}

// computeNormal calculates and caches the triangle's normal vector
func (t *Triangle) computeNormal() {
	// Calculate two edge vectors
	edge1 := t.V1.Subtract(t.V0)
	edge2 := t.V2.Subtract(t.V0)

	// Normal is the cross product of the two edges
	t.normal = edge1.Cross(edge2).Normalize()
}

// computeBoundingBox calculates and caches the triangle's bounding box
func (t *Triangle) computeBoundingBox() {
	t.bbox = core.NewAABBFromPoints(t.V0, t.V1, t.V2)
}

// Hit tests if a ray intersects with the triangle using the Möller-Trumbore algorithm
func (t *Triangle) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	const epsilon = 1e-8

	// Calculate two edge vectors
	edge1 := t.V1.Subtract(t.V0)
	edge2 := t.V2.Subtract(t.V0)

	// Calculate determinant
	h := ray.Direction.Cross(edge2)
	a := edge1.Dot(h)

	// If determinant is near zero, ray lies in plane of triangle
	if a > -epsilon && a < epsilon {
		return nil, false
	}

	f := 1.0 / a
	s := ray.Origin.Subtract(t.V0)
	u := f * s.Dot(h)

	// Check if intersection is outside triangle
	if u < 0.0 || u > 1.0 {
		return nil, false
	}

	q := s.Cross(edge1)
	v := f * ray.Direction.Dot(q)

	// Check if intersection is outside triangle
	if v < 0.0 || u+v > 1.0 {
		return nil, false
	}

	// Calculate t parameter
	t_param := f * edge2.Dot(q)

	// Check if intersection is within valid range
	if t_param < tMin || t_param > tMax {
		return nil, false
	}

	// Calculate intersection point
	hitPoint := ray.At(t_param)

	// Create hit record
	hitRecord := &core.HitRecord{
		T:        t_param,
		Point:    hitPoint,
		Material: t.Material,
	}

	// Set face normal
	hitRecord.SetFaceNormal(ray, t.normal)

	return hitRecord, true
}

// BoundingBox returns the axis-aligned bounding box for this triangle
func (t *Triangle) BoundingBox() core.AABB {
	return t.bbox
}

// GetNormal returns the triangle's normal vector
func (t *Triangle) GetNormal() core.Vec3 {
	return t.normal
}
