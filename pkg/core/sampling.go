package core

import (
	"math"
	"math/rand"
)

// PowerHeuristic implements the power heuristic for multiple importance sampling
// This balances between two sampling strategies (typically light sampling vs material sampling)
func PowerHeuristic(nf int, fPdf float64, ng int, gPdf float64) float64 {
	if fPdf == 0 {
		return 0
	}

	f := float64(nf) * fPdf
	g := float64(ng) * gPdf

	// Power heuristic with β = 2 (squared)
	return (f * f) / (f*f + g*g)
}

// BalanceHeuristic implements the balance heuristic for multiple importance sampling
func BalanceHeuristic(nf int, fPdf float64, ng int, gPdf float64) float64 {
	if fPdf == 0 {
		return 0
	}

	f := float64(nf) * fPdf
	g := float64(ng) * gPdf

	return f / (f + g)
}

// CombinePDFs combines light and material PDFs using multiple importance sampling
// Returns the MIS weight for the light sample
func CombinePDFs(lightPdf, materialPdf float64, usePowerHeuristic bool) float64 {
	if lightPdf == 0 {
		return 0
	}

	if usePowerHeuristic {
		return PowerHeuristic(1, lightPdf, 1, materialPdf)
	} else {
		return BalanceHeuristic(1, lightPdf, 1, materialPdf)
	}
}

// SphereUniformPDF returns the PDF for uniform sampling on a sphere
func SphereUniformPDF(radius float64) float64 {
	return 1.0 / (4.0 * math.Pi * radius * radius)
}

// SphereConePDF returns the PDF for sampling a sphere from a point using cone sampling
func SphereConePDF(distance, radius float64) float64 {
	if distance <= radius {
		// Point is inside sphere, use uniform sampling
		return SphereUniformPDF(radius)
	}

	sinThetaMax := radius / distance
	cosThetaMax := math.Sqrt(math.Max(0, 1.0-sinThetaMax*sinThetaMax))

	return 1.0 / (2.0 * math.Pi * (1.0 - cosThetaMax))
}

// CalculateLightPDF calculates the combined PDF for a given direction toward multiple lights
func CalculateLightPDF(lights []Light, point Vec3, direction Vec3) float64 {
	if len(lights) == 0 {
		return 0.0
	}

	totalPDF := 0.0
	for _, light := range lights {
		lightPDF := light.PDF(point, direction)
		// Weight by light selection probability (uniform selection)
		totalPDF += lightPDF / float64(len(lights))
	}

	return totalPDF
}

// SampleLight randomly selects and samples a light from the scene
func SampleLight(lights []Light, point Vec3, random *rand.Rand) (LightSample, bool) {
	if len(lights) == 0 {
		return LightSample{}, false
	}

	sampledLight := lights[random.Intn(len(lights))]
	sample := sampledLight.Sample(point, random)
	sample.PDF *= 1.0 / float64(len(lights))

	return sample, true
}
