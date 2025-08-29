package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestUniformInfiniteLight_Type(t *testing.T) {
	light := NewUniformInfiniteLight(core.NewVec3(1, 1, 1))

	if light.Type() != LightTypeInfinite {
		t.Errorf("Expected LightTypeInfinite, got %v", light.Type())
	}
}

func TestUniformInfiniteLight_Sample(t *testing.T) {
	emission := core.NewVec3(2, 3, 4)

	light := NewUniformInfiniteLight(emission)

	point := core.NewVec3(0, 0, 0)
	sample := core.NewVec2(0.5, 0.5)

	lightSample := light.Sample(point, core.NewVec3(0, 1, 0), sample)

	// Check that emission matches
	if lightSample.Emission != emission {
		t.Errorf("Expected emission %v, got %v", emission, lightSample.Emission)
	}

	// Check that distance is infinite
	if !math.IsInf(lightSample.Distance, 1) {
		t.Errorf("Expected infinite distance, got %f", lightSample.Distance)
	}

	// Check that PDF is cosine-weighted hemisphere
	// PDF should be cosTheta/π where cosTheta is direction dot surfaceNormal
	surfaceNormal := core.NewVec3(0, 1, 0)
	cosTheta := lightSample.Direction.Dot(surfaceNormal)
	expectedPDF := cosTheta / math.Pi
	if math.Abs(lightSample.PDF-expectedPDF) > 1e-6 {
		t.Errorf("Expected PDF %f (cosTheta=%f), got %f", expectedPDF, cosTheta, lightSample.PDF)
	}

	// Check that direction is normalized
	dirLength := lightSample.Direction.Length()
	if math.Abs(dirLength-1.0) > 1e-10 {
		t.Errorf("Expected normalized direction, got length %f", dirLength)
	}

	// Check that normal points toward scene (opposite to direction)
	expectedNormal := lightSample.Direction.Multiply(-1)
	if !lightSample.Normal.Equals(expectedNormal) {
		t.Errorf("Expected normal %v, got %v", expectedNormal, lightSample.Normal)
	}
}

func TestUniformInfiniteLight_PDF(t *testing.T) {
	light := NewUniformInfiniteLight(core.NewVec3(1, 1, 1))

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(0, 1, 0)

	normal := core.NewVec3(0, 1, 0)
	pdf := light.PDF(point, normal, direction)

	// For cosine-weighted hemisphere: PDF = cosTheta/π
	cosTheta := direction.Dot(normal)
	expectedPDF := cosTheta / math.Pi

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected PDF %f (cosTheta=%f), got %f", expectedPDF, cosTheta, pdf)
	}
}

func TestUniformInfiniteLight_SampleEmission(t *testing.T) {
	emission := core.NewVec3(5, 6, 7)
	worldCenter := core.NewVec3(1, 2, 3)
	worldRadius := 15.0

	light := NewUniformInfiniteLight(emission)
	light.Preprocess(&core.BVH{
		Center: worldCenter,
		Radius: worldRadius,
	})

	samplePoint := core.NewVec2(0.3, 0.7)
	sampleDirection := core.NewVec2(0.1, 0.9)

	emissionSample := light.SampleEmission(samplePoint, sampleDirection)

	// Check emission matches
	if emissionSample.Emission != emission {
		t.Errorf("Expected emission %v, got %v", emission, emissionSample.Emission)
	}

	// Check area PDF
	expectedAreaPDF := 1.0 / (math.Pi * worldRadius * worldRadius)
	if math.Abs(emissionSample.AreaPDF-expectedAreaPDF) > 1e-10 {
		t.Errorf("Expected area PDF %f, got %f", expectedAreaPDF, emissionSample.AreaPDF)
	}

	// Check direction PDF
	expectedDirectionPDF := 1.0 / (4.0 * math.Pi)
	if math.Abs(emissionSample.DirectionPDF-expectedDirectionPDF) > 1e-10 {
		t.Errorf("Expected direction PDF %f, got %f", expectedDirectionPDF, emissionSample.DirectionPDF)
	}

	// Check that normal points toward scene (opposite to ray direction)
	expectedNormal := emissionSample.Direction.Multiply(-1)
	if !emissionSample.Normal.Equals(expectedNormal) {
		t.Errorf("Expected normal %v (toward scene), got %v", expectedNormal, emissionSample.Normal)
	}
}

func TestUniformInfiniteLight_EmissionPDF(t *testing.T) {
	worldRadius := 20.0
	light := NewUniformInfiniteLight(core.NewVec3(1, 1, 1))

	light.Preprocess(&core.BVH{Radius: worldRadius})

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(1, 0, 0)

	pdf := light.EmissionPDF(point, direction)
	expectedPDF := 1.0 / (math.Pi * worldRadius * worldRadius)

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected emission PDF %f, got %f", expectedPDF, pdf)
	}
}

func TestUniformInfiniteLight_EmissionPDF_ZeroRadius(t *testing.T) {
	light := NewUniformInfiniteLight(core.NewVec3(1, 1, 1))

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(1, 0, 0)

	pdf := light.EmissionPDF(point, direction)

	if pdf != 0.0 {
		t.Errorf("Expected zero PDF for zero radius, got %f", pdf)
	}
}

func TestUniformSampleSphere(t *testing.T) {
	// Test that uniformSampleSphere produces normalized vectors
	samples := []core.Vec2{
		core.NewVec2(0, 0),
		core.NewVec2(0.5, 0.5),
		core.NewVec2(1, 1),
		core.NewVec2(0.25, 0.75),
	}

	for i, sample := range samples {
		direction := core.SampleUniformSphere(sample)

		// Check normalization
		length := direction.Length()
		if math.Abs(length-1.0) > 1e-10 {
			t.Errorf("Sample %d: expected normalized direction, got length %f", i, length)
		}

		// Check that z component is in valid range
		if direction.Z < -1.0 || direction.Z > 1.0 {
			t.Errorf("Sample %d: z component %f out of range [-1, 1]", i, direction.Z)
		}
	}
}

// TestUniformInfiniteLight_ParallelRays tests that rays with same direction sample are parallel
func TestUniformInfiniteLight_ParallelRays(t *testing.T) {
	worldRadius := 10.0
	light := NewUniformInfiniteLight(core.NewVec3(1, 1, 1))
	worldCenter := core.NewVec3(0, 0, 0)

	light.Preprocess(&core.BVH{Radius: worldRadius, Center: worldCenter})

	// Same direction sample, different point samples should give parallel rays
	directionSample := core.NewVec2(0.3, 0.7)

	sample1 := light.SampleEmission(core.NewVec2(0.1, 0.2), directionSample)
	sample2 := light.SampleEmission(core.NewVec2(0.8, 0.9), directionSample)
	sample3 := light.SampleEmission(core.NewVec2(0.5, 0.5), directionSample)

	// All rays should have the same direction
	if !sample1.Direction.Equals(sample2.Direction) {
		t.Errorf("Expected parallel rays, got directions %v and %v", sample1.Direction, sample2.Direction)
	}
	if !sample1.Direction.Equals(sample3.Direction) {
		t.Errorf("Expected parallel rays, got directions %v and %v", sample1.Direction, sample3.Direction)
	}
}
