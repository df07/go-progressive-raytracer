package geometry

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// PointSpotLight represents a directional point spot light with cone angle and falloff
type PointSpotLight struct {
	position        core.Vec3 // Light position in world space
	direction       core.Vec3 // Normalized direction vector (from -> to)
	emission        core.Vec3 // Light intensity/color
	cosTotalWidth   float64   // Cosine of total cone angle (outer edge)
	cosFalloffStart float64   // Cosine of falloff start angle (inner cone)
}

// NewPointSpotLight creates a new point spot light
// from: light position
// to: point the light is aimed at
// emission: light intensity/color
// coneAngleDegrees: total cone angle in degrees
// coneDeltaAngleDegrees: falloff transition angle in degrees
func NewPointSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees float64) *PointSpotLight {
	direction := to.Subtract(from).Normalize()

	// Convert to radians and compute cosines
	totalWidthRadians := coneAngleDegrees * math.Pi / 180.0
	falloffStartRadians := (coneAngleDegrees - coneDeltaAngleDegrees) * math.Pi / 180.0

	return &PointSpotLight{
		position:        from,
		direction:       direction,
		emission:        emission,
		cosTotalWidth:   math.Cos(totalWidthRadians),
		cosFalloffStart: math.Cos(falloffStartRadians),
	}
}

// Sample implements the Light interface - samples a point on the light for direct lighting
func (sl *PointSpotLight) Sample(point core.Vec3, random *rand.Rand) core.LightSample {
	// For a point light, the sample point is always the light position
	samplePoint := sl.position

	// Direction from shading point to light
	toLightVec := samplePoint.Subtract(point)
	distance := toLightVec.Length()

	if distance == 0 {
		// Avoid division by zero if point is exactly at light position
		return core.LightSample{
			Point:     samplePoint,
			Normal:    core.NewVec3(0, 1, 0), // Arbitrary normal
			Direction: core.NewVec3(0, 1, 0), // Arbitrary direction
			Distance:  0,
			Emission:  core.NewVec3(0, 0, 0), // No emission at same point
			PDF:       1.0,
		}
	}

	toLight := toLightVec.Normalize()

	// Calculate spot light attenuation using falloff
	// Direction from light to shading point (opposite of toLight)
	lightToPoint := toLight.Multiply(-1)

	// Angle between light direction and direction to shading point
	cosAngle := sl.direction.Dot(lightToPoint)

	// Calculate spot light falloff
	spotAttenuation := sl.falloff(cosAngle)

	// Calculate final emission with spot attenuation and distance falloff
	emission := sl.emission.Multiply(spotAttenuation / (distance * distance))

	// For point lights, PDF is delta function (represented as 1.0)
	pdf := 1.0

	// Normal points from light towards the shading point
	normal := toLight

	return core.LightSample{
		Point:     samplePoint,
		Normal:    normal,
		Direction: toLight,
		Distance:  distance,
		Emission:  emission,
		PDF:       pdf,
	}
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
// For point lights, this is effectively a delta function
func (sl *PointSpotLight) PDF(point core.Vec3, direction core.Vec3) float64 {
	// For point lights, PDF is essentially a delta function
	// We return 1.0 if the direction points toward the light, 0 otherwise

	toLightVec := sl.position.Subtract(point)
	if toLightVec.Length() == 0 {
		return 0.0
	}

	toLight := toLightVec.Normalize()

	// Check if direction is close to the light direction (within some tolerance)
	dot := direction.Dot(toLight)
	if dot > 0.999 { // Very close to light direction
		return 1.0
	}

	return 0.0
}

// falloff calculates the spot light falloff
// Based on the cosine of the angle between light direction and direction to point
func (sl *PointSpotLight) falloff(cosAngle float64) float64 {
	// Outside the total cone width
	if cosAngle < sl.cosTotalWidth {
		return 0.0
	}

	// Inside the inner cone (full intensity)
	if cosAngle >= sl.cosFalloffStart {
		return 1.0
	}

	// In the falloff transition region
	// Linear interpolation between falloff start and total width
	delta := (cosAngle - sl.cosTotalWidth) / (sl.cosFalloffStart - sl.cosTotalWidth)

	// Smooth falloff using quartic curve
	return delta * delta * delta * delta
}

// GetIntensityAt returns the light intensity at a given point
// This is useful for debugging and visualization
func (sl *PointSpotLight) GetIntensityAt(point core.Vec3) core.Vec3 {
	toLightVec := sl.position.Subtract(point)
	distance := toLightVec.Length()

	if distance == 0 {
		return core.NewVec3(0, 0, 0)
	}

	toLight := toLightVec.Normalize()
	lightToPoint := toLight.Multiply(-1)

	// Calculate spot attenuation using falloff
	cosAngle := sl.direction.Dot(lightToPoint)
	spotAttenuation := sl.falloff(cosAngle)

	// Return intensity with distance and spot falloff
	return sl.emission.Multiply(spotAttenuation / (distance * distance))
}
