package lights

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// MockLight implements the Light interface for testing
type MockLight struct {
	emission core.Vec3
	pdf      float64
}

func (ml *MockLight) Type() LightType {
	return LightTypeArea
}

func (ml *MockLight) Sample(point core.Vec3, normal core.Vec3, sample core.Vec2) LightSample {
	return LightSample{
		Point:     core.Vec3{X: 0, Y: 1, Z: 0},
		Normal:    core.Vec3{X: 0, Y: -1, Z: 0},
		Direction: core.Vec3{X: 0, Y: 1, Z: 0},
		Distance:  1.0,
		Emission:  ml.emission,
		PDF:       ml.pdf,
	}
}

func (ml *MockLight) PDF(point, normal, direction core.Vec3) float64 {
	return ml.pdf
}

func (ml *MockLight) SampleEmission(samplePoint core.Vec2, sampleDirection core.Vec2) EmissionSample {
	return EmissionSample{
		Point:        core.Vec3{X: 0, Y: 1, Z: 0},
		Normal:       core.Vec3{X: 0, Y: -1, Z: 0},
		Direction:    core.Vec3{X: 0, Y: -1, Z: 0},
		Emission:     ml.emission,
		AreaPDF:      ml.pdf,
		DirectionPDF: 1.0 / math.Pi, // cosine-weighted
	}
}

func (ml *MockLight) PDF_Le(point core.Vec3, direction core.Vec3) (pdfPos, pdfDir float64) {
	// Mock light returns same PDF for both (test convenience)
	return ml.pdf, ml.pdf
}

func (ml *MockLight) Emit(ray core.Ray, si *material.SurfaceInteraction) core.Vec3 {
	// Mock light doesn't emit in arbitrary directions (finite light)
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

func TestNewWeightedLightSampler(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	weights := []float64{0.3, 0.7}
	sceneRadius := 10.0

	sampler := NewWeightedLightSampler(lights, weights, sceneRadius)

	if len(sampler.lights) != 2 {
		t.Errorf("Expected 2 lights, got %d", len(sampler.lights))
	}
	if len(sampler.weights) != 2 {
		t.Errorf("Expected 2 weights, got %d", len(sampler.weights))
	}
	if sampler.sceneRadius != sceneRadius {
		t.Errorf("Expected scene radius %f, got %f", sceneRadius, sampler.sceneRadius)
	}

	// Check weights are normalized (already normalized in this case)
	expectedWeights := []float64{0.3, 0.7}
	for i, expected := range expectedWeights {
		if math.Abs(sampler.weights[i]-expected) > 1e-6 {
			t.Errorf("Weight %d: expected %f, got %f", i, expected, sampler.weights[i])
		}
	}
}

func TestNewWeightedLightSampler_Normalization(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	// Non-normalized weights
	weights := []float64{1.0, 3.0} // Should normalize to 0.25, 0.75

	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	expectedWeights := []float64{0.25, 0.75}
	for i, expected := range expectedWeights {
		if math.Abs(sampler.weights[i]-expected) > 1e-6 {
			t.Errorf("Normalized weight %d: expected %f, got %f", i, expected, sampler.weights[i])
		}
	}
}

func TestNewWeightedLightSampler_ZeroWeights(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	// All zero weights should fallback to uniform
	weights := []float64{0.0, 0.0}

	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	expectedWeights := []float64{0.5, 0.5} // Uniform fallback
	for i, expected := range expectedWeights {
		if math.Abs(sampler.weights[i]-expected) > 1e-6 {
			t.Errorf("Zero weight fallback %d: expected %f, got %f", i, expected, sampler.weights[i])
		}
	}
}

func TestNewWeightedLightSampler_MismatchedLength(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
	}
	weights := []float64{0.3, 0.7} // Mismatched length

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for mismatched lights/weights length")
		}
	}()

	NewWeightedLightSampler(lights, weights, 10.0)
}

func TestNewUniformLightSampler(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
		&MockLight{emission: core.Vec3{X: 3, Y: 3, Z: 3}, pdf: 0.2},
	}

	sampler := NewUniformLightSampler(lights, 10.0)

	// All weights should be equal (1/3)
	expectedWeight := 1.0 / 3.0
	for i := 0; i < 3; i++ {
		if math.Abs(sampler.weights[i]-expectedWeight) > 1e-6 {
			t.Errorf("Uniform weight %d: expected %f, got %f", i, expectedWeight, sampler.weights[i])
		}
	}
}

func TestNewUniformLightSampler_EmptyLights(t *testing.T) {
	lights := []Light{}
	sampler := NewUniformLightSampler(lights, 10.0)

	if len(sampler.lights) != 0 {
		t.Errorf("Expected 0 lights, got %d", len(sampler.lights))
	}
	if len(sampler.weights) != 0 {
		t.Errorf("Expected 0 weights, got %d", len(sampler.weights))
	}
}

func TestWeightedLightSampler_SampleLight(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	weights := []float64{0.3, 0.7}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	point := core.Vec3{X: 0, Y: 0, Z: 0}
	normal := core.Vec3{X: 0, Y: 1, Z: 0}

	// Test sampling with different u values
	testCases := []struct {
		u             float64
		expectedIndex int
		expectedProb  float64
	}{
		{0.0, 0, 0.3},  // Should select first light
		{0.1, 0, 0.3},  // Should select first light
		{0.29, 0, 0.3}, // Should select first light
		{0.3, 0, 0.3},  // Should select first light (boundary case)
		{0.31, 1, 0.7}, // Should select second light
		{0.5, 1, 0.7},  // Should select second light
		{0.99, 1, 0.7}, // Should select second light
	}

	for _, tc := range testCases {
		_, prob, lightIndex := sampler.SampleLight(point, normal, tc.u)

		if lightIndex != tc.expectedIndex {
			t.Errorf("u=%f: expected light %d, got light %d", tc.u, tc.expectedIndex, lightIndex)
		}
		if math.Abs(prob-tc.expectedProb) > 1e-6 {
			t.Errorf("u=%f: expected probability %f, got %f", tc.u, tc.expectedProb, prob)
		}
	}
}

func TestWeightedLightSampler_SampleLightEmission(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	weights := []float64{0.4, 0.6}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	// Test sampling with different u values
	testCases := []struct {
		u             float64
		expectedIndex int
		expectedProb  float64
	}{
		{0.0, 0, 0.4},  // Should select first light
		{0.2, 0, 0.4},  // Should select first light
		{0.39, 0, 0.4}, // Should select first light
		{0.4, 0, 0.4},  // Should select first light (boundary case)
		{0.41, 1, 0.6}, // Should select second light
		{0.7, 1, 0.6},  // Should select second light
		{0.99, 1, 0.6}, // Should select second light
	}

	for _, tc := range testCases {
		_, prob, lightIndex := sampler.SampleLightEmission(tc.u)

		if lightIndex != tc.expectedIndex {
			t.Errorf("u=%f: expected light %d, got light %d", tc.u, tc.expectedIndex, lightIndex)
		}
		if math.Abs(prob-tc.expectedProb) > 1e-6 {
			t.Errorf("u=%f: expected probability %f, got %f", tc.u, tc.expectedProb, prob)
		}
	}
}

func TestWeightedLightSampler_GetLightProbability(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
	}
	weights := []float64{0.2, 0.8}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	point := core.Vec3{X: 0, Y: 0, Z: 0}
	normal := core.Vec3{X: 0, Y: 1, Z: 0}

	// Test valid indices
	prob0 := sampler.GetLightProbability(0, point, normal)
	if math.Abs(prob0-0.2) > 1e-6 {
		t.Errorf("Light 0 probability: expected 0.2, got %f", prob0)
	}

	prob1 := sampler.GetLightProbability(1, point, normal)
	if math.Abs(prob1-0.8) > 1e-6 {
		t.Errorf("Light 1 probability: expected 0.8, got %f", prob1)
	}

	// Test invalid indices
	probInvalid := sampler.GetLightProbability(-1, point, normal)
	if probInvalid != 0.0 {
		t.Errorf("Invalid index probability: expected 0.0, got %f", probInvalid)
	}

	probInvalid2 := sampler.GetLightProbability(2, point, normal)
	if probInvalid2 != 0.0 {
		t.Errorf("Invalid index probability: expected 0.0, got %f", probInvalid2)
	}
}

func TestWeightedLightSampler_GetLightCount(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
		&MockLight{emission: core.Vec3{X: 3, Y: 3, Z: 3}, pdf: 0.2},
	}
	weights := []float64{0.33, 0.33, 0.34}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	count := sampler.GetLightCount()
	if count != 3 {
		t.Errorf("Expected light count 3, got %d", count)
	}
}

func TestWeightedLightSampler_EmptyLights(t *testing.T) {
	lights := []Light{}
	weights := []float64{}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	point := core.Vec3{X: 0, Y: 0, Z: 0}
	normal := core.Vec3{X: 0, Y: 1, Z: 0}

	// SampleLight should return nil
	light, prob, lightIndex := sampler.SampleLight(point, normal, 0.5)
	if light != nil {
		t.Error("Expected nil light for empty sampler")
	}
	if prob != 0.0 {
		t.Errorf("Expected 0.0 probability for empty sampler, got %f", prob)
	}
	if lightIndex != -1 {
		t.Errorf("Expected light index -1 for empty sampler, got %d", lightIndex)
	}

	// SampleLightEmission should return nil
	light2, prob2, lightIndex2 := sampler.SampleLightEmission(0.5)
	if light2 != nil {
		t.Error("Expected nil light for empty sampler")
	}
	if prob2 != 0.0 {
		t.Errorf("Expected 0.0 probability for empty sampler, got %f", prob2)
	}
	if lightIndex2 != -1 {
		t.Errorf("Expected light index -1 for empty sampler, got %d", lightIndex2)
	}

	// GetLightCount should return 0
	count := sampler.GetLightCount()
	if count != 0 {
		t.Errorf("Expected light count 0, got %d", count)
	}
}

func TestWeightedLightSampler_ProbabilitiesSum(t *testing.T) {
	lights := []Light{
		&MockLight{emission: core.Vec3{X: 1, Y: 1, Z: 1}, pdf: 0.5},
		&MockLight{emission: core.Vec3{X: 2, Y: 2, Z: 2}, pdf: 0.3},
		&MockLight{emission: core.Vec3{X: 3, Y: 3, Z: 3}, pdf: 0.2},
	}
	weights := []float64{0.15, 0.35, 0.5}
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	// Verify weights sum to 1.0
	sum := 0.0
	for _, weight := range sampler.weights {
		sum += weight
	}
	if math.Abs(sum-1.0) > 1e-6 {
		t.Errorf("Weights should sum to 1.0, got %f", sum)
	}
}
