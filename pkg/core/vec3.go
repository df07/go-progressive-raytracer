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
// Uses standard luminance weights: 0.299*R + 0.587*G + 0.114*B
func (v Vec3) Luminance() float64 {
	return 0.299*v.X + 0.587*v.Y + 0.114*v.Z
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

// RandomCosineDirection generates a cosine-weighted random direction in hemisphere around normal
func RandomCosineDirection(normal Vec3, sample Vec2) Vec3 {
	// Generate point in unit disk using uniform random sampling
	a := 2.0 * math.Pi * sample.X
	z := sample.Y
	r := math.Sqrt(z)

	x := r * math.Cos(a)
	y := r * math.Sin(a)
	zCoord := math.Sqrt(1.0 - z)

	// Create local coordinate system around normal
	// Find a vector perpendicular to normal
	var nt Vec3
	if math.Abs(normal.X) > 0.1 {
		nt = NewVec3(0, 1, 0)
	} else {
		nt = NewVec3(1, 0, 0)
	}

	// Create orthonormal basis
	tangent := nt.Cross(normal).Normalize()
	bitangent := normal.Cross(tangent)

	// Transform to world space
	return tangent.Multiply(x).Add(bitangent.Multiply(y)).Add(normal.Multiply(zCoord))
}

// RandomInUnitDisk generates a random point in a unit disk using concentric mapping
// This avoids rejection sampling by mapping a square uniformly to a disk
func RandomInUnitDisk(sample Vec2) Vec3 {
	// Map sample to [-1,1]² and handle degeneracy at the origin
	uOffset := NewVec2(2*sample.X-1, 2*sample.Y-1)
	if uOffset.X == 0 && uOffset.Y == 0 {
		return NewVec3(0, 0, 0)
	}

	// Apply concentric mapping to point
	var theta, r float64
	if math.Abs(uOffset.X) > math.Abs(uOffset.Y) {
		r = uOffset.X
		theta = math.Pi / 4 * (uOffset.Y / uOffset.X)
	} else {
		r = uOffset.Y
		theta = math.Pi/2 - math.Pi/4*(uOffset.X/uOffset.Y)
	}

	return NewVec3(r*math.Cos(theta), r*math.Sin(theta), 0)
}

// RandomInUnitSphere generates a random point inside a unit sphere using spherical coordinates
// This avoids rejection sampling by using the inverse CDF method
func RandomInUnitSphere(sample Vec3) Vec3 {
	// For uniform distribution inside sphere:
	// r = ∛(u₁) to account for volume scaling
	// φ = 2π * u₂ (azimuthal angle)
	// cos(θ) = 2 * u₃ - 1 (polar angle, uniform on [-1,1])

	r := math.Pow(sample.X, 1.0/3.0)
	phi := 2 * math.Pi * sample.Y
	cosTheta := 2*sample.Z - 1
	sinTheta := math.Sqrt(1 - cosTheta*cosTheta)

	// Convert spherical to Cartesian coordinates
	x := r * sinTheta * math.Cos(phi)
	y := r * sinTheta * math.Sin(phi)
	z := r * cosTheta

	return NewVec3(x, y, z)
}
