package material

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Lambertian represents a perfectly diffuse material
type Lambertian struct {
	Albedo ColorSource // Base color/reflectance (can be solid or textured)
}

// NewLambertian creates a new lambertian material with solid color (backward compatibility)
func NewLambertian(albedo core.Vec3) *Lambertian {
	return &Lambertian{Albedo: NewSolidColor(albedo)}
}

// NewTexturedLambertian creates a new lambertian material with texture
func NewTexturedLambertian(albedoTexture ColorSource) *Lambertian {
	return &Lambertian{Albedo: albedoTexture}
}

// Scatter implements the Material interface for lambertian scattering
func (l *Lambertian) Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool) {
	// Generate cosine-weighted random direction in hemisphere around normal
	scatterDirection := core.SampleCosineHemisphere(hit.Normal, sampler.Get2D())
	scattered := core.Ray{Origin: hit.Point, Direction: scatterDirection}

	// Calculate PDF: cos(θ) / π where θ is angle from normal
	cosTheta := scatterDirection.Normalize().Dot(hit.Normal)
	if cosTheta < 0 {
		cosTheta = 0 // Clamp to avoid negative values
	}
	pdf := cosTheta / math.Pi

	// Sample texture at UV coordinates to get albedo
	albedo := l.Albedo.Evaluate(hit.UV, hit.Point)

	// BRDF: albedo / π (proper energy conservation)
	attenuation := albedo.Multiply(1.0 / math.Pi)

	return ScatterResult{
		Incoming:    rayIn,
		Scattered:   scattered,
		Attenuation: attenuation,
		PDF:         pdf,
	}, true
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (l *Lambertian) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3 {
	// Lambertian BRDF is constant: albedo / π
	cosTheta := outgoingDir.Dot(hit.Normal)
	if cosTheta <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Below surface
	}

	// Sample texture at UV coordinates to get albedo
	albedo := l.Albedo.Evaluate(hit.UV, hit.Point)
	return albedo.Multiply(1.0 / math.Pi)
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (l *Lambertian) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Cosine-weighted hemisphere sampling: cos(θ) / π
	cosTheta := outgoingDir.Dot(normal)
	if cosTheta <= 0 {
		return 0.0, false
	}
	return cosTheta / math.Pi, false // Not a delta function
}
