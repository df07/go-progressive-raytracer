package renderer

import (
	"image"
	"image/color"
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// RenderStats contains statistics about the rendering process
type RenderStats struct {
	TotalPixels    int     // Total number of pixels rendered
	TotalSamples   int     // Total number of samples taken
	AverageSamples float64 // Average samples per pixel
	MaxSamples     int     // Maximum samples allowed per pixel
	MinSamples     int     // Minimum samples taken per pixel
	MaxSamplesUsed int     // Maximum samples actually used by any pixel
}

// SamplingConfig contains rendering configuration
type SamplingConfig struct {
	SamplesPerPixel int // Number of rays per pixel
	MaxDepth        int // Maximum ray bounce depth
}

// DefaultSamplingConfig returns sensible default values
func DefaultSamplingConfig() SamplingConfig {
	return SamplingConfig{
		SamplesPerPixel: 200,
		MaxDepth:        50,
	}
}

// Raytracer handles the rendering process
type Raytracer struct {
	scene  core.Scene
	width  int
	height int
	config SamplingConfig
	random *rand.Rand
}

// NewRaytracer creates a new raytracer
func NewRaytracer(scene core.Scene, width, height int) *Raytracer {
	return &Raytracer{
		scene:  scene,
		width:  width,
		height: height,
		config: DefaultSamplingConfig(),
		random: rand.New(rand.NewSource(42)), // Deterministic for testing
	}
}

// SetSamplingConfig updates the sampling configuration
func (rt *Raytracer) SetSamplingConfig(config SamplingConfig) {
	rt.config = config
}

// hitWorld checks if a ray hits any object in the scene
func (rt *Raytracer) hitWorld(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	var closestHit *core.HitRecord
	closestSoFar := tMax
	hitAnything := false

	for _, shape := range rt.scene.GetShapes() {
		if hit, isHit := shape.Hit(ray, tMin, closestSoFar); isHit {
			hitAnything = true
			closestSoFar = hit.T
			closestHit = hit
		}
	}

	return closestHit, hitAnything
}

// backgroundGradient returns a gradient color based on ray direction
func (rt *Raytracer) backgroundGradient(r core.Ray) core.Vec3 {
	// Get colors from the scene
	topColor, bottomColor := rt.scene.GetBackgroundColors()

	// Normalize the ray direction to get consistent results
	unitDirection := r.Direction.Normalize()

	// Use the y-component to create a gradient (map from -1,1 to 0,1)
	t := 0.5 * (unitDirection.Y + 1.0)

	// Linear interpolation: (1-t)*bottom + t*top
	return bottomColor.Multiply(1.0 - t).Add(topColor.Multiply(t))
}

// calculateSpecularColor handles specular material scattering
func (rt *Raytracer) calculateSpecularColor(scatter core.ScatterResult, depth int) core.Vec3 {
	return scatter.Attenuation.MultiplyVec(
		rt.rayColorRecursive(scatter.Scattered, depth-1))
}

// rayColorRecursive returns the color for a given ray with material support
func (rt *Raytracer) rayColorRecursive(r core.Ray, depth int) core.Vec3 {
	// If we've exceeded the ray bounce limit, no more light is gathered
	if depth <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check for intersections with objects
	hit, isHit := rt.hitWorld(r, 0.001, 1000.0)
	if !isHit {
		return rt.backgroundGradient(r)
	}

	// Start with emitted light from the hit material
	colorEmitted := rt.getEmittedLight(hit)

	// Try to scatter the ray
	scatter, didScatter := hit.Material.Scatter(r, *hit, rt.random)
	if !didScatter {
		// Material absorbed the ray, only return emitted light
		return colorEmitted
	}

	// Handle scattering based on material type
	var colorScattered core.Vec3
	if scatter.IsSpecular() {
		colorScattered = rt.calculateSpecularColor(scatter, depth)
	} else {
		colorScattered = rt.calculateDiffuseColor(scatter, hit, depth)
	}

	// Return emitted + scattered light
	return colorEmitted.Add(colorScattered)
}

// calculateDiffuseColor handles diffuse material scattering
func (rt *Raytracer) calculateDiffuseColor(scatter core.ScatterResult, hit *core.HitRecord, depth int) core.Vec3 {
	// Combine direct lighting and indirect lighting using Multiple Importance Sampling
	directLight := rt.calculateDirectLighting(rt.scene, scatter, hit)
	indirectLight := rt.calculateIndirectLighting(rt.scene, scatter, hit, depth)
	return directLight.Add(indirectLight)
}

// getEmittedLight returns the emitted light from a material if it's emissive
func (rt *Raytracer) getEmittedLight(hit *core.HitRecord) core.Vec3 {
	if emitter, isEmissive := hit.Material.(core.Emitter); isEmissive {
		return emitter.Emit()
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// calculateDirectLighting samples lights directly for direct illumination
func (rt *Raytracer) calculateDirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord) core.Vec3 {
	lights := scene.GetLights()

	// Sample a light
	lightSample, hasLight := core.SampleLight(lights, hit.Point, rt.random)
	if !hasLight {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check if light is visible (shadow ray)
	shadowRay := core.NewRay(hit.Point, lightSample.Direction)
	_, blocked := rt.hitWorld(shadowRay, 0.001, lightSample.Distance-0.001)
	if blocked {
		// Light is blocked, no direct contribution
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Calculate the cosine factor
	cosine := lightSample.Direction.Dot(hit.Normal)
	if cosine <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Light is behind the surface
	}

	// Get material PDF for this direction (for MIS)
	materialPDF := cosine / math.Pi // Lambertian PDF: cos(θ)/π

	// Calculate MIS weight
	misWeight := core.PowerHeuristic(1, lightSample.PDF, 1, materialPDF)

	// Calculate BRDF value (for Lambertian: albedo/π)
	brdf := scatter.Attenuation

	// Direct lighting contribution: BRDF * emission * cosine * MIS_weight / light_PDF
	if lightSample.PDF > 0 {
		contribution := brdf.MultiplyVec(lightSample.Emission).Multiply(cosine * misWeight / lightSample.PDF)
		return contribution
	}

	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// calculateIndirectLighting handles indirect illumination via material sampling
func (rt *Raytracer) calculateIndirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord, depth int) core.Vec3 {
	if scatter.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	scatterDirection := scatter.Scattered.Direction.Normalize()
	cosine := scatterDirection.Dot(hit.Normal)
	if cosine <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Get light PDF for this direction (for MIS)
	lights := scene.GetLights()
	lightPDF := core.CalculateLightPDF(lights, hit.Point, scatterDirection)

	// Calculate MIS weight
	misWeight := core.PowerHeuristic(1, scatter.PDF, 1, lightPDF)

	// Get incoming light from the scattered direction
	incomingLight := rt.rayColorRecursive(scatter.Scattered, depth-1)

	// Indirect lighting contribution with MIS
	contribution := scatter.Attenuation.Multiply(cosine * misWeight / scatter.PDF).MultiplyVec(incomingLight)
	return contribution
}

// vec3ToColor converts a Vec3 color to RGBA with proper clamping and gamma correction
func (rt *Raytracer) vec3ToColor(colorVec core.Vec3) color.RGBA {
	// Apply gamma correction (gamma = 2.0)
	colorVec = colorVec.GammaCorrect(2.0)

	// Clamp to valid color range
	colorVec = colorVec.Clamp(0.0, 1.0)

	return color.RGBA{
		R: uint8(255 * colorVec.X),
		G: uint8(255 * colorVec.Y),
		B: uint8(255 * colorVec.Z),
		A: 255,
	}
}

// RenderPass renders a single pass with adaptive sampling and returns an image and statistics.
// Adaptive sampling automatically adjusts the number of samples per pixel based on variance,
// using fewer samples for smooth areas and more samples for noisy/complex areas.
func (rt *Raytracer) RenderPass() (*image.RGBA, RenderStats) {
	img := image.NewRGBA(image.Rect(0, 0, rt.width, rt.height))
	camera := rt.scene.GetCamera()

	// Initialize statistics tracking
	stats := rt.initRenderStats()

	for j := 0; j < rt.height; j++ {
		for i := 0; i < rt.width; i++ {
			// Use adaptive sampling for this pixel
			colorVec, samplesUsed := rt.adaptiveSamplePixel(camera, i, j)
			rt.updateStats(&stats, samplesUsed)

			pixelColor := rt.vec3ToColor(colorVec)
			img.SetRGBA(i, j, pixelColor)
		}
	}

	// Finalize statistics
	rt.finalizeStats(&stats)
	return img, stats
}

// adaptiveSamplePixel uses adaptive sampling to determine the optimal number of samples for a pixel.
// It starts with a minimum number of samples and continues sampling until the variance is low enough
// or the maximum sample count is reached. Returns the final color and number of samples used.
func (rt *Raytracer) adaptiveSamplePixel(camera core.Camera, i, j int) (core.Vec3, int) {
	maxSamples := rt.config.SamplesPerPixel

	colorAccum := core.Vec3{X: 0, Y: 0, Z: 0}
	colorSqAccum := core.Vec3{X: 0, Y: 0, Z: 0}
	samplesTaken := 0

	// Take samples until we reach convergence or max samples
	for samplesTaken < maxSamples && !rt.sampleConverged(colorAccum, colorSqAccum, samplesTaken) {
		samplesTaken++

		// Take a sample
		ray := camera.GetRay(i, j)
		color := rt.rayColorRecursive(ray, rt.config.MaxDepth)

		colorAccum = colorAccum.Add(color)
		colorSqAccum = colorSqAccum.Add(color.Square())
	}

	// Return result
	return colorAccum.Multiply(1.0 / float64(samplesTaken)), samplesTaken
}

// sampleConverged determines if adaptive sampling should stop based on variance
func (rt *Raytracer) sampleConverged(colorAccum, colorSqAccum core.Vec3, samplesTaken int) bool {
	minSamples := 8

	// Don't stop before minimum samples
	if samplesTaken < minSamples {
		return false
	}

	// Calculate current variance
	mean := colorAccum.Multiply(1.0 / float64(samplesTaken))
	meanSq := colorSqAccum.Multiply(1.0 / float64(samplesTaken))
	variance := core.Vec3{
		X: math.Max(0, meanSq.X-mean.X*mean.X),
		Y: math.Max(0, meanSq.Y-mean.Y*mean.Y),
		Z: math.Max(0, meanSq.Z-mean.Z*mean.Z),
	}

	// Calculate maximum variance across color channels
	maxVariance := math.Max(math.Max(variance.X, variance.Y), variance.Z)

	// Use adaptive threshold that scales with brightness
	brightness := (mean.X + mean.Y + mean.Z) / 3.0
	threshold := 0.001 + brightness*0.01

	return maxVariance < threshold
}

// initRenderStats initializes the render statistics tracking
func (rt *Raytracer) initRenderStats() RenderStats {
	return RenderStats{
		TotalPixels:    rt.width * rt.height,
		TotalSamples:   0,
		AverageSamples: 0,
		MaxSamples:     rt.config.SamplesPerPixel,
		MinSamples:     rt.config.SamplesPerPixel, // Start with max, will be reduced
		MaxSamplesUsed: 0,
	}
}

// updateStats updates the render statistics with data from a single pixel
func (rt *Raytracer) updateStats(stats *RenderStats, samplesUsed int) {
	stats.TotalSamples += samplesUsed

	if samplesUsed < stats.MinSamples {
		stats.MinSamples = samplesUsed
	}
	if samplesUsed > stats.MaxSamplesUsed {
		stats.MaxSamplesUsed = samplesUsed
	}
}

// finalizeStats calculates final statistics after all pixels are rendered
func (rt *Raytracer) finalizeStats(stats *RenderStats) {
	stats.AverageSamples = float64(stats.TotalSamples) / float64(stats.TotalPixels)
}
