package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// AxisAlignment represents which axis a normal vector is aligned with
type AxisAlignment int

const (
	NotAxisAligned AxisAlignment = iota
	XAxisAligned                 // Normal aligned with X axis
	YAxisAligned                 // Normal aligned with Y axis
	ZAxisAligned                 // Normal aligned with Z axis
)

// getAxisAlignment checks if a normal vector is aligned with any coordinate axis
func getAxisAlignment(normal core.Vec3) AxisAlignment {
	const threshold = 0.9999
	const tolerance = 0.0001

	// Check X axis alignment
	if math.Abs(normal.X) > threshold && math.Abs(normal.Y) < tolerance && math.Abs(normal.Z) < tolerance {
		return XAxisAligned
	}

	// Check Y axis alignment
	if math.Abs(normal.Y) > threshold && math.Abs(normal.X) < tolerance && math.Abs(normal.Z) < tolerance {
		return YAxisAligned
	}

	// Check Z axis alignment
	if math.Abs(normal.Z) > threshold && math.Abs(normal.X) < tolerance && math.Abs(normal.Y) < tolerance {
		return ZAxisAligned
	}

	return NotAxisAligned
}

// createAxisAlignedAABB creates a thin bounding box for axis-aligned quads
func createAxisAlignedAABB(corners []core.Vec3, alignment AxisAlignment, fixedCoord float64) AABB {
	const epsilon = 0.001

	// Find min/max for the two varying coordinates
	switch alignment {
	case XAxisAligned:
		// Quad in YZ plane, X is fixed
		minY, maxY := findMinMax(corners, func(v core.Vec3) float64 { return v.Y })
		minZ, maxZ := findMinMax(corners, func(v core.Vec3) float64 { return v.Z })
		return NewAABB(
			core.NewVec3(fixedCoord-epsilon, minY, minZ),
			core.NewVec3(fixedCoord+epsilon, maxY, maxZ),
		)
	case YAxisAligned:
		// Quad in XZ plane, Y is fixed
		minX, maxX := findMinMax(corners, func(v core.Vec3) float64 { return v.X })
		minZ, maxZ := findMinMax(corners, func(v core.Vec3) float64 { return v.Z })
		return NewAABB(
			core.NewVec3(minX, fixedCoord-epsilon, minZ),
			core.NewVec3(maxX, fixedCoord+epsilon, maxZ),
		)
	case ZAxisAligned:
		// Quad in XY plane, Z is fixed
		minX, maxX := findMinMax(corners, func(v core.Vec3) float64 { return v.X })
		minY, maxY := findMinMax(corners, func(v core.Vec3) float64 { return v.Y })
		return NewAABB(
			core.NewVec3(minX, minY, fixedCoord-epsilon),
			core.NewVec3(maxX, maxY, fixedCoord+epsilon),
		)
	default:
		// Should not happen, but return a safe fallback
		return NewAABBFromPoints(corners[0], corners[1], corners[2], corners[3])
	}
}

// findMinMax finds the minimum and maximum values using the provided accessor function
func findMinMax(corners []core.Vec3, accessor func(core.Vec3) float64) (float64, float64) {
	min := accessor(corners[0])
	max := min

	for i := 1; i < len(corners); i++ {
		val := accessor(corners[i])
		if val < min {
			min = val
		}
		if val > max {
			max = val
		}
	}

	return min, max
}

// Quad represents a rectangular surface defined by a corner and two edge vectors
type Quad struct {
	Corner   core.Vec3         // One corner of the quad
	U        core.Vec3         // First edge vector
	V        core.Vec3         // Second edge vector
	Normal   core.Vec3         // Normal vector (computed from U × V)
	Material material.Material // Material of the quad
	D        float64           // Plane equation constant: ax + by + cz = d
	W        core.Vec3         // Cached cross product for barycentric coordinates
}

// NewQuad creates a new quad from a corner point and two edge vectors
func NewQuad(corner, u, v core.Vec3, material material.Material) *Quad {
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
func (q *Quad) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
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
	hitRecord := &material.SurfaceInteraction{
		T:        t,
		Point:    hitPoint,
		Material: q.Material,
	}

	// Set face normal
	hitRecord.SetFaceNormal(ray, q.Normal)

	return hitRecord, true
}

// BoundingBox returns the axis-aligned bounding box for this quad
func (q *Quad) BoundingBox() AABB {
	// Calculate the four corners of the quad
	corners := []core.Vec3{
		q.Corner,
		q.Corner.Add(q.U),
		q.Corner.Add(q.V),
		q.Corner.Add(q.U).Add(q.V),
	}

	// Check for axis alignment for tighter bounding box
	alignment := getAxisAlignment(q.Normal)
	if alignment != NotAxisAligned {
		// Use the first corner's coordinate for the fixed axis
		var fixedCoord float64
		switch alignment {
		case XAxisAligned:
			fixedCoord = corners[0].X
		case YAxisAligned:
			fixedCoord = corners[0].Y
		case ZAxisAligned:
			fixedCoord = corners[0].Z
		}
		return createAxisAlignedAABB(corners, alignment, fixedCoord)
	}

	// Not axis-aligned - use standard bounding box from all corners
	return NewAABBFromPoints(corners[0], corners[1], corners[2], corners[3])
}
