package renderer

import "github.com/df07/go-progressive-raytracer/pkg/core"

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
