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

func TestSphereLight_SampleEmission(t *testing.T) {
	const tolerance = 1e-9

	// Create a spherical light with emissive material
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	random := rand.New(rand.NewSource(42))
	sample := light.SampleEmission(random)

	// Verify sample point is on sphere surface
	distanceToCenter := sample.Point.Subtract(center).Length()
	if math.Abs(distanceToCenter-radius) > tolerance {
		t.Errorf("Sample point not on sphere surface: distance = %f, expected = %f", distanceToCenter, radius)
	}

	// Verify normal points outward
	expectedNormal := sample.Point.Subtract(center).Normalize()
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

	// Verify PDF is positive and reasonable
	if sample.PDF <= 0 {
		t.Errorf("PDF should be positive, got %f", sample.PDF)
	}

	// Expected PDF combines area and direction sampling
	expectedAreaPDF := 1.0 / (4.0 * math.Pi * radius * radius)
	expectedDirPDF := cosTheta / math.Pi
	expectedCombinedPDF := expectedAreaPDF * expectedDirPDF
	if math.Abs(sample.PDF-expectedCombinedPDF) > tolerance {
		t.Errorf("PDF incorrect: got %f, expected %f", sample.PDF, expectedCombinedPDF)
	}

	// Verify emission is correct
	if sample.Emission.X != emission.X || sample.Emission.Y != emission.Y || sample.Emission.Z != emission.Z {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}
}

func TestSphereLight_EmissionPDF(t *testing.T) {
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
			name:      "Point on sphere, direction in hemisphere",
			point:     core.NewVec3(1, 0, 0), // On sphere surface
			direction: core.NewVec3(1, 0, 0), // Same as normal
			expectPDF: true,
		},
		{
			name:      "Point on sphere, direction below surface",
			point:     core.NewVec3(1, 0, 0),  // On sphere surface
			direction: core.NewVec3(-1, 0, 0), // Opposite to normal
			expectPDF: false,
		},
		{
			name:      "Point not on sphere",
			point:     core.NewVec3(2, 0, 0), // Outside sphere
			direction: core.NewVec3(1, 0, 0),
			expectPDF: false,
		},
		{
			name:      "Point inside sphere",
			point:     core.NewVec3(0.5, 0, 0), // Inside sphere
			direction: core.NewVec3(1, 0, 0),
			expectPDF: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := light.EmissionPDF(tt.point, tt.direction)

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

			// Verify PDF formula for valid point on sphere
			normal := tt.point.Subtract(center).Normalize()
			cosTheta := tt.direction.Dot(normal)
			if cosTheta > 0 {
				expectedAreaPDF := 1.0 / (4.0 * math.Pi * radius * radius)
				expectedDirPDF := cosTheta / math.Pi
				expectedCombinedPDF := expectedAreaPDF * expectedDirPDF
				if math.Abs(pdf-expectedCombinedPDF) > tolerance {
					t.Errorf("PDF incorrect: got %f, expected %f", pdf, expectedCombinedPDF)
				}
			}
		})
	}
}

func TestSphereLight_EmissionSampling_Coverage(t *testing.T) {
	// Test that emission sampling covers the entire sphere surface
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	random := rand.New(rand.NewSource(42))
	numSamples := 1000

	// Track coverage in different octants
	octantCounts := make(map[string]int)

	for i := 0; i < numSamples; i++ {
		sample := light.SampleEmission(random)

		// Classify by octant
		x := sample.Point.X
		y := sample.Point.Y
		z := sample.Point.Z
		octant := ""
		if x >= 0 {
			octant += "+"
		} else {
			octant += "-"
		}
		if y >= 0 {
			octant += "+"
		} else {
			octant += "-"
		}
		if z >= 0 {
			octant += "+"
		} else {
			octant += "-"
		}
		octantCounts[octant]++

		// Verify sample is valid
		distanceToCenter := sample.Point.Subtract(center).Length()
		if math.Abs(distanceToCenter-radius) > 1e-6 {
			t.Errorf("Sample %d not on sphere surface", i)
		}

		// Verify direction is in correct hemisphere
		cosTheta := sample.Direction.Dot(sample.Normal)
		if cosTheta <= 0 {
			t.Errorf("Sample %d direction not in correct hemisphere", i)
		}

		// Verify PDF consistency
		calculatedPDF := light.EmissionPDF(sample.Point, sample.Direction)
		if math.Abs(sample.PDF-calculatedPDF) > 1e-6 {
			t.Errorf("Sample %d PDF inconsistent: sample=%f, calculated=%f", i, sample.PDF, calculatedPDF)
		}
	}

	// Verify all octants are represented (should have roughly uniform distribution)
	expectedPerOctant := numSamples / 8
	tolerance := expectedPerOctant / 2 // Allow 50% variation

	for octant, count := range octantCounts {
		if count < expectedPerOctant-tolerance || count > expectedPerOctant+tolerance {
			t.Errorf("Octant %s poorly sampled: %d samples (expected ~%d)", octant, count, expectedPerOctant)
		}
	}

	// Ensure all 8 octants are represented
	if len(octantCounts) != 8 {
		t.Errorf("Not all octants sampled: got %d octants", len(octantCounts))
	}
}
