package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TransportMode indicates the type of transport for proper BRDF evaluation
// Matches PBRT's TransportMode enum semantics
type TransportMode int

const (
	Radiance   TransportMode = iota // Radiance transport (used by camera vertices)
	Importance                      // Importance transport (used by light vertices)
)

// Material interface for objects that can scatter rays
type Material interface {
	// Generates random scattered direction
	Scatter(rayIn core.Ray, hit SurfaceInteraction, sampler core.Sampler) (ScatterResult, bool)

	// Evaluate BRDF for specific incoming/outgoing directions with transport mode
	EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *SurfaceInteraction, mode TransportMode) core.Vec3

	// Calculate PDF for specific incoming/outgoing directions
	// Returns (pdf, isDelta) where isDelta indicates if this is a delta function (specular)
	PDF(incomingDir, outgoingDir, normal core.Vec3) (pdf float64, isDelta bool)
}

// Emitter interface for materials that emit light
type Emitter interface {
	Emit(rayIn core.Ray, hit *SurfaceInteraction) core.Vec3
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

// SurfaceInteraction contains information about a ray-object intersection
type SurfaceInteraction struct {
	Point     core.Vec3 // Point of intersection
	Normal    core.Vec3 // Surface normal at intersection
	T         float64   // Parameter t along the ray
	FrontFace bool      // Whether ray hit the front face
	Material  Material  // Material of the hit object
	UV        core.Vec2 // Texture coordinates
}

// SetFaceNormal sets the normal vector and determines front/back face
func (h *SurfaceInteraction) SetFaceNormal(ray core.Ray, outwardNormal core.Vec3) {
	h.FrontFace = ray.Direction.Dot(outwardNormal) < 0
	if h.FrontFace {
		h.Normal = outwardNormal
	} else {
		h.Normal = outwardNormal.Multiply(-1)
	}
}
