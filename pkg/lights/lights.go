package lights

import "github.com/df07/go-progressive-raytracer/pkg/core"

type LightType string

const (
	LightTypeArea     LightType = "area"
	LightTypePoint    LightType = "point"
	LightTypeInfinite LightType = "infinite"
)

// Light interface for objects that can be sampled for direct lighting
type Light interface {
	Type() LightType

	// Sample samples light toward a specific point for direct lighting
	// Returns LightSample with direction FROM shading point TO light
	// For infinite lights: surfaceNormal constrains sampling to visible hemisphere
	Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample

	// PDF calculates the probability density for sampling a given direction toward the light
	// For infinite lights: surfaceNormal needed to compute cosine-weighted PDF
	PDF(point core.Vec3, normal core.Vec3, direction core.Vec3) float64

	// SampleEmission samples emission from the light surface for BDPT light path generation
	// Returns EmissionSample with direction FROM light surface (for light transport)
	SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample

	// EmissionPDF calculates PDF for emission sampling - needed for BDPT MIS calculations
	EmissionPDF(point core.Vec3, direction core.Vec3) float64

	// Emit evaluates emission in the direction of the given ray
	// For finite lights, returns zero. For infinite lights, returns emission based on ray direction.
	Emit(ray core.Ray) core.Vec3
}

// LightSample contains information about a sampled point on a light
type LightSample struct {
	Point     core.Vec3 // Point on the light source
	Normal    core.Vec3 // Normal at the light sample point
	Direction core.Vec3 // Direction from shading point to light
	Distance  float64   // Distance to light
	Emission  core.Vec3 // Emitted light
	PDF       float64   // Probability density of this sample
}

// EmissionSample contains information about a sampled emission for BDPT light path generation
type EmissionSample struct {
	Point        core.Vec3 // Point on the light surface
	Normal       core.Vec3 // Surface normal at the emission point (outward facing)
	Direction    core.Vec3 // Emission direction FROM the surface (cosine-weighted hemisphere)
	Emission     core.Vec3 // Emitted radiance at this point and direction
	AreaPDF      float64   // PDF for position sampling (per unit area)
	DirectionPDF float64
}

// LightSampler interface for different light sampling strategies
type LightSampler interface {
	// SampleLight selects a light for the given surface point and returns the light, selection probability, and light index
	SampleLight(point core.Vec3, normal core.Vec3, u float64) (Light, float64, int)

	// SampleLightEmission selects a light for emission sampling and returns the light, selection probability, and light index
	SampleLightEmission(u float64) (Light, float64, int)

	// GetLightProbability returns the selection probability for a specific light at a surface point
	GetLightProbability(lightIndex int, point core.Vec3, normal core.Vec3) float64

	// GetLightCount returns the number of lights in this sampler
	GetLightCount() int
}
