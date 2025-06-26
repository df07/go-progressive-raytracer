package geometry

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestDiscHit(t *testing.T) {
	// Create a disc at origin facing up with radius 1
	center := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	radius := 1.0
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	disc := NewDisc(center, normal, radius, mat)

	tests := []struct {
		name      string
		ray       core.Ray
		tMin      float64
		tMax      float64
		shouldHit bool
		expectedT float64
	}{
		{
			name:      "Ray hits center of disc",
			ray:       core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(0, -1, 0)),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
		{
			name:      "Ray hits edge of disc",
			ray:       core.NewRay(core.NewVec3(1, 1, 0), core.NewVec3(0, -1, 0)),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
		{
			name:      "Ray misses disc (outside radius)",
			ray:       core.NewRay(core.NewVec3(1.1, 1, 0), core.NewVec3(0, -1, 0)),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: false,
		},
		{
			name:      "Ray parallel to disc plane",
			ray:       core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(1, 0, 0)),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: false,
		},
		{
			name:      "Ray hits from below",
			ray:       core.NewRay(core.NewVec3(0, -1, 0), core.NewVec3(0, 1, 0)),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, didHit := disc.Hit(tt.ray, tt.tMin, tt.tMax)

			if didHit != tt.shouldHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.shouldHit, didHit)
			}

			if tt.shouldHit && hit != nil {
				if math.Abs(hit.T-tt.expectedT) > 1e-6 {
					t.Errorf("Expected t=%v, got t=%v", tt.expectedT, hit.T)
				}

				// Check that hit point is within radius
				distance := hit.Point.Subtract(center).Length()
				if distance > radius+1e-6 {
					t.Errorf("Hit point is outside disc radius: distance=%v, radius=%v", distance, radius)
				}
			}
		})
	}
}

func TestDiscBoundingBox(t *testing.T) {
	// Test with disc facing up
	center := core.NewVec3(1, 2, 3)
	normal := core.NewVec3(0, 1, 0)
	radius := 2.0
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	disc := NewDisc(center, normal, radius, mat)

	bbox := disc.BoundingBox()

	// For a disc facing up, the bounding box should extend radius in X and Z directions
	expectedMin := core.NewVec3(center.X-radius, center.Y, center.Z-radius)
	expectedMax := core.NewVec3(center.X+radius, center.Y, center.Z+radius)

	tolerance := 1e-6
	if math.Abs(bbox.Min.X-expectedMin.X) > tolerance ||
		math.Abs(bbox.Min.Y-expectedMin.Y) > tolerance ||
		math.Abs(bbox.Min.Z-expectedMin.Z) > tolerance {
		t.Errorf("Expected min %v, got %v", expectedMin, bbox.Min)
	}

	if math.Abs(bbox.Max.X-expectedMax.X) > tolerance ||
		math.Abs(bbox.Max.Y-expectedMax.Y) > tolerance ||
		math.Abs(bbox.Max.Z-expectedMax.Z) > tolerance {
		t.Errorf("Expected max %v, got %v", expectedMax, bbox.Max)
	}
}

func TestDiscSampleUniform(t *testing.T) {
	center := core.NewVec3(0, 0, 0)
	normal := core.NewVec3(0, 1, 0)
	radius := 1.0
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	disc := NewDisc(center, normal, radius, mat)

	random := rand.New(rand.NewSource(42))
	numSamples := 1000

	for i := 0; i < numSamples; i++ {
		point, sampleNormal := disc.SampleUniform(random)

		// Check that sampled point is within disc radius
		distance := point.Subtract(center).Length()
		if distance > radius+1e-6 {
			t.Errorf("Sampled point outside disc: distance=%v, radius=%v", distance, radius)
		}

		// Check that normal is correct
		if !sampleNormal.Equals(normal) {
			t.Errorf("Expected normal %v, got %v", normal, sampleNormal)
		}

		// Check that point lies on the disc plane
		pointOnPlane := point.Subtract(center)
		dotProduct := pointOnPlane.Dot(normal)
		if math.Abs(dotProduct) > 1e-6 {
			t.Errorf("Sampled point not on disc plane: dot product=%v", dotProduct)
		}
	}
}

func TestDiscOrthogonalVectors(t *testing.T) {
	tests := []struct {
		name   string
		normal core.Vec3
	}{
		{"Normal along Y", core.NewVec3(0, 1, 0)},
		{"Normal along X", core.NewVec3(1, 0, 0)},
		{"Normal along Z", core.NewVec3(0, 0, 1)},
		{"Diagonal normal", core.NewVec3(1, 1, 1).Normalize()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			center := core.NewVec3(0, 0, 0)
			radius := 1.0
			mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
			disc := NewDisc(center, tt.normal, radius, mat)

			// Check orthogonality
			rightDotNormal := disc.Right.Dot(disc.Normal)
			upDotNormal := disc.Up.Dot(disc.Normal)
			rightDotUp := disc.Right.Dot(disc.Up)

			tolerance := 1e-6
			if math.Abs(rightDotNormal) > tolerance {
				t.Errorf("Right vector not orthogonal to normal: dot=%v", rightDotNormal)
			}
			if math.Abs(upDotNormal) > tolerance {
				t.Errorf("Up vector not orthogonal to normal: dot=%v", upDotNormal)
			}
			if math.Abs(rightDotUp) > tolerance {
				t.Errorf("Right and Up vectors not orthogonal: dot=%v", rightDotUp)
			}

			// Check normalization
			if math.Abs(disc.Right.Length()-1.0) > tolerance {
				t.Errorf("Right vector not normalized: length=%v", disc.Right.Length())
			}
			if math.Abs(disc.Up.Length()-1.0) > tolerance {
				t.Errorf("Up vector not normalized: length=%v", disc.Up.Length())
			}
		})
	}
}
