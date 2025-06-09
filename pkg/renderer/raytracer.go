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

// PixelStats tracks sampling statistics for a single pixel
type PixelStats struct {
	ColorAccum       core.Vec3 // RGB accumulator for final result
	LuminanceAccum   float64   // Luminance accumulator for convergence
	LuminanceSqAccum float64   // Luminance squared for variance
	SampleCount      int       // Number of samples taken
}

// AddSample adds a new color sample to the pixel statistics
func (ps *PixelStats) AddSample(color core.Vec3) {
	ps.ColorAccum = ps.ColorAccum.Add(color)
	luminance := color.Luminance()
	ps.LuminanceAccum += luminance
	ps.LuminanceSqAccum += luminance * luminance
	ps.SampleCount++
}

// GetColor returns the current average color for this pixel
func (ps *PixelStats) GetColor() core.Vec3 {
	if ps.SampleCount == 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}
	return ps.ColorAccum.Multiply(1.0 / float64(ps.SampleCount))
}

// Raytracer handles the rendering process
type Raytracer struct {
	scene  core.Scene
	width  int
	height int
	config core.SamplingConfig
	bvh    *core.BVH // BVH for fast ray-object intersection
}

// NewRaytracer creates a new raytracer
func NewRaytracer(scene core.Scene, width, height int) *Raytracer {
	samplingConfig := scene.GetSamplingConfig()

	// Build BVH from scene shapes for fast intersection
	shapes := scene.GetShapes()
	bvh := core.NewBVH(shapes)

	return &Raytracer{
		scene:  scene,
		width:  width,
		height: height,
		config: samplingConfig,
		bvh:    bvh,
	}
}

// MergeSamplingConfig updates only the non-zero fields from the provided config
func (rt *Raytracer) MergeSamplingConfig(updates core.SamplingConfig) {
	if updates.SamplesPerPixel != 0 {
		rt.config.SamplesPerPixel = updates.SamplesPerPixel
	}
	if updates.MaxDepth != 0 {
		rt.config.MaxDepth = updates.MaxDepth
	}
	if updates.RussianRouletteMinBounces != 0 {
		rt.config.RussianRouletteMinBounces = updates.RussianRouletteMinBounces
	}
	if updates.RussianRouletteMinSamples != 0 {
		rt.config.RussianRouletteMinSamples = updates.RussianRouletteMinSamples
	}
}

// GetSamplingConfig returns the current sampling configuration
func (rt *Raytracer) GetSamplingConfig() core.SamplingConfig {
	return rt.config
}

// SetSamplingConfig updates the sampling configuration
func (rt *Raytracer) SetSamplingConfig(config core.SamplingConfig) {
	rt.config = config
}

// hitWorld checks if a ray hits any object in the scene using BVH
func (rt *Raytracer) hitWorld(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	return rt.bvh.Hit(ray, tMin, tMax)
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

// calculateSpecularColor handles specular material scattering with the provided random generator
func (rt *Raytracer) calculateSpecularColor(scatter core.ScatterResult, depth int, throughput core.Vec3, sampleIndex int, random *rand.Rand) core.Vec3 {
	// Update throughput with material attenuation
	newThroughput := throughput.MultiplyVec(scatter.Attenuation)
	return scatter.Attenuation.MultiplyVec(
		rt.rayColorRecursive(scatter.Scattered, depth-1, newThroughput, sampleIndex, random))
}

// applyRussianRoulette determines if a ray should be terminated and returns the compensation factor
// Returns (shouldTerminate, compensationFactor)
func (rt *Raytracer) applyRussianRoulette(depth int, throughput core.Vec3, sampleIndex int, random *rand.Rand) (bool, float64) {
	// Apply Russian Roulette after minimum bounces AND minimum samples per pixel
	initialDepth := rt.config.MaxDepth
	currentBounce := initialDepth - depth

	shouldApplyRR := currentBounce >= rt.config.RussianRouletteMinBounces && sampleIndex >= rt.config.RussianRouletteMinSamples

	if !shouldApplyRR {
		return false, 1.0 // Don't terminate, no compensation needed
	}

	// Calculate survival probability based on throughput
	// Use luminance for perceptually accurate survival probability
	luminance := throughput.Luminance()

	// Conservative bounds: survivalProb between 0.5 and 0.95
	// This naturally limits compensation factor to between 1.05x and 2.0x
	survivalProb := math.Min(0.95, math.Max(0.5, luminance))

	// Russian Roulette test
	if random.Float64() > survivalProb {
		return true, 0.0 // Terminate ray
	}

	// Energy-conserving compensation (no artificial cap)
	compensationFactor := 1.0 / survivalProb
	return false, compensationFactor
}

// rayColorRecursive returns the color for a given ray with material support and Russian Roulette termination
func (rt *Raytracer) rayColorRecursive(r core.Ray, depth int, throughput core.Vec3, sampleIndex int, random *rand.Rand) core.Vec3 {
	// If we've exceeded the ray bounce limit, no more light is gathered
	if depth <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Apply Russian Roulette termination
	shouldTerminate, rrCompensation := rt.applyRussianRoulette(depth, throughput, sampleIndex, random)
	if shouldTerminate {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check for intersections with objects
	hit, isHit := rt.hitWorld(r, 0.001, 1000.0)
	if !isHit {
		bgColor := rt.backgroundGradient(r)
		return bgColor.Multiply(rrCompensation)
	}

	// Start with emitted light from the hit material
	colorEmitted := rt.getEmittedLight(hit)

	// Try to scatter the ray
	scatter, didScatter := hit.Material.Scatter(r, *hit, random)
	if !didScatter {
		// Material absorbed the ray, only return emitted light
		return colorEmitted.Multiply(rrCompensation)
	}

	// Handle scattering based on material type
	var colorScattered core.Vec3
	if scatter.IsSpecular() {
		colorScattered = rt.calculateSpecularColor(scatter, depth, throughput, sampleIndex, random)
	} else {
		colorScattered = rt.calculateDiffuseColor(scatter, hit, depth, throughput, sampleIndex, random)
	}

	// Apply Russian Roulette compensation to the final result
	finalColor := colorEmitted.Add(colorScattered)
	return finalColor.Multiply(rrCompensation)
}

// calculateDiffuseColor handles diffuse material scattering with throughput tracking
func (rt *Raytracer) calculateDiffuseColor(scatter core.ScatterResult, hit *core.HitRecord, depth int, throughput core.Vec3, sampleIndex int, random *rand.Rand) core.Vec3 {
	// Combine direct lighting and indirect lighting using Multiple Importance Sampling
	directLight := rt.calculateDirectLighting(rt.scene, scatter, hit, random)
	indirectLight := rt.calculateIndirectLighting(rt.scene, scatter, hit, depth, throughput, sampleIndex, random)
	return directLight.Add(indirectLight)
}

// getEmittedLight returns the emitted light from a material if it's emissive
func (rt *Raytracer) getEmittedLight(hit *core.HitRecord) core.Vec3 {
	if emitter, isEmissive := hit.Material.(core.Emitter); isEmissive {
		return emitter.Emit()
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// calculateDirectLighting samples lights directly for direct illumination with the provided random generator
func (rt *Raytracer) calculateDirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord, random *rand.Rand) core.Vec3 {
	lights := scene.GetLights()

	// Sample a light
	lightSample, hasLight := core.SampleLight(lights, hit.Point, random)
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

// calculateIndirectLighting handles indirect illumination via material sampling with throughput tracking
func (rt *Raytracer) calculateIndirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord, depth int, throughput core.Vec3, sampleIndex int, random *rand.Rand) core.Vec3 {
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

	// Update throughput for the recursive call
	newThroughput := throughput.MultiplyVec(scatter.Attenuation).Multiply(cosine / scatter.PDF)

	// Get incoming light from the scattered direction with throughput tracking
	incomingLight := rt.rayColorRecursive(scatter.Scattered, depth-1, newThroughput, sampleIndex, random)

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

// RenderBounds renders pixels within the specified bounds using the provided pixel stats and random generator
func (rt *Raytracer) RenderBounds(bounds image.Rectangle, pixelStats [][]PixelStats, random *rand.Rand) RenderStats {
	camera := rt.scene.GetCamera()

	// Initialize statistics tracking for this specific bounds
	stats := rt.initRenderStatsForBounds(bounds)

	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			samplesUsed := rt.adaptiveSamplePixel(camera, i, j, &pixelStats[j][i], random)
			rt.updateStats(&stats, samplesUsed)
		}
	}

	// Finalize statistics
	rt.finalizeStats(&stats)
	return stats
}

// adaptiveSamplePixel uses adaptive sampling to sample a pixel up to the configured maximum.
// It continues sampling until either the maximum is reached or adaptive convergence is achieved.
// Returns the number of samples actually added this call.
func (rt *Raytracer) adaptiveSamplePixel(camera core.Camera, i, j int, ps *PixelStats, random *rand.Rand) int {
	initialSampleCount := ps.SampleCount
	maxSamples := rt.config.SamplesPerPixel

	// Take samples until we reach convergence or max samples
	for ps.SampleCount < maxSamples && !rt.shouldStopSampling(ps) {
		ray := camera.GetRay(i, j, random)
		// Use sample-aware Russian Roulette that protects early samples
		color := rt.rayColorRecursive(ray, rt.config.MaxDepth, core.Vec3{X: 1.0, Y: 1.0, Z: 1.0}, ps.SampleCount, random)
		ps.AddSample(color)
	}

	return ps.SampleCount - initialSampleCount
}

// shouldStopSampling determines if adaptive sampling should stop based on perceptual relative error
func (rt *Raytracer) shouldStopSampling(ps *PixelStats) bool {
	minSamples := rt.config.AdaptiveMinSamples

	// Don't stop before minimum samples
	if ps.SampleCount < minSamples {
		return false
	}

	// Calculate variance from accumulated statistics
	mean := ps.LuminanceAccum / float64(ps.SampleCount)
	meanSq := ps.LuminanceSqAccum / float64(ps.SampleCount)
	variance := math.Max(0, meanSq-mean*mean)

	// Avoid division by zero for black pixels
	if mean <= 1e-8 {
		return variance < rt.config.AdaptiveDarkThreshold
	}

	// Calculate coefficient of variation (relative error)
	relativeError := math.Sqrt(variance) / mean

	// Stop when relative error is below configured threshold
	return relativeError < rt.config.AdaptiveThreshold
}

// initRenderStatsForBounds initializes the render statistics tracking for specific bounds
func (rt *Raytracer) initRenderStatsForBounds(bounds image.Rectangle) RenderStats {
	pixelCount := bounds.Dx() * bounds.Dy()
	return RenderStats{
		TotalPixels:    pixelCount,
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
	stats.MinSamples = min(stats.MinSamples, samplesUsed)
	stats.MaxSamplesUsed = max(stats.MaxSamplesUsed, samplesUsed)
}

// finalizeStats calculates final statistics after all pixels are rendered
func (rt *Raytracer) finalizeStats(stats *RenderStats) {
	stats.AverageSamples = float64(stats.TotalSamples) / float64(stats.TotalPixels)
}

// RenderPass renders a single pass with adaptive sampling and returns an image and statistics.
// Adaptive sampling automatically adjusts the number of samples per pixel based on variance,
// using fewer samples for smooth areas and more samples for noisy/complex areas.
func (rt *Raytracer) RenderPass() (*image.RGBA, RenderStats) {
	random := rand.New(rand.NewSource(42))

	// Initialize pixel statistics for all pixels
	pixelStats := make([][]PixelStats, rt.height)
	for j := range pixelStats {
		pixelStats[j] = make([]PixelStats, rt.width)
	}

	// Render the entire image bounds
	bounds := image.Rect(0, 0, rt.width, rt.height)
	stats := rt.RenderBounds(bounds, pixelStats, random)

	// Create final image from pixel stats
	img := image.NewRGBA(bounds)
	for j := 0; j < rt.height; j++ {
		for i := 0; i < rt.width; i++ {
			colorVec := pixelStats[j][i].GetColor()
			pixelColor := rt.vec3ToColor(colorVec)
			img.SetRGBA(i, j, pixelColor)
		}
	}

	return img, stats
}
