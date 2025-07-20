package renderer

import (
	"image"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// MockIntegratorWithSplats creates splat rays to test the splat system
type MockIntegratorWithSplats struct{}

func (m *MockIntegratorWithSplats) RayColor(ray core.Ray, sceneObj core.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay) {
	// Regular pixel color (simple test pattern)
	pixelColor := core.Vec3{X: 0.2, Y: 0.4, Z: 0.6}

	// Create some test splat rays
	var splats []core.SplatRay

	// Add a splat ray for testing - ray pointing slightly off from original
	splatDirection := ray.Direction.Add(core.Vec3{X: 0.1, Y: 0.0, Z: 0.0}).Normalize()
	splatRay := core.SplatRay{
		Ray:   core.NewRay(ray.Origin, splatDirection),
		Color: core.Vec3{X: 0.8, Y: 0.2, Z: 0.1}, // Distinct color for splats
	}
	splats = append(splats, splatRay)

	return pixelColor, splats
}

func TestTileRendererWithSplats(t *testing.T) {
	// Create a simple test scene
	cameraConfig := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       10,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(cameraConfig)

	samplingConfig := core.SamplingConfig{
		Width:           10,
		Height:          10,
		SamplesPerPixel: 2,
		MaxDepth:        3,
	}

	sceneObj := &MockScene{
		camera: camera,
		config: samplingConfig,
	}

	// Create mock integrator that generates splats
	integrator := &MockIntegratorWithSplats{}

	// Create tile renderer
	tileRenderer := NewTileRenderer(sceneObj, integrator)

	// Create test image bounds
	width, height := 10, 10
	bounds := image.Rect(0, 0, width, height)

	// Initialize pixel stats
	pixelStats := make([][]PixelStats, height)
	for y := range pixelStats {
		pixelStats[y] = make([]PixelStats, width)
	}

	// Create splat queue
	splatQueue := NewSplatQueue()

	// Create random generator
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Render the tile
	stats := tileRenderer.RenderTileBounds(bounds, pixelStats, splatQueue, sampler, 2)

	// Verify render stats
	if stats.TotalPixels != width*height {
		t.Errorf("Expected %d total pixels, got %d", width*height, stats.TotalPixels)
	}

	if stats.TotalSamples == 0 {
		t.Error("Expected some samples to be taken")
	}

	// Verify pixel stats have been populated
	samplesFound := false
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if pixelStats[y][x].SampleCount > 0 {
				samplesFound = true

				// Check that we have the expected regular pixel color contribution
				color := pixelStats[y][x].GetColor()
				if color.X == 0 && color.Y == 0 && color.Z == 0 {
					t.Errorf("Pixel (%d,%d) has zero color despite samples", x, y)
				}
			}
		}
	}

	if !samplesFound {
		t.Error("No samples found in pixel stats")
	}

	// The splat queue should be empty after tile processing (splats extracted and applied)
	if count := splatQueue.GetSplatCount(); count != 0 {
		t.Errorf("Expected empty splat queue after tile processing, got %d splats", count)
	}
}

func TestSplatSystemIntegration(t *testing.T) {
	// Create BDPT integrator to test real splat generation
	config := core.SamplingConfig{
		Width:                     20,
		Height:                    20,
		SamplesPerPixel:           1,
		MaxDepth:                  3,
		RussianRouletteMinBounces: 2,
		AdaptiveMinSamples:        0.1,
		AdaptiveThreshold:         0.01,
	}

	bdptIntegrator := integrator.NewBDPTIntegrator(config)

	// Create scene with actual geometry and lighting for meaningful BDPT testing
	cameraConfig := CameraConfig{
		Center:      core.NewVec3(0, 0, 5),
		LookAt:      core.NewVec3(0, 0, 0),
		Up:          core.NewVec3(0, 1, 0),
		Width:       20,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(cameraConfig)

	// Create materials
	lambertian := material.NewLambertian(core.Vec3{X: 0.7, Y: 0.3, Z: 0.3})
	light := material.NewEmissive(core.Vec3{X: 4.0, Y: 4.0, Z: 4.0})
	metal := material.NewMetal(core.Vec3{X: 0.7, Y: 0.6, Z: 0.5}, 0.1)

	// Create shapes
	var shapes []core.Shape
	var lights []core.Light

	// Add a sphere with lambertian material
	sphere := geometry.NewSphere(core.NewVec3(0, 0, 0), 1.0, lambertian)
	shapes = append(shapes, sphere)

	// Add a metallic sphere that can create caustics
	metallicSphere := geometry.NewSphere(core.NewVec3(2, 0, 0), 0.8, metal)
	shapes = append(shapes, metallicSphere)

	// Add an area light
	lightQuad := geometry.NewQuadLight(
		core.NewVec3(-2, 3, -2),
		core.NewVec3(4, 0, 0),
		core.NewVec3(0, 0, 4),
		light,
	)
	shapes = append(shapes, lightQuad)
	lights = append(lights, lightQuad)

	// Create scene with geometry
	sceneObj := &MockScene{
		camera:      camera,
		config:      config,
		shapes:      shapes,
		lights:      lights,
		topColor:    core.Vec3{X: 0.1, Y: 0.1, Z: 0.1},
		bottomColor: core.Vec3{X: 0.05, Y: 0.05, Z: 0.05},
		width:       config.Width,
		height:      config.Height,
	}

	// Create progressive raytracer
	progressiveConfig := ProgressiveConfig{
		TileSize:           8,
		InitialSamples:     1,
		MaxSamplesPerPixel: 2,
		MaxPasses:          1,
		NumWorkers:         1,
	}

	logger := NewDefaultLogger()
	raytracer := NewProgressiveRaytracer(sceneObj, progressiveConfig, bdptIntegrator, logger)

	// Render one pass
	img, stats, err := raytracer.RenderPass(1, nil)

	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if img == nil {
		t.Fatal("Expected rendered image, got nil")
	}

	if stats.TotalSamples == 0 {
		t.Error("Expected some samples to be rendered")
	}

	// Check that we got a valid image
	bounds := img.Bounds()
	if bounds.Dx() != config.Width || bounds.Dy() != config.Height {
		t.Errorf("Expected image size %dx%d, got %dx%d",
			config.Width, config.Height, bounds.Dx(), bounds.Dy())
	}

	// Check for non-zero pixels (should have some content)
	nonZeroPixels := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 0 || g > 0 || b > 0 {
				nonZeroPixels++
			}
		}
	}

	if nonZeroPixels == 0 {
		t.Error("Expected some non-zero pixels in rendered image")
	}

	t.Logf("Rendered %dx%d image with %d non-zero pixels in %d total samples",
		bounds.Dx(), bounds.Dy(), nonZeroPixels, stats.TotalSamples)
}
