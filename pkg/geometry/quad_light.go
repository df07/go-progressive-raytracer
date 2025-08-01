package geometry

import (
	"math"

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

func (ql *QuadLight) Type() core.LightType {
	return core.LightTypeArea
}

// Sample implements the Light interface - samples a point on the quad for direct lighting
func (ql *QuadLight) Sample(point core.Vec3, sample core.Vec2) core.LightSample {
	// Sample uniformly on the quad surface
	samplePoint := ql.Corner.Add(ql.U.Multiply(sample.X)).Add(ql.V.Multiply(sample.Y))

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
		emission = emitter.Emit(core.NewRay(point, direction))
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

// SampleEmission implements the Light interface - samples emission from the quad surface
// Used for BDPT light path generation
func (ql *QuadLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) core.EmissionSample {
	// Sample point uniformly on quad surface
	point := ql.Corner.Add(ql.U.Multiply(samplePoint.X)).Add(ql.V.Multiply(samplePoint.Y))

	// Sample emission direction (cosine-weighted hemisphere)
	emissionDir := core.RandomCosineDirection(ql.Normal, sampleDirection)

	// Calculate PDFs separately for BDPT
	// areaPDF: probability per unit area on the light surface
	// Units: [1/length²]
	areaPDF := 1.0 / ql.Area

	// directionPDF: probability per unit solid angle for cosine-weighted hemisphere sampling
	// PBRT formula: PDF = cos(θ)/π
	// Units: [1/steradian]
	cosTheta := emissionDir.Dot(ql.Normal)
	directionPDF := cosTheta / math.Pi

	// Get emission from material
	var emission core.Vec3
	if emitter, ok := ql.Material.(core.Emitter); ok {
		emission = emitter.Emit(core.NewRay(point, emissionDir))
	}

	return core.EmissionSample{
		Point:        point,
		Normal:       ql.Normal,
		Direction:    emissionDir,
		Emission:     emission,
		AreaPDF:      areaPDF,
		DirectionPDF: directionPDF,
	}
}

// EmissionPDF implements the Light interface - calculates PDF for emission sampling
func (ql *QuadLight) EmissionPDF(point core.Vec3, direction core.Vec3) float64 {
	// Check if point is on quad surface by solving point = corner + alpha*u + beta*v
	toPoint := point.Subtract(ql.Corner)

	// Project onto u and v vectors to get parametric coordinates
	uDotU := ql.U.Dot(ql.U)
	vDotV := ql.V.Dot(ql.V)
	uDotV := ql.U.Dot(ql.V)

	if uDotU == 0 || vDotV == 0 {
		return 0.0 // Degenerate quad
	}

	// Solve the 2x2 system for alpha and beta
	det := uDotU*vDotV - uDotV*uDotV
	if math.Abs(det) < 1e-8 {
		return 0.0 // Degenerate or nearly parallel vectors
	}

	toDotU := toPoint.Dot(ql.U)
	toDotV := toPoint.Dot(ql.V)

	alpha := (vDotV*toDotU - uDotV*toDotV) / det
	beta := (uDotU*toDotV - uDotV*toDotU) / det

	// Check if point is within quad bounds
	if alpha < 0 || alpha > 1 || beta < 0 || beta > 1 {
		return 0.0 // Point outside quad
	}

	// Verify the point is actually on the quad plane
	reconstructed := ql.Corner.Add(ql.U.Multiply(alpha)).Add(ql.V.Multiply(beta))
	if reconstructed.Subtract(point).Length() > 0.001 {
		return 0.0 // Point not on quad surface
	}

	// For BDPT, use area measure only (probability per unit area)
	areaPDF := 1.0 / ql.Area
	return areaPDF
}
