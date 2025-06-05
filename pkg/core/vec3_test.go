package core

import (
	"math"
	"math/rand"
	"testing"
)

func TestRandomCosineDirection(t *testing.T) {
	random := rand.New(rand.NewSource(42))
	normal := NewVec3(0, 0, 1) // Z-up normal

	// Test statistical properties over many samples
	const numSamples = 10000
	var totalCosine float64
	belowHemisphere := 0

	for i := 0; i < numSamples; i++ {
		dir := RandomCosineDirection(normal, random)

		// All directions should be unit vectors
		length := dir.Length()
		if math.Abs(length-1.0) > 1e-3 { // More realistic tolerance for accumulated floating point ops
			t.Errorf("Generated direction not unit length: %f", length)
		}

		// All directions should be in upper hemisphere
		cosTheta := dir.Dot(normal)
		if cosTheta < 0 {
			belowHemisphere++
		}

		totalCosine += math.Max(0, cosTheta) // Clamp negative values for averaging
	}

	// Should have no rays below hemisphere
	if belowHemisphere > 0 {
		t.Errorf("Found %d rays below hemisphere out of %d", belowHemisphere, numSamples)
	}

	// For cosine-weighted sampling, average cosine should be around 2/π ≈ 0.637
	avgCosine := totalCosine / float64(numSamples)
	expectedAvgCosine := 2.0 / math.Pi
	tolerance := 0.05 // Allow 5% variance due to sampling method and randomness
	if math.Abs(avgCosine-expectedAvgCosine) > tolerance {
		t.Errorf("Average cosine %f doesn't match expected %f (±%f)",
			avgCosine, expectedAvgCosine, tolerance)
	}
}

func TestRandomCosineDirection_OrthonormalBasis(t *testing.T) {
	random := rand.New(rand.NewSource(42))

	// Test with different normals to verify basis creation
	testNormals := []Vec3{
		NewVec3(0, 0, 1),             // Z-up
		NewVec3(0, 1, 0),             // Y-up
		NewVec3(1, 0, 0),             // X-up
		NewVec3(0.577, 0.577, 0.577), // Diagonal
	}

	for _, normal := range testNormals {
		// Generate multiple samples to test basis consistency
		for i := 0; i < 100; i++ {
			dir := RandomCosineDirection(normal, random)

			// Direction should be unit length
			if math.Abs(dir.Length()-1.0) > 1e-3 {
				t.Errorf("Non-unit direction for normal %v: length=%f", normal, dir.Length())
			}

			// Should be in upper hemisphere relative to normal
			cosTheta := dir.Dot(normal)
			if cosTheta < -1e-10 { // Small tolerance for floating point errors
				t.Errorf("Direction below hemisphere for normal %v: cosθ=%f", normal, cosTheta)
			}
		}
	}
}
