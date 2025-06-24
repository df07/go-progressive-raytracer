package geometry

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestDiscLightSample(t *testing.T) {
	// Create a disc light at origin facing up
	center := core.NewVec3(0, 1, 0)
	normal := core.NewVec3(0, -1, 0) // Light facing down
	radius := 1.0
	emission := core.NewVec3(10, 10, 10)
	emissive := material.NewEmissive(emission)
	discLight := NewDiscLight(center, normal, radius, emissive)

	// Test point below the light
	testPoint := core.NewVec3(0, 0, 0)
	random := rand.New(rand.NewSource(42))

	sample := discLight.Sample(testPoint, random)

	// Check that sample point is on the disc
	distanceFromCenter := sample.Point.Subtract(center).Length()
	if distanceFromCenter > radius+1e-6 {
		t.Errorf("Sample point outside disc: distance=%v, radius=%v", distanceFromCenter, radius)
	}

	// Check that direction points from test point to sample point
	expectedDirection := sample.Point.Subtract(testPoint).Normalize()
	if !sample.Direction.Equals(expectedDirection) {
		t.Errorf("Expected direction %v, got %v", expectedDirection, sample.Direction)
	}

	// Check that distance is correct
	expectedDistance := sample.Point.Subtract(testPoint).Length()
	if math.Abs(sample.Distance-expectedDistance) > 1e-6 {
		t.Errorf("Expected distance %v, got %v", expectedDistance, sample.Distance)
	}

	// Check that PDF is positive
	if sample.PDF <= 0 {
		t.Errorf("PDF should be positive, got %v", sample.PDF)
	}

	// Check that emission is present
	if sample.Emission.Equals(core.NewVec3(0, 0, 0)) {
		t.Errorf("Expected non-zero emission, got %v", sample.Emission)
	}
}

func TestDiscLightPDF(t *testing.T) {
	center := core.NewVec3(0, 1, 0)
	normal := core.NewVec3(0, -1, 0)
	radius := 1.0
	emission := core.NewVec3(10, 10, 10)
	emissive := material.NewEmissive(emission)
	discLight := NewDiscLight(center, normal, radius, emissive)

	testPoint := core.NewVec3(0, 0, 0)

	tests := []struct {
		name      string
		direction core.Vec3
		shouldHit bool
	}{
		{
			name:      "Direction hits center of disc",
			direction: core.NewVec3(0, 1, 0).Normalize(),
			shouldHit: true,
		},
		{
			name:      "Direction hits edge of disc",
			direction: core.NewVec3(1, 1, 0).Normalize(),
			shouldHit: true,
		},
		{
			name:      "Direction misses disc",
			direction: core.NewVec3(2, 1, 0).Normalize(),
			shouldHit: false,
		},
		{
			name:      "Direction parallel to disc",
			direction: core.NewVec3(1, 0, 0).Normalize(),
			shouldHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := discLight.PDF(testPoint, tt.direction)

			if tt.shouldHit {
				if pdf <= 0 {
					t.Errorf("Expected positive PDF for hit, got %v", pdf)
				}
			} else {
				if pdf != 0 {
					t.Errorf("Expected zero PDF for miss, got %v", pdf)
				}
			}
		})
	}
}

func TestDiscLightSampleConsistency(t *testing.T) {
	// Test that PDF(point, direction) matches the PDF returned by Sample()
	center := core.NewVec3(0, 2, 0)
	normal := core.NewVec3(0, -1, 0)
	radius := 0.5
	emission := core.NewVec3(5, 5, 5)
	emissive := material.NewEmissive(emission)
	discLight := NewDiscLight(center, normal, radius, emissive)

	testPoint := core.NewVec3(0, 0, 0)
	random := rand.New(rand.NewSource(123))

	// Take multiple samples and check PDF consistency
	for i := 0; i < 100; i++ {
		sample := discLight.Sample(testPoint, random)
		pdfFromMethod := discLight.PDF(testPoint, sample.Direction)

		// The PDFs should be reasonably close (allowing for numerical precision)
		tolerance := 1e-10
		if math.Abs(sample.PDF-pdfFromMethod) > tolerance {
			t.Errorf("PDF mismatch: Sample PDF=%v, Method PDF=%v", sample.PDF, pdfFromMethod)
		}
	}
}

func TestDiscLightSampleDistribution(t *testing.T) {
	// Test that sampling produces roughly uniform distribution on disc
	center := core.NewVec3(0, 1, 0)
	normal := core.NewVec3(0, -1, 0)
	radius := 1.0
	emission := core.NewVec3(1, 1, 1)
	emissive := material.NewEmissive(emission)
	discLight := NewDiscLight(center, normal, radius, emissive)

	testPoint := core.NewVec3(0, 0, 0)
	random := rand.New(rand.NewSource(456))

	numSamples := 10000
	centerCount := 0
	outerCount := 0

	for i := 0; i < numSamples; i++ {
		sample := discLight.Sample(testPoint, random)
		
		// Check if sample is in center circle (radius 0.5) or outer ring
		distanceFromCenter := sample.Point.Subtract(center).Length()
		if distanceFromCenter <= 0.5 {
			centerCount++
		} else {
			outerCount++
		}
	}

	// Area of inner circle = π * 0.5² = π/4
	// Area of outer ring = π * 1² - π * 0.5² = 3π/4
	// So we expect roughly 1:3 ratio (center:outer)
	expectedCenterRatio := 0.25
	actualCenterRatio := float64(centerCount) / float64(numSamples)

	tolerance := 0.05 // 5% tolerance
	if math.Abs(actualCenterRatio-expectedCenterRatio) > tolerance {
		t.Errorf("Expected center ratio ~%v, got %v", expectedCenterRatio, actualCenterRatio)
	}
}

func TestDiscLightEdgeCase(t *testing.T) {
	// Test edge case where test point is close to but not exactly at the light
	center := core.NewVec3(0, 1, 0)
	normal := core.NewVec3(0, -1, 0)
	radius := 1.0
	emission := core.NewVec3(1, 1, 1)
	emissive := material.NewEmissive(emission)
	discLight := NewDiscLight(center, normal, radius, emissive)

	testPoint := core.NewVec3(0, 0.99, 0) // Very close to disc
	random := rand.New(rand.NewSource(789))

	sample := discLight.Sample(testPoint, random)

	// Should handle very small distances gracefully
	if sample.Distance <= 0 {
		t.Errorf("Expected positive distance, got %v", sample.Distance)
	}

	// PDF should be positive for valid samples
	if sample.PDF <= 0 {
		t.Errorf("Expected positive PDF, got %v", sample.PDF)
	}

	// Sample point should be on the disc
	distanceFromCenter := sample.Point.Subtract(center).Length()
	if distanceFromCenter > radius+1e-6 {
		t.Errorf("Sample point outside disc: distance=%v, radius=%v", distanceFromCenter, radius)
	}
}