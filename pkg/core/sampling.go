package core

import (
	"math"
	"math/rand"
)

// Sampler provides random sampling for rendering algorithms
// Can be swapped out for deterministic testing or different sampling patterns
type Sampler interface {
	Get1D() float64
	Get2D() Vec2
	Get3D() Vec3
}

// RandomSampler wraps a standard Go random generator
type RandomSampler struct {
	random *rand.Rand
}

// NewRandomSampler creates a sampler from a Go random generator
func NewRandomSampler(random *rand.Rand) *RandomSampler {
	return &RandomSampler{random: random}
}

// Get1D returns a random float64 in [0, 1)
func (r *RandomSampler) Get1D() float64 {
	return r.random.Float64()
}

// Get2D returns two random float64 values in [0, 1)
func (r *RandomSampler) Get2D() Vec2 {
	return NewVec2(r.random.Float64(), r.random.Float64())
}

// Get3D returns three random float64 values in [0, 1)
func (r *RandomSampler) Get3D() Vec3 {
	return NewVec3(r.random.Float64(), r.random.Float64(), r.random.Float64())
}

// SampleCosineHemisphere generates a cosine-weighted random direction in hemisphere around normal
func SampleCosineHemisphere(normal Vec3, sample Vec2) Vec3 {
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

// SampleCone samples a direction uniformly within a cone
func SampleCone(direction Vec3, cosTotalWidth float64, sample Vec2) Vec3 {
	// Create coordinate system with z-axis pointing in cone direction
	w := direction
	var u Vec3
	if math.Abs(w.X) > 0.1 {
		u = NewVec3(0, 1, 0)
	} else {
		u = NewVec3(1, 0, 0)
	}
	u = u.Cross(w).Normalize()
	v := w.Cross(u)

	// Sample direction within the cone
	cosTheta := 1.0 - sample.X*(1.0-cosTotalWidth)
	sinTheta := math.Sqrt(math.Max(0, 1.0-cosTheta*cosTheta))
	phi := 2.0 * math.Pi * sample.Y

	// Convert to Cartesian coordinates in local space
	x := sinTheta * math.Cos(phi)
	y := sinTheta * math.Sin(phi)
	z := cosTheta

	// Transform to world space
	return u.Multiply(x).Add(v.Multiply(y)).Add(w.Multiply(z))
}

// SampleOnUnitSphere generates a uniform random direction on the unit sphere
func SampleOnUnitSphere(sample Vec2) Vec3 {
	z := 1.0 - 2.0*sample.X // z ∈ [-1, 1]
	r := math.Sqrt(math.Max(0, 1.0-z*z))
	phi := 2.0 * math.Pi * sample.Y
	x := r * math.Cos(phi)
	y := r * math.Sin(phi)
	return NewVec3(x, y, z)
}

// SamplePointInUnitDisk generates a random point in a unit disk using concentric mapping
// This avoids rejection sampling by mapping a square uniformly to a disk
func SamplePointInUnitDisk(sample Vec2) Vec3 {
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

// SamplePointInUnitSphere generates a random point inside a unit sphere using spherical coordinates
// This avoids rejection sampling by using the inverse CDF method
func SamplePointInUnitSphere(sample Vec3) Vec3 {
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
