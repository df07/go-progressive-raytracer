package material

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Mix represents a material that probabilistically chooses between two materials
type Mix struct {
	Material1 core.Material
	Material2 core.Material
	Ratio     float64 // 0.0 = all material1, 1.0 = all material2
}

// NewMix creates a new mix material
func NewMix(material1, material2 core.Material, ratio float64) *Mix {
	// Clamp ratio to valid range
	ratio = math.Max(0.0, math.Min(ratio, 1.0))

	return &Mix{
		Material1: material1,
		Material2: material2,
		Ratio:     ratio,
	}
}

// Scatter implements the Material interface for mix material
func (m *Mix) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	// Choose material based on ratio
	if random.Float64() < m.Ratio {
		return m.Material2.Scatter(rayIn, hit, random)
	} else {
		return m.Material1.Scatter(rayIn, hit, random)
	}
}
