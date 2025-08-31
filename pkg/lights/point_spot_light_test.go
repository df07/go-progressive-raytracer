package lights

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestPointSpotLight_NewPointSpotLight(t *testing.T) {
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(1, 1, 1)
	coneAngleDegrees := 45.0
	coneDeltaAngleDegrees := 5.0

	light := NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)

	if light.position.Subtract(from).Length() > 1e-6 {
		t.Errorf("Expected position %v, got %v", from, light.position)
	}

	expectedDirection := to.Subtract(from).Normalize()
	if light.direction.Subtract(expectedDirection).Length() > 1e-6 {
		t.Errorf("Expected direction %v, got %v", expectedDirection, light.direction)
	}

	if light.emission.Subtract(emission).Length() > 1e-6 {
		t.Errorf("Expected emission %v, got %v", emission, light.emission)
	}

	// Check that cosine values were computed correctly
	expectedCosTotalWidth := math.Cos(coneAngleDegrees * math.Pi / 180.0)
	if math.Abs(light.cosTotalWidth-expectedCosTotalWidth) > 1e-6 {
		t.Errorf("Expected cosTotalWidth %v, got %v", expectedCosTotalWidth, light.cosTotalWidth)
	}

	expectedCosFalloffStart := math.Cos((coneAngleDegrees - coneDeltaAngleDegrees) * math.Pi / 180.0)
	if math.Abs(light.cosFalloffStart-expectedCosFalloffStart) > 1e-6 {
		t.Errorf("Expected cosFalloffStart %v, got %v", expectedCosFalloffStart, light.cosFalloffStart)
	}
}

func TestPointSpotLight_Sample_WithinCone(t *testing.T) {
	// Light pointing down from (0,5,0) to (0,0,0)
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(100, 100, 100)
	coneAngleDegrees := 60.0     // 60 degrees total
	coneDeltaAngleDegrees := 5.0 // 5 degree falloff

	light := NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)

	// Test point directly below the light (should receive full intensity)
	testPoint := core.NewVec3(0, 1, 0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := light.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

	// Check that sample point is the light position
	if sample.Point.Subtract(from).Length() > 1e-6 {
		t.Errorf("Expected sample point to be light position %v, got %v", from, sample.Point)
	}

	// Check direction points towards light
	expectedDirection := from.Subtract(testPoint).Normalize()
	if sample.Direction.Subtract(expectedDirection).Length() > 1e-6 {
		t.Errorf("Expected direction %v, got %v", expectedDirection, sample.Direction)
	}

	// Check distance
	expectedDistance := from.Subtract(testPoint).Length()
	if math.Abs(sample.Distance-expectedDistance) > 1e-6 {
		t.Errorf("Expected distance %v, got %v", expectedDistance, sample.Distance)
	}

	// Point directly below should receive maximum intensity (cosAngle = 1, spotAttenuation = 1)
	// Emission should be reduced by distance squared: emission / (distance^2)
	originalEmissionMagnitude := emission.Length() // sqrt(100² + 100² + 100²)
	expectedEmissionMagnitude := originalEmissionMagnitude / (expectedDistance * expectedDistance)
	actualEmissionMagnitude := sample.Emission.Length()
	if math.Abs(actualEmissionMagnitude-expectedEmissionMagnitude) > 1e-6 {
		t.Errorf("Expected emission magnitude %v, got %v", expectedEmissionMagnitude, actualEmissionMagnitude)
	}
}

func TestPointSpotLight_Sample_OutsideCone(t *testing.T) {
	// Light pointing down with narrow cone
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(100, 100, 100)
	coneAngleDegrees := 30.0     // 30 degrees total
	coneDeltaAngleDegrees := 5.0 // 5 degree falloff

	light := NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)

	// Test point far to the side (should be outside cone)
	testPoint := core.NewVec3(10, 1, 0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := light.Sample(testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

	// Should receive minimal or no light due to being outside cone
	emissionMagnitude := sample.Emission.Length()
	if emissionMagnitude > 0.1 { // Should be very dim or zero
		t.Errorf("Expected very low emission for point outside cone, got magnitude %v", emissionMagnitude)
	}
}

func TestPointSpotLight_PDF(t *testing.T) {
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(100, 100, 100)
	coneAngleDegrees := 60.0
	coneDeltaAngleDegrees := 5.0

	light := NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)

	testPoint := core.NewVec3(0, 1, 0)

	// Direction pointing towards light
	directionToLight := from.Subtract(testPoint).Normalize()
	pdf := light.PDF(testPoint, core.NewVec3(0, 1, 0), directionToLight)
	if pdf != 1.0 {
		t.Errorf("Expected PDF 1.0 for direction towards light, got %v", pdf)
	}

	// Direction pointing away from light
	directionAway := directionToLight.Multiply(-1)
	pdf = light.PDF(testPoint, core.NewVec3(0, 1, 0), directionAway)
	if pdf != 0.0 {
		t.Errorf("Expected PDF 0.0 for direction away from light, got %v", pdf)
	}

	// Random direction not towards light
	randomDirection := core.NewVec3(1, 0, 0).Normalize()
	pdf = light.PDF(testPoint, core.NewVec3(0, 1, 0), randomDirection)
	if pdf != 0.0 {
		t.Errorf("Expected PDF 0.0 for random direction, got %v", pdf)
	}
}

func TestPointSpotLight_GetIntensityAt(t *testing.T) {
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(100, 100, 100)
	coneAngleDegrees := 60.0
	coneDeltaAngleDegrees := 5.0

	light := NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)

	// Test point directly below (should get full intensity)
	testPoint := core.NewVec3(0, 1, 0)
	intensity := light.GetIntensityAt(testPoint)

	distance := from.Subtract(testPoint).Length()
	expectedIntensity := emission.Multiply(1.0 / (distance * distance)) // Full spot attenuation (1.0)

	if intensity.Subtract(expectedIntensity).Length() > 1e-6 {
		t.Errorf("Expected intensity %v, got %v", expectedIntensity, intensity)
	}

	// Test point far to the side (should get no intensity)
	testPointOutside := core.NewVec3(10, 1, 0)
	intensity = light.GetIntensityAt(testPointOutside)

	if intensity.Length() > 1e-6 {
		t.Errorf("Expected zero intensity for point outside cone, got %v", intensity)
	}
}
