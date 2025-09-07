package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Disc represents a circular disc in 3D space
type Disc struct {
	Center   core.Vec3         // Center of the disc
	Normal   core.Vec3         // Normal vector (pointing "up" from the disc)
	Radius   float64           // Radius of the disc
	Material material.Material // Material of the disc
	Right    core.Vec3         // Right vector (perpendicular to normal)
	Up       core.Vec3         // Up vector (perpendicular to normal and right)
}

// NewDisc creates a new disc
func NewDisc(center, normal core.Vec3, radius float64, material material.Material) *Disc {
	normalNormalized := normal.Normalize()

	// Create orthogonal vectors
	var right core.Vec3
	if math.Abs(normalNormalized.X) > 0.1 {
		right = core.NewVec3(0, 1, 0)
	} else {
		right = core.NewVec3(1, 0, 0)
	}

	right = right.Cross(normalNormalized).Normalize()
	up := normalNormalized.Cross(right).Normalize()

	return &Disc{
		Center:   center,
		Normal:   normalNormalized,
		Radius:   radius,
		Material: material,
		Right:    right,
		Up:       up,
	}
}

// Hit implements the Shape interface
func (d *Disc) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
	// Check if ray intersects the plane containing the disc
	denom := d.Normal.Dot(ray.Direction)
	if math.Abs(denom) < 1e-6 {
		return nil, false // Ray is parallel to disc
	}

	// Calculate intersection with plane
	t := d.Normal.Dot(d.Center.Subtract(ray.Origin)) / denom
	if t < tMin || t > tMax {
		return nil, false
	}

	// Check if intersection point is within disc radius
	hitPoint := ray.At(t)
	centerToHit := hitPoint.Subtract(d.Center)
	distanceSquared := centerToHit.LengthSquared()

	if distanceSquared > d.Radius*d.Radius {
		return nil, false // Outside disc
	}

	// Create hit record
	hitRecord := &material.SurfaceInteraction{
		Point:    hitPoint,
		T:        t,
		Material: d.Material,
	}

	// Set face normal
	hitRecord.SetFaceNormal(ray, d.Normal)

	return hitRecord, true
}

// BoundingBox implements the Shape interface
func (d *Disc) BoundingBox() AABB {
	// Create a bounding box that encompasses the disc
	// The disc extends radius in all directions perpendicular to the normal

	// Find the extent in each axis
	rightExtent := d.Right.Multiply(d.Radius)
	upExtent := d.Up.Multiply(d.Radius)

	// Calculate the corners of the bounding box
	corner1 := d.Center.Add(rightExtent).Add(upExtent)
	corner2 := d.Center.Add(rightExtent).Subtract(upExtent)
	corner3 := d.Center.Subtract(rightExtent).Add(upExtent)
	corner4 := d.Center.Subtract(rightExtent).Subtract(upExtent)

	// Find min and max coordinates
	minX := math.Min(math.Min(corner1.X, corner2.X), math.Min(corner3.X, corner4.X))
	minY := math.Min(math.Min(corner1.Y, corner2.Y), math.Min(corner3.Y, corner4.Y))
	minZ := math.Min(math.Min(corner1.Z, corner2.Z), math.Min(corner3.Z, corner4.Z))

	maxX := math.Max(math.Max(corner1.X, corner2.X), math.Max(corner3.X, corner4.X))
	maxY := math.Max(math.Max(corner1.Y, corner2.Y), math.Max(corner3.Y, corner4.Y))
	maxZ := math.Max(math.Max(corner1.Z, corner2.Z), math.Max(corner3.Z, corner4.Z))

	return AABB{
		Min: core.NewVec3(minX, minY, minZ),
		Max: core.NewVec3(maxX, maxY, maxZ),
	}
}

// SampleUniform samples a random point uniformly on the disc surface
func (d *Disc) SampleUniform(sample core.Vec2) (core.Vec3, core.Vec3) {
	// Sample uniformly on unit disc using polar coordinates
	r := math.Sqrt(sample.X) * d.Radius
	theta := 2.0 * math.Pi * sample.Y

	// Convert to Cartesian coordinates in disc space
	x := r * math.Cos(theta)
	y := r * math.Sin(theta)

	// Transform to world space
	point := d.Center.Add(d.Right.Multiply(x)).Add(d.Up.Multiply(y))
	normal := d.Normal

	return point, normal
}
