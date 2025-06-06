package geometry

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestSphereLight_Sample_UniformSampling(t *testing.T) {
	const tolerance = 1e-9

	// Create a spherical light with emissive material
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	// Point inside the sphere should use uniform sampling
	insidePoint := core.NewVec3(0.5, 0, 0)
	random := rand.New(rand.NewSource(42))

	sample := light.Sample(insidePoint, random)

	// Verify sample is on the sphere surface
	distanceToCenter := sample.Point.Subtract(center).Length()
	if math.Abs(distanceToCenter-radius) > tolerance {
		t.Errorf("Sample point not on sphere surface: distance = %f, expected = %f", distanceToCenter, radius)
	}

	// Verify PDF is correct for uniform sphere sampling
	expectedPDF := 1.0 / (4.0 * math.Pi * radius * radius)
	if math.Abs(sample.PDF-expectedPDF) > tolerance {
		t.Errorf("PDF incorrect: got %f, expected %f", sample.PDF, expectedPDF)
	}

	// Verify emission is correct
	if sample.Emission.X != emission.X || sample.Emission.Y != emission.Y || sample.Emission.Z != emission.Z {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}
}

func TestSphereLight_Sample_ConeSampling(t *testing.T) {
	const tolerance = 1e-6

	// Create a spherical light
	emission := core.NewVec3(2.0, 2.0, 2.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	// Point outside the sphere should use cone sampling
	outsidePoint := core.NewVec3(5, 0, 0)
	random := rand.New(rand.NewSource(42))

	sample := light.Sample(outsidePoint, random)

	// Verify sample is on the sphere surface
	distanceToCenter := sample.Point.Subtract(center).Length()
	if math.Abs(distanceToCenter-radius) > tolerance {
		t.Errorf("Sample point not on sphere surface: distance = %f, expected = %f", distanceToCenter, radius)
	}

	// Verify direction points from shading point to sample
	expectedDirection := sample.Point.Subtract(outsidePoint).Normalize()
	directionError := sample.Direction.Subtract(expectedDirection).Length()
	if directionError > tolerance {
		t.Errorf("Direction incorrect: error = %f", directionError)
	}

	// Verify PDF matches cone sampling formula
	distanceToLight := outsidePoint.Subtract(center).Length()
	sinThetaMax := radius / distanceToLight
	cosThetaMax := math.Sqrt(math.Max(0, 1.0-sinThetaMax*sinThetaMax))
	expectedPDF := 1.0 / (2.0 * math.Pi * (1.0 - cosThetaMax))

	if math.Abs(sample.PDF-expectedPDF) > tolerance {
		t.Errorf("PDF incorrect: got %f, expected %f", sample.PDF, expectedPDF)
	}
}

func TestSphereLight_PDF(t *testing.T) {
	const tolerance = 1e-9

	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	tests := []struct {
		name      string
		point     core.Vec3
		direction core.Vec3
		expectPDF bool
	}{
		{
			name:      "Direction hits sphere",
			point:     core.NewVec3(3, 0, 0),
			direction: core.NewVec3(-1, 0, 0),
			expectPDF: true,
		},
		{
			name:      "Direction misses sphere",
			point:     core.NewVec3(3, 0, 0),
			direction: core.NewVec3(0, 1, 0),
			expectPDF: false,
		},
		{
			name:      "Point inside sphere",
			point:     core.NewVec3(0.5, 0, 0),
			direction: core.NewVec3(1, 0, 0),
			expectPDF: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := light.PDF(tt.point, tt.direction)

			if !tt.expectPDF {
				if pdf != 0 {
					t.Errorf("Expected PDF = 0 for direction that misses sphere, got %f", pdf)
				}
				return
			}

			// Verify PDF is positive for valid directions
			if pdf <= 0 {
				t.Errorf("Expected positive PDF, got %f", pdf)
			}

			// For point inside sphere, should match uniform sampling PDF
			distanceToCenter := tt.point.Subtract(center).Length()
			if distanceToCenter <= radius {
				expectedPDF := 1.0 / (4.0 * math.Pi * radius * radius)
				if math.Abs(pdf-expectedPDF) > tolerance {
					t.Errorf("PDF for point inside sphere: got %f, expected %f", pdf, expectedPDF)
				}
			}
		})
	}
}

func TestSphereLight_Sample_MultipleDirections(t *testing.T) {
	// Test that sampling produces diverse directions
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	outsidePoint := core.NewVec3(5, 0, 0)
	random := rand.New(rand.NewSource(42))

	// Generate multiple samples
	numSamples := 100
	samples := make([]core.LightSample, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = light.Sample(outsidePoint, random)
	}

	// Verify all samples are valid
	for i, sample := range samples {
		// Check that sample is on sphere
		distanceToCenter := sample.Point.Subtract(center).Length()
		if math.Abs(distanceToCenter-radius) > 1e-6 {
			t.Errorf("Sample %d not on sphere surface", i)
		}

		// Check that PDF is positive
		if sample.PDF <= 0 {
			t.Errorf("Sample %d has non-positive PDF: %f", i, sample.PDF)
		}

		// Check that direction is normalized
		dirLength := sample.Direction.Length()
		if math.Abs(dirLength-1.0) > 1e-6 {
			t.Errorf("Sample %d direction not normalized: length = %f", i, dirLength)
		}
	}
}

func TestSphereLight_EdgeCase_ZeroRadius(t *testing.T) {
	// Test with very small radius (edge case)
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1e-10
	light := NewSphereLight(center, radius, emissiveMat)

	outsidePoint := core.NewVec3(1, 0, 0)
	random := rand.New(rand.NewSource(42))

	sample := light.Sample(outsidePoint, random)

	// Should still produce valid sample
	if sample.PDF <= 0 {
		t.Errorf("Expected positive PDF even for tiny sphere, got %f", sample.PDF)
	}
}
