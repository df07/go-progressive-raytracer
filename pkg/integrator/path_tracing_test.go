package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// MockScene implements core.Scene for testing
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

// MockCamera implements core.Camera for testing
type MockCamera struct{}

func (m *MockCamera) GetRay(i, j int, random *rand.Rand) core.Ray {
	// Simple ray pointing forward
	return core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
}

func (m *MockCamera) CalculateRayPDFs(ray core.Ray) (float64, float64) {
	return 1.0, 1.0
}

func (m *MockCamera) GetCameraForward() core.Vec3 {
	return core.NewVec3(0, 0, -1)
}

// createTestScene creates a simple scene with a sphere for testing
func createTestScene() *MockScene {
	// Create a simple lambertian sphere
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.3, 0.3))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	// Create a simple mock camera
	camera := &MockCamera{}

	return &MockScene{
		shapes:      []core.Shape{sphere},
		lights:      []core.Light{},
		topColor:    core.NewVec3(0.5, 0.7, 1.0),
		bottomColor: core.NewVec3(1.0, 1.0, 1.0),
		camera:      camera,
		config: core.SamplingConfig{
			MaxDepth:                  10,
			RussianRouletteMinBounces: 5,
			RussianRouletteMinSamples: 3,
		},
	}
}

// TestPathTracingBackgroundGradient tests the background gradient calculation
func TestPathTracingBackgroundGradient(t *testing.T) {
	scene := createTestScene()
	integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())

	// Test ray pointing up (should get top color)
	upRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0))
	upColor := integrator.BackgroundGradient(upRay, scene)

	// Test ray pointing down (should get bottom color)
	downRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, -1, 0))
	downColor := integrator.BackgroundGradient(downRay, scene)

	// Colors should be different
	if upColor == downColor {
		t.Error("Expected different colors for up and down rays")
	}

	// Up ray should be closer to top color (blue-ish)
	if upColor.Z < downColor.Z {
		t.Error("Expected up ray to have more blue component")
	}

	// Colors should be in valid range
	for _, color := range []core.Vec3{upColor, downColor} {
		if color.X < 0 || color.Y < 0 || color.Z < 0 {
			t.Errorf("Color has negative components: %v", color)
		}
		if color.X > 1 || color.Y > 1 || color.Z > 1 {
			t.Errorf("Color has components > 1: %v", color)
		}
	}
}

// TestPathTracingDepthTermination tests that ray depth is properly limited
func TestPathTracingDepthTermination(t *testing.T) {
	scene := createTestScene()
	config := core.SamplingConfig{
		MaxDepth:                  2,  // Very shallow depth
		RussianRouletteMinBounces: 10, // Disable Russian roulette
		RussianRouletteMinSamples: 10,
	}
	integrator := NewPathTracingIntegrator(config)
	random := rand.New(rand.NewSource(42))

	// Ray pointing at the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}

	// Test with depth 0 (should return black)
	colorDepth0 := integrator.RayColor(ray, scene, random, 0, throughput, 0)
	if colorDepth0 != (core.Vec3{}) {
		t.Errorf("Expected black color for depth 0, got %v", colorDepth0)
	}

	// Test with positive depth (should return some color)
	colorDepth2 := integrator.RayColor(ray, scene, random, 2, throughput, 0)
	if colorDepth2 == (core.Vec3{}) {
		t.Error("Expected non-black color for positive depth")
	}
}

// TestPathTracingRussianRoulette tests Russian roulette termination
func TestPathTracingRussianRoulette(t *testing.T) {
	config := core.SamplingConfig{
		MaxDepth:                  50, // Very deep
		RussianRouletteMinBounces: 1,  // Start RR immediately
		RussianRouletteMinSamples: 1,  // Start RR immediately
	}
	integrator := NewPathTracingIntegrator(config)

	// Test with very low throughput (should often terminate)
	lowThroughput := core.Vec3{X: 0.01, Y: 0.01, Z: 0.01}
	terminationCount := 0
	testCount := 100

	for i := 0; i < testCount; i++ {
		random := rand.New(rand.NewSource(int64(i)))
		shouldTerminate, _ := integrator.ApplyRussianRoulette(10, lowThroughput, 5, random)
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
		random := rand.New(rand.NewSource(int64(i)))
		shouldTerminate, _ := integrator.ApplyRussianRoulette(10, highThroughput, 5, random)
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
	camera := &MockCamera{}

	scene := &MockScene{
		shapes:      []core.Shape{sphere},
		lights:      []core.Light{},
		topColor:    core.NewVec3(0.5, 0.7, 1.0),
		bottomColor: core.NewVec3(1.0, 1.0, 1.0),
		camera:      camera,
		config: core.SamplingConfig{
			MaxDepth:                  10,
			RussianRouletteMinBounces: 5,
			RussianRouletteMinSamples: 3,
		},
	}

	integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())
	random := rand.New(rand.NewSource(42))

	// Ray hitting the metallic sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}

	color := integrator.RayColor(ray, scene, random, 5, throughput, 0)

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
	camera := &MockCamera{}

	scene := &MockScene{
		shapes:      []core.Shape{sphere},
		lights:      []core.Light{},
		topColor:    core.NewVec3(0.0, 0.0, 0.0), // Black background
		bottomColor: core.NewVec3(0.0, 0.0, 0.0),
		camera:      camera,
		config: core.SamplingConfig{
			MaxDepth: 10,
		},
	}

	integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())
	random := rand.New(rand.NewSource(42))

	// Ray hitting the emissive sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}

	color := integrator.RayColor(ray, scene, random, 5, throughput, 0)

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
	integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())
	random := rand.New(rand.NewSource(42))

	// Ray that misses the sphere (pointing up)
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0))
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}

	color := integrator.RayColor(ray, scene, random, 5, throughput, 0)

	// Should get background gradient color
	if color == (core.Vec3{}) {
		t.Error("Expected background color, got black")
	}

	// Should be similar to what BackgroundGradient returns
	expectedBg := integrator.BackgroundGradient(ray, scene)
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
	integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())

	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}

	// Render the same ray with the same random seed twice
	random1 := rand.New(rand.NewSource(42))
	color1 := integrator.RayColor(ray, scene, random1, 5, throughput, 0)

	random2 := rand.New(rand.NewSource(42))
	color2 := integrator.RayColor(ray, scene, random2, 5, throughput, 0)

	// Results should be identical
	if color1 != color2 {
		t.Errorf("Expected deterministic results, got %v and %v", color1, color2)
	}
}
