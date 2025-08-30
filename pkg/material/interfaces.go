package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Material interface for objects that can scatter rays
type Material interface {
	// Existing method - generates random scattered direction
	Scatter(rayIn core.Ray, hit HitRecord, sampler core.Sampler) (ScatterResult, bool)

	// NEW: Evaluate BRDF for specific incoming/outgoing directions
	EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3

	// NEW: Calculate PDF for specific incoming/outgoing directions
	// Returns (pdf, isDelta) where isDelta indicates if this is a delta function (specular)
	PDF(incomingDir, outgoingDir, normal core.Vec3) (pdf float64, isDelta bool)
}

// Emitter interface for materials that emit light
type Emitter interface {
	Emit(rayIn core.Ray) core.Vec3
}

// ScatterResult contains the result of material scattering
type ScatterResult struct {
	Incoming    core.Ray  // The incoming ray
	Scattered   core.Ray  // The scattered ray
	Attenuation core.Vec3 // Color attenuation
	PDF         float64   // Probability density function (0 for specular materials)
}

// IsSpecular returns true if this is specular scattering (no PDF)
func (s ScatterResult) IsSpecular() bool {
	return s.PDF <= 0
}

// HitRecord contains information about a ray-object intersection
type HitRecord struct {
	Point     core.Vec3 // Point of intersection
	Normal    core.Vec3 // Surface normal at intersection
	T         float64   // Parameter t along the ray
	FrontFace bool      // Whether ray hit the front face
	Material  Material  // Material of the hit object
}

// SetFaceNormal sets the normal vector and determines front/back face
func (h *HitRecord) SetFaceNormal(ray core.Ray, outwardNormal core.Vec3) {
	h.FrontFace = ray.Direction.Dot(outwardNormal) < 0
	if h.FrontFace {
		h.Normal = outwardNormal
	} else {
		h.Normal = outwardNormal.Multiply(-1)
	}
}
