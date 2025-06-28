package integrator

import (
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestBDPTIntegratorCreation(t *testing.T) {
	config := core.SamplingConfig{
		Width:           64,
		Height:          64,
		MaxDepth:        5,
		SamplesPerPixel: 1,
	}

	integrator := NewBDPTIntegrator(config)
	if integrator == nil {
		t.Fatal("Failed to create BDPT integrator")
	}

	if integrator.PathTracingIntegrator == nil {
		t.Fatal("BDPT integrator should embed PathTracingIntegrator")
	}
}

func TestVertexCreation(t *testing.T) {
	point := core.NewVec3(1, 2, 3)
	normal := core.NewVec3(0, 1, 0)

	vertex := Vertex{
		Point:   point,
		Normal:  normal,
		IsLight: true,
	}

	if !vertex.Point.Equals(point) {
		t.Errorf("Expected vertex point %v, got %v", point, vertex.Point)
	}

	if !vertex.Normal.Equals(normal) {
		t.Errorf("Expected vertex normal %v, got %v", normal, vertex.Normal)
	}

	if !vertex.IsLight {
		t.Error("Expected vertex to be marked as light")
	}
}

func TestPathCreation(t *testing.T) {
	path := Path{
		Vertices: make([]Vertex, 0, 10),
		Length:   0,
	}

	if path.Length != 0 {
		t.Errorf("Expected empty path length 0, got %d", path.Length)
	}

	if len(path.Vertices) != 0 {
		t.Errorf("Expected empty vertices slice, got length %d", len(path.Vertices))
	}
}

func TestBDPTStrategyCreation(t *testing.T) {
	strategy := bdptStrategy{
		s:            2,
		t:            3,
		contribution: core.NewVec3(0.5, 0.6, 0.7),
		pdf:          0.1,
	}

	if strategy.s != 2 || strategy.t != 3 {
		t.Errorf("Expected strategy (s=2, t=3), got (s=%d, t=%d)", strategy.s, strategy.t)
	}

	if strategy.pdf != 0.1 {
		t.Errorf("Expected strategy PDF 0.1, got %f", strategy.pdf)
	}
}

// Test basic light path generation
func TestLightPathGeneration(t *testing.T) {
	// Create a simple scene with a light
	emissiveMaterial := material.NewEmissive(core.NewVec3(1, 1, 1))
	light := geometry.NewSphereLight(core.NewVec3(0, 5, 0), 1.0, emissiveMaterial)

	testScene := &MockScene{
		lights: []core.Light{light},
		shapes: []core.Shape{light},
		config: core.SamplingConfig{MaxDepth: 3},
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	lightPath := integrator.generateLightSubpath(testScene, random, 3)

	if lightPath.Length == 0 {
		t.Error("Expected light path to have at least one vertex")
	}

	if lightPath.Length > 0 {
		firstVertex := lightPath.Vertices[0]
		if !firstVertex.IsLight {
			t.Error("First vertex in light path should be marked as light")
		}

		if firstVertex.EmittedLight.Luminance() <= 0 {
			t.Error("Light vertex should have positive emission")
		}
	}
}

// Test basic camera path generation
func TestCameraPathGeneration(t *testing.T) {
	// Create a simple scene with a sphere
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	testScene := &MockScene{
		shapes: []core.Shape{sphere},
		config: core.SamplingConfig{MaxDepth: 3},
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	cameraPath := integrator.generateCameraSubpath(ray, testScene, random, 3, throughput, 0)

	if cameraPath.Length == 0 {
		t.Error("Expected camera path to have at least one vertex")
	}

	if cameraPath.Length > 0 {
		firstVertex := cameraPath.Vertices[0]
		if firstVertex.IsLight {
			t.Error("First vertex in camera path should not be marked as light")
		}
	}
}

// Test MIS weight calculation
func TestMISWeightCalculation(t *testing.T) {
	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	strategies := []bdptStrategy{
		{s: 0, t: 2, contribution: core.NewVec3(0.5, 0.5, 0.5), pdf: 0.1},
		{s: 1, t: 1, contribution: core.NewVec3(0.3, 0.3, 0.3), pdf: 0.2},
		{s: 2, t: 0, contribution: core.NewVec3(0.2, 0.2, 0.2), pdf: 0.15},
	}

	weight := integrator.calculateMISWeight(strategies[0], strategies)

	// Power heuristic: weight = pdf^2 / sum(all_pdf^2)
	expectedWeight := (0.1 * 0.1) / (0.1*0.1 + 0.2*0.2 + 0.15*0.15)

	if abs(weight-expectedWeight) > 1e-6 {
		t.Errorf("Expected MIS weight %f, got %f", expectedWeight, weight)
	}
}

// Test BDPT vs Path Tracing consistency
func TestBDPTvsPathTracingConsistency(t *testing.T) {
	// Create a simple scene with a light and diffuse surface
	emissiveMaterial := material.NewEmissive(core.NewVec3(2, 2, 2))
	light := geometry.NewSphereLight(core.NewVec3(0, 3, 0), 0.5, emissiveMaterial)

	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	bvh := core.NewBVH([]core.Shape{light, sphere})

	testScene := &MockScene{
		lights:      []core.Light{light},
		shapes:      []core.Shape{light, sphere},
		config:      core.SamplingConfig{MaxDepth: 5},
		bvh:         bvh,
		topColor:    core.NewVec3(0.1, 0.1, 0.1),
		bottomColor: core.NewVec3(0.05, 0.05, 0.05),
	}

	config := core.SamplingConfig{MaxDepth: 5}

	// Create both integrators
	pathTracer := NewPathTracingIntegrator(config)
	bdptTracer := NewBDPTIntegrator(config)

	// Test ray that should hit the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	// Sample multiple times to get average (reduces noise)
	numSamples := 10
	var pathTracingTotal, bdptTotal core.Vec3

	for i := 0; i < numSamples; i++ {
		random := rand.New(rand.NewSource(int64(42 + i)))

		// Path tracing result
		ptResult := pathTracer.RayColor(ray, testScene, random, config.MaxDepth, throughput, i)
		pathTracingTotal = pathTracingTotal.Add(ptResult)

		// BDPT result
		random = rand.New(rand.NewSource(int64(42 + i))) // Reset seed for fair comparison
		bdptResult := bdptTracer.RayColor(ray, testScene, random, config.MaxDepth, throughput, i)
		bdptTotal = bdptTotal.Add(bdptResult)
	}

	// Average the results
	pathTracingAvg := pathTracingTotal.Multiply(1.0 / float64(numSamples))
	bdptAvg := bdptTotal.Multiply(1.0 / float64(numSamples))

	// Results should be similar (within reasonable tolerance due to different sampling strategies)
	tolerance := 0.5 // BDPT and PT can have different variance characteristics

	if abs(pathTracingAvg.X-bdptAvg.X) > tolerance ||
		abs(pathTracingAvg.Y-bdptAvg.Y) > tolerance ||
		abs(pathTracingAvg.Z-bdptAvg.Z) > tolerance {
		t.Errorf("BDPT and Path Tracing results differ too much:\nPath Tracing: %v\nBDPT: %v",
			pathTracingAvg, bdptAvg)
	}

	// Both should produce some illumination (not black)
	if pathTracingAvg.Luminance() < 0.01 {
		t.Error("Path tracing produced unexpectedly dark result")
	}

	if bdptAvg.Luminance() < 0.01 {
		t.Error("BDPT produced unexpectedly dark result")
	}
}

// Test that BDPT handles specular materials correctly
func TestBDPTSpecularHandling(t *testing.T) {
	// Create scene with metal sphere
	metal := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0)
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, metal)

	bvh := core.NewBVH([]core.Shape{sphere})

	testScene := &MockScene{
		shapes: []core.Shape{sphere},
		config: core.SamplingConfig{MaxDepth: 3},
		bvh:    bvh,
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	// Should not crash on specular materials
	result := integrator.RayColor(ray, testScene, random, config.MaxDepth, throughput, 0)

	// Result should be valid (not NaN/Inf)
	if result.X != result.X || result.Y != result.Y || result.Z != result.Z {
		t.Error("BDPT produced NaN result with specular material")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
