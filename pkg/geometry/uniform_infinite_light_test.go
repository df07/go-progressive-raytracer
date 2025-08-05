package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestUniformInfiniteLight_Type(t *testing.T) {
	light := NewUniformInfiniteLight(
		core.NewVec3(1, 1, 1),
		core.NewVec3(0, 0, 0),
		10,
	)

	if light.Type() != core.LightTypeInfinite {
		t.Errorf("Expected LightTypeInfinite, got %v", light.Type())
	}
}

func TestUniformInfiniteLight_Sample(t *testing.T) {
	emission := core.NewVec3(2, 3, 4)
	worldCenter := core.NewVec3(0, 0, 0)
	worldRadius := 10.0

	light := NewUniformInfiniteLight(emission, worldCenter, worldRadius)

	point := core.NewVec3(0, 0, 0)
	sample := core.NewVec2(0.5, 0.5)

	lightSample := light.Sample(point, sample)

	// Check that emission matches
	if lightSample.Emission != emission {
		t.Errorf("Expected emission %v, got %v", emission, lightSample.Emission)
	}

	// Check that distance is infinite
	if !math.IsInf(lightSample.Distance, 1) {
		t.Errorf("Expected infinite distance, got %f", lightSample.Distance)
	}

	// Check that PDF is uniform over sphere
	expectedPDF := 1.0 / (4.0 * math.Pi)
	if math.Abs(lightSample.PDF-expectedPDF) > 1e-10 {
		t.Errorf("Expected PDF %f, got %f", expectedPDF, lightSample.PDF)
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
	light := NewUniformInfiniteLight(
		core.NewVec3(1, 1, 1),
		core.NewVec3(0, 0, 0),
		10,
	)

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(0, 1, 0)

	pdf := light.PDF(point, direction)
	expectedPDF := 1.0 / (4.0 * math.Pi)

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected PDF %f, got %f", expectedPDF, pdf)
	}
}

func TestUniformInfiniteLight_SampleEmission(t *testing.T) {
	emission := core.NewVec3(5, 6, 7)
	worldCenter := core.NewVec3(1, 2, 3)
	worldRadius := 15.0

	light := NewUniformInfiniteLight(emission, worldCenter, worldRadius)

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

	// Check that emission point is on the world boundary sphere
	vectorFromCenter := emissionSample.Point.Subtract(worldCenter)
	distanceFromCenter := vectorFromCenter.Length()
	if math.Abs(distanceFromCenter-worldRadius) > 1e-6 {
		t.Errorf("Expected emission point at distance %f from center, got %f", worldRadius, distanceFromCenter)
	}

	// Check that direction and normal match
	if !emissionSample.Direction.Equals(emissionSample.Normal) {
		t.Errorf("Expected direction and normal to match for infinite light")
	}
}

func TestUniformInfiniteLight_EmissionPDF(t *testing.T) {
	worldRadius := 20.0
	light := NewUniformInfiniteLight(
		core.NewVec3(1, 1, 1),
		core.NewVec3(0, 0, 0),
		worldRadius,
	)

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(1, 0, 0)

	pdf := light.EmissionPDF(point, direction)
	expectedPDF := 1.0 / (math.Pi * worldRadius * worldRadius)

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected emission PDF %f, got %f", expectedPDF, pdf)
	}
}

func TestUniformInfiniteLight_EmissionPDF_ZeroRadius(t *testing.T) {
	light := NewUniformInfiniteLight(
		core.NewVec3(1, 1, 1),
		core.NewVec3(0, 0, 0),
		0, // Zero radius
	)

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
		direction := uniformSampleSphere(sample)

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
