package geometry

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// gradientInfiniteLightMaterial implements gradient emission for infinite lights
type gradientInfiniteLightMaterial struct {
	topColor    core.Vec3 // Top gradient color
	bottomColor core.Vec3 // Bottom gradient color
}

// Scatter implements the Material interface (infinite lights don't scatter, only emit)
func (gilm *gradientInfiniteLightMaterial) Scatter(rayIn core.Ray, hit core.HitRecord, sampler core.Sampler) (core.ScatterResult, bool) {
	return core.ScatterResult{}, false // No scattering, only emission
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
func (gilm *gradientInfiniteLightMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Lights don't scatter, so no PDF
	return 0.0, true // isDelta = true
}

// GradientInfiniteLight represents a gradient infinite area light (like current background gradients)
type GradientInfiniteLight struct {
	topColor    core.Vec3     // Top gradient color
	bottomColor core.Vec3     // Bottom gradient color
	worldCenter core.Vec3     // Finite scene center from BVH (consistent with uniform)
	worldRadius float64       // Finite scene radius from BVH (consistent with uniform)
	material    core.Material // Material for emission
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

func (gil *GradientInfiniteLight) Type() core.LightType {
	return core.LightTypeInfinite
}

// GetMaterial returns the material for emission calculations
func (gil *GradientInfiniteLight) GetMaterial() core.Material {
	return gil.material
}

// emissionForDirection calculates gradient emission for a given direction
func (gil *GradientInfiniteLight) emissionForDirection(direction core.Vec3) core.Vec3 {
	t := 0.5 * (direction.Y + 1.0) // Map Y from [-1,1] to [0,1]
	return gil.bottomColor.Multiply(1.0 - t).Add(gil.topColor.Multiply(t))
}

// Sample implements the Light interface - samples the infinite light for direct lighting
func (gil *GradientInfiniteLight) Sample(point core.Vec3, sample core.Vec2) core.LightSample {
	// For infinite lights, we sample a direction uniformly on the sphere
	// and treat it as coming from infinite distance
	direction := uniformSampleSphere(sample)
	emission := gil.emissionForDirection(direction)

	return core.LightSample{
		Point:     point.Add(direction.Multiply(1e10)), // Far away point
		Normal:    direction.Multiply(-1),              // Points toward scene
		Direction: direction,
		Distance:  math.Inf(1),
		Emission:  emission,
		PDF:       1.0 / (4.0 * math.Pi), // Uniform over sphere
	}
}

// PDF implements the Light interface - returns probability density for direct lighting sampling
func (gil *GradientInfiniteLight) PDF(point core.Vec3, direction core.Vec3) float64 {
	// Uniform sampling over sphere
	return 1.0 / (4.0 * math.Pi)
}

// SampleEmission implements the Light interface - samples emission for BDPT light path generation
func (gil *GradientInfiniteLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) core.EmissionSample {
	// For BDPT light path generation, we need to:
	// 1. Sample a direction uniformly on the sphere
	// 2. Find where this direction intersects the scene bounding sphere
	// 3. Create emission ray from that point toward the scene

	direction := uniformSampleSphere(sampleDirection)
	emission := gil.emissionForDirection(direction)

	// Find scene center and create ray from scene boundary
	// Use consistent finite scene bounds from BVH
	emissionPoint := gil.worldCenter.Add(direction.Multiply(-gil.worldRadius))

	return core.EmissionSample{
		Point:        emissionPoint,
		Normal:       direction, // Points toward scene
		Direction:    direction,
		Emission:     emission,
		AreaPDF:      1.0 / (math.Pi * gil.worldRadius * gil.worldRadius), // PBRT: planar density
		DirectionPDF: 1.0 / (4.0 * math.Pi),                               // Uniform over sphere
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
func (gil *GradientInfiniteLight) Preprocess(scene core.Scene) error {
	bvh := scene.GetBVH()
	gil.worldCenter = bvh.Center
	gil.worldRadius = bvh.Radius
	return nil
}
