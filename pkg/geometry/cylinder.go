package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Cylinder represents a finite cylinder shape
type Cylinder struct {
	BaseCenter core.Vec3
	TopCenter  core.Vec3
	Radius     float64
	Capped     bool // Whether to include circular end caps
	Material   material.Material

	// Cached derived values
	axis   core.Vec3 // Unit vector from base to top
	height float64   // Distance between base and top
}

// NewCylinder creates a new cylinder
func NewCylinder(baseCenter, topCenter core.Vec3, radius float64, capped bool, mat material.Material) *Cylinder {
	// Calculate derived values
	axisVector := topCenter.Subtract(baseCenter)
	height := axisVector.Length()
	axis := axisVector.Normalize()

	return &Cylinder{
		BaseCenter: baseCenter,
		TopCenter:  topCenter,
		Radius:     radius,
		Capped:     capped,
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
	var closestHit *material.SurfaceInteraction
	closestT := tMax

	// Check cylinder body intersection
	if bodyHit := c.hitBody(ray, tMin, closestT); bodyHit != nil {
		closestHit = bodyHit
		closestT = bodyHit.T
	}

	// Check cap intersections if capped
	if c.Capped {
		// Check base cap
		if baseHit := c.hitCap(ray, c.BaseCenter, c.axis.Negate(), tMin, closestT); baseHit != nil {
			closestHit = baseHit
			closestT = baseHit.T
		}

		// Check top cap
		if topHit := c.hitCap(ray, c.TopCenter, c.axis, tMin, closestT); topHit != nil {
			closestHit = topHit
			closestT = topHit.T
		}
	}

	if closestHit != nil {
		return closestHit, true
	}
	return nil, false
}

// hitBody checks for intersection with the cylinder body (curved surface)
func (c *Cylinder) hitBody(ray core.Ray, tMin, tMax float64) *material.SurfaceInteraction {
	// Vector from ray origin to base center
	delta := ray.Origin.Subtract(c.BaseCenter)

	// Precompute dot products
	DV := ray.Direction.Dot(c.axis) // D · V̂
	deltaV := delta.Dot(c.axis)     // Δ · V̂

	// Quadratic equation coefficients: at² + bt + cc = 0
	a := ray.Direction.LengthSquared() - DV*DV
	b := 2.0 * (delta.Dot(ray.Direction) - deltaV*DV)
	cc := delta.LengthSquared() - deltaV*deltaV - c.Radius*c.Radius

	// Check for parallel ray (a ≈ 0)
	const epsilon = 1e-8
	if math.Abs(a) < epsilon {
		return nil
	}

	// Compute discriminant
	discriminant := b*b - 4*a*cc
	if discriminant < 0 {
		return nil
	}

	sqrtD := math.Sqrt(discriminant)

	// Try the closer intersection point first
	t := (-b - sqrtD) / (2 * a)
	if t < tMin || t > tMax {
		// Try the farther intersection point
		t = (-b + sqrtD) / (2 * a)
		if t < tMin || t > tMax {
			return nil
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
				return nil
			}
			point = ray.At(t)
			h = point.Subtract(c.BaseCenter).Dot(c.axis)
			if h < 0 || h > c.height {
				return nil
			}
		} else {
			return nil
		}
	}

	// Calculate surface normal (radial direction from axis to point)
	axisPoint := c.BaseCenter.Add(c.axis.Multiply(h))
	outwardNormal := point.Subtract(axisPoint).Normalize()

	// Compute UV coordinates
	// V: height along cylinder (0 at base, 1 at top)
	v := h / c.height

	// U: angle around the axis (0 to 1 for full circle)
	radial := point.Subtract(axisPoint)
	var refVector core.Vec3
	if math.Abs(c.axis.Y) < 0.9 {
		refVector = core.NewVec3(0, 1, 0)
	} else {
		refVector = core.NewVec3(1, 0, 0)
	}
	tangent := c.axis.Cross(refVector).Normalize()
	bitangent := c.axis.Cross(tangent)

	u := math.Atan2(radial.Dot(bitangent), radial.Dot(tangent))
	u = (u + math.Pi) / (2.0 * math.Pi) // Map from [-π, π] to [0, 1]

	uv := core.NewVec2(u, v)

	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    point,
		Material: c.Material,
		UV:       uv,
	}
	hitRecord.SetFaceNormal(ray, outwardNormal)

	return hitRecord
}

// hitCap checks for intersection with a circular cap (disc)
func (c *Cylinder) hitCap(ray core.Ray, center, normal core.Vec3, tMin, tMax float64) *material.SurfaceInteraction {
	const epsilon = 1e-8

	// Ray-plane intersection
	denom := ray.Direction.Dot(normal)
	if math.Abs(denom) < epsilon {
		// Ray is parallel to cap plane
		return nil
	}

	t := center.Subtract(ray.Origin).Dot(normal) / denom
	if t < tMin || t > tMax {
		return nil
	}

	// Check if intersection point is within disc radius
	point := ray.At(t)
	distFromCenter := point.Subtract(center).Length()
	if distFromCenter > c.Radius {
		return nil
	}

	// Compute UV coordinates for the cap (disc)
	localPoint := point.Subtract(center)

	var refVector core.Vec3
	if math.Abs(normal.Y) < 0.9 {
		refVector = core.NewVec3(0, 1, 0)
	} else {
		refVector = core.NewVec3(1, 0, 0)
	}
	tangent := normal.Cross(refVector).Normalize()
	bitangent := normal.Cross(tangent)

	u := (localPoint.Dot(tangent)/c.Radius + 1.0) / 2.0
	v := (localPoint.Dot(bitangent)/c.Radius + 1.0) / 2.0
	uv := core.NewVec2(u, v)

	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    point,
		Material: c.Material,
		UV:       uv,
	}
	hitRecord.SetFaceNormal(ray, normal)

	return hitRecord
}
