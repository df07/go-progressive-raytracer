package renderer

import (
	"image"
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// testLogger implements core.Logger for testing by discarding all output
type testLogger struct{}

// Ensure testLogger implements core.Logger
var _ core.Logger = (*testLogger)(nil)

func (tl *testLogger) Printf(format string, args ...interface{}) {
	// Discard log output during tests
}

func TestBDPTPathTracingLuminanceComparison(t *testing.T) {
	// Create Cornell box scene with empty geometry (no boxes/spheres)
	cornellScene := scene.NewCornellScene(scene.CornellEmpty)

	// Override scene image size for fast testing
	cornellScene.SamplingConfig.Width = 32
	cornellScene.SamplingConfig.Height = 32

	// Configure progressive rendering with minimal samples for quick test
	config := DefaultProgressiveConfig()
	config.InitialSamples = 1
	config.MaxSamplesPerPixel = 4 // Very low for fast test
	config.MaxPasses = 1          // Single pass
	config.TileSize = 32          // Single tile for 32x32 image

	logger := &testLogger{}

	// Test path tracing
	pathIntegrator := integrator.NewPathTracingIntegrator(cornellScene.SamplingConfig)
	pathRenderer, err := NewProgressiveRaytracer(cornellScene, config, pathIntegrator, logger)
	if err != nil {
		t.Fatalf("Failed to create path tracing renderer: %v", err)
	}

	pathImage, _, err := pathRenderer.RenderPass(1, nil)
	if err != nil {
		t.Fatalf("Path tracing render failed: %v", err)
	}
	pathLuminance := calculateAverageLuminance(pathImage)

	// Test BDPT
	bdptIntegrator := integrator.NewBDPTIntegrator(cornellScene.SamplingConfig)
	bdptRenderer, err := NewProgressiveRaytracer(cornellScene, config, bdptIntegrator, logger)
	if err != nil {
		t.Fatalf("Failed to create BDPT renderer: %v", err)
	}

	bdptImage, _, err := bdptRenderer.RenderPass(1, nil)
	if err != nil {
		t.Fatalf("BDPT render failed: %v", err)
	}
	bdptLuminance := calculateAverageLuminance(bdptImage)

	t.Logf("Path tracing luminance: %.6f", pathLuminance)
	t.Logf("BDPT luminance: %.6f", bdptLuminance)

	// Calculate percentage difference
	if pathLuminance == 0 && bdptLuminance == 0 {
		t.Fatal("Both renderers produced zero luminance - scene setup issue")
	}

	if pathLuminance == 0 {
		t.Fatal("Path tracing produced zero luminance")
	}

	percentDiff := math.Abs(bdptLuminance-pathLuminance) / pathLuminance * 100
	t.Logf("Luminance difference: %.2f%%", percentDiff)

	// Test should FAIL with current BDPT brightness issue (~25% difference)
	// Once the BDPT issue is fixed, this threshold should be lowered
	tolerance := 15.0 // 15% tolerance
	if percentDiff > tolerance {
		t.Errorf("BDPT and path tracing luminance differ by %.2f%%, exceeds %.1f%% tolerance. "+
			"BDPT: %.6f, Path tracing: %.6f",
			percentDiff, tolerance, bdptLuminance, pathLuminance)
	}
}

// calculateAverageLuminance computes the average luminance of an image
func calculateAverageLuminance(img *image.RGBA) float64 {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return 0.0
	}

	totalLuminance := 0.0
	pixelCount := bounds.Dx() * bounds.Dy()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			// Convert to normalized RGB values
			r := float64(c.R) / 255.0
			g := float64(c.G) / 255.0
			b := float64(c.B) / 255.0
			// Calculate luminance using standard formula
			luminance := 0.299*r + 0.587*g + 0.114*b
			totalLuminance += luminance
		}
	}

	return totalLuminance / float64(pixelCount)
}
