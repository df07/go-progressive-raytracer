package renderer

import (
	"image"

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

// AddSplat adds light from bidirectional path connections without affecting sampling statistics
// Splats represent deterministic light contributions discovered through BDPT connections,
// not stochastic samples from the primary sampling distribution. Including them in luminance
// statistics would corrupt variance calculations since:
// 1. Splats arrive after samples are processed, making attribution timing unclear
// 2. Multiple splats would be incorrectly attributed to the "last sample"
// 3. Race conditions make sample-to-splat mapping unreliable
// This may cause adaptive sampling to think pixels have "converged" prematurely when
// splats contribute significant light, but this is preferable to corrupted variance stats.
func (ps *PixelStats) AddSplat(color core.Vec3) {
	ps.ColorAccum = ps.ColorAccum.Add(color)
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

// CalculateAverageLuminance calculates the average luminance of an image
// It requires *image.RGBA as that is the standard format for this raytracer
func CalculateAverageLuminance(img *image.RGBA) float64 {
	if img == nil {
		return 0.0
	}

	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y
	totalPixels := width * height

	if totalPixels == 0 {
		return 0.0
	}

	var totalLuminance float64

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Note: image.RGBA stores premultiplied alpha, but for raytracer output alpha is usually 255.
			r, g, b, _ := img.At(x, y).RGBA()

			// Convert from 16-bit to float [0, 1]
			// RGBA() returns values in [0, 65535]
			rF := float64(r) / 65535.0
			gF := float64(g) / 65535.0
			bF := float64(b) / 65535.0

			color := core.Vec3{X: rF, Y: gF, Z: bF}
			totalLuminance += color.Luminance()
		}
	}

	return totalLuminance / float64(totalPixels)
}
