package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Cylinder represents a finite cylinder shape (open-ended, no caps)
type Cylinder struct {
	BaseCenter core.Vec3
	TopCenter  core.Vec3
	Radius     float64
	Material   material.Material

	// Cached derived values
	axis   core.Vec3 // Unit vector from base to top
	height float64   // Distance between base and top
}

// NewCylinder creates a new cylinder
func NewCylinder(baseCenter, topCenter core.Vec3, radius float64, mat material.Material) *Cylinder {
	// Calculate derived values
	axisVector := topCenter.Subtract(baseCenter)
	height := axisVector.Length()
	axis := axisVector.Normalize()

	return &Cylinder{
		BaseCenter: baseCenter,
		TopCenter:  topCenter,
		Radius:     radius,
		Material:   mat,
		axis:       axis,
		height:     height,
	}
}

// BoundingBox returns the axis-aligned bounding box for this cylinder
func (c *Cylinder) BoundingBox() AABB {
	// Find the AABB of the line segment from base to top
	minCorner := core.NewVec3(
		math.Min(c.BaseCenter.X, c.TopCenter.X),
		math.Min(c.BaseCenter.Y, c.TopCenter.Y),
		math.Min(c.BaseCenter.Z, c.TopCenter.Z),
	)
	maxCorner := core.NewVec3(
		math.Max(c.BaseCenter.X, c.TopCenter.X),
		math.Max(c.BaseCenter.Y, c.TopCenter.Y),
		math.Max(c.BaseCenter.Z, c.TopCenter.Z),
	)

	// For each axis direction, determine the extent
	// If the cylinder axis is parallel to a coordinate axis, don't extend in that direction
	// Otherwise, extend by the radius
	const parallelThreshold = 0.9999 // Very close to 1.0

	extentX := c.Radius
	extentY := c.Radius
	extentZ := c.Radius

	// If axis is parallel to X, don't extend in X
	if math.Abs(c.axis.X) > parallelThreshold {
		extentX = 0
	}
	// If axis is parallel to Y, don't extend in Y
	if math.Abs(c.axis.Y) > parallelThreshold {
		extentY = 0
	}
	// If axis is parallel to Z, don't extend in Z
	if math.Abs(c.axis.Z) > parallelThreshold {
		extentZ = 0
	}

	return NewAABB(
		core.NewVec3(
			minCorner.X-extentX,
			minCorner.Y-extentY,
			minCorner.Z-extentZ,
		),
		core.NewVec3(
			maxCorner.X+extentX,
			maxCorner.Y+extentY,
			maxCorner.Z+extentZ,
		),
	)
}

// Hit tests if a ray intersects with the cylinder
func (c *Cylinder) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
	// Vector from ray origin to base center
	delta := ray.Origin.Subtract(c.BaseCenter)

	// Precompute dot products
	DV := ray.Direction.Dot(c.axis) // D · V̂
	deltaV := delta.Dot(c.axis)     // Δ · V̂

	// Quadratic equation coefficients: at² + bt + cc = 0
	// From spec:
	// a = |D|² - (D·V̂)²
	// b = 2[Δ·D - (Δ·V̂)(D·V̂)]
	// cc = |Δ|² - (Δ·V̂)² - r²
	a := ray.Direction.LengthSquared() - DV*DV
	b := 2.0 * (delta.Dot(ray.Direction) - deltaV*DV)
	cc := delta.LengthSquared() - deltaV*deltaV - c.Radius*c.Radius

	// Check for parallel ray (a ≈ 0)
	const epsilon = 1e-8
	if math.Abs(a) < epsilon {
		// Ray is parallel to cylinder axis - will miss
		return nil, false
	}

	// Compute discriminant
	discriminant := b*b - 4*a*cc

	// No intersection if discriminant is negative
	if discriminant < 0 {
		return nil, false
	}

	// Find the nearest intersection point within the valid range
	sqrtD := math.Sqrt(discriminant)

	// Try the closer intersection point first
	t := (-b - sqrtD) / (2 * a)
	if t < tMin || t > tMax {
		// Try the farther intersection point
		t = (-b + sqrtD) / (2 * a)
		if t < tMin || t > tMax {
			// Both intersections are outside valid range
			return nil, false
		}
	}

	// Compute intersection point
	point := ray.At(t)

	// Check height bounds
	h := point.Subtract(c.BaseCenter).Dot(c.axis)
	if h < 0 || h > c.height {
		// Try the other root
		if t == (-b-sqrtD)/(2*a) {
			t = (-b + sqrtD) / (2 * a)
			if t < tMin || t > tMax {
				return nil, false
			}
			point = ray.At(t)
			h = point.Subtract(c.BaseCenter).Dot(c.axis)
			if h < 0 || h > c.height {
				return nil, false
			}
		} else {
			return nil, false
		}
	}

	// Calculate surface normal (radial direction from axis to point)
	// Point on axis at same height as intersection
	axisPoint := c.BaseCenter.Add(c.axis.Multiply(h))
	// Normal points radially outward
	outwardNormal := point.Subtract(axisPoint).Normalize()

	// Create hit record
	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    point,
		Material: c.Material,
	}
	hitRecord.SetFaceNormal(ray, outwardNormal)

	return hitRecord, true
}
