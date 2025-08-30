package core

// Preprocessor interface for objects that need scene preprocessing
// Note: Most preprocessors now take parameters directly rather than full BVH

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
