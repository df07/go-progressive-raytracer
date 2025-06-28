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

func (ml *MockLight) Sample(point Vec3, random *rand.Rand) LightSample {
	return LightSample{
		Point:     Vec3{X: 0, Y: 1, Z: 0},
		Normal:    Vec3{X: 0, Y: -1, Z: 0},
		Direction: Vec3{X: 0, Y: 1, Z: 0},
		Distance:  1.0,
		Emission:  ml.emission,
		PDF:       ml.pdf,
	}
}

func (ml *MockLight) PDF(point Vec3, direction Vec3) float64 {
	return ml.pdf
}

func (ml *MockLight) SampleEmission(random *rand.Rand) EmissionSample {
	return EmissionSample{
		Point:     Vec3{X: 0, Y: 1, Z: 0},
		Normal:    Vec3{X: 0, Y: -1, Z: 0},
		Direction: Vec3{X: 0, Y: -1, Z: 0},
		Emission:  ml.emission,
		PDF:       ml.pdf,
	}
}

func (ml *MockLight) EmissionPDF(point Vec3, direction Vec3) float64 {
	return ml.pdf
}

func TestSampleLightEmission(t *testing.T) {
	// Test with no lights
	var emptyLights []Light
	_, found := SampleLightEmission(emptyLights, rand.New(rand.NewSource(42)))
	if found {
		t.Error("Expected no sample from empty light list")
	}

	// Test with single light
	emission := NewVec3(5.0, 5.0, 5.0)
	mockLight := &MockLight{emission: emission, pdf: 0.5}
	lights := []Light{mockLight}

	random := rand.New(rand.NewSource(42))
	sample, found := SampleLightEmission(lights, random)

	if !found {
		t.Error("Expected to find sample from single light")
	}

	// PDF should be divided by number of lights
	expectedPDF := mockLight.pdf / float64(len(lights))
	if math.Abs(sample.PDF-expectedPDF) > 1e-9 {
		t.Errorf("PDF incorrect: got %f, expected %f", sample.PDF, expectedPDF)
	}

	if sample.Emission != emission {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}

	// Test with multiple lights
	mockLight2 := &MockLight{emission: NewVec3(3.0, 3.0, 3.0), pdf: 0.8}
	multiLights := []Light{mockLight, mockLight2}

	sample2, found2 := SampleLightEmission(multiLights, random)
	if !found2 {
		t.Error("Expected to find sample from multiple lights")
	}

	// PDF should be divided by number of lights
	if sample2.PDF > 1.0 {
		t.Errorf("PDF too high for multiple lights: %f", sample2.PDF)
	}
}

func TestCalculateLightEmissionPDF(t *testing.T) {
	// Test with no lights
	var emptyLights []Light
	pdf := CalculateLightEmissionPDF(emptyLights, Vec3{}, Vec3{})
	if pdf != 0.0 {
		t.Errorf("Expected 0 PDF for no lights, got %f", pdf)
	}

	// Test with single light
	mockLight := &MockLight{emission: NewVec3(1.0, 1.0, 1.0), pdf: 0.5}
	lights := []Light{mockLight}

	point := NewVec3(0, 0, 0)
	direction := NewVec3(0, 1, 0)
	pdf = CalculateLightEmissionPDF(lights, point, direction)

	expectedPDF := mockLight.pdf / float64(len(lights))
	if math.Abs(pdf-expectedPDF) > 1e-9 {
		t.Errorf("PDF incorrect: got %f, expected %f", pdf, expectedPDF)
	}

	// Test with multiple lights
	mockLight2 := &MockLight{emission: NewVec3(2.0, 2.0, 2.0), pdf: 0.3}
	multiLights := []Light{mockLight, mockLight2}

	pdf = CalculateLightEmissionPDF(multiLights, point, direction)
	expectedTotal := (mockLight.pdf + mockLight2.pdf) / float64(len(multiLights))
	if math.Abs(pdf-expectedTotal) > 1e-9 {
		t.Errorf("Total PDF incorrect: got %f, expected %f", pdf, expectedTotal)
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
