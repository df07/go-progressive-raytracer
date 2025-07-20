package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Layered represents a material with two layers - an outer and inner material
// Light hits the outer layer first, then if it scatters inward, hits the inner layer
// This simulates coatings, films, or other layered surface treatments
type Layered struct {
	Outer core.Material // Outer layer material (e.g., coating, surface treatment)
	Inner core.Material // Inner layer material (e.g., base material)
}

// NewLayered creates a new layered material
func NewLayered(outer, inner core.Material) *Layered {
	return &Layered{
		Outer: outer,
		Inner: inner,
	}
}

// Scatter implements the Material interface for layered scattering
func (l *Layered) Scatter(rayIn core.Ray, hit core.HitRecord, sampler core.Sampler) (core.ScatterResult, bool) {
	// Step 1: Ray hits the outer material first
	outerHit := hit
	outerHit.Material = l.Outer

	outerResult, outerScatters := l.Outer.Scatter(rayIn, outerHit, sampler)

	// If outer material doesn't scatter, no interaction occurs
	if !outerScatters {
		return core.ScatterResult{}, false
	}

	// Step 2: Check if the scattered ray from outer material points inward
	// (toward the surface, indicating it penetrates to the inner layer)
	scatteredDirection := outerResult.Scattered.Direction.Normalize()
	pointsInward := scatteredDirection.Dot(hit.Normal) < 0

	if !pointsInward {
		// Ray scattered outward from outer layer - only outer interaction
		return outerResult, true
	}

	// Step 3: Ray points inward - it hits the inner material at the same point
	// but with the new direction from the outer material's scattering
	innerRay := core.Ray{
		Origin:    hit.Point,
		Direction: scatteredDirection,
	}

	innerHit := hit
	innerHit.Material = l.Inner

	innerResult, innerScatters := l.Inner.Scatter(innerRay, innerHit, sampler)

	if !innerScatters {
		// Inner material absorbs - return outer result only
		return outerResult, true
	}

	// Step 4: Combine both interactions
	// The final scattered ray comes from the inner material
	// The attenuation is the product of both materials (light filtered through both)
	combinedAttenuation := outerResult.Attenuation.MultiplyVec(innerResult.Attenuation)

	return core.ScatterResult{
		Incoming:    rayIn,
		Scattered:   innerResult.Scattered,
		Attenuation: combinedAttenuation,
		PDF:         innerResult.PDF,
	}, true
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (l *Layered) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	// Our layered material models two-step scattering:
	// 1. Outer material scatters first
	// 2. If outer scatters inward, inner material scatters (ignoring outer on exit)
	//
	// Assumption: outer material is always dielectric (can reflect or transmit)
	// We can check if the incoming/outgoing pair represents reflection or transmission

	// Check if this looks like a reflection from the outer dielectric
	if isReflectionPath(incomingDir, outgoingDir, normal) {
		return l.Outer.EvaluateBRDF(incomingDir, outgoingDir, normal)
	}

	// Transmission path - inner material PDF
	return l.Inner.EvaluateBRDF(incomingDir, outgoingDir, normal)
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (l *Layered) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Same logic as BRDF: either reflection from outer or transmission + inner scattering
	// Treat layered as finite PDF material even if components are delta
	if isReflectionPath(incomingDir, outgoingDir, normal) {
		pdf, _ := l.Outer.PDF(incomingDir, outgoingDir, normal)
		return pdf, false // Always treat layered as non-delta
	}

	// Transmission path - inner material PDF
	pdf, _ := l.Inner.PDF(incomingDir, outgoingDir, normal)
	return pdf, false // Always treat layered as non-delta
}

// isReflectionPath checks if the incoming/outgoing direction pair represents
// a reflection off the outer surface (assuming outer is dielectric)
func isReflectionPath(incomingDir, outgoingDir, normal core.Vec3) bool {
	// Calculate perfect reflection direction
	// Note: incomingDir points toward surface, so negate it for reflection calculation
	incidentDir := incomingDir.Negate()
	reflectedDir := incidentDir.Subtract(normal.Multiply(2 * incidentDir.Dot(normal)))

	// Check if outgoing direction is close to perfect reflection
	tolerance := 0.1 // Adjust as needed
	return outgoingDir.Subtract(reflectedDir).Length() < tolerance
}
