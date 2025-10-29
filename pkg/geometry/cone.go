package geometry

import (
	"fmt"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Cone represents a finite cone or frustum shape
type Cone struct {
	BaseCenter core.Vec3
	BaseRadius float64
	TopCenter  core.Vec3
	TopRadius  float64 // 0 for pointed cone, >0 for frustum
	Capped     bool    // Whether to include circular end cap(s)
	Material   material.Material

	// Cached derived values
	axis     core.Vec3 // Unit vector from base to top
	height   float64   // Distance between base and top
	tanAngle float64   // tan(cone angle) = (BaseRadius - TopRadius) / height
	apex     core.Vec3 // Apex of the infinite cone extended from frustum
}

// NewCone creates a new cone or frustum
func NewCone(baseCenter core.Vec3, baseRadius float64, topCenter core.Vec3, topRadius float64, capped bool, mat material.Material) (*Cone, error) {
	// Validate parameters
	if baseRadius <= 0 {
		return nil, fmt.Errorf("base radius must be positive, got %f", baseRadius)
	}
	if topRadius < 0 {
		return nil, fmt.Errorf("top radius must be non-negative, got %f", topRadius)
	}
	if baseRadius <= topRadius {
		return nil, fmt.Errorf("base radius must be greater than top radius for a cone (got base=%f, top=%f). Use Cylinder for equal radii", baseRadius, topRadius)
	}

	// Calculate derived values
	axisVector := topCenter.Subtract(baseCenter)
	height := axisVector.Length()

	if height <= 0 {
		return nil, fmt.Errorf("height must be positive (base and top centers cannot be the same)")
	}

	axis := axisVector.Normalize()
	tanAngle := (baseRadius - topRadius) / height

	// Calculate apex position
	var apex core.Vec3
	if topRadius == 0 {
		// For a pointed cone, the top center IS the apex
		apex = topCenter
	} else {
		// For a frustum, calculate where the apex of the infinite cone would be
		// The apex is beyond the top where radius would be 0
		// Distance from top to apex: topRadius / tan(angle) = topRadius * height / (baseRadius - topRadius)
		dFromTop := topRadius * height / (baseRadius - topRadius)
		apex = topCenter.Add(axis.Multiply(dFromTop))
	}

	return &Cone{
		BaseCenter: baseCenter,
		BaseRadius: baseRadius,
		TopCenter:  topCenter,
		TopRadius:  topRadius,
		Capped:     capped,
		Material:   mat,
		axis:       axis,
		height:     height,
		tanAngle:   tanAngle,
		apex:       apex,
	}, nil
}

// BoundingBox returns the axis-aligned bounding box for this cone
func (c *Cone) BoundingBox() AABB {
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
	// If the cone axis is parallel to a coordinate axis, use appropriate radius
	const parallelThreshold = 0.9999

	extentX := c.BaseRadius // Conservative: use base radius
	extentY := c.BaseRadius
	extentZ := c.BaseRadius

	// If axis is parallel to X, use max radius for Y and Z only
	if math.Abs(c.axis.X) > parallelThreshold {
		extentX = 0
		extentY = c.BaseRadius
		extentZ = c.BaseRadius
	}
	// If axis is parallel to Y, use max radius for X and Z only
	if math.Abs(c.axis.Y) > parallelThreshold {
		extentX = c.BaseRadius
		extentY = 0
		extentZ = c.BaseRadius
	}
	// If axis is parallel to Z, use max radius for X and Y only
	if math.Abs(c.axis.Z) > parallelThreshold {
		extentX = c.BaseRadius
		extentY = c.BaseRadius
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

// Hit tests if a ray intersects with the cone (body and optionally caps)
func (c *Cone) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
	var closestHit *material.SurfaceInteraction
	closestT := tMax

	// Check cone body intersection
	if bodyHit := c.hitBody(ray, tMin, closestT); bodyHit != nil {
		closestHit = bodyHit
		closestT = bodyHit.T
	}

	// Check cap intersections if capped
	if c.Capped {
		// Always check base cap
		if baseHit := c.hitCap(ray, c.BaseCenter, c.axis.Negate(), c.BaseRadius, tMin, closestT); baseHit != nil {
			closestHit = baseHit
			closestT = baseHit.T
		}

		// Check top cap only for frustums (topRadius > 0)
		if c.TopRadius > 0 {
			if topHit := c.hitCap(ray, c.TopCenter, c.axis, c.TopRadius, tMin, closestT); topHit != nil {
				closestHit = topHit
				closestT = topHit.T
			}
		}
	}

	if closestHit != nil {
		return closestHit, true
	}
	return nil, false
}

// hitBody checks for intersection with the cone body (curved surface)
func (c *Cone) hitBody(ray core.Ray, tMin, tMax float64) *material.SurfaceInteraction {
	// Vector from ray origin to apex
	CO := ray.Origin.Subtract(c.apex)

	// Precompute dot products
	DdotV := ray.Direction.Dot(c.axis)
	COdotV := CO.Dot(c.axis)

	// k = tan²(α)
	k := c.tanAngle * c.tanAngle

	// Quadratic equation coefficients: at² + bt + cc = 0
	// From spec:
	// a = D·D - (1 + k)·DdotV²
	// b = 2[D·CO - (1 + k)·DdotV·COdotV]
	// cc = CO·CO - (1 + k)·COdotV²
	a := ray.Direction.LengthSquared() - (1+k)*DdotV*DdotV
	b := 2.0 * (ray.Direction.Dot(CO) - (1+k)*DdotV*COdotV)
	cc := CO.LengthSquared() - (1+k)*COdotV*COdotV

	// Check for nearly parallel ray (a ≈ 0)
	const epsilon = 1e-8
	if math.Abs(a) < epsilon {
		// Ray is nearly parallel to cone surface - will likely miss
		return nil
	}

	// Compute discriminant
	discriminant := b*b - 4*a*cc

	// No intersection if discriminant is negative
	if discriminant < 0 {
		return nil
	}

	// Find the nearest intersection point within the valid range
	sqrtD := math.Sqrt(discriminant)

	// Try the closer intersection point first
	t := (-b - sqrtD) / (2 * a)
	if !c.validateIntersection(ray, t, tMin, tMax) {
		// Try the farther intersection point
		t = (-b + sqrtD) / (2 * a)
		if !c.validateIntersection(ray, t, tMin, tMax) {
			// Both intersections are invalid
			return nil
		}
	}

	// Compute intersection point
	point := ray.At(t)

	// Calculate surface normal
	// Height along cone from base
	h := point.Subtract(c.BaseCenter).Dot(c.axis)

	// Center point on axis at this height
	centerPoint := c.BaseCenter.Add(c.axis.Multiply(h))

	// Radial vector from axis to point
	radial := point.Subtract(centerPoint)

	// Normal calculation: normalize(radial + (BaseRadius - TopRadius) / height * axis)
	normalScale := (c.BaseRadius - c.TopRadius) / c.height
	outwardNormal := radial.Add(c.axis.Multiply(normalScale)).Normalize()

	// Create hit record
	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    point,
		Material: c.Material,
	}
	hitRecord.SetFaceNormal(ray, outwardNormal)

	return hitRecord
}

// validateIntersection checks if an intersection at parameter t is valid
func (c *Cone) validateIntersection(ray core.Ray, t, tMin, tMax float64) bool {
	const epsilon = 1e-8

	// Check ray parameter bounds
	if t < tMin || t > tMax {
		return false
	}

	// Compute intersection point
	point := ray.At(t)

	// Check height bounds
	h := point.Subtract(c.BaseCenter).Dot(c.axis)
	if h < -epsilon || h > c.height+epsilon {
		return false
	}

	// Check shadow cone: verify we're on the correct cone nappe
	// The apex is always above/beyond the top in our coordinate system
	// Valid points (between base and top) should have negative dot product with axis from apex
	apexToPoint := point.Subtract(c.apex)
	dotProduct := apexToPoint.Dot(c.axis)

	// Valid points should be "below" the apex (negative dot product)
	if dotProduct > epsilon {
		return false
	}

	return true
}

// hitCap checks for intersection with a circular cap (disc)
func (c *Cone) hitCap(ray core.Ray, center, normal core.Vec3, radius, tMin, tMax float64) *material.SurfaceInteraction {
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
	if distFromCenter > radius {
		return nil
	}

	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    point,
		Material: c.Material,
	}
	hitRecord.SetFaceNormal(ray, normal)

	return hitRecord
}
