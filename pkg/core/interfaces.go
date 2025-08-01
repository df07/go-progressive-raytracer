package core

// SplatRay represents a ray-based color contribution that needs to be mapped to pixels
type SplatRay struct {
	Ray   Ray  // Ray that should contribute to some pixel
	Color Vec3 // Color contribution
}

// Integrator defines the interface for light transport algorithms
type Integrator interface {
	// RayColor computes color for a ray, with support for ray-based splatting
	// Returns (pixel color, splat rays)
	RayColor(ray Ray, scene Scene, sampler Sampler) (Vec3, []SplatRay)
}

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

type LightType string

const (
	LightTypeArea  LightType = "area"
	LightTypePoint LightType = "point"
)

// Light interface for objects that can be sampled for direct lighting
type Light interface {
	Type() LightType

	// Sample samples light toward a specific point for direct lighting
	// Returns LightSample with direction FROM shading point TO light
	Sample(point Vec3, sample Vec2) LightSample

	// PDF calculates the probability density for sampling a given direction toward the light
	PDF(point Vec3, direction Vec3) float64

	// SampleEmission samples emission from the light surface for BDPT light path generation
	// Returns EmissionSample with direction FROM light surface (for light transport)
	SampleEmission(samplePoint Vec2, sampleDirection Vec2) EmissionSample

	// EmissionPDF calculates PDF for emission sampling - needed for BDPT MIS calculations
	EmissionPDF(point Vec3, direction Vec3) float64
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

// CameraSample represents camera sampling result for t=1 strategies
type CameraSample struct {
	Ray    Ray     // Ray from camera toward reference point
	Weight Vec3    // Camera importance weight (We function result)
	PDF    float64 // Probability density for this sample
}

// Camera interface for cameras to avoid circular imports
type Camera interface {
	GetRay(i, j int, samplePoint Vec2, sampleJitter Vec2) Ray

	// BDPT support: calculate area and direction PDFs for a camera ray
	CalculateRayPDFs(ray Ray) (areaPDF, directionPDF float64)

	// Get camera forward direction for BDPT calculations
	GetCameraForward() Vec3

	// Sample camera from a reference point for t=1 strategies
	// Camera handles lens sampling internally, returns complete sample
	SampleCameraFromPoint(refPoint Vec3, samplePoint Vec2) *CameraSample

	// Map ray back to pixel coordinates (for splat placement)
	MapRayToPixel(ray Ray) (x, y int, ok bool)

	// Verbose logging bool
	SetVerbose(verbose bool)
}

// Scene interface for scene management
type Scene interface {
	GetCamera() Camera
	GetBackgroundColors() (topColor, bottomColor Vec3)
	GetShapes() []Shape
	GetLights() []Light
	GetSamplingConfig() SamplingConfig
	GetBVH() *BVH // For integrators to access acceleration structure
}

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
