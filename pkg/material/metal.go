package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Metal represents a metallic material with specular reflection
type Metal struct {
	Albedo   core.Vec3 // Metal color
	Fuzzness float64   // 0.0 = perfect mirror, 1.0 = very fuzzy
}

// NewMetal creates a new metal material
func NewMetal(albedo core.Vec3, fuzzness float64) *Metal {
	// Clamp fuzzness to valid range
	if fuzzness > 1.0 {
		fuzzness = 1.0
	}
	if fuzzness < 0.0 {
		fuzzness = 0.0
	}
	return &Metal{Albedo: albedo, Fuzzness: fuzzness}
}

// Scatter implements the Material interface for metal scattering
func (m *Metal) Scatter(rayIn core.Ray, hit core.HitRecord, sampler core.Sampler) (core.ScatterResult, bool) {
	// Calculate perfect reflection direction
	reflected := reflect(rayIn.Direction.Normalize(), hit.Normal)

	// Add fuzziness by perturbing the reflection direction
	if m.Fuzzness > 0 {
		perturbation := core.RandomInUnitSphere(sampler.Get3D()).Multiply(m.Fuzzness)
		reflected = reflected.Add(perturbation)
	}

	scattered := core.Ray{Origin: hit.Point, Direction: reflected}

	// Only scatter if the ray is above the surface (not absorbed)
	scatters := scattered.Direction.Dot(hit.Normal) > 0

	return core.ScatterResult{
		Incoming:    rayIn,
		Scattered:   scattered,
		Attenuation: m.Albedo, // No π factor for specular
		PDF:         0,        // Specular materials have no PDF
	}, scatters
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (m *Metal) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	// Perfect reflection only - delta function
	reflected := reflect(incomingDir.Negate(), normal)

	// Check if outgoing direction matches perfect reflection (within tolerance)
	if outgoingDir.Subtract(reflected).Length() < 0.001 {
		return m.Albedo // Delta function contribution
	}

	return core.Vec3{X: 0, Y: 0, Z: 0} // No contribution for non-reflection directions
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (m *Metal) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// For specular materials: always return (0.0, true) indicating delta function
	// This is consistent with scatter.PDF = 0 and matches PBRT approach
	return 0.0, true
}

// reflect calculates the reflection of a vector v off a surface with normal n
func reflect(v, n core.Vec3) core.Vec3 {
	// r = v - 2*dot(v,n)*n
	return v.Subtract(n.Multiply(2 * v.Dot(n)))
}
