package geometry

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// DiscLight represents a circular area light
type DiscLight struct {
	*Disc // Embed disc for hit testing
}

// NewDiscLight creates a new circular disc light
func NewDiscLight(center, normal core.Vec3, radius float64, material core.Material) *DiscLight {
	return &DiscLight{
		Disc: NewDisc(center, normal, radius, material),
	}
}

// Sample implements the Light interface - samples a point on the disc for direct lighting
func (dl *DiscLight) Sample(point core.Vec3, random *rand.Rand) core.LightSample {
	// Sample a point on the disc
	samplePoint, normal := dl.Disc.SampleUniform(random)

	// Calculate direction and distance
	direction := samplePoint.Subtract(point)
	distance := direction.Length()
	dirNormalized := direction.Normalize()

	// Check for degenerate case
	if distance == 0 {
		return core.LightSample{
			Point:     samplePoint,
			Normal:    normal,
			Direction: core.NewVec3(0, 1, 0),
			Distance:  0,
			Emission:  core.NewVec3(0, 0, 0),
			PDF:       1.0,
		}
	}

	// Calculate PDF
	// For uniform sampling on disc: PDF = 1 / (π * radius²)
	pdf := 1.0 / (math.Pi * dl.Radius * dl.Radius)

	// Convert to solid angle PDF
	cosTheta := math.Abs(normal.Dot(dirNormalized.Multiply(-1)))
	if cosTheta < 1e-6 {
		// Grazing angle, very low probability
		pdf = 0.0
	} else {
		solidAnglePDF := pdf * distance * distance / cosTheta
		pdf = solidAnglePDF
	}

	// Get emission from material if it's an emitter
	var emission core.Vec3
	if emitter, ok := dl.Material.(core.Emitter); ok {
		// Create dummy ray and hit record for emission calculation
		dummyRay := core.NewRay(point, dirNormalized)
		dummyHit := core.HitRecord{
			Point:    samplePoint,
			Normal:   normal,
			Material: dl.Material,
		}
		emission = emitter.Emit(dummyRay, dummyHit)
	}

	return core.LightSample{
		Point:     samplePoint,
		Normal:    normal,
		Direction: dirNormalized,
		Distance:  distance,
		Emission:  emission,
		PDF:       pdf,
	}
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
func (dl *DiscLight) PDF(point core.Vec3, direction core.Vec3) float64 {
	// Check if ray from point in direction hits the disc
	ray := core.NewRay(point, direction)
	hitRecord, hit := dl.Disc.Hit(ray, 0.001, math.Inf(1))
	if !hit {
		return 0.0
	}

	// Calculate solid angle PDF
	// First get the area PDF
	areaPDF := 1.0 / (math.Pi * dl.Radius * dl.Radius)

	// Convert to solid angle using the actual hit point
	distance := hitRecord.T
	cosTheta := math.Abs(dl.Normal.Dot(direction.Multiply(-1)))

	if cosTheta < 1e-6 {
		return 0.0
	}

	return areaPDF * distance * distance / cosTheta
}

// SampleEmission implements the Light interface - samples emission from the disc surface
func (dl *DiscLight) SampleEmission(random *rand.Rand) core.EmissionSample {
	// Sample point uniformly on disc surface
	samplePoint, normal := dl.Disc.SampleUniform(random)

	// Use shared emission sampling function
	areaPDF := 1.0 / (math.Pi * dl.Radius * dl.Radius)
	return core.SampleEmissionDirection(samplePoint, normal, areaPDF, dl.Material, random)
}

// EmissionPDF implements the Light interface - calculates PDF for emission sampling
func (dl *DiscLight) EmissionPDF(point core.Vec3, direction core.Vec3) float64 {
	// Validate point is on disc surface
	if !core.ValidatePointOnDisc(point, dl.Center, dl.Normal, dl.Radius, 0.001) {
		return 0.0
	}

	// Check if direction is in correct hemisphere
	if direction.Dot(dl.Normal) <= 0 {
		return 0.0
	}

	// Return area PDF only (direction PDF handled separately in new interface)
	areaPDF := 1.0 / (math.Pi * dl.Radius * dl.Radius)
	return areaPDF
}
