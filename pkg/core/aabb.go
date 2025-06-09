package core

import "math"

// AABB represents an axis-aligned bounding box
type AABB struct {
	Min Vec3 // Minimum corner
	Max Vec3 // Maximum corner
}

// NewAABB creates a new AABB from min and max points
func NewAABB(min, max Vec3) AABB {
	return AABB{Min: min, Max: max}
}

// NewAABBFromPoints creates an AABB that bounds all given points
func NewAABBFromPoints(points ...Vec3) AABB {
	if len(points) == 0 {
		return AABB{}
	}

	min := points[0]
	max := points[0]

	for _, point := range points[1:] {
		min.X = math.Min(min.X, point.X)
		min.Y = math.Min(min.Y, point.Y)
		min.Z = math.Min(min.Z, point.Z)

		max.X = math.Max(max.X, point.X)
		max.Y = math.Max(max.Y, point.Y)
		max.Z = math.Max(max.Z, point.Z)
	}

	return AABB{Min: min, Max: max}
}

// Hit tests if a ray intersects with this AABB using the slab method
func (aabb AABB) Hit(ray Ray, tMin, tMax float64) bool {
	for axis := 0; axis < 3; axis++ {
		var min, max, origin, direction float64

		switch axis {
		case 0: // X axis
			min = aabb.Min.X
			max = aabb.Max.X
			origin = ray.Origin.X
			direction = ray.Direction.X
		case 1: // Y axis
			min = aabb.Min.Y
			max = aabb.Max.Y
			origin = ray.Origin.Y
			direction = ray.Direction.Y
		case 2: // Z axis
			min = aabb.Min.Z
			max = aabb.Max.Z
			origin = ray.Origin.Z
			direction = ray.Direction.Z
		}

		// Handle parallel rays (direction near zero)
		if math.Abs(direction) < 1e-8 {
			// Ray is parallel to this axis
			if origin < min || origin > max {
				return false // Ray origin outside slab
			}
			continue
		}

		// Calculate intersection distances for this axis
		invDirection := 1.0 / direction
		t1 := (min - origin) * invDirection
		t2 := (max - origin) * invDirection

		// Ensure t1 <= t2 (swap if needed)
		if t1 > t2 {
			t1, t2 = t2, t1
		}

		// Update overall intersection interval
		tMin = math.Max(tMin, t1)
		tMax = math.Min(tMax, t2)

		// No intersection if tMin > tMax
		if tMin > tMax {
			return false
		}
	}

	return true
}

// Union returns an AABB that bounds both this AABB and another
func (aabb AABB) Union(other AABB) AABB {
	min := Vec3{
		X: math.Min(aabb.Min.X, other.Min.X),
		Y: math.Min(aabb.Min.Y, other.Min.Y),
		Z: math.Min(aabb.Min.Z, other.Min.Z),
	}
	max := Vec3{
		X: math.Max(aabb.Max.X, other.Max.X),
		Y: math.Max(aabb.Max.Y, other.Max.Y),
		Z: math.Max(aabb.Max.Z, other.Max.Z),
	}
	return AABB{Min: min, Max: max}
}

// Center returns the center point of the AABB
func (aabb AABB) Center() Vec3 {
	return aabb.Min.Add(aabb.Max).Multiply(0.5)
}

// Size returns the size (extent) of the AABB along each axis
func (aabb AABB) Size() Vec3 {
	return aabb.Max.Subtract(aabb.Min)
}

// SurfaceArea returns the surface area of the AABB
func (aabb AABB) SurfaceArea() float64 {
	size := aabb.Size()
	return 2.0 * (size.X*size.Y + size.Y*size.Z + size.Z*size.X)
}

// LongestAxis returns the axis (0=X, 1=Y, 2=Z) with the longest extent
func (aabb AABB) LongestAxis() int {
	size := aabb.Size()
	if size.X > size.Y && size.X > size.Z {
		return 0 // X axis
	}
	if size.Y > size.Z {
		return 1 // Y axis
	}
	return 2 // Z axis
}

// IsValid returns true if this is a valid AABB (min <= max for all axes)
func (aabb AABB) IsValid() bool {
	return aabb.Min.X <= aabb.Max.X &&
		aabb.Min.Y <= aabb.Max.Y &&
		aabb.Min.Z <= aabb.Max.Z
}

// Expand returns an AABB expanded by the given amount in all directions
func (aabb AABB) Expand(amount float64) AABB {
	expansion := NewVec3(amount, amount, amount)
	return AABB{
		Min: aabb.Min.Subtract(expansion),
		Max: aabb.Max.Add(expansion),
	}
}
