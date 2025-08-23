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
func CalculateLightPDF(scene Scene, point, normal, direction Vec3) float64 {
	lights := scene.GetLights()
	if len(lights) == 0 {
		return 0.0
	}

	lightSampler := scene.GetLightSampler()
	totalPDF := 0.0

	// For each light, calculate the PDF weighted by its selection probability
	for i, light := range lights {
		lightPDF := light.PDF(point, normal, direction)
		lightSelectionPdf := lightSampler.GetLightProbability(i, point, normal)
		totalPDF += lightPDF * lightSelectionPdf
	}

	return totalPDF
}

// SampleLight selects and samples a light from the scene using importance sampling
func SampleLight(scene Scene, point Vec3, normal Vec3, sampler Sampler) (LightSample, Light, bool) {
	lights := scene.GetLights()
	if len(lights) == 0 {
		return LightSample{}, nil, false
	}

	// Use importance-based light sampler that considers surface point and normal
	lightSampler := scene.GetLightSampler()
	selectedLight, lightSelectionPdf, _ := lightSampler.SampleLight(point, normal, sampler.Get1D())

	sample := selectedLight.Sample(point, normal, sampler.Get2D())
	sample.PDF *= lightSelectionPdf // Combined PDF for MIS calculations

	return sample, selectedLight, true
}

// SampleLightEmission selects and samples emission from a light using uniform sampling
// For emission sampling, we don't have a specific surface point, so use uniform distribution
func SampleLightEmission(scene Scene, sampler Sampler) (EmissionSample, bool) {
	lights := scene.GetLights()
	if len(lights) == 0 {
		return EmissionSample{}, false
	}

	// Use uniform sampling for emission since we don't have a specific surface point
	lightSampler := scene.GetLightSampler()
	selectedLight, lightSelectionPdf, _ := lightSampler.SampleLightEmission(sampler.Get1D())

	sample := selectedLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
	// Apply light selection probability to area PDF only (combined effect when multiplied)
	sample.AreaPDF *= lightSelectionPdf
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

// SampleUniformSphere generates a uniform random direction on the unit sphere
func SampleUniformSphere(sample Vec2) Vec3 {
	z := 1.0 - 2.0*sample.X // z ∈ [-1, 1]
	r := math.Sqrt(math.Max(0, 1.0-z*z))
	phi := 2.0 * math.Pi * sample.Y
	x := r * math.Cos(phi)
	y := r * math.Sin(phi)
	return NewVec3(x, y, z)
}

// SampleInfiniteLight samples emission from an infinite light using PBRT's disk sampling approach
// Returns emission ray, area PDF, and direction PDF
func SampleInfiniteLight(worldCenter Vec3, worldRadius float64, samplePoint Vec2, sampleDirection Vec2) (Ray, float64, float64) {
	// Sample direction uniformly on sphere
	direction := SampleUniformSphere(sampleDirection)

	// Create orthonormal basis with direction as one axis
	var up Vec3
	if math.Abs(direction.X) > 0.9 {
		up = NewVec3(0, 1, 0)
	} else {
		up = NewVec3(1, 0, 0)
	}
	right := direction.Cross(up).Normalize()
	up = right.Cross(direction).Normalize()

	// Sample point on unit disk, then scale by world radius
	diskSample := RandomInUnitDisk(samplePoint)
	diskPoint := worldCenter.Add(right.Multiply(diskSample.X * worldRadius)).Add(up.Multiply(diskSample.Y * worldRadius))

	// Emission point is behind the disk, ray travels in sampled direction (parallel rays)
	emissionPoint := diskPoint.Add(direction.Multiply(-worldRadius))

	// PBRT PDF calculations
	areaPDF := 1.0 / (math.Pi * worldRadius * worldRadius) // Planar density
	directionPDF := 1.0 / (4.0 * math.Pi)                  // Uniform over sphere

	return NewRay(emissionPoint, direction), areaPDF, directionPDF
}
