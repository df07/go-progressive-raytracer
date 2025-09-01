package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/lights"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// createTestScene creates a simple scene with a sphere for testing
func createTestScene() *scene.Scene {
	// Create a simple lambertian sphere
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.3, 0.3))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	infiniteLight := lights.NewGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white ground)
	)

	// Create a simple mock camera
	camera := &geometry.Camera{}

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere},
		Lights: []lights.Light{infiniteLight},
		Camera: camera,
		SamplingConfig: scene.SamplingConfig{
			MaxDepth:                  10,
			RussianRouletteMinBounces: 5,
		},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()
	return scene
}

// TestPathTracingDepthTermination tests that ray depth is properly limited
func TestPathTracingDepthTermination(t *testing.T) {
	sc := createTestScene()
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray pointing at the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	// Test with depth 0 (should return black)
	integrator := NewPathTracingIntegrator(scene.SamplingConfig{MaxDepth: 0, RussianRouletteMinBounces: 100})
	colorDepth0, _ := integrator.RayColor(ray, sc, sampler)
	if colorDepth0 != (core.Vec3{}) {
		t.Errorf("Expected black color for depth 0, got %v", colorDepth0)
	}

	// Test with positive depth (should return some color)
	integrator = NewPathTracingIntegrator(scene.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100})
	colorDepth2, _ := integrator.RayColor(ray, sc, sampler)
	if colorDepth2 == (core.Vec3{}) {
		t.Error("Expected non-black color for positive depth")
	}
}

// TestPathTracingRussianRoulette tests Russian roulette termination
func TestPathTracingRussianRoulette(t *testing.T) {
	config := scene.SamplingConfig{
		MaxDepth:                  50, // Very deep
		RussianRouletteMinBounces: 1,  // Start RR immediately
	}
	integrator := NewPathTracingIntegrator(config)

	// Test with very low throughput (should often terminate)
	lowThroughput := core.Vec3{X: 0.01, Y: 0.01, Z: 0.01}
	terminationCount := 0
	testCount := 100

	for i := 0; i < testCount; i++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(i))))
		shouldTerminate, _ := integrator.ApplyRussianRoulette(10, lowThroughput, sampler.Get1D())
		if shouldTerminate {
			terminationCount++
		}
	}

	// With low throughput, we should see some terminations
	if terminationCount == 0 {
		t.Error("Expected some Russian roulette terminations with low throughput")
	}

	// But not all rays should terminate
	if terminationCount >= testCount {
		t.Error("Expected some rays to survive Russian roulette")
	}

	// Test with high throughput (should rarely terminate)
	highThroughput := core.Vec3{X: 0.9, Y: 0.9, Z: 0.9}
	highTerminationCount := 0

	for i := 0; i < testCount; i++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(i))))
		shouldTerminate, _ := integrator.ApplyRussianRoulette(10, highThroughput, sampler.Get1D())
		if shouldTerminate {
			highTerminationCount++
		}
	}

	// High throughput should terminate less often than low throughput
	if highTerminationCount >= terminationCount {
		t.Error("Expected high throughput to terminate less often than low throughput")
	}
}

// TestPathTracingSpecularMaterial tests specular material handling
func TestPathTracingSpecularMaterial(t *testing.T) {
	// Create scene with metallic sphere
	metal := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0) // Perfect mirror
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, metal)

	cameraConfig := geometry.CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       100,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	infiniteLight := lights.NewGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white ground)
	)

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere},
		Lights: []lights.Light{infiniteLight},
		Camera: camera,
		SamplingConfig: scene.SamplingConfig{
			MaxDepth:                  5,
			RussianRouletteMinBounces: 5,
		},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()

	integrator := NewPathTracingIntegrator(scene.SamplingConfig)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray hitting the metallic sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	color, _ := integrator.RayColor(ray, scene, sampler)

	// Should get some reflection color (not black)
	if color == (core.Vec3{}) {
		t.Error("Expected non-black color from metallic reflection")
	}

	// Color should be reasonable (not excessive)
	if color.X > 2 || color.Y > 2 || color.Z > 2 {
		t.Errorf("Expected reasonable color values, got %v", color)
	}
}

// TestPathTracingEmissiveMaterial tests emissive material handling
func TestPathTracingEmissiveMaterial(t *testing.T) {
	// Create scene with emissive sphere (light)
	emission := core.NewVec3(2.0, 1.0, 0.5) // Bright orange light
	emissive := material.NewEmissive(emission)
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, emissive)

	cameraConfig := geometry.CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       100,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere},
		Lights: []lights.Light{},
		Camera: camera,
		SamplingConfig: scene.SamplingConfig{
			MaxDepth: 10,
		},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()

	integrator := NewPathTracingIntegrator(scene.SamplingConfig)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray hitting the emissive sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	color, _ := integrator.RayColor(ray, scene, sampler)

	// Should get the emission color
	if color == (core.Vec3{}) {
		t.Error("Expected emitted light, got black")
	}

	// Red component should be highest (emission is 2.0, 1.0, 0.5)
	if color.X <= color.Y || color.Y <= color.Z {
		t.Errorf("Expected emission color pattern (R>G>B), got %v", color)
	}
}

// TestPathTracingMissedRay tests background handling for rays that miss all objects
func TestPathTracingMissedRay(t *testing.T) {
	scene := createTestScene()
	integrator := NewPathTracingIntegrator(scene.SamplingConfig)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray that misses the sphere (pointing up)
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0))

	color, _ := integrator.RayColor(ray, scene, sampler)

	// Should get infinite light color
	if color == (core.Vec3{}) {
		t.Error("Expected infinite light color, got black")
	}

	// Should be similar to what infinite light returns
	expectedBg := scene.Lights[0].Emit(ray)
	tolerance := 0.01
	if math.Abs(color.X-expectedBg.X) > tolerance ||
		math.Abs(color.Y-expectedBg.Y) > tolerance ||
		math.Abs(color.Z-expectedBg.Z) > tolerance {
		t.Errorf("Expected background color %v, got %v", expectedBg, color)
	}
}

// TestPathTracingDeterministic tests that identical inputs produce identical outputs
func TestPathTracingDeterministic(t *testing.T) {
	scene := createTestScene()
	integrator := NewPathTracingIntegrator(scene.SamplingConfig)

	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	// Render the same ray with the same random seed twice
	sampler1 := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	color1, _ := integrator.RayColor(ray, scene, sampler1)

	sampler2 := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	color2, _ := integrator.RayColor(ray, scene, sampler2)

	// Results should be identical
	if color1 != color2 {
		t.Errorf("Expected deterministic results, got %v and %v", color1, color2)
	}
}

func TestPowerHeuristic(t *testing.T) {
	tests := []struct {
		name     string
		nf       int
		fPdf     float64
		ng       int
		gPdf     float64
		expected float64
	}{
		{
			name:     "Equal PDFs",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.5,
			expected: 0.5,
		},
		{
			name:     "First PDF zero",
			nf:       1,
			fPdf:     0.0,
			ng:       1,
			gPdf:     0.5,
			expected: 0.0,
		},
		{
			name:     "Second PDF zero",
			nf:       1,
			fPdf:     0.5,
			ng:       1,
			gPdf:     0.0,
			expected: 1.0,
		},
		{
			name:     "First PDF higher",
			nf:       1,
			fPdf:     0.8,
			ng:       1,
			gPdf:     0.2,
			expected: 0.941176, // (0.8²) / (0.8² + 0.2²)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := powerHeuristic(tt.nf, tt.fPdf, tt.ng, tt.gPdf)
			if math.Abs(result-tt.expected) > 1e-5 {
				t.Errorf("PowerHeuristic: got %f, expected %f", result, tt.expected)
			}
		})
	}
}
