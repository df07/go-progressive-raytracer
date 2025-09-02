package lights

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestDiscSpotLightFalloff(t *testing.T) {
	from := core.NewVec3(0, 5, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(10, 10, 10)
	coneAngle := 30.0 // degrees
	deltaAngle := 5.0 // degrees
	radius := 0.1

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	tests := []struct {
		name           string
		cosAngle       float64
		expectedResult float64
	}{
		{
			name:           "Inside inner cone (full intensity)",
			cosAngle:       math.Cos(20 * math.Pi / 180), // 20 degrees < (30-5) = 25 degrees
			expectedResult: 1.0,
		},
		{
			name:           "At falloff start edge",
			cosAngle:       math.Cos(25 * math.Pi / 180), // Exactly at falloff start
			expectedResult: 1.0,
		},
		{
			name:           "In falloff region",
			cosAngle:       math.Cos(27.5 * math.Pi / 180), // Halfway between 25 and 30
			expectedResult: -1,                             // Will calculate dynamically
		},
		{
			name:           "At total width edge",
			cosAngle:       math.Cos(30 * math.Pi / 180), // Exactly at cone edge
			expectedResult: 0.0,
		},
		{
			name:           "Outside cone",
			cosAngle:       math.Cos(35 * math.Pi / 180), // Outside cone
			expectedResult: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := spotLight.falloff(tt.cosAngle)

			if tt.expectedResult == -1 {
				// Dynamic calculation for falloff region
				cosTotalWidth := math.Cos(30 * math.Pi / 180)
				cosFalloffStart := math.Cos(25 * math.Pi / 180)
				delta := (tt.cosAngle - cosTotalWidth) / (cosFalloffStart - cosTotalWidth)
				expected := delta * delta * delta * delta

				tolerance := 1e-6
				if math.Abs(result-expected) > tolerance {
					t.Errorf("Expected falloff=%v, got %v", expected, result)
				}
			} else {
				tolerance := 1e-6
				if math.Abs(result-tt.expectedResult) > tolerance {
					t.Errorf("Expected falloff=%v, got %v", tt.expectedResult, result)
				}
			}
		})
	}
}

func TestDiscSpotLightSample(t *testing.T) {
	from := core.NewVec3(0, 2, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(5, 5, 5)
	coneAngle := 45.0
	deltaAngle := 10.0
	radius := 0.2

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	tests := []struct {
		name           string
		testPoint      core.Vec3
		expectEmission bool
	}{
		{
			name:           "Point directly below (center of cone)",
			testPoint:      core.NewVec3(0, 0, 0),
			expectEmission: true,
		},
		{
			name:           "Point at edge of inner cone",
			testPoint:      core.NewVec3(0.7, 0, 0), // ~35 degrees from vertical
			expectEmission: true,
		},
		{
			name:           "Point in falloff region",
			testPoint:      core.NewVec3(1.0, 0, 0), // ~45 degrees from vertical
			expectEmission: true,
		},
		{
			name:           "Point outside cone",
			testPoint:      core.NewVec3(3.0, 0, 0), // ~56 degrees from vertical, clearly outside 45 degree cone
			expectEmission: false,
		},
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := spotLight.Sample(tt.testPoint, core.NewVec3(0, 1, 0), sampler.Get2D())

			// Check that sample point is on the disc
			distanceFromCenter := sample.Point.Subtract(from).Length()
			if distanceFromCenter > radius+1e-6 {
				t.Errorf("Sample point outside disc: distance=%v, radius=%v", distanceFromCenter, radius)
			}

			// Check emission based on spot light cone
			if tt.expectEmission {
				if sample.Emission.Equals(core.NewVec3(0, 0, 0)) {
					t.Errorf("Expected non-zero emission for point %v, got %v", tt.testPoint, sample.Emission)
				}
			} else {
				if !sample.Emission.Equals(core.NewVec3(0, 0, 0)) {
					t.Errorf("Expected zero emission for point %v, got %v", tt.testPoint, sample.Emission)
				}
			}

			// Check that PDF is positive when there's emission
			if tt.expectEmission && sample.PDF <= 0 {
				t.Errorf("Expected positive PDF for illuminated point, got %v", sample.PDF)
			}
		})
	}
}

func TestDiscSpotLightGetIntensityAt(t *testing.T) {
	from := core.NewVec3(0, 1, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(1, 1, 1)
	coneAngle := 60.0
	deltaAngle := 15.0
	radius := 0.1

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	tests := []struct {
		name       string
		testPoint  core.Vec3
		expectZero bool
	}{
		{
			name:       "Point directly below",
			testPoint:  core.NewVec3(0, 0, 0),
			expectZero: false,
		},
		{
			name:       "Point in inner cone",
			testPoint:  core.NewVec3(0.5, 0, 0),
			expectZero: false,
		},
		{
			name:       "Point outside cone",
			testPoint:  core.NewVec3(2, 0, 0),
			expectZero: true,
		},
		{
			name:       "Point at light position",
			testPoint:  from,
			expectZero: true, // Zero distance case
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intensity := spotLight.GetIntensityAt(tt.testPoint)

			if tt.expectZero {
				if !intensity.Equals(core.NewVec3(0, 0, 0)) {
					t.Errorf("Expected zero intensity, got %v", intensity)
				}
			} else {
				if intensity.Equals(core.NewVec3(0, 0, 0)) {
					t.Errorf("Expected non-zero intensity, got %v", intensity)
				}

				// Intensity should decrease with distance (inverse square law)
				distance := tt.testPoint.Subtract(from).Length()
				if distance > 0 {
					expectedMagnitude := 1.0 / (distance * distance) // Base expectation
					actualMagnitude := intensity.Length()

					// Should be reasonable magnitude (allowing for spot light falloff)
					if actualMagnitude > expectedMagnitude*2 {
						t.Errorf("Intensity magnitude too high: expected ~%v, got %v", expectedMagnitude, actualMagnitude)
					}
				}
			}
		})
	}
}

func TestDiscSpotLightShapeInterface(t *testing.T) {
	// Test that DiscSpotLight properly implements Shape interface
	from := core.NewVec3(0, 1, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(1, 1, 1)
	coneAngle := 30.0
	deltaAngle := 5.0
	radius := 0.5

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	// Test Hit method
	ray := core.NewRay(core.NewVec3(0, 2, 0), core.NewVec3(0, -1, 0))
	hit := &material.HitRecord{}
	didHit := spotLight.Hit(ray, 0.001, 10.0, hit)

	if !didHit {
		t.Errorf("Expected ray to hit spot light disc")
	}

	if hit != nil {
		expectedT := 1.0
		if math.Abs(hit.T-expectedT) > 1e-6 {
			t.Errorf("Expected t=%v, got t=%v", expectedT, hit.T)
		}
	}

	// Test BoundingBox method
	bbox := spotLight.BoundingBox()

	// Bounding box should contain the light position
	if from.X < bbox.Min.X || from.X > bbox.Max.X ||
		from.Y < bbox.Min.Y || from.Y > bbox.Max.Y ||
		from.Z < bbox.Min.Z || from.Z > bbox.Max.Z {
		t.Errorf("Bounding box should contain light position %v", from)
	}

	// Bounding box should have reasonable size based on radius
	size := bbox.Max.Subtract(bbox.Min)
	expectedSize := 2 * radius
	if size.X < expectedSize-1e-6 || size.Z < expectedSize-1e-6 {
		t.Errorf("Bounding box too small: expected size ~%v, got %v", expectedSize, size)
	}
}

func TestDiscSpotLightConsistentFalloff(t *testing.T) {
	// This test catches the bug where falloff calculation uses center position
	// instead of actual sampled point, causing inconsistent results
	from := core.NewVec3(0, 3, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(10, 10, 10)
	coneAngle := 30.0 // 30 degree cone
	deltaAngle := 5.0
	radius := 1.5 // Large radius to amplify the bug

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	// Test point that's right at the edge of the cone from center
	// From center (0,3,0) to (1.5,0,0): angle = atan(1.5/3) ≈ 26.6 degrees (inside cone)
	// From edge point (1.5,3,0) to (1.5,0,0): angle = 0 degrees (definitely inside)
	// From opposite edge (-1.5,3,0) to (1.5,0,0): angle = atan(3/3) = 45 degrees (outside cone)
	testPoint := core.NewVec3(1.5, 0, 0)
	normal := core.NewVec3(0, 1, 0) // unused

	// Force samples at specific disc positions to test consistency
	samples := []core.Vec2{
		{X: 0.0, Y: 0.0},  // Center of disc
		{X: 1.0, Y: 0.0},  // Edge of disc closest to test point
		{X: -1.0, Y: 0.0}, // Edge of disc farthest from test point
	}

	var emissions []core.Vec3
	for _, samplePos := range samples {
		lightSample := spotLight.Sample(testPoint, normal, samplePos)
		emissions = append(emissions, lightSample.Emission)
	}

	// With the bug, these emissions will be identical (all based on center calculation)
	// With the fix, they should vary based on actual sampled positions

	centerEmission := emissions[0]
	closeEdgeEmission := emissions[1]
	farEdgeEmission := emissions[2]

	// The bug manifests as all emissions being identical (using center calculation)
	allIdentical := centerEmission.Equals(closeEdgeEmission) &&
		centerEmission.Equals(farEdgeEmission)

	if allIdentical && !centerEmission.Equals(core.NewVec3(0, 0, 0)) {
		t.Errorf("Bug detected: All emissions identical despite different sample positions. "+
			"This indicates falloff calculated from center rather than sampled point. "+
			"Emissions: center=%v, close=%v, far=%v",
			centerEmission, closeEdgeEmission, farEdgeEmission)
	}

	// Additional check: far edge should have less emission than close edge
	if !farEdgeEmission.Equals(core.NewVec3(0, 0, 0)) &&
		!closeEdgeEmission.Equals(core.NewVec3(0, 0, 0)) {
		farIntensity := farEdgeEmission.Length()
		closeIntensity := closeEdgeEmission.Length()

		if farIntensity >= closeIntensity {
			t.Errorf("Expected far edge emission (%v) to be less than close edge emission (%v)",
				farIntensity, closeIntensity)
		}
	}
}

func TestDiscSpotLightCreation(t *testing.T) {
	from := core.NewVec3(1, 2, 3)
	to := core.NewVec3(4, 5, 6)
	emission := core.NewVec3(2, 3, 4)
	coneAngle := 25.0
	deltaAngle := 8.0
	radius := 0.3

	spotLight := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	// Check position
	if !spotLight.position.Equals(from) {
		t.Errorf("Expected position %v, got %v", from, spotLight.position)
	}

	// Check direction
	expectedDirection := to.Subtract(from).Normalize()
	if !spotLight.direction.Equals(expectedDirection) {
		t.Errorf("Expected direction %v, got %v", expectedDirection, spotLight.direction)
	}

	// Check emission
	if !spotLight.emission.Equals(emission) {
		t.Errorf("Expected emission %v, got %v", emission, spotLight.emission)
	}

	// Check angle calculations
	expectedCosTotalWidth := math.Cos(coneAngle * math.Pi / 180.0)
	expectedCosFalloffStart := math.Cos((coneAngle - deltaAngle) * math.Pi / 180.0)

	if math.Abs(spotLight.cosTotalWidth-expectedCosTotalWidth) > 1e-6 {
		t.Errorf("Expected cosTotalWidth %v, got %v", expectedCosTotalWidth, spotLight.cosTotalWidth)
	}

	if math.Abs(spotLight.cosFalloffStart-expectedCosFalloffStart) > 1e-6 {
		t.Errorf("Expected cosFalloffStart %v, got %v", expectedCosFalloffStart, spotLight.cosFalloffStart)
	}

	// Check that underlying disc is created properly
	disc := spotLight.GetDisc()
	if disc == nil {
		t.Errorf("Expected non-nil disc")
	}

	if disc.Radius != radius {
		t.Errorf("Expected disc radius %v, got %v", radius, disc.Radius)
	}
}
