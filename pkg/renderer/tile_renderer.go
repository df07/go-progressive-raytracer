package renderer

import (
	"image"
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TileRenderer handles the actual rendering of individual tiles using an integrator
type TileRenderer struct {
	scene      core.Scene
	integrator core.Integrator
}

// NewTileRenderer creates a new tile renderer with the given scene and integrator
func NewTileRenderer(scene core.Scene, integratorInst core.Integrator) *TileRenderer {
	return &TileRenderer{
		scene:      scene,
		integrator: integratorInst,
	}
}

// RenderTileBounds renders pixels within the specified bounds using the integrator
func (tr *TileRenderer) RenderTileBounds(bounds image.Rectangle, pixelStats [][]PixelStats, random *rand.Rand, targetSamples int) RenderStats {
	camera := tr.scene.GetCamera()
	samplingConfig := tr.scene.GetSamplingConfig()

	// Initialize statistics tracking for this specific bounds
	stats := tr.initRenderStatsForBounds(bounds, targetSamples)

	for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
		for i := bounds.Min.X; i < bounds.Max.X; i++ {
			samplesUsed := tr.adaptiveSamplePixelWithIntegrator(camera, i, j, &pixelStats[j][i], random, targetSamples, samplingConfig)
			tr.updateStats(&stats, samplesUsed)
		}
	}

	// Finalize statistics
	tr.finalizeStats(&stats)
	return stats
}

// adaptiveSamplePixelWithIntegrator uses adaptive sampling with the integrator
func (tr *TileRenderer) adaptiveSamplePixelWithIntegrator(camera core.Camera, i, j int, ps *PixelStats, random *rand.Rand, maxSamples int, samplingConfig core.SamplingConfig) int {
	initialSampleCount := ps.SampleCount

	// Take samples until we reach convergence or max samples
	for ps.SampleCount < maxSamples && !tr.shouldStopSampling(ps, maxSamples, samplingConfig) {
		ray := camera.GetRay(i, j, random)
		// Use integrator to compute color with proper depth and throughput
		color, _ := tr.integrator.RayColor(ray, tr.scene, random, ps.SampleCount)
		// TODO: handle splat rays
		ps.AddSample(color)
	}

	return ps.SampleCount - initialSampleCount
}

// shouldStopSampling determines if adaptive sampling should stop based on perceptual relative error
func (tr *TileRenderer) shouldStopSampling(ps *PixelStats, maxSamples int, samplingConfig core.SamplingConfig) bool {
	// Calculate minimum samples as percentage of max samples, but ensure at least 1 sample
	minSamples := max(1, int(float64(maxSamples)*samplingConfig.AdaptiveMinSamples))

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
		return variance < 1e-6 // Hardcoded epsilon for dark pixels
	}

	// Calculate coefficient of variation (relative error)
	relativeError := math.Sqrt(variance) / mean

	// Stop when relative error is below configured threshold
	return relativeError < samplingConfig.AdaptiveThreshold
}

// initRenderStatsForBounds initializes the render statistics tracking for specific bounds
func (tr *TileRenderer) initRenderStatsForBounds(bounds image.Rectangle, maxSamples int) RenderStats {
	pixelCount := bounds.Dx() * bounds.Dy()
	return RenderStats{
		TotalPixels:    pixelCount,
		TotalSamples:   0,
		AverageSamples: 0,
		MaxSamples:     maxSamples,
		MinSamples:     maxSamples, // Start with max, will be reduced
		MaxSamplesUsed: 0,
	}
}

// updateStats updates the render statistics with data from a single pixel
func (tr *TileRenderer) updateStats(stats *RenderStats, samplesUsed int) {
	stats.TotalSamples += samplesUsed
	stats.MinSamples = min(stats.MinSamples, samplesUsed)
	stats.MaxSamplesUsed = max(stats.MaxSamplesUsed, samplesUsed)
}

// finalizeStats calculates final statistics after all pixels are rendered
func (tr *TileRenderer) finalizeStats(stats *RenderStats) {
	stats.AverageSamples = float64(stats.TotalSamples) / float64(stats.TotalPixels)
}
