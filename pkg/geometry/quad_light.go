package geometry

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// QuadLight represents a rectangular area light
type QuadLight struct {
	*Quad         // Embed quad for hit testing
	Area  float64 // Cached area for PDF calculations
}

// NewQuadLight creates a new quad light
func NewQuadLight(corner, u, v core.Vec3, material core.Material) *QuadLight {
	quad := NewQuad(corner, u, v, material)

	// Calculate area of the quad: |u × v|
	area := u.Cross(v).Length()

	return &QuadLight{
		Quad: quad,
		Area: area,
	}
}

// Sample implements the Light interface - samples a point on the quad for direct lighting
func (ql *QuadLight) Sample(point core.Vec3, random *rand.Rand) core.LightSample {
	// Sample uniformly on the quad surface
	// Generate random barycentric coordinates
	alpha := random.Float64()
	beta := random.Float64()

	// Calculate sample point: corner + alpha * u + beta * v
	samplePoint := ql.Corner.Add(ql.U.Multiply(alpha)).Add(ql.V.Multiply(beta))

	// Calculate direction from shading point to light sample
	toLight := samplePoint.Subtract(point)
	distance := toLight.Length()
	direction := toLight.Multiply(1.0 / distance) // Normalize

	// Calculate PDF: 1/Area for uniform sampling
	pdf := 1.0 / ql.Area

	// Convert to solid angle PDF
	// PDF_solid_angle = PDF_area * distance² / |cos(θ)|
	// where θ is the angle between light normal and direction to shading point
	cosTheta := math.Abs(ql.Normal.Dot(direction.Multiply(-1)))
	if cosTheta < 1e-8 {
		// Light is edge-on, no contribution
		return core.LightSample{
			Point:     samplePoint,
			Normal:    ql.Normal,
			Direction: direction,
			Distance:  distance,
			Emission:  core.Vec3{},
			PDF:       0,
		}
	}

	solidAnglePDF := pdf * distance * distance / cosTheta

	// Get emission from material if it's an emitter
	var emission core.Vec3
	if emitter, ok := ql.Material.(core.Emitter); ok {
		emission = emitter.Emit()
	}

	return core.LightSample{
		Point:     samplePoint,
		Normal:    ql.Normal,
		Direction: direction,
		Distance:  distance,
		Emission:  emission,
		PDF:       solidAnglePDF,
	}
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
func (ql *QuadLight) PDF(point core.Vec3, direction core.Vec3) float64 {
	// Check if ray from point in direction hits the quad
	ray := core.NewRay(point, direction)
	hitRecord, hit := ql.Quad.Hit(ray, 0.001, math.Inf(1))
	if !hit {
		return 0.0
	}

	// Calculate solid angle PDF
	distance := hitRecord.T
	cosTheta := math.Abs(ql.Normal.Dot(direction.Multiply(-1)))

	if cosTheta < 1e-8 {
		return 0.0
	}

	// PDF_solid_angle = PDF_area * distance² / |cos(θ)|
	areaPDF := 1.0 / ql.Area
	return areaPDF * distance * distance / cosTheta
}
