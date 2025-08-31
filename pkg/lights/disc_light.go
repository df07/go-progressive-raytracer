package lights

import (
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// DiscLight represents a circular area light
type DiscLight struct {
	*geometry.Disc // Embed disc for hit testing
}

// NewDiscLight creates a new circular disc light
func NewDiscLight(center, normal core.Vec3, radius float64, material material.Material) *DiscLight {
	return &DiscLight{
		Disc: geometry.NewDisc(center, normal, radius, material),
	}
}

func (dl *DiscLight) Type() LightType {
	return LightTypeArea
}

// Sample implements the Light interface - samples a point on the disc for direct lighting
func (dl *DiscLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	// Sample a point on the disc
	samplePoint, normal := dl.Disc.SampleUniform(sample)

	// Calculate direction and distance
	direction := samplePoint.Subtract(point)
	distance := direction.Length()
	dirNormalized := direction.Normalize()

	// Check for degenerate case
	if distance == 0 {
		return LightSample{
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

	// Get emission from this light
	emission := dl.Emit(core.NewRay(point, dirNormalized))

	return LightSample{
		Point:     samplePoint,
		Normal:    normal,
		Direction: dirNormalized,
		Distance:  distance,
		Emission:  emission,
		PDF:       pdf,
	}
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
func (dl *DiscLight) PDF(point, normal, direction core.Vec3) float64 {
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
func (dl *DiscLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	// Sample point uniformly on disc surface
	point, normal := dl.Disc.SampleUniform(samplePoint)

	// Use shared emission sampling function
	areaPDF := 1.0 / (math.Pi * dl.Radius * dl.Radius)
	return SampleEmissionDirection(point, normal, areaPDF, dl.Material, sampleDirection)
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

// Emit implements the Light interface - returns material emission
func (dl *DiscLight) Emit(ray core.Ray) core.Vec3 {
	// Area lights emit according to their material
	if emitter, isEmissive := dl.Material.(material.Emitter); isEmissive {
		return emitter.Emit(ray)
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}
