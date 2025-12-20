package lights

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := discLight.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

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
			pdf := discLight.PDF(testPoint, core.NewVec3(0, 1, 0), tt.direction)

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(123)))

	// Take multiple samples and check PDF consistency
	for i := 0; i < 100; i++ {
		sample := discLight.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())
		pdfFromMethod := discLight.PDF(testPoint, core.NewVec3(0, 1, 0), sample.Direction)

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(456)))

	numSamples := 10000
	centerCount := 0
	outerCount := 0

	for i := 0; i < numSamples; i++ {
		sample := discLight.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(789)))

	sample := discLight.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

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

func TestDiscLight_SampleEmission(t *testing.T) {
	const tolerance = 1e-9

	// Create a disc light facing upward
	center := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	radius := 1.0
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissiveMat := material.NewEmissive(emission)
	light := NewDiscLight(center, normal, radius, emissiveMat)

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	sample := light.SampleEmission(sampler.Get2D(), sampler.Get2D())

	// Verify sample point is on disc surface
	distanceFromCenter := sample.Point.Subtract(center).Length()
	if distanceFromCenter > radius+tolerance {
		t.Errorf("Sample point not on disc surface: distance = %f, radius = %f", distanceFromCenter, radius)
	}

	// Verify point is on the disc plane
	heightAbovePlane := math.Abs((sample.Point.Subtract(center)).Dot(normal))
	if heightAbovePlane > tolerance {
		t.Errorf("Sample point not on disc plane: height = %f", heightAbovePlane)
	}

	// Verify normal is correct
	expectedNormal := normal
	normalError := sample.Normal.Subtract(expectedNormal).Length()
	if normalError > tolerance {
		t.Errorf("Normal incorrect: error = %f", normalError)
	}

	// Verify direction is in correct hemisphere (cosine with normal > 0)
	cosTheta := sample.Direction.Dot(sample.Normal)
	if cosTheta <= 0 {
		t.Errorf("Emission direction not in correct hemisphere: cos(theta) = %f", cosTheta)
	}

	// Verify direction is normalized
	dirLength := sample.Direction.Length()
	if math.Abs(dirLength-1.0) > tolerance {
		t.Errorf("Direction not normalized: length = %f", dirLength)
	}

	// Verify PDFs are positive and reasonable
	if sample.AreaPDF <= 0 {
		t.Errorf("AreaPDF should be positive, got %f", sample.AreaPDF)
	}
	if sample.DirectionPDF <= 0 {
		t.Errorf("DirectionPDF should be positive, got %f", sample.DirectionPDF)
	}

	// Expected PDFs
	expectedAreaPDF := 1.0 / (math.Pi * radius * radius)
	expectedDirPDF := cosTheta / math.Pi
	if math.Abs(sample.AreaPDF-expectedAreaPDF) > tolerance {
		t.Errorf("AreaPDF incorrect: got %f, expected %f", sample.AreaPDF, expectedAreaPDF)
	}
	if math.Abs(sample.DirectionPDF-expectedDirPDF) > tolerance {
		t.Errorf("DirectionPDF incorrect: got %f, expected %f", sample.DirectionPDF, expectedDirPDF)
	}

	// Verify emission is correct
	if sample.Emission.X != emission.X || sample.Emission.Y != emission.Y || sample.Emission.Z != emission.Z {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}
}

func TestDiscLight_EmissionPDF(t *testing.T) {
	const tolerance = 1e-9

	center := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	radius := 1.0
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	light := NewDiscLight(center, normal, radius, emissiveMat)

	tests := []struct {
		name      string
		point     core.Vec3
		direction core.Vec3
		expectPDF bool
	}{
		{
			name:      "Point on disc, direction in hemisphere",
			point:     core.NewVec3(0.5, 0, 0), // On disc surface
			direction: core.NewVec3(0, 1, 0),   // Same as normal
			expectPDF: true,
		},
		{
			name:      "Point on disc, direction below surface",
			point:     core.NewVec3(0.5, 0, 0), // On disc surface
			direction: core.NewVec3(0, -1, 0),  // Opposite to normal
			expectPDF: false,
		},
		{
			name:      "Point outside disc radius",
			point:     core.NewVec3(2, 0, 0), // Outside disc radius
			direction: core.NewVec3(0, 1, 0),
			expectPDF: false,
		},
		{
			name:      "Point above disc plane",
			point:     core.NewVec3(0.5, 1, 0), // Above disc plane
			direction: core.NewVec3(0, 1, 0),
			expectPDF: false,
		},
		{
			name:      "Point at disc center",
			point:     center, // geometry.Disc center
			direction: normal, // Normal direction
			expectPDF: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf, _ := light.PDF_Le(tt.point, tt.direction)

			if !tt.expectPDF {
				if pdf != 0 {
					t.Errorf("Expected PDF = 0, got %f", pdf)
				}
				return
			}

			// Verify PDF is positive for valid cases
			if pdf <= 0 {
				t.Errorf("Expected positive PDF, got %f", pdf)
			}

			// Verify PDF formula for valid point on disc (area-only PDF)
			cosTheta := tt.direction.Dot(normal)
			if cosTheta > 0 {
				expectedAreaPDF := 1.0 / (math.Pi * radius * radius)
				if math.Abs(pdf-expectedAreaPDF) > tolerance {
					t.Errorf("PDF incorrect: got %f, expected %f", pdf, expectedAreaPDF)
				}
			}
		})
	}
}

func TestDiscLight_EmissionSampling_Coverage(t *testing.T) {
	// Test that emission sampling covers the entire disc surface
	center := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	radius := 1.0
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	light := NewDiscLight(center, normal, radius, emissiveMat)

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	numSamples := 1000

	// Track coverage in different regions (center vs edge)
	centerCount := 0
	edgeCount := 0

	for i := 0; i < numSamples; i++ {
		sample := light.SampleEmission(sampler.Get2D(), sampler.Get2D())

		// Classify by distance from center
		distanceFromCenter := sample.Point.Subtract(center).Length()
		if distanceFromCenter <= 0.5 {
			centerCount++
		} else {
			edgeCount++
		}

		// Verify sample is valid
		if distanceFromCenter > radius+1e-6 {
			t.Errorf("Sample %d not on disc surface", i)
		}

		// Verify point is on disc plane
		heightAbovePlane := math.Abs((sample.Point.Subtract(center)).Dot(normal))
		if heightAbovePlane > 1e-6 {
			t.Errorf("Sample %d not on disc plane", i)
		}

		// Verify direction is in correct hemisphere
		cosTheta := sample.Direction.Dot(sample.Normal)
		if cosTheta <= 0 {
			t.Errorf("Sample %d direction not in correct hemisphere", i)
		}

		// Verify area PDF consistency (PDF_Le returns area PDF as first value)
		calculatedAreaPDF, _ := light.PDF_Le(sample.Point, sample.Direction)
		if math.Abs(sample.AreaPDF-calculatedAreaPDF) > 1e-6 {
			t.Errorf("Sample %d AreaPDF inconsistent: sample=%f, calculated=%f", i, sample.AreaPDF, calculatedAreaPDF)
		}
	}

	// Verify distribution (center has 1/4 area, edge has 3/4 area)
	expectedCenterRatio := 0.25
	actualCenterRatio := float64(centerCount) / float64(numSamples)
	tolerance := 0.1 // Allow 10% variation

	if math.Abs(actualCenterRatio-expectedCenterRatio) > tolerance {
		t.Errorf("Center region poorly sampled: %f ratio (expected ~%f)", actualCenterRatio, expectedCenterRatio)
	}
}
