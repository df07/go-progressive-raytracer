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

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (m *Mix) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	// Combine BRDFs from both materials with appropriate weights
	brdf1 := m.Material1.EvaluateBRDF(incomingDir, outgoingDir, normal)
	brdf2 := m.Material2.EvaluateBRDF(incomingDir, outgoingDir, normal)
	return brdf1.Multiply(1.0 - m.Ratio).Add(brdf2.Multiply(m.Ratio))
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (m *Mix) PDF(incomingDir, outgoingDir, normal core.Vec3) float64 {
	// Combine PDFs from both materials with appropriate weights
	pdf1 := m.Material1.PDF(incomingDir, outgoingDir, normal)
	pdf2 := m.Material2.PDF(incomingDir, outgoingDir, normal)
	return pdf1*(1.0-m.Ratio) + pdf2*m.Ratio
}
