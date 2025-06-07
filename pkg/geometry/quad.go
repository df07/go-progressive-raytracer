package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Quad represents a rectangular surface defined by a corner and two edge vectors
type Quad struct {
	Corner   core.Vec3     // One corner of the quad
	U        core.Vec3     // First edge vector
	V        core.Vec3     // Second edge vector
	Normal   core.Vec3     // Normal vector (computed from U × V)
	Material core.Material // Material of the quad
	D        float64       // Plane equation constant: ax + by + cz = d
	W        core.Vec3     // Cached cross product for barycentric coordinates
}

// NewQuad creates a new quad from a corner point and two edge vectors
func NewQuad(corner, u, v core.Vec3, material core.Material) *Quad {
	// Calculate normal from cross product of edge vectors
	normal := u.Cross(v).Normalize()

	// Calculate plane equation constant: d = normal · corner
	d := normal.Dot(corner)

	// Calculate w vector for barycentric coordinate calculations
	// w = normal / (normal · (u × v))
	cross := u.Cross(v)
	w := normal.Multiply(1.0 / normal.Dot(cross))

	return &Quad{
		Corner:   corner,
		U:        u,
		V:        v,
		Normal:   normal,
		Material: material,
		D:        d,
		W:        w,
	}
}

// Hit tests if a ray intersects with the quad
func (q *Quad) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	// Calculate denominator: dot product of ray direction and quad normal
	denominator := ray.Direction.Dot(q.Normal)

	// If denominator is close to zero, ray is parallel to quad (no intersection)
	if math.Abs(denominator) < 1e-8 {
		return nil, false
	}

	// Calculate t parameter for plane intersection
	t := (q.D - ray.Origin.Dot(q.Normal)) / denominator

	// Check if intersection is within valid range
	if t < tMin || t > tMax {
		return nil, false
	}

	// Calculate intersection point
	hitPoint := ray.At(t)

	// Check if hit point is within the quad bounds using barycentric coordinates
	hitVector := hitPoint.Subtract(q.Corner)

	// Calculate barycentric coordinates
	alpha := q.W.Dot(hitVector.Cross(q.V))
	beta := q.W.Dot(q.U.Cross(hitVector))

	// Check if point is within quad bounds
	if alpha < 0 || alpha > 1 || beta < 0 || beta > 1 {
		return nil, false
	}

	// Create hit record
	hitRecord := &core.HitRecord{
		T:        t,
		Point:    hitPoint,
		Material: q.Material,
	}

	// Set face normal
	hitRecord.SetFaceNormal(ray, q.Normal)

	return hitRecord, true
}
