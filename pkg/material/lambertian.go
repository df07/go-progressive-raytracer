package material

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Lambertian represents a perfectly diffuse material
type Lambertian struct {
	Albedo core.Vec3 // Base color/reflectance
}

// NewLambertian creates a new lambertian material
func NewLambertian(albedo core.Vec3) *Lambertian {
	return &Lambertian{Albedo: albedo}
}

// Scatter implements the Material interface for lambertian scattering
func (l *Lambertian) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	// Generate cosine-weighted random direction in hemisphere around normal
	scatterDirection := core.RandomCosineDirection(hit.Normal, random)
	scattered := core.Ray{Origin: hit.Point, Direction: scatterDirection}

	// Calculate PDF: cos(θ) / π where θ is angle from normal
	cosTheta := scatterDirection.Normalize().Dot(hit.Normal)
	if cosTheta < 0 {
		cosTheta = 0 // Clamp to avoid negative values
	}
	pdf := cosTheta / math.Pi

	// BRDF: albedo / π (proper energy conservation)
	attenuation := l.Albedo.Multiply(1.0 / math.Pi)

	return core.ScatterResult{
		Scattered:   scattered,
		Attenuation: attenuation,
		PDF:         pdf,
	}, true
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (l *Lambertian) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	// Lambertian BRDF is constant: albedo / π
	cosTheta := outgoingDir.Dot(normal)
	if cosTheta <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Below surface
	}
	return l.Albedo.Multiply(1.0 / math.Pi)
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (l *Lambertian) PDF(incomingDir, outgoingDir, normal core.Vec3) float64 {
	// Cosine-weighted hemisphere sampling: cos(θ) / π
	cosTheta := outgoingDir.Dot(normal)
	if cosTheta <= 0 {
		return 0.0
	}
	return cosTheta / math.Pi
}
