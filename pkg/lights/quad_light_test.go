package lights

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestQuadLight_Sample_BasicSampling(t *testing.T) {
	const tolerance = 1e-9

	// Create a quad light (unit square in XY plane)
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-0.5, -0.5, 0)
	u := core.NewVec3(1, 0, 0) // X direction
	v := core.NewVec3(0, 1, 0) // Y direction
	light := NewQuadLight(corner, u, v, emissiveMat)

	// Sample point from above the quad
	shadingPoint := core.NewVec3(0, 0, 2)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := light.Sample(shadingPoint, core.NewVec3(0, 0, 1), sampler.Get2D())

	// Verify sample is on the quad surface (Z should be 0)
	if math.Abs(sample.Point.Z) > tolerance {
		t.Errorf("Sample point not on quad surface: Z = %f, expected = 0", sample.Point.Z)
	}

	// Verify sample is within quad bounds
	if sample.Point.X < -0.5 || sample.Point.X > 0.5 ||
		sample.Point.Y < -0.5 || sample.Point.Y > 0.5 {
		t.Errorf("Sample point outside quad bounds: %v", sample.Point)
	}

	// Verify direction points from shading point to light sample
	expectedDirection := sample.Point.Subtract(shadingPoint).Normalize()
	directionError := sample.Direction.Subtract(expectedDirection).Length()
	if directionError > tolerance {
		t.Errorf("Direction incorrect: error = %f", directionError)
	}

	// Verify PDF is positive
	if sample.PDF <= 0 {
		t.Errorf("Expected positive PDF, got %f", sample.PDF)
	}

	// Verify emission is correct
	if sample.Emission.X != emission.X || sample.Emission.Y != emission.Y || sample.Emission.Z != emission.Z {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}
}

func TestQuadLight_Sample_EdgeOnLight(t *testing.T) {
	// Test edge case where light appears edge-on (cosTheta ≈ 0)
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, -0.5, 0)
	u := core.NewVec3(0, 1, 0) // Y direction
	v := core.NewVec3(0, 0, 1) // Z direction
	light := NewQuadLight(corner, u, v, emissiveMat)

	// Sample from a point in the YZ plane (edge-on view)
	// The quad normal is u × v = (0,1,0) × (0,0,1) = (1,0,0)
	// Point at (0,2,0) means any direction to the quad will be perpendicular to the normal
	shadingPoint := core.NewVec3(0, 2, 0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := light.Sample(shadingPoint, core.NewVec3(0, 0, 1), sampler.Get2D())

	// For truly edge-on view, cosTheta should be 0
	// Direction from (0,2,0) to any point (0,Y,Z) on the quad gives direction (0,Y-2,Z)
	// Normal is (1,0,0), so cosTheta = |normal · (-direction)| = |1*0 + 0*(Y-2) + 0*Z| = 0
	if sample.PDF != 0 {
		t.Errorf("Expected PDF = 0 for edge-on light, got %f", sample.PDF)
	}

	// Should return zero emission
	expectedEmission := core.Vec3{X: 0, Y: 0, Z: 0}
	if sample.Emission != expectedEmission {
		t.Errorf("Expected zero emission for edge-on light, got %v", sample.Emission)
	}
}

func TestQuadLight_PDF_HitAndMiss(t *testing.T) {
	const tolerance = 1e-9

	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-1, -1, 0)
	u := core.NewVec3(2, 0, 0) // 2x2 quad
	v := core.NewVec3(0, 2, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	tests := []struct {
		name      string
		point     core.Vec3
		direction core.Vec3
		expectHit bool
	}{
		{
			name:      "Direction hits center of quad",
			point:     core.NewVec3(0, 0, 2),
			direction: core.NewVec3(0, 0, -1),
			expectHit: true,
		},
		{
			name:      "Direction hits corner of quad",
			point:     core.NewVec3(-1, -1, 2),
			direction: core.NewVec3(0, 0, -1),
			expectHit: true,
		},
		{
			name:      "Direction misses quad",
			point:     core.NewVec3(0, 0, 2),
			direction: core.NewVec3(1, 1, -1).Normalize(),
			expectHit: false,
		},
		{
			name:      "Direction away from quad",
			point:     core.NewVec3(0, 0, 2),
			direction: core.NewVec3(0, 0, 1),
			expectHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := light.PDF(tt.point, core.NewVec3(0, 0, 1), tt.direction)

			if !tt.expectHit {
				if pdf != 0 {
					t.Errorf("Expected PDF = 0 for direction that misses quad, got %f", pdf)
				}
				return
			}

			// Verify PDF is positive for valid directions
			if pdf <= 0 {
				t.Errorf("Expected positive PDF for hit, got %f", pdf)
			}
		})
	}
}

func TestQuadLight_PDF_SolidAngleCalculation(t *testing.T) {
	const tolerance = 1e-6

	// Create unit square quad
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-0.5, -0.5, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	// Test from point directly above center
	point := core.NewVec3(0, 0, 1)
	direction := core.NewVec3(0, 0, -1)

	pdf := light.PDF(point, core.NewVec3(0, 0, 1), direction)

	// Calculate expected solid angle PDF
	// For a unit square at distance 1, with normal aligned with ray:
	// Area = 1, distance = 1, cosTheta = 1
	// PDF = (1/Area) * distance² / cosTheta = 1 * 1 / 1 = 1
	expectedPDF := 1.0
	if math.Abs(pdf-expectedPDF) > tolerance {
		t.Errorf("PDF calculation incorrect: got %f, expected %f", pdf, expectedPDF)
	}
}

func TestQuadLight_SampleEmission_BasicProperties(t *testing.T) {
	const tolerance = 1e-9

	// Create a quad light
	emission := core.NewVec3(3.0, 3.0, 3.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-1, -1, 0)
	u := core.NewVec3(2, 0, 0) // 2x2 quad
	v := core.NewVec3(0, 2, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	sample := light.SampleEmission(sampler.Get2D(), sampler.Get2D())

	// Verify sample point is on quad surface (Z = 0)
	if math.Abs(sample.Point.Z) > tolerance {
		t.Errorf("Sample point not on quad surface: Z = %f", sample.Point.Z)
	}

	// Verify sample point is within quad bounds
	if sample.Point.X < -1 || sample.Point.X > 1 ||
		sample.Point.Y < -1 || sample.Point.Y > 1 {
		t.Errorf("Sample point outside quad bounds: %v", sample.Point)
	}

	// Verify normal is correct (should be (0,0,1) for XY plane)
	expectedNormal := core.NewVec3(0, 0, 1)
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

	// Verify PDFs are positive
	if sample.AreaPDF <= 0 {
		t.Errorf("AreaPDF should be positive, got %f", sample.AreaPDF)
	}
	if sample.DirectionPDF <= 0 {
		t.Errorf("DirectionPDF should be positive, got %f", sample.DirectionPDF)
	}

	// Expected area PDF: 1/Area = 1/4
	expectedAreaPDF := 1.0 / 4.0
	if math.Abs(sample.AreaPDF-expectedAreaPDF) > tolerance {
		t.Errorf("AreaPDF incorrect: got %f, expected %f", sample.AreaPDF, expectedAreaPDF)
	}

	// Expected direction PDF: cosTheta/π
	expectedDirPDF := cosTheta / math.Pi
	if math.Abs(sample.DirectionPDF-expectedDirPDF) > tolerance {
		t.Errorf("DirectionPDF incorrect: got %f, expected %f", sample.DirectionPDF, expectedDirPDF)
	}

	// Verify emission is correct
	if sample.Emission.X != emission.X || sample.Emission.Y != emission.Y || sample.Emission.Z != emission.Z {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}
}

func TestQuadLight_EmissionPDF_PointValidation(t *testing.T) {
	const tolerance = 1e-6

	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	tests := []struct {
		name      string
		point     core.Vec3
		direction core.Vec3
		expectPDF bool
	}{
		{
			name:      "Point on quad surface",
			point:     core.NewVec3(0.5, 0.5, 0),
			direction: core.NewVec3(0, 0, 1),
			expectPDF: true,
		},
		{
			name:      "Point at quad corner",
			point:     core.NewVec3(0, 0, 0),
			direction: core.NewVec3(0, 0, 1),
			expectPDF: true,
		},
		{
			name:      "Point at quad edge",
			point:     core.NewVec3(1, 0.5, 0),
			direction: core.NewVec3(0, 0, 1),
			expectPDF: true,
		},
		{
			name:      "Point outside quad bounds",
			point:     core.NewVec3(2, 2, 0),
			direction: core.NewVec3(0, 0, 1),
			expectPDF: false,
		},
		{
			name:      "Point not on quad plane",
			point:     core.NewVec3(0.5, 0.5, 1),
			direction: core.NewVec3(0, 0, 1),
			expectPDF: false,
		},
		{
			name:      "Negative parametric coordinates",
			point:     core.NewVec3(-0.5, 0.5, 0),
			direction: core.NewVec3(0, 0, 1),
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

			// Verify PDF formula (area-only PDF)
			expectedAreaPDF := 1.0 / 1.0 // Unit square has area 1
			if math.Abs(pdf-expectedAreaPDF) > tolerance {
				t.Errorf("PDF incorrect: got %f, expected %f", pdf, expectedAreaPDF)
			}
		})
	}
}

func TestQuadLight_Type(t *testing.T) {
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	if light.Type() != LightTypeArea {
		t.Errorf("Expected LightTypeArea, got %v", light.Type())
	}
}

func TestQuadLight_Emit_WithEmissiveMaterial(t *testing.T) {
	emission := core.NewVec3(2.0, 3.0, 4.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	ray := core.NewRay(core.NewVec3(0, 0, 1), core.NewVec3(0, 0, -1))
	result := light.Emit(ray, nil)

	if result.X != emission.X || result.Y != emission.Y || result.Z != emission.Z {
		t.Errorf("Emit incorrect: got %v, expected %v", result, emission)
	}
}

func TestQuadLight_Emit_WithNonEmissiveMaterial(t *testing.T) {
	// Test with a non-emissive material
	lambertian := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, lambertian)

	ray := core.NewRay(core.NewVec3(0, 0, 1), core.NewVec3(0, 0, -1))
	result := light.Emit(ray, nil)

	expectedEmission := core.Vec3{X: 0, Y: 0, Z: 0}
	if result != expectedEmission {
		t.Errorf("Emit should be zero for non-emissive material: got %v", result)
	}
}

func TestQuadLight_MultipleDirections_Coverage(t *testing.T) {
	// Test that sampling produces diverse points across the quad surface
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-1, -1, 0)
	u := core.NewVec3(2, 0, 0) // 2x2 quad
	v := core.NewVec3(0, 2, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	shadingPoint := core.NewVec3(0, 0, 2)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Generate multiple samples
	numSamples := 100
	samples := make([]LightSample, numSamples)
	for i := 0; i < numSamples; i++ {
		samples[i] = light.Sample(shadingPoint, core.NewVec3(0, 0, 1), sampler.Get2D())
	}

	// Verify all samples are valid
	for i, sample := range samples {
		// Check that sample is on quad surface
		if math.Abs(sample.Point.Z) > 1e-6 {
			t.Errorf("Sample %d not on quad surface", i)
		}

		// Check that sample is within quad bounds
		if sample.Point.X < -1 || sample.Point.X > 1 ||
			sample.Point.Y < -1 || sample.Point.Y > 1 {
			t.Errorf("Sample %d outside quad bounds", i)
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

	// Check for coverage in different quadrants
	quadrantCounts := make(map[string]int)
	for _, sample := range samples {
		quadrant := ""
		if sample.Point.X >= 0 {
			quadrant += "+"
		} else {
			quadrant += "-"
		}
		if sample.Point.Y >= 0 {
			quadrant += "+"
		} else {
			quadrant += "-"
		}
		quadrantCounts[quadrant]++
	}

	// Verify all quadrants are represented
	expectedQuadrants := []string{"++", "+-", "-+", "--"}
	for _, quadrant := range expectedQuadrants {
		if quadrantCounts[quadrant] == 0 {
			t.Errorf("Quadrant %s not sampled", quadrant)
		}
	}
}

func TestQuadLight_EdgeCase_ZeroArea(t *testing.T) {
	// Test with degenerate quad (zero area)
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(0, 0, 0) // Zero vector
	v := core.NewVec3(1, 0, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	// Should handle gracefully
	if light.Area != 0 {
		t.Errorf("Expected zero area for degenerate quad, got %f", light.Area)
	}

	shadingPoint := core.NewVec3(1, 1, 1)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	sample := light.Sample(shadingPoint, core.NewVec3(0, 0, 1), sampler.Get2D())

	// PDF should be infinite (1/0), but handled gracefully in practice
	// The exact behavior may depend on implementation details
	if !math.IsInf(sample.PDF, 1) && sample.PDF != 0 {
		t.Logf("PDF for zero-area quad: %f", sample.PDF)
	}
}

func TestQuadLight_ConsistencyBetweenSampleAndPDF(t *testing.T) {
	// Test consistency between Sample() and PDF() methods
	const tolerance = 1e-6

	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-0.5, -0.5, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	shadingPoint := core.NewVec3(0, 0, 1)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Generate samples and verify PDF consistency
	numSamples := 50
	for i := 0; i < numSamples; i++ {
		sample := light.Sample(shadingPoint, core.NewVec3(0, 0, 1), sampler.Get2D())

		// Calculate PDF for the same direction
		calculatedPDF := light.PDF(shadingPoint, core.NewVec3(0, 0, 1), sample.Direction)

		// They should match
		if math.Abs(sample.PDF-calculatedPDF) > tolerance {
			t.Errorf("Sample %d: PDF inconsistent - sample=%f, calculated=%f", i, sample.PDF, calculatedPDF)
		}
	}
}

func TestQuadLight_EmissionSampling_ConsistencyWithEmissionPDF(t *testing.T) {
	// Test consistency between SampleEmission() and EmissionPDF() methods
	const tolerance = 1e-6

	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Generate emission samples and verify PDF consistency
	numSamples := 50
	for i := 0; i < numSamples; i++ {
		sample := light.SampleEmission(sampler.Get2D(), sampler.Get2D())

		// Calculate emission PDF for the same point and direction
		calculatedAreaPDF := light.EmissionPDF(sample.Point, sample.Direction)

		// Area PDFs should match
		if math.Abs(sample.AreaPDF-calculatedAreaPDF) > tolerance {
			t.Errorf("Sample %d: AreaPDF inconsistent - sample=%f, calculated=%f", i, sample.AreaPDF, calculatedAreaPDF)
		}
	}
}
