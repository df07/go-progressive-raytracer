package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Sphere represents a sphere shape
type Sphere struct {
	Center   core.Vec3
	Radius   float64
	Material material.Material
}

// NewSphere creates a new sphere
func NewSphere(center core.Vec3, radius float64, material material.Material) *Sphere {
	return &Sphere{
		Center:   center,
		Radius:   radius,
		Material: material,
	}
}

// Hit tests if a ray intersects with the sphere
func (s *Sphere) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
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

	// Calculate intersection point
	point := ray.At(root)

	// Calculate outward normal (from center to hit point)
	outwardNormal := point.Subtract(s.Center).Multiply(1.0 / s.Radius)

	// Compute UV coordinates from spherical coordinates
	// outwardNormal is (x, y, z) on unit sphere
	theta := math.Acos(-outwardNormal.Y)                           // Angle from top pole [0, π]
	phi := math.Atan2(-outwardNormal.Z, outwardNormal.X) + math.Pi // Angle around equator [0, 2π]
	uv := core.NewVec2(phi/(2.0*math.Pi), theta/math.Pi)

	// Create hit record with material
	hitRecord := &material.SurfaceInteraction{
		T:        root,
		Point:    point,
		Material: s.Material,
		UV:       uv,
	}

	hitRecord.SetFaceNormal(ray, outwardNormal)

	return hitRecord, true
}

// BoundingBox returns the axis-aligned bounding box for this sphere
func (s *Sphere) BoundingBox() AABB {
	radius := core.NewVec3(s.Radius, s.Radius, s.Radius)
	return NewAABB(
		s.Center.Subtract(radius),
		s.Center.Add(radius),
	)
}
