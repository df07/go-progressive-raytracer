package material

import (
	"math/rand"

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
func (l *Layered) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	// Step 1: Ray hits the outer material first
	outerHit := hit
	outerHit.Material = l.Outer

	outerResult, outerScatters := l.Outer.Scatter(rayIn, outerHit, random)

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

	innerResult, innerScatters := l.Inner.Scatter(innerRay, innerHit, random)

	if !innerScatters {
		// Inner material absorbs - return outer result only
		return outerResult, true
	}

	// Step 4: Combine both interactions
	// The final scattered ray comes from the inner material
	// The attenuation is the product of both materials (light filtered through both)
	combinedAttenuation := outerResult.Attenuation.MultiplyVec(innerResult.Attenuation)

	return core.ScatterResult{
		Scattered:   innerResult.Scattered,
		Attenuation: combinedAttenuation,
		PDF:         innerResult.PDF,
	}, true
}
