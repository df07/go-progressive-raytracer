package core

import (
	"math"
	"math/rand"
)

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

// LengthSquared returns the squared magnitude of the vector
func (v Vec3) LengthSquared() float64 {
	return v.X*v.X + v.Y*v.Y + v.Z*v.Z
}

// Dot returns the dot product of two vectors
func (v Vec3) Dot(other Vec3) float64 {
	return v.X*other.X + v.Y*other.Y + v.Z*other.Z
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

// Ray represents a ray with an origin and direction
type Ray struct {
	Origin    Vec3
	Direction Vec3
}

// NewRay creates a new ray
func NewRay(origin, direction Vec3) Ray {
	return Ray{Origin: origin, Direction: direction}
}

// At returns the point at parameter t along the ray
func (r Ray) At(t float64) Vec3 {
	return r.Origin.Add(r.Direction.Multiply(t))
}

// RandomCosineDirection generates a cosine-weighted random direction in hemisphere around normal
func RandomCosineDirection(normal Vec3, random *rand.Rand) Vec3 {
	// Generate point in unit disk using uniform random sampling
	a := 2.0 * math.Pi * random.Float64()
	z := random.Float64()
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

// RandomInUnitDisk generates a random point in a unit disk (for depth of field)
func RandomInUnitDisk(random *rand.Rand) Vec3 {
	for {
		// Generate random point in [-1,1] x [-1,1] square
		p := NewVec3(2*random.Float64()-1, 2*random.Float64()-1, 0)
		// Accept if inside unit disk
		if p.Dot(p) <= 1.0 {
			return p
		}
	}
}
