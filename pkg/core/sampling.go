package core

import (
	"math"
	"math/rand"
)

// Sampler provides random sampling for rendering algorithms
// Can be swapped out for deterministic testing or different sampling patterns
type Sampler interface {
	Get1D() float64
	Get2D() Vec2
	Get3D() Vec3
}

// RandomSampler wraps a standard Go random generator
type RandomSampler struct {
	random *rand.Rand
}

// NewRandomSampler creates a sampler from a Go random generator
func NewRandomSampler(random *rand.Rand) *RandomSampler {
	return &RandomSampler{random: random}
}

// Get1D returns a random float64 in [0, 1)
func (r *RandomSampler) Get1D() float64 {
	return r.random.Float64()
}

// Get2D returns two random float64 values in [0, 1)
func (r *RandomSampler) Get2D() Vec2 {
	return NewVec2(r.random.Float64(), r.random.Float64())
}

// Get3D returns three random float64 values in [0, 1)
func (r *RandomSampler) Get3D() Vec3 {
	return NewVec3(r.random.Float64(), r.random.Float64(), r.random.Float64())
}

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
func SampleLight(lights []Light, point Vec3, sampler Sampler) (LightSample, Light, bool) {
	if len(lights) == 0 {
		return LightSample{}, nil, false
	}

	sampledLight := lights[int(sampler.Get1D()*float64(len(lights)))]
	sample := sampledLight.Sample(point, sampler.Get2D())
	sample.PDF *= 1.0 / float64(len(lights))

	return sample, sampledLight, true
}

// SampleLightEmission randomly selects and samples emission from a light in the scene
func SampleLightEmission(lights []Light, sampler Sampler) (EmissionSample, bool) {
	if len(lights) == 0 {
		return EmissionSample{}, false
	}

	sampledLight := lights[int(sampler.Get1D()*float64(len(lights)))]
	sample := sampledLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
	// Apply light selection probability to area PDF only (combined effect when multiplied)
	lightSelectionProb := 1.0 / float64(len(lights))
	sample.AreaPDF *= lightSelectionProb
	// Don't modify DirectionPDF - it's independent of light selection

	return sample, true
}

// SampleEmissionDirection samples a cosine-weighted emission direction from a surface
// and returns both the direction and the emission sample with separate area and direction PDFs
func SampleEmissionDirection(point Vec3, normal Vec3, areaPDF float64, material Material, sample Vec2) EmissionSample {
	// Sample emission direction (cosine-weighted hemisphere)
	emissionDir := RandomCosineDirection(normal, sample)

	// Calculate direction PDF separately (cosine-weighted)
	cosTheta := emissionDir.Dot(normal)
	directionPDF := cosTheta / math.Pi

	// Get emission from material
	var emission Vec3
	if emitter, ok := material.(Emitter); ok {
		emission = emitter.Emit(NewRay(point, emissionDir))
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
func SampleUniformCone(direction Vec3, cosTotalWidth float64, sample Vec2) Vec3 {
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
	cosTheta := 1.0 - sample.X*(1.0-cosTotalWidth)
	sinTheta := math.Sqrt(math.Max(0, 1.0-cosTheta*cosTheta))
	phi := 2.0 * math.Pi * sample.Y

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
