package core

import (
	"fmt"
	"math"
)

// Vec3 represents a 3D vector
type Vec3 struct {
	X, Y, Z float64
}

// Vec2 represents a 2D vector (for texture coordinates, etc.)
type Vec2 struct {
	X, Y float64
}

// NewVec3 creates a new Vec3
func NewVec3(x, y, z float64) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

// NewVec2 creates a new Vec2
func NewVec2(x, y float64) Vec2 {
	return Vec2{X: x, Y: y}
}

// Add returns the sum of two Vec2 values
func (v Vec2) Add(other Vec2) Vec2 {
	return Vec2{v.X + other.X, v.Y + other.Y}
}

// Multiply returns the Vec2 scaled by a scalar
func (v Vec2) Multiply(scalar float64) Vec2 {
	return Vec2{v.X * scalar, v.Y * scalar}
}

func (v Vec3) String() string {
	return fmt.Sprintf("{%.3g, %.3g, %.3g}", v.X, v.Y, v.Z)
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

// LengthSquared returns the squared magnitude of the vector
func (v Vec3) LengthSquared() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

// Dot returns the dot product of two vectors
func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
}

// AbsDot returns the absolute value of the dot product of two vectors
func (v Vec3) AbsDot(other Vec3) float64 {
	return math.Abs(v.Dot(other))
}

// Clamp returns a vector with components clamped to [min, max]
func (v Vec3) Clamp(minVal, maxVal float64) Vec3 {
	return Vec3{
		X: max(minVal, min(maxVal, v.X)),
		Y: max(minVal, min(maxVal, v.Y)),
		Z: max(minVal, min(maxVal, v.Z)),
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

// Cross returns the cross product of two vectors
func (v Vec3) Cross(other Vec3) Vec3 {
	return Vec3{
		X: v.Y*other.Z - v.Z*other.Y,
		Y: v.Z*other.X - v.X*other.Z,
		Z: v.X*other.Y - v.Y*other.X,
	}
}

// MultiplyVec returns component-wise multiplication of two vectors
func (v Vec3) MultiplyVec(other Vec3) Vec3 {
	return Vec3{
		X: v.X * other.X,
		Y: v.Y * other.Y,
		Z: v.Z * other.Z,
	}
}

// Square returns component-wise squares of the vector
func (v Vec3) Square() Vec3 {
	return Vec3{
		X: v.X * v.X,
		Y: v.Y * v.Y,
		Z: v.Z * v.Z,
	}
}

// Luminance returns the perceptual luminance of an RGB color
// Uses Rec. 709 luminance weights (sRGB standard): 0.2126*R + 0.7152*G + 0.0722*B
func (v Vec3) Luminance() float64 {
	return 0.2126*v.X + 0.7152*v.Y + 0.0722*v.Z
}

// IsZero returns true if the vector is zero
func (v Vec3) IsZero() bool {
	return v.X == 0 && v.Y == 0 && v.Z == 0
}

// Negate returns the negative of the vector
func (v Vec3) Negate() Vec3 {
	return Vec3{
		X: -v.X,
		Y: -v.Y,
		Z: -v.Z,
	}
}

// Equals compares two Vec3 values with a small tolerance for floating point precision
func (v Vec3) Equals(other Vec3) bool {
	const tolerance = 1e-9
	return math.Abs(v.X-other.X) < tolerance &&
		math.Abs(v.Y-other.Y) < tolerance &&
		math.Abs(v.Z-other.Z) < tolerance
}

// Rotate applies rotation around X, Y, Z axes (in that order) to the vector
// Rotation angles are in radians
func (v Vec3) Rotate(rotation Vec3) Vec3 {
	// If no rotation, return original vector
	if rotation.X == 0 && rotation.Y == 0 && rotation.Z == 0 {
		return v
	}

	result := v

	// Rotation around X axis
	if rotation.X != 0 {
		cosX := math.Cos(rotation.X)
		sinX := math.Sin(rotation.X)
		y := result.Y*cosX - result.Z*sinX
		z := result.Y*sinX + result.Z*cosX
		result = NewVec3(result.X, y, z)
	}

	// Rotation around Y axis
	if rotation.Y != 0 {
		cosY := math.Cos(rotation.Y)
		sinY := math.Sin(rotation.Y)
		x := result.X*cosY + result.Z*sinY
		z := -result.X*sinY + result.Z*cosY
		result = NewVec3(x, result.Y, z)
	}

	// Rotation around Z axis
	if rotation.Z != 0 {
		cosZ := math.Cos(rotation.Z)
		sinZ := math.Sin(rotation.Z)
		x := result.X*cosZ - result.Y*sinZ
		y := result.X*sinZ + result.Y*cosZ
		result = NewVec3(x, y, result.Z)
	}

	return result
}

// Ray represents a ray with an origin and direction
type Ray struct {
	Origin    Vec3
	Direction Vec3
}

// NewRay creates a new ray
func NewRay(origin, direction Vec3) Ray {
	return Ray{Origin: origin, Direction: direction}
}

func NewRayTo(origin, target Vec3) Ray {
	return NewRay(origin, target.Subtract(origin).Normalize())
}

// At returns the point at parameter t along the ray
func (r Ray) At(t float64) Vec3 {
	return r.Origin.Add(r.Direction.Multiply(t))
}
