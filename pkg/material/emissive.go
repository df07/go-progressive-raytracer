package material

import (
	"math/rand"

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
func (e *Emissive) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	// Emissive materials don't scatter - they absorb all incoming rays
	return core.ScatterResult{}, false
}

// Emit returns the emitted light for this material
func (e *Emissive) Emit(rayIn core.Ray, hit core.HitRecord) core.Vec3 {
	return e.Emission
}
