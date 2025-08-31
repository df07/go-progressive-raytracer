package lights

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestCalculateLightPDF_EmptyLights(t *testing.T) {
	// Test with empty lights array
	lights := []Light{}
	sampler := NewUniformLightSampler(lights, 10.0)
	point := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 0, 1)
	direction := core.NewVec3(1, 0, 0)

	pdf := CalculateLightPDF(lights, sampler, point, normal, direction)

	if pdf != 0.0 {
		t.Errorf("Expected PDF = 0 for empty lights, got %f", pdf)
	}
}

func TestCalculateLightPDF_SingleLight(t *testing.T) {
	const tolerance = 1e-9

	// Create a single sphere light
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	lights := []Light{light}
	sampler := NewUniformLightSampler(lights, 10.0)

	// Test point outside sphere
	point := core.NewVec3(3, 0, 0)
	normal := core.NewVec3(0, 0, 1)
	direction := core.NewVec3(-1, 0, 0) // Direction toward sphere

	pdf := CalculateLightPDF(lights, sampler, point, normal, direction)

	// For single light with uniform sampling, combined PDF should equal light's PDF
	expectedPDF := light.PDF(point, normal, direction) * 1.0 // * light selection probability (1.0 for single light)
	if math.Abs(pdf-expectedPDF) > tolerance {
		t.Errorf("PDF incorrect: got %f, expected %f", pdf, expectedPDF)
	}

	// Test direction that misses the light
	missDirection := core.NewVec3(0, 1, 0)
	missPDF := CalculateLightPDF(lights, sampler, point, normal, missDirection)
	if missPDF != 0.0 {
		t.Errorf("Expected PDF = 0 for direction missing light, got %f", missPDF)
	}
}

func TestCalculateLightPDF_MultipleLights(t *testing.T) {
	const tolerance = 1e-9

	// Create two sphere lights
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)

	light1 := NewSphereLight(core.NewVec3(-2, 0, 0), 1.0, emissiveMat)
	light2 := NewSphereLight(core.NewVec3(2, 0, 0), 1.0, emissiveMat)

	lights := []Light{light1, light2}
	sampler := NewUniformLightSampler(lights, 10.0)

	point := core.NewVec3(0, 0, 2)
	normal := core.NewVec3(0, 0, 1)
	direction := core.NewVec3(-1, 0, -1).Normalize() // Direction toward light1

	pdf := CalculateLightPDF(lights, sampler, point, normal, direction)

	// Calculate expected PDF: sum of (light PDF * selection probability)
	expectedPDF := 0.0
	for i, light := range lights {
		lightPDF := light.PDF(point, normal, direction)
		selectionProb := sampler.GetLightProbability(i, point, normal)
		expectedPDF += lightPDF * selectionProb
	}

	if math.Abs(pdf-expectedPDF) > tolerance {
		t.Errorf("Combined PDF incorrect: got %f, expected %f", pdf, expectedPDF)
	}
}

func TestCalculateLightPDF_WeightedSampling(t *testing.T) {
	const tolerance = 1e-9

	// Create two sphere lights with different weights
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)

	light1 := NewSphereLight(core.NewVec3(-2, 0, 0), 1.0, emissiveMat)
	light2 := NewSphereLight(core.NewVec3(2, 0, 0), 1.0, emissiveMat)

	lights := []Light{light1, light2}
	weights := []float64{0.3, 0.7} // Light2 should be selected more often
	sampler := NewWeightedLightSampler(lights, weights, 10.0)

	point := core.NewVec3(0, 0, 2)
	normal := core.NewVec3(0, 0, 1)
	direction := core.NewVec3(-1, 0, -1).Normalize()

	pdf := CalculateLightPDF(lights, sampler, point, normal, direction)

	// Calculate expected PDF with weighted probabilities
	expectedPDF := 0.0
	for i, light := range lights {
		lightPDF := light.PDF(point, normal, direction)
		selectionProb := sampler.GetLightProbability(i, point, normal)
		expectedPDF += lightPDF * selectionProb
	}

	if math.Abs(pdf-expectedPDF) > tolerance {
		t.Errorf("Weighted PDF incorrect: got %f, expected %f", pdf, expectedPDF)
	}
}

func TestSampleLight_EmptyLights(t *testing.T) {
	lights := []Light{}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	point := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 0, 1)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample, selectedLight, success := SampleLight(lights, lightSampler, point, normal, sampler)

	if success {
		t.Error("Expected failure for empty lights array")
	}
	if selectedLight != nil {
		t.Error("Expected nil light for empty array")
	}
	if sample.PDF != 0 {
		t.Errorf("Expected zero PDF for failed sampling, got %f", sample.PDF)
	}
}

func TestSampleLight_SingleLight(t *testing.T) {
	// Create a single quad light
	emission := core.NewVec3(2.0, 2.0, 2.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(-1, -1, 0)
	u := core.NewVec3(2, 0, 0)
	v := core.NewVec3(0, 2, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	lights := []Light{light}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	point := core.NewVec3(0, 0, 2) // Above the quad
	normal := core.NewVec3(0, 0, 1)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample, selectedLight, success := SampleLight(lights, lightSampler, point, normal, sampler)

	if !success {
		t.Fatal("Expected successful sampling")
	}
	if selectedLight != light {
		t.Error("Expected selected light to match input light")
	}
	if sample.PDF <= 0 {
		t.Errorf("Expected positive PDF, got %f", sample.PDF)
	}
	if sample.Emission != emission {
		t.Errorf("Expected emission %v, got %v", emission, sample.Emission)
	}

	// For single light, selection probability is 1.0, so combined PDF should equal light's PDF
	expectedBasePDF := light.PDF(point, normal, sample.Direction)
	expectedCombinedPDF := expectedBasePDF * 1.0 // * selection probability
	if math.Abs(sample.PDF-expectedCombinedPDF) > 1e-9 {
		t.Errorf("Combined PDF incorrect: got %f, expected %f", sample.PDF, expectedCombinedPDF)
	}
}

func TestSampleLight_MultipleLights(t *testing.T) {
	// Create multiple lights
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)

	light1 := NewSphereLight(core.NewVec3(-3, 0, 0), 1.0, emissiveMat)
	light2 := NewSphereLight(core.NewVec3(3, 0, 0), 1.0, emissiveMat)
	light3 := NewQuadLight(core.NewVec3(-1, -1, -2), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), emissiveMat)

	lights := []Light{light1, light2, light3}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	point := core.NewVec3(0, 0, 2)
	normal := core.NewVec3(0, 0, 1)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Sample multiple times to verify different lights can be selected
	lightCounts := make(map[Light]int)
	numSamples := 300

	for i := 0; i < numSamples; i++ {
		sample, selectedLight, success := SampleLight(lights, lightSampler, point, normal, sampler)

		if !success {
			t.Fatalf("Sampling failed at iteration %d", i)
		}
		if selectedLight == nil {
			t.Fatalf("Selected light is nil at iteration %d", i)
		}
		if sample.PDF <= 0 {
			t.Errorf("Non-positive PDF at iteration %d: %f", i, sample.PDF)
		}

		lightCounts[selectedLight]++
	}

	// Verify all lights were selected at least once (with high probability)
	if len(lightCounts) < 3 {
		t.Errorf("Not all lights were selected: %d different lights sampled", len(lightCounts))
	}

	// For uniform sampling, expect roughly equal distribution
	expectedCount := numSamples / 3
	tolerance := expectedCount / 2 // Allow 50% variation
	for light, count := range lightCounts {
		if count < expectedCount-tolerance || count > expectedCount+tolerance {
			t.Errorf("Light %v poorly sampled: %d samples (expected ~%d)", light, count, expectedCount)
		}
	}
}

func TestSampleLightEmission_EmptyLights(t *testing.T) {
	lights := []Light{}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample, success := SampleLightEmission(lights, lightSampler, sampler)

	if success {
		t.Error("Expected failure for empty lights array")
	}
	if sample.AreaPDF != 0 {
		t.Errorf("Expected zero area PDF for failed sampling, got %f", sample.AreaPDF)
	}
}

func TestSampleLightEmission_SingleLight(t *testing.T) {
	const tolerance = 1e-9

	// Create a single sphere light
	emission := core.NewVec3(3.0, 3.0, 3.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	lights := []Light{light}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample, success := SampleLightEmission(lights, lightSampler, sampler)

	if !success {
		t.Fatal("Expected successful emission sampling")
	}

	// Verify sample properties
	if sample.AreaPDF <= 0 {
		t.Errorf("Expected positive area PDF, got %f", sample.AreaPDF)
	}
	if sample.DirectionPDF <= 0 {
		t.Errorf("Expected positive direction PDF, got %f", sample.DirectionPDF)
	}
	if sample.Emission != emission {
		t.Errorf("Expected emission %v, got %v", emission, sample.Emission)
	}

	// Verify point is on sphere surface
	distanceToCenter := sample.Point.Subtract(center).Length()
	if math.Abs(distanceToCenter-radius) > tolerance {
		t.Errorf("Sample point not on sphere surface: distance = %f, expected = %f", distanceToCenter, radius)
	}

	// Verify direction is in correct hemisphere
	cosTheta := sample.Direction.Dot(sample.Normal)
	if cosTheta <= 0 {
		t.Errorf("Emission direction not in correct hemisphere: cos(theta) = %f", cosTheta)
	}

	// For single light, area PDF should include selection probability (1.0)
	expectedAreaPDF := 1.0 / (4.0 * math.Pi * radius * radius) * 1.0
	if math.Abs(sample.AreaPDF-expectedAreaPDF) > tolerance {
		t.Errorf("Area PDF incorrect: got %f, expected %f", sample.AreaPDF, expectedAreaPDF)
	}
}

func TestSampleLightEmission_MultipleLights(t *testing.T) {
	// Create multiple lights
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)

	light1 := NewSphereLight(core.NewVec3(-2, 0, 0), 1.0, emissiveMat)
	light2 := NewSphereLight(core.NewVec3(2, 0, 0), 0.5, emissiveMat) // Smaller sphere
	light3 := NewQuadLight(core.NewVec3(-1, -1, 2), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), emissiveMat)

	lights := []Light{light1, light2, light3}
	lightSampler := NewUniformLightSampler(lights, 10.0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Sample multiple times
	numSamples := 300
	validSamples := 0

	for i := 0; i < numSamples; i++ {
		sample, success := SampleLightEmission(lights, lightSampler, sampler)

		if !success {
			t.Fatalf("Emission sampling failed at iteration %d", i)
		}

		if sample.AreaPDF <= 0 {
			t.Errorf("Non-positive area PDF at iteration %d: %f", i, sample.AreaPDF)
		}
		if sample.DirectionPDF <= 0 {
			t.Errorf("Non-positive direction PDF at iteration %d: %f", i, sample.DirectionPDF)
		}

		// Verify direction is normalized
		dirLength := sample.Direction.Length()
		if math.Abs(dirLength-1.0) > 1e-6 {
			t.Errorf("Direction not normalized at iteration %d: length = %f", i, dirLength)
		}

		validSamples++
	}

	if validSamples != numSamples {
		t.Errorf("Some emission samples were invalid: %d/%d successful", validSamples, numSamples)
	}
}

func TestSampleLightEmission_WeightedSampling(t *testing.T) {
	// Create lights with weighted sampling
	emission := core.NewVec3(1.0, 1.0, 1.0)
	emissiveMat := material.NewEmissive(emission)

	light1 := NewSphereLight(core.NewVec3(-2, 0, 0), 1.0, emissiveMat)
	light2 := NewSphereLight(core.NewVec3(2, 0, 0), 1.0, emissiveMat)

	lights := []Light{light1, light2}
	weights := []float64{0.2, 0.8} // Light2 should be selected much more often
	lightSampler := NewWeightedLightSampler(lights, weights, 10.0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	numSamples := 1000
	light1Samples := 0
	light2Samples := 0

	for i := 0; i < numSamples; i++ {
		sample, success := SampleLightEmission(lights, lightSampler, sampler)

		if !success {
			t.Fatalf("Emission sampling failed at iteration %d", i)
		}

		// Determine which light was sampled based on area PDF
		// Light1 area: 4π*1² = 4π, base PDF: 1/(4π)
		// Light2 area: 4π*1² = 4π, base PDF: 1/(4π)
		// With weights 0.2 and 0.8, combined PDFs: 0.2/(4π) and 0.8/(4π)
		basePDF := 1.0 / (4.0 * math.Pi)
		if math.Abs(sample.AreaPDF-0.2*basePDF) < 1e-6 {
			light1Samples++
		} else if math.Abs(sample.AreaPDF-0.8*basePDF) < 1e-6 {
			light2Samples++
		}
	}

	// Verify weighted distribution
	expectedLight1 := int(float64(numSamples) * 0.2)
	expectedLight2 := int(float64(numSamples) * 0.8)
	tolerance := numSamples / 10 // 10% tolerance

	if math.Abs(float64(light1Samples-expectedLight1)) > float64(tolerance) {
		t.Errorf("Light1 poorly sampled: %d samples (expected ~%d)", light1Samples, expectedLight1)
	}
	if math.Abs(float64(light2Samples-expectedLight2)) > float64(tolerance) {
		t.Errorf("Light2 poorly sampled: %d samples (expected ~%d)", light2Samples, expectedLight2)
	}
}

func TestSampleEmissionDirection_BasicProperties(t *testing.T) {
	const tolerance = 1e-9

	point := core.NewVec3(1, 2, 3)
	normal := core.NewVec3(0, 0, 1)
	areaPDF := 0.25 // Some area PDF
	emission := core.NewVec3(2.0, 3.0, 4.0)
	emissiveMat := material.NewEmissive(emission)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := SampleEmissionDirection(point, normal, areaPDF, emissiveMat, sampler.Get2D())

	// Verify basic properties
	if sample.Point != point {
		t.Errorf("Point incorrect: got %v, expected %v", sample.Point, point)
	}
	if sample.Normal != normal {
		t.Errorf("Normal incorrect: got %v, expected %v", sample.Normal, normal)
	}
	if sample.AreaPDF != areaPDF {
		t.Errorf("Area PDF incorrect: got %f, expected %f", sample.AreaPDF, areaPDF)
	}
	if sample.Emission != emission {
		t.Errorf("Emission incorrect: got %v, expected %v", sample.Emission, emission)
	}

	// Verify direction is normalized
	dirLength := sample.Direction.Length()
	if math.Abs(dirLength-1.0) > tolerance {
		t.Errorf("Direction not normalized: length = %f", dirLength)
	}

	// Verify direction is in correct hemisphere
	cosTheta := sample.Direction.Dot(normal)
	if cosTheta <= 0 {
		t.Errorf("Direction not in correct hemisphere: cos(theta) = %f", cosTheta)
	}

	// Verify direction PDF formula: cosTheta/π
	expectedDirPDF := cosTheta / math.Pi
	if math.Abs(sample.DirectionPDF-expectedDirPDF) > tolerance {
		t.Errorf("Direction PDF incorrect: got %f, expected %f", sample.DirectionPDF, expectedDirPDF)
	}
}

func TestSampleEmissionDirection_NonEmissiveMaterial(t *testing.T) {
	point := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	areaPDF := 0.5
	lambertian := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5)) // Non-emissive
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	sample := SampleEmissionDirection(point, normal, areaPDF, lambertian, sampler.Get2D())

	// Should still sample direction correctly
	if sample.DirectionPDF <= 0 {
		t.Errorf("Expected positive direction PDF, got %f", sample.DirectionPDF)
	}

	// But emission should be zero
	expectedEmission := core.Vec3{X: 0, Y: 0, Z: 0}
	if sample.Emission != expectedEmission {
		t.Errorf("Expected zero emission for non-emissive material, got %v", sample.Emission)
	}
}

func TestSampleEmissionDirection_CosineWeighting(t *testing.T) {
	// Test that the cosine-weighted distribution is working correctly
	point := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 0, 1) // Z-up
	areaPDF := 1.0
	emissiveMat := material.NewEmissive(core.NewVec3(1, 1, 1))
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	numSamples := 1000
	totalCosTheta := 0.0

	for i := 0; i < numSamples; i++ {
		sample := SampleEmissionDirection(point, normal, areaPDF, emissiveMat, sampler.Get2D())

		cosTheta := sample.Direction.Dot(normal)
		if cosTheta <= 0 {
			t.Errorf("Sample %d: direction not in correct hemisphere", i)
		}

		totalCosTheta += cosTheta

		// Verify PDF formula matches
		expectedDirPDF := cosTheta / math.Pi
		if math.Abs(sample.DirectionPDF-expectedDirPDF) > 1e-9 {
			t.Errorf("Sample %d: direction PDF incorrect: got %f, expected %f", i, sample.DirectionPDF, expectedDirPDF)
		}
	}

	// For cosine-weighted sampling, average cosTheta should be around 2/π ≈ 0.637
	avgCosTheta := totalCosTheta / float64(numSamples)
	expectedAvgCosTheta := 2.0 / math.Pi
	tolerance := 0.05 // 5% tolerance

	if math.Abs(avgCosTheta-expectedAvgCosTheta) > tolerance {
		t.Errorf("Average cosTheta incorrect: got %f, expected %f", avgCosTheta, expectedAvgCosTheta)
	}
}

func TestSampleEmissionDirection_DifferentNormals(t *testing.T) {
	const tolerance = 1e-9

	// Test with different normal orientations
	normals := []core.Vec3{
		core.NewVec3(1, 0, 0),             // X-axis
		core.NewVec3(0, 1, 0),             // Y-axis
		core.NewVec3(0, 0, 1),             // Z-axis
		core.NewVec3(0, 0, -1),            // Negative Z-axis
		core.NewVec3(1, 1, 1).Normalize(), // Diagonal
	}

	point := core.NewVec3(0, 0, 0)
	areaPDF := 1.0
	emissiveMat := material.NewEmissive(core.NewVec3(1, 1, 1))
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	for i, normal := range normals {
		sample := SampleEmissionDirection(point, normal, areaPDF, emissiveMat, sampler.Get2D())

		// Verify direction is in correct hemisphere relative to normal
		cosTheta := sample.Direction.Dot(normal)
		if cosTheta <= 0 {
			t.Errorf("Normal %d (%v): direction not in correct hemisphere, cos(theta) = %f", i, normal, cosTheta)
		}

		// Verify direction PDF is correct
		expectedDirPDF := cosTheta / math.Pi
		if math.Abs(sample.DirectionPDF-expectedDirPDF) > tolerance {
			t.Errorf("Normal %d: direction PDF incorrect: got %f, expected %f", i, sample.DirectionPDF, expectedDirPDF)
		}

		// Verify direction is normalized
		dirLength := sample.Direction.Length()
		if math.Abs(dirLength-1.0) > tolerance {
			t.Errorf("Normal %d: direction not normalized: length = %f", i, dirLength)
		}
	}
}
