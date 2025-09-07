package lights

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// uniformInfiniteLightMaterial implements uniform emission for infinite lights
type uniformInfiniteLightMaterial struct {
	emission core.Vec3 // Uniform emission color
}

// Scatter implements the Material interface (infinite lights don't scatter, only emit)
func (uilm *uniformInfiniteLightMaterial) Scatter(rayIn core.Ray, hit material.HitRecord, sampler core.Sampler) (material.ScatterResult, bool) {
	return material.ScatterResult{}, false // No scattering, only emission
}

// Emit implements the Emitter interface with uniform emission
func (uilm *uniformInfiniteLightMaterial) Emit(rayIn core.Ray) core.Vec3 {
	// Uniform infinite light emits the same color in all directions
	return uilm.emission
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (uilm *uniformInfiniteLightMaterial) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *material.HitRecord, mode material.TransportMode) core.Vec3 {
	// Lights don't reflect - they only emit
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (uilm *uniformInfiniteLightMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Lights don't scatter, so no PDF
	return 0.0, true // isDelta = true
}

// UniformInfiniteLight represents a uniform infinite area light (constant emission in all directions)
type UniformInfiniteLight struct {
	emission    core.Vec3         // Uniform emission color
	worldCenter core.Vec3         // Finite scene center from BVH
	worldRadius float64           // Finite scene radius from BVH
	material    material.Material // Material for emission
}

// NewUniformInfiniteLight creates a new uniform infinite light
func NewUniformInfiniteLight(emission core.Vec3) *UniformInfiniteLight {
	material := &uniformInfiniteLightMaterial{emission: emission}
	return &UniformInfiniteLight{
		emission: emission,
		material: material,
	}
}

func (uil *UniformInfiniteLight) Type() LightType {
	return LightTypeInfinite
}

// GetMaterial returns the material for emission calculations
func (uil *UniformInfiniteLight) GetMaterial() material.Material {
	return uil.material
}

// Sample implements the Light interface - samples the infinite light for direct lighting
func (uil *UniformInfiniteLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	// For infinite lights, sample the visible hemisphere using cosine-weighted sampling
	// This provides better importance sampling since cosine terms cancel in the rendering equation
	direction := core.SampleCosineHemisphere(normal, sample)
	cosTheta := direction.Dot(normal)

	return LightSample{
		Point:     point.Add(direction.Multiply(1e10)), // Far away point
		Normal:    direction.Multiply(-1),              // Points toward scene
		Direction: direction,
		Distance:  math.Inf(1),
		Emission:  uil.emission,
		PDF:       cosTheta / math.Pi, // Cosine-weighted hemisphere PDF
	}
}

// PDF implements the Light interface - returns probability density for direct lighting sampling
func (uil *UniformInfiniteLight) PDF(point, normal, direction core.Vec3) float64 {
	// Cosine-weighted hemisphere PDF
	cosTheta := direction.Dot(normal)
	if cosTheta <= 0 {
		return 0.0 // Direction is below hemisphere
	}
	return cosTheta / math.Pi
}

// SampleEmission implements the Light interface - samples emission for BDPT light path generation
func (uil *UniformInfiniteLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	// Use PBRT's disk sampling approach from shared function
	emissionRay, areaPDF, directionPDF := SampleInfiniteLight(uil.worldCenter, uil.worldRadius, samplePoint, sampleDirection)

	return EmissionSample{
		Point:        emissionRay.Origin,
		Normal:       emissionRay.Direction.Multiply(-1), // Points toward scene
		Direction:    emissionRay.Direction,              // Ray direction (parallel rays)
		Emission:     uil.emission,
		AreaPDF:      areaPDF,
		DirectionPDF: directionPDF,
	}
}

// EmissionPDF implements the Light interface - calculates PDF for BDPT MIS calculations
func (uil *UniformInfiniteLight) EmissionPDF(point core.Vec3, direction core.Vec3) float64 {
	// PBRT: For infinite lights, return planar sampling density
	if uil.worldRadius <= 0 {
		return 0.0
	}
	return 1.0 / (math.Pi * uil.worldRadius * uil.worldRadius)
}

// Emit implements the Light interface - evaluates emission in ray direction
func (uil *UniformInfiniteLight) Emit(ray core.Ray) core.Vec3 {
	// Uniform infinite light emits the same color in all directions
	return uil.emission
}

// Preprocess implements the Preprocessor interface - sets world bounds from scene
func (uil *UniformInfiniteLight) Preprocess(worldCenter core.Vec3, worldRadius float64) error {
	uil.worldCenter = worldCenter
	uil.worldRadius = worldRadius
	return nil
}
