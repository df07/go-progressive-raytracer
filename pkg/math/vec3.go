package math

import "math"

// Vec3 represents a 3D vector
type Vec3 struct {
	X, Y, Z float64
}

// NewVec3 creates a new Vec3
func NewVec3(x, y, z float64) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// Add returns the sum of two vectors
func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{v.X + other.X, v.Y + other.Y, v.Z + other.Z}
}

// Subtract returns the difference of two vectors
func (v Vec3) Subtract(other Vec3) Vec3 {
	return Vec3{v.X - other.X, v.Y - other.Y, v.Z - other.Z}
}

// Multiply returns the vector scaled by a scalar
func (v Vec3) Multiply(scalar float64) Vec3 {
	return Vec3{v.X * scalar, v.Y * scalar, v.Z * scalar}
}

// Length returns the magnitude of the vector
func (v Vec3) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z)
}

// Dot returns the dot product of two vectors
func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// Clamp returns a vector with components clamped to [min, max]
func (v Vec3) Clamp(min, max float64) Vec3 {
	clampValue := func(val, min, max float64) float64 {
		if val < min {
			return min
		}
		if val > max {
			return max
		}
		return val
	}

	return Vec3{
		X: clampValue(v.X, min, max),
		Y: clampValue(v.Y, min, max),
		Z: clampValue(v.Z, min, max),
	}
}

// GammaCorrect applies gamma correction to color values
func (v Vec3) GammaCorrect(gamma float64) Vec3 {
	invGamma := 1.0 / gamma
	return Vec3{
		X: math.Pow(v.X, invGamma),
		Y: math.Pow(v.Y, invGamma),
		Z: math.Pow(v.Z, invGamma),
	}
}

// Normalize returns a unit vector in the same direction
func (v Vec3) Normalize() Vec3 {
	length := v.Length()
	if length == 0 {
		return Vec3{0, 0, 0}
	}
	return Vec3{v.X / length, v.Y / length, v.Z / length}
}
