package renderer

import (
	"image"
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// MockIntegrator for testing
type MockIntegrator struct {
	returnColor core.Vec3
	callCount   int
}

func (m *MockIntegrator) RayColor(ray core.Ray, scene core.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay) {
	m.callCount++
	return m.returnColor, nil
}

// MockScene for tile renderer testing
type MockScene struct {
	width       int
	height      int
	shapes      []core.Shape
	lights      []core.Light
	topColor    core.Vec3
	bottomColor core.Vec3
	camera      core.Camera
	config      core.SamplingConfig
	bvh         *core.BVH
}

func (m *MockScene) GetWidth() int                               { return m.width }
func (m *MockScene) GetHeight() int                              { return m.height }
func (m *MockScene) GetCamera() core.Camera                      { return m.camera }
func (m *MockScene) GetBackgroundColors() (core.Vec3, core.Vec3) { return m.topColor, m.bottomColor }
func (m *MockScene) GetShapes() []core.Shape                     { return m.shapes }
func (m *MockScene) GetLights() []core.Light                     { return m.lights }
func (m *MockScene) GetSamplingConfig() core.SamplingConfig      { return m.config }
func (m *MockScene) GetBVH() *core.BVH {
	if m.bvh == nil {
		m.bvh = core.NewBVH(m.shapes)
	}
	return m.bvh
}

// createMockScene creates a simple test scene
func createMockScene() *MockScene {
	// Simple camera
	cameraConfig := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       100,
		AspectRatio: 1.0,
		VFov:        45.0,
		Aperture:    0.0,
	}
	camera := NewCamera(cameraConfig)

	// Simple sphere
	lambertian := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	return &MockScene{
		shapes: []core.Shape{sphere},
		lights: []core.Light{},
		camera: camera,
		config: core.SamplingConfig{
			MaxDepth:           10,
			AdaptiveMinSamples: 0.1,
			AdaptiveThreshold:  0.05,
		},
	}
}

// TestTileRendererCreation tests basic tile renderer creation
func TestTileRendererCreation(t *testing.T) {
	scene := createMockScene()
	mockIntegrator := &MockIntegrator{returnColor: core.NewVec3(0.5, 0.5, 0.5)}

	renderer := NewTileRenderer(scene, mockIntegrator)

	if renderer == nil {
		t.Fatal("Expected non-nil tile renderer")
	}

	if renderer.scene != scene {
		t.Error("Expected tile renderer to store scene reference")
	}

	if renderer.integrator != mockIntegrator {
		t.Error("Expected tile renderer to store integrator reference")
	}
}

// TestTileRendererPixelSampling tests that the tile renderer calls the integrator
func TestTileRendererPixelSampling(t *testing.T) {
	scene := createMockScene()
	mockIntegrator := &MockIntegrator{returnColor: core.NewVec3(0.7, 0.3, 0.1)}
	renderer := NewTileRenderer(scene, mockIntegrator)

	// Create a small tile (2x2 pixels)
	bounds := image.Rect(0, 0, 2, 2)
	pixelStats := make([][]PixelStats, 2)
	for i := range pixelStats {
		pixelStats[i] = make([]PixelStats, 2)
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	targetSamples := 4

	// Render the tile
	queue := NewSplatQueue()
	stats := renderer.RenderTileBounds(bounds, pixelStats, queue, sampler, targetSamples)

	// Check that integrator was called
	if mockIntegrator.callCount == 0 {
		t.Error("Expected integrator to be called")
	}

	// Check render stats
	if stats.TotalPixels != 4 {
		t.Errorf("Expected 4 pixels, got %d", stats.TotalPixels)
	}

	if stats.MaxSamples != targetSamples {
		t.Errorf("Expected max samples %d, got %d", targetSamples, stats.MaxSamples)
	}

	// Check that pixels have samples
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			if pixelStats[y][x].SampleCount == 0 {
				t.Errorf("Expected pixel [%d][%d] to have samples", y, x)
			}

			// Check that color was accumulated
			color := pixelStats[y][x].GetColor()
			if color == (core.Vec3{}) {
				t.Errorf("Expected pixel [%d][%d] to have color", y, x)
			}
		}
	}
}

// TestTileRendererAdaptiveSampling tests adaptive sampling behavior
func TestTileRendererAdaptiveSampling(t *testing.T) {
	scene := createMockScene()

	// Configure for very low adaptive threshold to test convergence
	scene.config.AdaptiveMinSamples = 0.1  // 10% minimum
	scene.config.AdaptiveThreshold = 0.001 // Very low threshold

	// Integrator that returns consistent color (should converge quickly)
	consistentIntegrator := &MockIntegrator{returnColor: core.NewVec3(0.5, 0.5, 0.5)}
	renderer := NewTileRenderer(scene, consistentIntegrator)

	// Single pixel
	bounds := image.Rect(0, 0, 1, 1)
	pixelStats := make([][]PixelStats, 1)
	pixelStats[0] = make([]PixelStats, 1)

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	targetSamples := 100 // High target

	queue := NewSplatQueue()
	stats := renderer.RenderTileBounds(bounds, pixelStats, queue, sampler, targetSamples)

	// With consistent color, adaptive sampling should stop early
	actualSamples := pixelStats[0][0].SampleCount

	// Verify stats are reasonable
	if stats.TotalPixels != 1 {
		t.Errorf("Expected 1 pixel, got %d", stats.TotalPixels)
	}
	if actualSamples >= targetSamples {
		t.Errorf("Expected adaptive sampling to stop early, but used %d/%d samples", actualSamples, targetSamples)
	}

	// Should have taken at least minimum samples
	minSamples := int(float64(targetSamples) * scene.config.AdaptiveMinSamples)
	if actualSamples < minSamples {
		t.Errorf("Expected at least %d samples (minimum), got %d", minSamples, actualSamples)
	}
}

// TestTileRendererStatistics tests that render statistics are calculated correctly
func TestTileRendererStatistics(t *testing.T) {
	scene := createMockScene()
	mockIntegrator := &MockIntegrator{returnColor: core.NewVec3(0.4, 0.6, 0.2)}
	renderer := NewTileRenderer(scene, mockIntegrator)

	// 3x2 tile
	bounds := image.Rect(0, 0, 3, 2)
	pixelStats := make([][]PixelStats, 2)
	for i := range pixelStats {
		pixelStats[i] = make([]PixelStats, 3)
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	targetSamples := 5

	queue := NewSplatQueue()
	stats := renderer.RenderTileBounds(bounds, pixelStats, queue, sampler, targetSamples)

	// Check basic statistics
	expectedPixels := 6
	if stats.TotalPixels != expectedPixels {
		t.Errorf("Expected %d pixels, got %d", expectedPixels, stats.TotalPixels)
	}

	if stats.TotalSamples == 0 {
		t.Error("Expected non-zero total samples")
	}

	if stats.AverageSamples <= 0 {
		t.Error("Expected positive average samples")
	}

	if stats.MaxSamplesUsed == 0 {
		t.Error("Expected non-zero max samples used")
	}

	if stats.MinSamples > stats.MaxSamplesUsed {
		t.Error("Expected min samples <= max samples")
	}

	// Average should be total/pixels
	expectedAverage := float64(stats.TotalSamples) / float64(stats.TotalPixels)
	if math.Abs(stats.AverageSamples-expectedAverage) > 0.001 {
		t.Errorf("Expected average %f, got %f", expectedAverage, stats.AverageSamples)
	}
}

// TestTileRendererDeterministic tests that identical seeds produce identical results
func TestTileRendererDeterministic(t *testing.T) {
	scene := createMockScene()

	// Use real integrator for more realistic test
	pathIntegrator := integrator.NewPathTracingIntegrator(scene.GetSamplingConfig())
	renderer := NewTileRenderer(scene, pathIntegrator)

	bounds := image.Rect(0, 0, 2, 2)
	targetSamples := 3

	// First render
	pixelStats1 := make([][]PixelStats, 2)
	for i := range pixelStats1 {
		pixelStats1[i] = make([]PixelStats, 2)
	}
	sampler1 := core.NewRandomSampler(rand.New(rand.NewSource(123)))
	queue1 := NewSplatQueue()
	stats1 := renderer.RenderTileBounds(bounds, pixelStats1, queue1, sampler1, targetSamples)

	// Second render with same seed
	pixelStats2 := make([][]PixelStats, 2)
	for i := range pixelStats2 {
		pixelStats2[i] = make([]PixelStats, 2)
	}
	sampler2 := core.NewRandomSampler(rand.New(rand.NewSource(123)))
	queue2 := NewSplatQueue()
	stats2 := renderer.RenderTileBounds(bounds, pixelStats2, queue2, sampler2, targetSamples)

	// Results should be identical
	if stats1.TotalSamples != stats2.TotalSamples {
		t.Errorf("Expected same total samples, got %d and %d", stats1.TotalSamples, stats2.TotalSamples)
	}

	// Pixel colors should be identical
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			color1 := pixelStats1[y][x].GetColor()
			color2 := pixelStats2[y][x].GetColor()
			if color1 != color2 {
				t.Errorf("Expected identical colors for pixel [%d][%d], got %v and %v", y, x, color1, color2)
			}
		}
	}
}

// TestTileRendererBoundsClipping tests that rendering respects tile bounds
func TestTileRendererBoundsClipping(t *testing.T) {
	scene := createMockScene()
	mockIntegrator := &MockIntegrator{returnColor: core.NewVec3(1.0, 0.0, 0.0)}
	renderer := NewTileRenderer(scene, mockIntegrator)

	// Create large pixel stats array
	pixelStats := make([][]PixelStats, 5)
	for i := range pixelStats {
		pixelStats[i] = make([]PixelStats, 5)
	}

	// Only render a 2x2 subset
	bounds := image.Rect(1, 1, 3, 3)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	queue := NewSplatQueue()
	stats := renderer.RenderTileBounds(bounds, pixelStats, queue, sampler, 2)

	// Should only have processed 4 pixels
	if stats.TotalPixels != 4 {
		t.Errorf("Expected 4 pixels processed, got %d", stats.TotalPixels)
	}

	// Only pixels within bounds should have samples
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			inBounds := (x >= 1 && x < 3 && y >= 1 && y < 3)
			hasSamples := pixelStats[y][x].SampleCount > 0

			if inBounds && !hasSamples {
				t.Errorf("Expected pixel [%d][%d] in bounds to have samples", y, x)
			}
			if !inBounds && hasSamples {
				t.Errorf("Expected pixel [%d][%d] outside bounds to have no samples", y, x)
			}
		}
	}
}
