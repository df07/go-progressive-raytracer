package material

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Mix represents a material that probabilistically chooses between two materials
type Mix struct {
	Material1 Material
	Material2 Material
	Ratio     float64 // 0.0 = all material1, 1.0 = all material2
}

// NewMix creates a new mix material
func NewMix(material1, material2 Material, ratio float64) *Mix {
	// Clamp ratio to valid range
	ratio = math.Max(0.0, math.Min(ratio, 1.0))

	return &Mix{
		Material1: material1,
		Material2: material2,
		Ratio:     ratio,
	}
}

// Scatter implements the Material interface for mix material
func (m *Mix) Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool) {
	// Choose material based on ratio
	if sampler.Get1D() < m.Ratio {
		return m.Material2.Scatter(rayIn, hit, sampler)
	} else {
		return m.Material1.Scatter(rayIn, hit, sampler)
	}
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (m *Mix) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3 {
	// Combine BRDFs from both materials with appropriate weights
	brdf1 := m.Material1.EvaluateBRDF(incomingDir, outgoingDir, hit, mode)
	brdf2 := m.Material2.EvaluateBRDF(incomingDir, outgoingDir, hit, mode)
	return brdf1.Multiply(1.0 - m.Ratio).Add(brdf2.Multiply(m.Ratio))
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (m *Mix) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Combine PDFs from both materials with appropriate weights
	// Treat mix as finite PDF material even if it contains delta components
	pdf1, _ := m.Material1.PDF(incomingDir, outgoingDir, normal)
	pdf2, _ := m.Material2.PDF(incomingDir, outgoingDir, normal)
	combinedPDF := pdf1*(1.0-m.Ratio) + pdf2*m.Ratio
	return combinedPDF, false // Always treat mix as non-delta
}
