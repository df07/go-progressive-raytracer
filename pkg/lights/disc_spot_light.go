package lights

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/geometry"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// discSpotLightMaterial implements directional emission for disc spot lights
type discSpotLightMaterial struct {
	baseEmission    core.Vec3
	spotDirection   core.Vec3 // Direction the spot light points
	cosTotalWidth   float64   // Cosine of total cone angle (outer edge)
	cosFalloffStart float64   // Cosine of falloff start angle (inner cone)
}

// Scatter implements the Material interface (spot lights don't scatter, only emit)
func (dslm *discSpotLightMaterial) Scatter(rayIn core.Ray, hit material.SurfaceInteraction, sampler core.Sampler) (material.ScatterResult, bool) {
	return material.ScatterResult{}, false // No scattering, only emission
}

// Emit implements the Emitter interface with directional spot light falloff
func (dslm *discSpotLightMaterial) Emit(rayIn core.Ray, hit *material.SurfaceInteraction) core.Vec3 {
	// Calculate directional emission for indirect rays (caustics)
	// Check if we're hitting the "back" face of the disc (the emitting side)

	// Only emit from the front face
	if hit != nil && !hit.FrontFace {
		return core.NewVec3(0, 0, 0)
	}

	// The ray direction should be roughly opposite to the spot direction for proper emission
	rayDirection := rayIn.Direction
	cosAngleToSpot := rayDirection.Dot(dslm.spotDirection)

	// Only emit if the ray is coming from the "back" side (opposite to spot direction)
	// Allow some tolerance since the disc has finite size
	if cosAngleToSpot > -0.3 { // Increased tolerance for larger disc
		return core.NewVec3(0, 0, 0) // No emission for rays from the front/side
	}

	// For caustics, emit full intensity to ensure they're visible
	// The directional control happens primarily in direct light sampling
	falloff := 1.0

	return dslm.baseEmission.Multiply(falloff)
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions
func (dslm *discSpotLightMaterial) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *material.SurfaceInteraction, mode material.TransportMode) core.Vec3 {
	// Lights don't reflect - they only emit
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (dslm *discSpotLightMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// Emissive materials don't scatter, so PDF is always 0
	return 0.0, false // Not a delta function, just no scattering
}

// DiscSpotLight represents a directional spot light implemented as a disc area light
type DiscSpotLight struct {
	position        core.Vec3  // Light position in world space
	direction       core.Vec3  // Normalized direction vector (from -> to)
	emission        core.Vec3  // Light intensity/color
	cosTotalWidth   float64    // Cosine of total cone angle (outer edge)
	cosFalloffStart float64    // Cosine of falloff start angle (inner cone)
	discLight       *DiscLight // geometry.Disc representing the area light
}

// NewDiscSpotLight creates a new disc spot light
// from: light position
// to: point the light is aimed at
// emission: light intensity/color
// coneAngleDegrees: total cone angle in degrees
// coneDeltaAngleDegrees: falloff transition angle in degrees
// radius: radius of the disc light in world units
func NewDiscSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees, radius float64) *DiscSpotLight {
	direction := to.Subtract(from).Normalize()

	// Convert to radians and compute cosines
	totalWidthRadians := coneAngleDegrees * math.Pi / 180.0
	falloffStartRadians := (coneAngleDegrees - coneDeltaAngleDegrees) * math.Pi / 180.0

	// Create directional material
	mat := &discSpotLightMaterial{
		baseEmission:    emission,
		spotDirection:   direction,
		cosTotalWidth:   math.Cos(totalWidthRadians),
		cosFalloffStart: math.Cos(falloffStartRadians),
	}

	// Create a circular disc light oriented towards the target
	// The disc normal should point in the spot light direction
	discLight := NewDiscLight(from, direction, radius, mat)

	return &DiscSpotLight{
		position:        from,
		direction:       direction,
		emission:        emission,
		cosTotalWidth:   math.Cos(totalWidthRadians),
		cosFalloffStart: math.Cos(falloffStartRadians),
		discLight:       discLight,
	}
}

func (dsl *DiscSpotLight) Type() LightType {
	return LightTypeArea
}

// Sample implements the Light interface - samples a point on the disc for direct lighting
func (dsl *DiscSpotLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	// Sample the underlying disc light
	lightSample := dsl.discLight.Sample(point, normal, sample)

	// Apply spot light directional falloff
	// Calculate direction from actual sampled point on disc to shading point
	lightToPoint := point.Subtract(lightSample.Point).Normalize()
	cosAngle := dsl.direction.Dot(lightToPoint)
	spotAttenuation := dsl.falloff(cosAngle)

	// Modify the emission with spot light falloff
	lightSample.Emission = lightSample.Emission.Multiply(spotAttenuation)

	return lightSample
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
func (dsl *DiscSpotLight) PDF(point, normal, direction core.Vec3) float64 {
	return dsl.discLight.PDF(point, normal, direction)
}

// falloff calculates the spot light falloff
// Based on the cosine of the angle between light direction and direction to point
func (dsl *DiscSpotLight) falloff(cosAngle float64) float64 {
	// Outside the total cone width
	if cosAngle < dsl.cosTotalWidth {
		return 0.0
	}

	// Inside the inner cone (full intensity)
	if cosAngle >= dsl.cosFalloffStart {
		return 1.0
	}

	// In the falloff transition region
	// Linear interpolation between falloff start and total width
	delta := (cosAngle - dsl.cosTotalWidth) / (dsl.cosFalloffStart - dsl.cosTotalWidth)

	// Smooth falloff using quartic curve
	return delta * delta * delta * delta
}

// GetIntensityAt returns the light intensity at a given point
// This is useful for debugging and visualization
func (dsl *DiscSpotLight) GetIntensityAt(point core.Vec3) core.Vec3 {
	toLightVec := dsl.position.Subtract(point)
	distance := toLightVec.Length()

	if distance == 0 {
		return core.NewVec3(0, 0, 0)
	}

	toLight := toLightVec.Normalize()
	lightToPoint := toLight.Multiply(-1)

	// Calculate spot attenuation using falloff
	cosAngle := dsl.direction.Dot(lightToPoint)
	spotAttenuation := dsl.falloff(cosAngle)

	// Return intensity with distance and spot falloff
	return dsl.emission.Multiply(spotAttenuation / (distance * distance))
}

// Hit implements the Shape interface for caustic ray intersection
func (dsl *DiscSpotLight) Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool) {
	return dsl.discLight.Hit(ray, tMin, tMax)
}

// BoundingBox implements the Shape interface
func (dsl *DiscSpotLight) BoundingBox() geometry.AABB {
	return dsl.discLight.BoundingBox()
}

// GetDisc returns the underlying disc light for scene integration
func (dsl *DiscSpotLight) GetDisc() *geometry.Disc {
	return dsl.discLight.Disc
}

// SampleEmission implements the Light interface - samples emission from the disc spot light surface
func (dsl *DiscSpotLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	// Sample a point on the disc
	point, normal := dsl.discLight.Disc.SampleUniform(samplePoint)

	// Sample emission within the cone
	emissionDir := core.SampleCone(dsl.direction, dsl.cosTotalWidth, sampleDirection)

	// Calculate spot light falloff
	cosTheta := emissionDir.Dot(dsl.direction)
	spotAttenuation := dsl.falloff(cosTheta)

	// Calculate PDF for cone sampling
	conePDF := UniformConePDF(dsl.cosTotalWidth)
	areaPDF := 1.0 / (math.Pi * dsl.discLight.Radius * dsl.discLight.Radius)

	// Apply spot attenuation to emission
	emission := dsl.emission.Multiply(spotAttenuation)

	return EmissionSample{
		Point:        point,
		Normal:       normal,
		Direction:    emissionDir,
		Emission:     emission,
		AreaPDF:      areaPDF,
		DirectionPDF: conePDF, // Extract direction component
	}
}

// EmissionPDF implements the Light interface - calculates PDF for emission sampling
func (dsl *DiscSpotLight) EmissionPDF(point core.Vec3, direction core.Vec3) float64 {
	// First check if point is on disc surface
	basePDF := dsl.discLight.EmissionPDF(point, direction)
	if basePDF == 0.0 {
		return 0.0 // Point not on disc or direction below surface
	}

	// Check if direction is within the spot cone
	cosAngleToSpot := direction.Dot(dsl.direction)
	if cosAngleToSpot < dsl.cosTotalWidth {
		return 0.0 // Direction outside spot cone
	}

	// Return area PDF only (direction PDF handled separately in new interface)
	areaPDF := 1.0 / (math.Pi * dsl.discLight.Radius * dsl.discLight.Radius)
	return areaPDF
}

// Emit implements the Light interface - returns material emission
func (dsl *DiscSpotLight) Emit(ray core.Ray, hit *material.SurfaceInteraction) core.Vec3 {
	// Spot lights emit according to their material
	if emitter, isEmissive := dsl.discLight.Material.(material.Emitter); isEmissive {
		return emitter.Emit(ray, hit)
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}
