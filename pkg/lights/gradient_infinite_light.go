package lights

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// gradientInfiniteLightMaterial implements gradient emission for infinite lights
type gradientInfiniteLightMaterial struct {
	topColor    core.Vec3 // Top gradient color
	bottomColor core.Vec3 // Bottom gradient color
}

// Scatter implements the Material interface (infinite lights don't scatter, only emit)
func (gilm *gradientInfiniteLightMaterial) Scatter(rayIn core.Ray, hit material.HitRecord, sampler core.Sampler) (material.ScatterResult, bool) {
	return material.ScatterResult{}, false // No scattering, only emission
}

// Emit implements the Emitter interface with gradient emission based on ray direction
func (gilm *gradientInfiniteLightMaterial) Emit(rayIn core.Ray) core.Vec3 {
	// Use ray direction to determine gradient position
	direction := rayIn.Direction.Normalize()
	t := 0.5 * (direction.Y + 1.0) // Map Y from [-1,1] to [0,1]
	return gilm.bottomColor.Multiply(1.0 - t).Add(gilm.topColor.Multiply(t))
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (gilm *gradientInfiniteLightMaterial) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	// Lights don't reflect - they only emit
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (gilm *gradientInfiniteLightMaterial) PDF(incomingDir, normal core.Vec3, outgoingDir core.Vec3) (float64, bool) {
	// Lights don't scatter, so no PDF
	return 0.0, true // isDelta = true
}

// GradientInfiniteLight represents a gradient infinite area light (like current background gradients)
type GradientInfiniteLight struct {
	topColor    core.Vec3         // Top gradient color
	bottomColor core.Vec3         // Bottom gradient color
	worldCenter core.Vec3         // Finite scene center from BVH (consistent with uniform)
	worldRadius float64           // Finite scene radius from BVH (consistent with uniform)
	material    material.Material // Material for emission
}

// NewGradientInfiniteLight creates a new gradient infinite light
func NewGradientInfiniteLight(topColor, bottomColor core.Vec3) *GradientInfiniteLight {
	material := &gradientInfiniteLightMaterial{topColor: topColor, bottomColor: bottomColor}
	return &GradientInfiniteLight{
		topColor:    topColor,
		bottomColor: bottomColor,
		material:    material,
	}
}

func (gil *GradientInfiniteLight) Type() LightType {
	return LightTypeInfinite
}

// GetMaterial returns the material for emission calculations
func (gil *GradientInfiniteLight) GetMaterial() material.Material {
	return gil.material
}

// emissionForDirection calculates gradient emission for a given direction
func (gil *GradientInfiniteLight) emissionForDirection(direction core.Vec3) core.Vec3 {
	t := 0.5 * (direction.Y + 1.0) // Map Y from [-1,1] to [0,1]
	return gil.bottomColor.Multiply(1.0 - t).Add(gil.topColor.Multiply(t))
}

// Sample implements the Light interface - samples the infinite light for direct lighting
func (gil *GradientInfiniteLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	// For infinite lights, sample the visible hemisphere using cosine-weighted sampling
	// This provides better importance sampling since cosine terms cancel in the rendering equation
	direction := core.RandomCosineDirection(normal, sample)
	cosTheta := direction.Dot(normal)
	emission := gil.emissionForDirection(direction)

	return LightSample{
		Point:     point.Add(direction.Multiply(1e10)), // Far away point
		Normal:    direction.Multiply(-1),              // Points toward scene
		Direction: direction,
		Distance:  math.Inf(1),
		Emission:  emission,
		PDF:       cosTheta / math.Pi, // Cosine-weighted hemisphere PDF
	}
}

// PDF implements the Light interface - returns probability density for direct lighting sampling
func (gil *GradientInfiniteLight) PDF(point, normal, direction core.Vec3) float64 {
	// Cosine-weighted hemisphere PDF
	cosTheta := direction.Dot(normal)
	if cosTheta <= 0 {
		return 0.0 // Direction is below hemisphere
	}
	return cosTheta / math.Pi
}

// SampleEmission implements the Light interface - samples emission for BDPT light path generation
func (gil *GradientInfiniteLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	// Use PBRT's disk sampling approach from shared function
	emissionRay, areaPDF, directionPDF := core.SampleInfiniteLight(gil.worldCenter, gil.worldRadius, samplePoint, sampleDirection)
	emission := gil.emissionForDirection(emissionRay.Direction)

	return EmissionSample{
		Point:        emissionRay.Origin,
		Normal:       emissionRay.Direction.Multiply(-1), // Points toward scene
		Direction:    emissionRay.Direction,              // Ray direction (parallel rays)
		Emission:     emission,
		AreaPDF:      areaPDF,
		DirectionPDF: directionPDF,
	}
}

// EmissionPDF implements the Light interface - calculates PDF for BDPT MIS calculations
func (gil *GradientInfiniteLight) EmissionPDF(point core.Vec3, direction core.Vec3) float64 {
	// PBRT: For infinite lights, return planar sampling density
	if gil.worldRadius <= 0 {
		return 0.0
	}
	return 1.0 / (math.Pi * gil.worldRadius * gil.worldRadius)
}

// Emit implements the Light interface - evaluates emission in ray direction
func (gil *GradientInfiniteLight) Emit(ray core.Ray) core.Vec3 {
	// Use ray direction to determine gradient position
	direction := ray.Direction.Normalize()
	t := 0.5 * (direction.Y + 1.0) // Map Y from [-1,1] to [0,1]
	return gil.bottomColor.Multiply(1.0 - t).Add(gil.topColor.Multiply(t))
}

// Preprocess implements the Preprocessor interface - sets world bounds from scene
func (gil *GradientInfiniteLight) Preprocess(worldCenter core.Vec3, worldRadius float64) error {
	gil.worldCenter = worldCenter
	gil.worldRadius = worldRadius
	return nil
}
