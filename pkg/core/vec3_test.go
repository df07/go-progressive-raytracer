package core

import (
	"math"
	"math/rand"
	"testing"
)

func TestRandomCosineDirection(t *testing.T) {
	sampler := NewRandomSampler(rand.New(rand.NewSource(42)))
	normal := NewVec3(0, 0, 1) // Z-up normal

	// Test statistical properties over many samples
	const numSamples = 10000
	var totalCosine float64
	belowHemisphere := 0

	for i := 0; i < numSamples; i++ {
		dir := RandomCosineDirection(normal, sampler.Get2D())

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
	sampler := NewRandomSampler(rand.New(rand.NewSource(42)))

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
			dir := RandomCosineDirection(normal, sampler.Get2D())

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

func TestVec3_Rotate(t *testing.T) {
	tests := []struct {
		name     string
		vector   Vec3
		rotation Vec3
		expected Vec3
	}{
		{
			name:     "No rotation",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, 0, 0),
			expected: NewVec3(1, 0, 0),
		},
		{
			name:     "90 degree rotation around Z axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, 0, math.Pi/2),
			expected: NewVec3(0, 1, 0),
		},
		{
			name:     "90 degree rotation around Y axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi/2, 0),
			expected: NewVec3(0, 0, -1),
		},
		{
			name:     "90 degree rotation around X axis",
			vector:   NewVec3(0, 1, 0),
			rotation: NewVec3(math.Pi/2, 0, 0),
			expected: NewVec3(0, 0, 1),
		},
		{
			name:     "180 degree rotation around Y axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi, 0),
			expected: NewVec3(-1, 0, 0),
		},
		{
			name:     "Combined rotations",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi/2, math.Pi/2), // 90° Y then 90° Z
			expected: NewVec3(0, 0, -1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.vector.Rotate(tt.rotation)

			const tolerance = 1e-9
			if result.Subtract(tt.expected).Length() > tolerance {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
