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

// SampleLightEmission randomly selects and samples emission from a light in the scene
func SampleLightEmission(lights []Light, random *rand.Rand) (EmissionSample, bool) {
	if len(lights) == 0 {
		return EmissionSample{}, false
	}

	sampledLight := lights[random.Intn(len(lights))]
	sample := sampledLight.SampleEmission(random)
	// Apply light selection probability to area PDF only (combined effect when multiplied)
	lightSelectionProb := 1.0 / float64(len(lights))
	sample.AreaPDF *= lightSelectionProb
	// Don't modify DirectionPDF - it's independent of light selection

	return sample, true
}

// SampleEmissionDirection samples a cosine-weighted emission direction from a surface
// and returns both the direction and the emission sample with separate area and direction PDFs
func SampleEmissionDirection(point Vec3, normal Vec3, areaPDF float64, material Material, random *rand.Rand) EmissionSample {
	// Sample emission direction (cosine-weighted hemisphere)
	emissionDir := RandomCosineDirection(normal, random)

	// Calculate direction PDF separately (cosine-weighted)
	cosTheta := emissionDir.Dot(normal)
	directionPDF := cosTheta / math.Pi

	// Get emission from material
	var emission Vec3
	if emitter, ok := material.(Emitter); ok {
		dummyRay := NewRay(point, emissionDir)
		dummyHit := HitRecord{Point: point, Normal: normal, Material: material}
		emission = emitter.Emit(dummyRay, dummyHit)
	}

	return EmissionSample{
		Point:        point,
		Normal:       normal,
		Direction:    emissionDir,
		Emission:     emission,
		AreaPDF:      areaPDF,
		DirectionPDF: directionPDF,
	}
}

// UniformConePDF calculates the PDF for uniform sampling within a cone
func UniformConePDF(cosTotalWidth float64) float64 {
	return 1.0 / (2.0 * math.Pi * (1.0 - cosTotalWidth))
}

// SampleUniformCone samples a direction uniformly within a cone
func SampleUniformCone(direction Vec3, cosTotalWidth float64, random *rand.Rand) Vec3 {
	// Create coordinate system with z-axis pointing in cone direction
	w := direction
	var u Vec3
	if math.Abs(w.X) > 0.1 {
		u = NewVec3(0, 1, 0)
	} else {
		u = NewVec3(1, 0, 0)
	}
	u = u.Cross(w).Normalize()
	v := w.Cross(u)

	// Sample direction within the cone
	cosTheta := 1.0 - random.Float64()*(1.0-cosTotalWidth)
	sinTheta := math.Sqrt(math.Max(0, 1.0-cosTheta*cosTheta))
	phi := 2.0 * math.Pi * random.Float64()

	// Convert to Cartesian coordinates in local space
	x := sinTheta * math.Cos(phi)
	y := sinTheta * math.Sin(phi)
	z := cosTheta

	// Transform to world space
	return u.Multiply(x).Add(v.Multiply(y)).Add(w.Multiply(z))
}

// ValidatePointOnSphere checks if a point lies on a sphere surface within tolerance
func ValidatePointOnSphere(point Vec3, center Vec3, radius float64, tolerance float64) bool {
	distFromCenter := point.Subtract(center).Length()
	return math.Abs(distFromCenter-radius) <= tolerance
}

// ValidatePointOnDisc checks if a point lies on a disc surface within tolerance
func ValidatePointOnDisc(point Vec3, center Vec3, normal Vec3, radius float64, tolerance float64) bool {
	toPoint := point.Subtract(center)

	// Check distance to plane
	distanceToPlane := math.Abs(toPoint.Dot(normal))
	if distanceToPlane > tolerance {
		return false
	}

	// Check if within disc radius
	projectedPoint := toPoint.Subtract(normal.Multiply(toPoint.Dot(normal)))
	return projectedPoint.Length() <= radius
}

// ValidateDirectionInHemisphere checks if a direction is in the correct hemisphere
func ValidateDirectionInHemisphere(direction Vec3, normal Vec3) bool {
	return direction.Dot(normal) > 0
}
