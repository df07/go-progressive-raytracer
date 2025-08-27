package core

import (
	"math"
	"math/rand"
	"testing"
)

// MockLight implements the Light interface for testing
type MockLight struct {
	emission Vec3
	pdf      float64
}

func (ml *MockLight) Type() LightType {
	return LightTypeArea
}

func (ml *MockLight) Sample(point Vec3, normal Vec3, sample Vec2) LightSample {
	return LightSample{
		Point:     Vec3{X: 0, Y: 1, Z: 0},
		Normal:    Vec3{X: 0, Y: -1, Z: 0},
		Direction: Vec3{X: 0, Y: 1, Z: 0},
		Distance:  1.0,
		Emission:  ml.emission,
		PDF:       ml.pdf,
	}
}

func (ml *MockLight) PDF(point, normal, direction Vec3) float64 {
	return ml.pdf
}

func (ml *MockLight) SampleEmission(samplePoint Vec2, sampleDirection Vec2) EmissionSample {
	return EmissionSample{
		Point:        Vec3{X: 0, Y: 1, Z: 0},
		Normal:       Vec3{X: 0, Y: -1, Z: 0},
		Direction:    Vec3{X: 0, Y: -1, Z: 0},
		Emission:     ml.emission,
		AreaPDF:      ml.pdf,
		DirectionPDF: 1.0 / math.Pi, // cosine-weighted
	}
}

func (ml *MockLight) EmissionPDF(point Vec3, direction Vec3) float64 {
	return ml.pdf
}

func (ml *MockLight) Emit(ray Ray) Vec3 {
	// Mock light doesn't emit in arbitrary directions (finite light)
	return Vec3{X: 0, Y: 0, Z: 0}
}

// Note: CalculatePower method removed - using importance-based sampling now

// MockScene implements the Scene interface for testing
type MockScene struct {
	lights       []Light
	lightSampler LightSampler
	bvh          *BVH
}

func NewMockScene(lights []Light) *MockScene {
	// Create a minimal BVH for testing
	bvh := &BVH{Root: nil, Center: Vec3{}, Radius: 10.0}
	lightSampler := NewUniformLightSampler(lights, 10.0)

	return &MockScene{
		lights:       lights,
		lightSampler: lightSampler,
		bvh:          bvh,
	}
}

func (ms *MockScene) GetCamera() Camera                 { return nil }
func (ms *MockScene) GetShapes() []Shape                { return nil }
func (ms *MockScene) GetLights() []Light                { return ms.lights }
func (ms *MockScene) GetLightSampler() LightSampler     { return ms.lightSampler }
func (ms *MockScene) GetSamplingConfig() SamplingConfig { return SamplingConfig{} }
func (ms *MockScene) GetBVH() *BVH                      { return ms.bvh }
func (ms *MockScene) Preprocess() error                 { return nil }

func TestSampleLightEmission(t *testing.T) {
	// Test with no lights
	emptyScene := NewMockScene([]Light{})
	_, found := SampleLightEmission(emptyScene.lights, emptyScene.lightSampler, NewRandomSampler(rand.New(rand.NewSource(42))))
	if found {
		t.Error("Expected no sample from empty light list")
	}

	// Test with single light
	emission := NewVec3(5.0, 5.0, 5.0)
	mockLight := &MockLight{emission: emission, pdf: 0.5}
	singleLightScene := NewMockScene([]Light{mockLight})

	random := rand.New(rand.NewSource(42))
	sample, found := SampleLightEmission(singleLightScene.lights, singleLightScene.lightSampler, NewRandomSampler(random))

	if !found {
		t.Error("Expected to find sample from single light")
	}

	// With importance sampling for emission, we use uniform selection (no surface point)
	// So the area PDF should just be the light's PDF
	expectedAreaPDF := mockLight.pdf
	if math.Abs(sample.AreaPDF-expectedAreaPDF) > 1e-9 {
		t.Errorf("AreaPDF incorrect: got %f, expected %f", sample.AreaPDF, expectedAreaPDF)
	}

	if sample.Emission != emission {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}

	// Test with multiple lights
	mockLight2 := &MockLight{emission: NewVec3(3.0, 3.0, 3.0), pdf: 0.8}
	multiLightScene := NewMockScene([]Light{mockLight, mockLight2})

	sample2, found2 := SampleLightEmission(multiLightScene.lights, multiLightScene.lightSampler, NewRandomSampler(random))
	if !found2 {
		t.Error("Expected to find sample from multiple lights")
	}

	// Area PDF should be reasonable for multiple lights
	if sample2.AreaPDF > 1.0 {
		t.Errorf("AreaPDF too high for multiple lights: %f", sample2.AreaPDF)
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
			result := PowerHeuristic(tt.nf, tt.fPdf, tt.ng, tt.gPdf)
			if math.Abs(result-tt.expected) > 1e-5 {
				t.Errorf("PowerHeuristic: got %f, expected %f", result, tt.expected)
			}
		})
	}
}

func TestBalanceHeuristic(t *testing.T) {
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
			expected: 0.8, // 0.8 / (0.8 + 0.2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BalanceHeuristic(tt.nf, tt.fPdf, tt.ng, tt.gPdf)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("BalanceHeuristic: got %f, expected %f", result, tt.expected)
			}
		})
	}
}
