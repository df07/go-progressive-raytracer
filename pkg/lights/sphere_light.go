package lights

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/geometry"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// SphereLight represents a spherical area light
type SphereLight struct {
	*geometry.Sphere // Embed sphere for hit testing
}

// NewSphereLight creates a new spherical light
func NewSphereLight(center core.Vec3, radius float64, material material.Material) *SphereLight {
	return &SphereLight{
		Sphere: geometry.NewSphere(center, radius, material),
	}
}

func (sl *SphereLight) Type() LightType {
	return LightTypeArea
}

// Sample implements the Light interface - samples a point on the sphere for direct lighting
func (sl *SphereLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	// Vector from shading point to sphere center
	toCenter := sl.Center.Subtract(point)
	distanceToCenter := toCenter.Length()

	// If point is inside the sphere, sample uniformly on the sphere
	if distanceToCenter <= sl.Radius {
		return sl.sampleUniform(point, sample)
	}

	// Sample the sphere as seen from the shading point (visible hemisphere)
	return sl.sampleVisible(point, sample)
}

// sampleUniform samples uniformly on the entire sphere surface
func (sl *SphereLight) sampleUniform(point core.Vec3, sample core.Vec2) LightSample {
	// Generate uniform direction on unit sphere
	z := 1.0 - 2.0*sample.X // z ∈ [-1, 1]
	r := math.Sqrt(math.Max(0, 1.0-z*z))
	phi := 2.0 * math.Pi * sample.Y
	x := r * math.Cos(phi)
	y := r * math.Sin(phi)

	// Scale to sphere radius and translate to sphere center
	localDir := core.NewVec3(x, y, z)
	samplePoint := sl.Center.Add(localDir.Multiply(sl.Radius))

	// Calculate properties
	direction := samplePoint.Subtract(point)
	distance := direction.Length()
	dirNormalized := direction.Normalize()

	// Surface normal points outward from sphere center
	normal := localDir

	// PDF for uniform sphere sampling = 1 / (4π * radius²)
	pdf := 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)

	// Get emission from this light
	emission := sl.Emit(core.NewRay(point, dirNormalized), nil)

	return LightSample{
		Point:     samplePoint,
		Normal:    normal,
		Direction: dirNormalized,
		Distance:  distance,
		Emission:  emission,
		PDF:       pdf,
	}
}

// sampleVisible samples only the visible hemisphere of the sphere as seen from the shading point
func (sl *SphereLight) sampleVisible(point core.Vec3, sample core.Vec2) LightSample {
	// Vector from shading point to sphere center
	toCenter := sl.Center.Subtract(point)
	distanceToCenter := toCenter.Length()

	// Create coordinate system with z-axis pointing toward sphere center
	w := toCenter.Normalize()

	// Find a vector not parallel to w
	var u core.Vec3
	if math.Abs(w.X) > 0.1 {
		u = core.NewVec3(0, 1, 0)
	} else {
		u = core.NewVec3(1, 0, 0)
	}

	// Create orthonormal basis
	u = u.Cross(w).Normalize()
	v := w.Cross(u)

	// Calculate the half-angle of the cone subtended by the sphere
	sinThetaMax := sl.Radius / distanceToCenter
	cosThetaMax := math.Sqrt(math.Max(0, 1.0-sinThetaMax*sinThetaMax))

	// Sample direction within the cone toward the sphere
	cosTheta := 1.0 - sample.X*(1.0-cosThetaMax)
	sinTheta := math.Sqrt(math.Max(0, 1.0-cosTheta*cosTheta))
	phi := 2.0 * math.Pi * sample.Y

	// Convert to Cartesian coordinates in local space
	x := sinTheta * math.Cos(phi)
	y := sinTheta * math.Sin(phi)
	z := cosTheta

	// Transform to world space
	direction := u.Multiply(x).Add(v.Multiply(y)).Add(w.Multiply(z))

	// Find intersection with sphere
	ray := core.NewRay(point, direction)
	hitRecord, hit := sl.Sphere.Hit(ray, 0.001, math.Inf(1))
	if !hit {
		// This shouldn't happen if our math is correct, but handle it gracefully
		return sl.sampleUniform(point, sample)
	}

	// Calculate PDF for cone sampling
	// PDF = 1 / (2π * (1 - cos(θ_max)))
	pdf := 1.0 / (2.0 * math.Pi * (1.0 - cosThetaMax))

	// Get emission from this light
	emission := sl.Emit(ray, nil)

	return LightSample{
		Point:     hitRecord.Point,
		Normal:    hitRecord.Normal,
		Direction: direction,
		Distance:  hitRecord.T,
		Emission:  emission,
		PDF:       pdf,
	}
}

// PDF implements the Light interface - returns the probability density for sampling a given direction
func (sl *SphereLight) PDF(point, normal, direction core.Vec3) float64 {
	// Check if ray from point in direction hits the sphere
	ray := core.NewRay(point, direction)
	_, hit := sl.Sphere.Hit(ray, 0.001, math.Inf(1))
	if !hit {
		return 0.0
	}

	// Vector from shading point to sphere center
	toCenter := sl.Center.Subtract(point)
	distanceToCenter := toCenter.Length()

	// If point is inside the sphere, PDF is for uniform sphere sampling
	if distanceToCenter <= sl.Radius {
		return 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)
	}

	// PDF for visible hemisphere sampling
	sinThetaMax := sl.Radius / distanceToCenter
	cosThetaMax := math.Sqrt(math.Max(0, 1.0-sinThetaMax*sinThetaMax))

	return 1.0 / (2.0 * math.Pi * (1.0 - cosThetaMax))
}

// SampleEmission implements the Light interface - samples emission from the sphere surface
func (sl *SphereLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	// Sample point uniformly on ENTIRE sphere surface
	z := 1.0 - 2.0*samplePoint.X // z ∈ [-1, 1]
	r := math.Sqrt(math.Max(0, 1.0-z*z))
	phi := 2.0 * math.Pi * samplePoint.Y
	x := r * math.Cos(phi)
	y := r * math.Sin(phi)

	localDir := core.NewVec3(x, y, z)
	point := sl.Center.Add(localDir.Multiply(sl.Radius))
	normal := localDir // Surface normal points outward

	// Use shared emission sampling function
	areaPDF := 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)
	return SampleEmissionDirection(point, normal, areaPDF, sl.Material, sampleDirection)
}

// PDF_Le implements the Light interface - returns both position and directional PDFs
func (sl *SphereLight) PDF_Le(point core.Vec3, direction core.Vec3) (pdfPos, pdfDir float64) {
	// Validate point is on sphere surface
	if !validatePointOnSphere(point, sl.Center, sl.Radius, 0.001) {
		return 0.0, 0.0
	}

	// Calculate surface normal
	normal := point.Subtract(sl.Center).Normalize()

	// Check if direction is in correct hemisphere
	if direction.Dot(normal) <= 0 {
		return 0.0, 0.0
	}

	// Position PDF: uniform sampling over sphere surface
	pdfPos = 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)

	// Directional PDF: cosine-weighted hemisphere for Lambertian emission
	cosTheta := direction.Dot(normal)
	pdfDir = cosTheta / math.Pi

	return pdfPos, pdfDir
}

// Emit implements the Light interface - returns material emission
func (sl *SphereLight) Emit(ray core.Ray, hit *material.SurfaceInteraction) core.Vec3 {
	// Area lights emit according to their material
	if emitter, isEmissive := sl.Material.(material.Emitter); isEmissive {
		return emitter.Emit(ray, hit)
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// ValidatePointOnSphere checks if a point lies on a sphere surface within tolerance
func validatePointOnSphere(point core.Vec3, center core.Vec3, radius float64, tolerance float64) bool {
	distFromCenter := point.Subtract(center).Length()
	return math.Abs(distFromCenter-radius) <= tolerance
}
