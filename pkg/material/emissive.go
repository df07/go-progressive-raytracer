package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Emissive represents a light-emitting material
type Emissive struct {
	Emission core.Vec3 // Emitted light color/intensity
}

// NewEmissive creates a new emissive material
func NewEmissive(emission core.Vec3) *Emissive {
	return &Emissive{Emission: emission}
}

// Scatter implements the Material interface for emissive materials
// Emissive materials don't scatter rays - they only emit light
func (e *Emissive) Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool) {
	// Emissive materials don't scatter - they absorb all incoming rays
	return ScatterResult{}, false
}

// Emit returns the emitted light for this material
func (e *Emissive) Emit(rayIn core.Ray) core.Vec3 {
	return e.Emission
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (e *Emissive) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3 {
	// Lights don't reflect - they only emit
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (e *Emissive) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Emissive materials don't scatter, so PDF is always 0
	return 0.0, false // Not a delta function, just no scattering
}
