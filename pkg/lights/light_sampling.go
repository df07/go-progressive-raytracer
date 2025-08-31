package lights

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// CalculateLightPDF calculates the combined PDF for a given direction toward multiple lights
func CalculateLightPDF(lights []Light, lightSampler LightSampler, point, normal, direction core.Vec3) float64 {
	if len(lights) == 0 {
		return 0.0
	}
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
func SampleLight(lights []Light, lightSampler LightSampler, point core.Vec3, normal core.Vec3, sampler core.Sampler) (LightSample, Light, bool) {
	if len(lights) == 0 {
		return LightSample{}, nil, false
	}
	selectedLight, lightSelectionPdf, _ := lightSampler.SampleLight(point, normal, sampler.Get1D())

	sample := selectedLight.Sample(point, normal, sampler.Get2D())
	sample.PDF *= lightSelectionPdf // Combined PDF for MIS calculations

	return sample, selectedLight, true
}

// SampleLightEmission selects and samples emission from a light using uniform sampling
// For emission sampling, we don't have a specific surface point, so use uniform distribution
func SampleLightEmission(lights []Light, lightSampler LightSampler, sampler core.Sampler) (EmissionSample, bool) {
	if len(lights) == 0 {
		return EmissionSample{}, false
	}
	selectedLight, lightSelectionPdf, _ := lightSampler.SampleLightEmission(sampler.Get1D())

	sample := selectedLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
	// Apply light selection probability to area PDF only (combined effect when multiplied)
	sample.AreaPDF *= lightSelectionPdf
	// Don't modify DirectionPDF - it's independent of light selection

	return sample, true
}

// SampleEmissionDirection samples a cosine-weighted emission direction from a surface
// and returns both the direction and the emission sample with separate area and direction PDFs
func SampleEmissionDirection(point core.Vec3, normal core.Vec3, areaPDF float64, mat material.Material, sample core.Vec2) EmissionSample {
	// Sample emission direction (cosine-weighted hemisphere)
	emissionDir := core.RandomCosineDirection(normal, sample)

	// Calculate direction PDF separately (cosine-weighted)
	cosTheta := emissionDir.Dot(normal)
	directionPDF := cosTheta / math.Pi

	// Get emission from material
	var emission core.Vec3
	if emitter, ok := mat.(material.Emitter); ok {
		emission = emitter.Emit(core.NewRay(point, emissionDir))
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
