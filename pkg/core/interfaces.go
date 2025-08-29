package core

// Shape interface for objects that can be hit by rays
type Shape interface {
	Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool)
	BoundingBox() AABB
}

// Material interface for objects that can scatter rays
type Material interface {
	// Existing method - generates random scattered direction
	Scatter(rayIn Ray, hit HitRecord, sampler Sampler) (ScatterResult, bool)

	// NEW: Evaluate BRDF for specific incoming/outgoing directions
	EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3

	// NEW: Calculate PDF for specific incoming/outgoing directions
	// Returns (pdf, isDelta) where isDelta indicates if this is a delta function (specular)
	PDF(incomingDir, outgoingDir, normal Vec3) (pdf float64, isDelta bool)
}

// Emitter interface for materials that emit light
type Emitter interface {
	Emit(rayIn Ray) Vec3
}

// Preprocessor interface for objects that need scene preprocessing
type Preprocessor interface {
	Preprocess(bvh *BVH) error
}

// Scene interface for scene management
// Scene interface removed - use scene.Scene struct directly

// Logger interface for raytracer logging
type Logger interface {
	Printf(format string, args ...interface{})
}

// SamplingConfig contains rendering configuration
type SamplingConfig struct {
	Width                     int     // Image width
	Height                    int     // Image height
	SamplesPerPixel           int     // Number of rays per pixel
	MaxDepth                  int     // Maximum ray bounce depth
	RussianRouletteMinBounces int     // Minimum bounces before Russian Roulette can activate
	AdaptiveMinSamples        float64 // Minimum samples as percentage of max samples (0.0-1.0)
	AdaptiveThreshold         float64 // Relative error threshold for adaptive convergence (0.01 = 1%)
}

// ScatterResult contains the result of material scattering
type ScatterResult struct {
	Incoming    Ray     // The incoming ray
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
