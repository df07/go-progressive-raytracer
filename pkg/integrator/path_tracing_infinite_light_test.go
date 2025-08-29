package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// createSceneWithInfiniteLight creates a test scene with an infinite light instead of background gradient
func createSceneWithInfiniteLight() *scene.Scene {
	// Create a simple lambertian sphere
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.3, 0.3))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	// Create a simple mock camera
	camera := &geometry.Camera{}

	scene := &scene.Scene{
		Shapes: []core.Shape{sphere},
		Lights: []geometry.Light{},
		Camera: camera,
		SamplingConfig: core.SamplingConfig{
			MaxDepth:                  10,
			RussianRouletteMinBounces: 5,
		},
	}

	// Add a gradient infinite light (this replaces background gradient functionality)
	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 0.8, 0.6), // bottomColor (warm ground)
	)
	scene.Lights = append(scene.Lights, infiniteLight)

	scene.Preprocess()

	return scene
}

// TestPathTracingInfiniteLight tests that path tracing correctly samples infinite lights
func TestPathTracingInfiniteLight(t *testing.T) {
	scene := createSceneWithInfiniteLight()
	integrator := NewPathTracingIntegrator(scene.SamplingConfig)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray that misses the sphere and should hit the infinite light (pointing up)
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0))

	color, _ := integrator.RayColor(ray, scene, sampler)

	// Should get color from infinite light (not black like the background gradient)
	if color == (core.Vec3{}) {
		t.Error("Expected color from infinite light, got black")
	}

	// The ray pointing up (Y=1) should get a color close to the top color of the gradient
	// Since the gradient interpolates based on Y direction, upward ray should be bluish
	if color.Z <= color.X || color.Z <= color.Y {
		t.Errorf("Expected blue-dominant color for upward ray, got %v", color)
	}

	// Color should be reasonable (not excessive)
	if color.X > 2 || color.Y > 2 || color.Z > 2 {
		t.Errorf("Expected reasonable color values, got %v", color)
	}
}

// TestPathTracingInfiniteLight_GradientVariation tests that different directions get different colors
func TestPathTracingInfiniteLight_GradientVariation(t *testing.T) {
	scene := createSceneWithInfiniteLight()
	integrator := NewPathTracingIntegrator(scene.SamplingConfig)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Test rays in different directions
	upRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0))    // Should get top color
	downRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, -1, 0)) // Should get bottom color

	upColor, _ := integrator.RayColor(upRay, scene, sampler)
	downColor, _ := integrator.RayColor(downRay, scene, sampler)

	// Colors should be different
	if upColor == downColor {
		t.Error("Expected different colors for up and down rays hitting infinite light")
	}

	// Up ray should be more blue (Z component higher)
	if upColor.Z <= downColor.Z {
		t.Errorf("Expected upward ray to be more blue than downward ray. Up: %v, Down: %v", upColor, downColor)
	}

	// Both should be non-zero (getting light from infinite light)
	if upColor == (core.Vec3{}) || downColor == (core.Vec3{}) {
		t.Error("Expected both rays to get color from infinite light")
	}
}

// TestUniformInfiniteLight_PathTracing tests uniform infinite light with path tracing
func TestUniformInfiniteLight_PathTracing(t *testing.T) {
	// Create scene with uniform infinite light
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.3, 0.3))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)
	camera := &geometry.Camera{}

	scene := &scene.Scene{
		Shapes: []core.Shape{sphere},
		Lights: []geometry.Light{},
		Camera: camera,
		SamplingConfig: core.SamplingConfig{
			MaxDepth:                  10,
			RussianRouletteMinBounces: 5,
		},
	}

	// Add uniform infinite light
	uniformLight := geometry.NewUniformInfiniteLight(core.NewVec3(0.8, 0.6, 0.4))
	scene.Lights = append(scene.Lights, uniformLight)

	scene.Preprocess()

	integrator := NewPathTracingIntegrator(scene.SamplingConfig)

	// Test rays in different directions - should all get the same uniform color
	directions := []core.Vec3{
		core.NewVec3(0, 1, 0),  // up
		core.NewVec3(0, -1, 0), // down
		core.NewVec3(1, 0, 0),  // right
		core.NewVec3(-1, 0, 0), // left
		core.NewVec3(0, 0, 1),  // toward camera
	}

	colors := make([]core.Vec3, len(directions))
	for i, dir := range directions {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(42 + i))))
		ray := core.NewRay(core.NewVec3(0, 0, 0), dir)
		colors[i], _ = integrator.RayColor(ray, scene, sampler)

		// Should get non-black color
		if colors[i] == (core.Vec3{}) {
			t.Errorf("Direction %v: expected non-black color from uniform infinite light", dir)
		}
	}

	// All colors should be similar (uniform light)
	baseColor := colors[0]
	tolerance := 0.1 // Allow some Monte Carlo variance
	for i, color := range colors[1:] {
		if math.Abs(color.X-baseColor.X) > tolerance ||
			math.Abs(color.Y-baseColor.Y) > tolerance ||
			math.Abs(color.Z-baseColor.Z) > tolerance {
			t.Errorf("Direction %d: expected similar color to base %v, got %v", i+1, baseColor, color)
		}
	}
}
