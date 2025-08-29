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
	Sample(point Vec3, normal Vec3, sample Vec2) LightSample

	// PDF calculates the probability density for sampling a given direction toward the light
	// For infinite lights: surfaceNormal needed to compute cosine-weighted PDF
	PDF(point Vec3, normal Vec3, direction Vec3) float64

	// SampleEmission samples emission from the light surface for BDPT light path generation
	// Returns EmissionSample with direction FROM light surface (for light transport)
	SampleEmission(samplePoint Vec2, sampleDirection Vec2) EmissionSample

	// EmissionPDF calculates PDF for emission sampling - needed for BDPT MIS calculations
	EmissionPDF(point Vec3, direction Vec3) float64

	// Emit evaluates emission in the direction of the given ray
	// For finite lights, returns zero. For infinite lights, returns emission based on ray direction.
	Emit(ray Ray) Vec3
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

// EmissionSample contains information about a sampled emission for BDPT light path generation
type EmissionSample struct {
	Point        Vec3    // Point on the light surface
	Normal       Vec3    // Surface normal at the emission point (outward facing)
	Direction    Vec3    // Emission direction FROM the surface (cosine-weighted hemisphere)
	Emission     Vec3    // Emitted radiance at this point and direction
	AreaPDF      float64 // PDF for position sampling (per unit area)
	DirectionPDF float64
}

// LightSampler interface for different light sampling strategies
type LightSampler interface {
	// SampleLight selects a light for the given surface point and returns the light, selection probability, and light index
	SampleLight(point Vec3, normal Vec3, u float64) (Light, float64, int)

	// SampleLightEmission selects a light for emission sampling and returns the light, selection probability, and light index
	SampleLightEmission(u float64) (Light, float64, int)

	// GetLightProbability returns the selection probability for a specific light at a surface point
	GetLightProbability(lightIndex int, point Vec3, normal Vec3) float64

	// GetLightCount returns the number of lights in this sampler
	GetLightCount() int
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
