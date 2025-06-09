package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Sphere represents a sphere shape
type Sphere struct {
	Center   core.Vec3
	Radius   float64
	Material core.Material
}

// NewSphere creates a new sphere
func NewSphere(center core.Vec3, radius float64, material core.Material) *Sphere {
	return &Sphere{
		Center:   center,
		Radius:   radius,
		Material: material,
	}
}

// Hit tests if a ray intersects with the sphere
func (s *Sphere) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	// Vector from ray origin to sphere center
	oc := ray.Origin.Subtract(s.Center)

	// Quadratic equation coefficients: at² + bt + c = 0
	a := ray.Direction.Dot(ray.Direction)
	halfB := oc.Dot(ray.Direction)
	c := oc.Dot(oc) - s.Radius*s.Radius

	// Discriminant
	discriminant := halfB*halfB - a*c

	// No intersection if discriminant is negative
	if discriminant < 0 {
		return nil, false
	}

	// Find the nearest intersection point within the valid range
	sqrtD := math.Sqrt(discriminant)

	// Try the closer intersection point first
	root := (-halfB - sqrtD) / a
	if root < tMin || root > tMax {
		// Try the farther intersection point
		root = (-halfB + sqrtD) / a
		if root < tMin || root > tMax {
			// Both intersections are outside valid range
			return nil, false
		}
	}

	// Create hit record with material
	hitRecord := &core.HitRecord{
		T:        root,
		Point:    ray.At(root),
		Material: s.Material,
	}

	// Calculate outward normal (from center to hit point)
	outwardNormal := hitRecord.Point.Subtract(s.Center).Multiply(1.0 / s.Radius)
	hitRecord.SetFaceNormal(ray, outwardNormal)

	return hitRecord, true
}

// BoundingBox returns the axis-aligned bounding box for this sphere
func (s *Sphere) BoundingBox() core.AABB {
	radius := core.NewVec3(s.Radius, s.Radius, s.Radius)
	return core.NewAABB(
		s.Center.Subtract(radius),
		s.Center.Add(radius),
	)
}
