package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Plane represents an infinite plane defined by a point and normal
type Plane struct {
	Point    core.Vec3     // A point on the plane
	Normal   core.Vec3     // Normal vector (should be normalized)
	Material core.Material // Material of the plane
}

// NewPlane creates a new plane
func NewPlane(point, normal core.Vec3, material core.Material) *Plane {
	return &Plane{
		Point:    point,
		Normal:   normal.Normalize(), // Ensure normal is normalized
		Material: material,
	}
}

// Hit tests if a ray intersects with the plane
func (p *Plane) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	// Calculate denominator: dot product of ray direction and plane normal
	denominator := ray.Direction.Dot(p.Normal)

	// If denominator is close to zero, ray is parallel to plane (no intersection)
	if math.Abs(denominator) < 1e-8 {
		return nil, false
	}

	// Calculate t parameter: t = (point_on_plane - ray_origin) · normal / (ray_direction · normal)
	t := p.Point.Subtract(ray.Origin).Dot(p.Normal) / denominator

	// Check if intersection is within valid range
	if t < tMin || t > tMax {
		return nil, false
	}

	// Calculate intersection point
	hitPoint := ray.At(t)

	// Create hit record
	hitRecord := &core.HitRecord{
		T:        t,
		Point:    hitPoint,
		Material: p.Material,
	}

	// Set face normal (plane normal always points in the same direction)
	hitRecord.SetFaceNormal(ray, p.Normal)

	return hitRecord, true
}

// BoundingBox returns a bounding box for this plane
func (p *Plane) BoundingBox() core.AABB {
	const largeValue = 1e6
	const epsilon = 0.001 // Small thickness to avoid zero-width bounding box

	// Check if the plane is axis-aligned for better BVH performance
	alignment := getAxisAlignment(p.Normal)

	switch alignment {
	case XAxisAligned:
		// Plane is perpendicular to X axis (e.g., wall at x = constant)
		x := p.Point.X
		return core.NewAABB(
			core.NewVec3(x-epsilon, -largeValue, -largeValue),
			core.NewVec3(x+epsilon, largeValue, largeValue),
		)
	case YAxisAligned:
		// Plane is perpendicular to Y axis (e.g., ground plane at y = constant)
		y := p.Point.Y
		return core.NewAABB(
			core.NewVec3(-largeValue, y-epsilon, -largeValue),
			core.NewVec3(largeValue, y+epsilon, largeValue),
		)
	case ZAxisAligned:
		// Plane is perpendicular to Z axis (e.g., back wall at z = constant)
		z := p.Point.Z
		return core.NewAABB(
			core.NewVec3(-largeValue, -largeValue, z-epsilon),
			core.NewVec3(largeValue, largeValue, z+epsilon),
		)
	default:
		// Not axis-aligned - use large bounding box (less optimal but correct)
		return core.NewAABB(
			core.NewVec3(-largeValue, -largeValue, -largeValue),
			core.NewVec3(largeValue, largeValue, largeValue),
		)
	}
}
