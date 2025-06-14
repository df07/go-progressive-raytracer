package core

import (
	"math/rand"
)

// Shape interface for objects that can be hit by rays
type Shape interface {
	Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool)
	BoundingBox() AABB
}

// Material interface for objects that can scatter rays
type Material interface {
	Scatter(rayIn Ray, hit HitRecord, random *rand.Rand) (ScatterResult, bool)
}

// Emitter interface for materials that emit light
type Emitter interface {
	Emit() Vec3
}

// Light interface for objects that can be sampled for direct lighting
type Light interface {
	Sample(point Vec3, random *rand.Rand) LightSample
	PDF(point Vec3, direction Vec3) float64
}

// LightSample contains information about a sampled point on a light
type LightSample struct {
	Point     Vec3    // Point on the light source
	Normal    Vec3    // Normal at the light sample point
	Direction Vec3    // Direction from shading point to light
	Distance  float64 // Distance to light
	Emission  Vec3    // Emitted light
	PDF       float64 // Probability density of this sample
}

// Camera interface for cameras to avoid circular imports
type Camera interface {
	GetRay(i, j int, random *rand.Rand) Ray
}

// Scene interface for scene management
type Scene interface {
	GetCamera() Camera
	GetBackgroundColors() (topColor, bottomColor Vec3)
	GetShapes() []Shape
	GetLights() []Light
	GetSamplingConfig() SamplingConfig
}

// SamplingConfig contains rendering configuration
type SamplingConfig struct {
	SamplesPerPixel           int     // Number of rays per pixel
	MaxDepth                  int     // Maximum ray bounce depth
	RussianRouletteMinBounces int     // Minimum bounces before Russian Roulette can activate
	RussianRouletteMinSamples int     // Minimum samples per pixel before Russian Roulette can activate
	AdaptiveMinSamples        float64 // Minimum samples as percentage of max samples (0.0-1.0)
	AdaptiveThreshold         float64 // Relative error threshold for adaptive convergence (0.01 = 1%)
}

// ScatterResult contains the result of material scattering
type ScatterResult struct {
	Scattered   Ray     // The scattered ray
	Attenuation Vec3    // Color attenuation
	PDF         float64 // Probability density function (0 for specular materials)
}

// IsSpecular returns true if this is specular scattering (no PDF)
func (s ScatterResult) IsSpecular() bool {
	return s.PDF <= 0
}

// HitRecord contains information about a ray-object intersection
type HitRecord struct {
	Point     Vec3     // Point of intersection
	Normal    Vec3     // Surface normal at intersection
	T         float64  // Parameter t along the ray
	FrontFace bool     // Whether ray hit the front face
	Material  Material // Material of the hit object
}

// SetFaceNormal sets the normal vector and determines front/back face
func (h *HitRecord) SetFaceNormal(ray Ray, outwardNormal Vec3) {
	h.FrontFace = ray.Direction.Dot(outwardNormal) < 0
	if h.FrontFace {
		h.Normal = outwardNormal
	} else {
		h.Normal = outwardNormal.Multiply(-1)
	}
}
