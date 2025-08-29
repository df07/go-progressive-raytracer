package geometry

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TestSampleCameraFromPoint tests the camera sampling for t=1 strategies
func TestSampleCameraFromPoint(t *testing.T) {
	// Create a simple perspective camera
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.1, // Small aperture for testing
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Test sampling from a point in front of the camera
	refPoint := core.NewVec3(0.5, 0.3, -2.0)

	sample := camera.SampleCameraFromPoint(refPoint, sampler.Get2D())

	// Should return a valid sample
	if sample == nil {
		t.Fatal("SampleCameraFromPoint returned nil for valid reference point")
	}

	// Ray should point from camera toward reference point
	expectedDirection := refPoint.Subtract(sample.Ray.Origin).Normalize()
	actualDirection := sample.Ray.Direction.Normalize()

	// Check if directions are approximately equal (allowing for lens sampling variation)
	dotProduct := expectedDirection.Dot(actualDirection)
	if dotProduct < 0.8 { // Allow some variation due to lens sampling
		t.Errorf("Ray direction mismatch. Expected ~%v, got %v (dot product: %f)",
			expectedDirection, actualDirection, dotProduct)
	}

	// PDF should be positive
	if sample.PDF <= 0 {
		t.Errorf("Expected positive PDF, got %f", sample.PDF)
	}

	// Weight should be positive
	if sample.Weight.Luminance() <= 0 {
		t.Errorf("Expected positive weight, got %v", sample.Weight)
	}
}

// TestSampleCameraFromPointBehindCamera tests sampling from behind the camera
func TestSampleCameraFromPointBehindCamera(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.1,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Test sampling from a point behind the camera
	refPointBehind := core.NewVec3(0, 0, 1.0) // Behind camera

	sample := camera.SampleCameraFromPoint(refPointBehind, sampler.Get2D())

	// Should return nil for points behind camera
	if sample != nil {
		t.Error("Expected nil sample for reference point behind camera")
	}
}

// TestMapRayToPixel tests the ray-to-pixel mapping
func TestMapRayToPixel(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.0, // Pinhole for deterministic testing
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Test center ray maps to center pixel
	centerRay := camera.GetRay(400, 225, sampler.Get2D(), sampler.Get2D()) // Center of 800x450 image

	x, y, ok := camera.MapRayToPixel(centerRay)
	if !ok {
		t.Fatal("Failed to map center ray to pixel")
	}

	// Should map approximately to center (allowing for some variation due to jitter)
	expectedX, expectedY := 400, 225
	tolerance := 2 // Allow small variation due to anti-aliasing jitter

	if abs(x-expectedX) > tolerance || abs(y-expectedY) > tolerance {
		t.Errorf("Center ray mapping error. Expected (~%d, ~%d), got (%d, %d)",
			expectedX, expectedY, x, y)
	}

	// Test corner rays
	testCases := []struct {
		name      string
		pixelX    int
		pixelY    int
		tolerance int
	}{
		{"top-left", 0, 0, 5},
		{"top-right", 799, 0, 5},
		{"bottom-left", 0, 449, 5},
		{"bottom-right", 799, 449, 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ray := camera.GetRay(tc.pixelX, tc.pixelY, sampler.Get2D(), sampler.Get2D())
			mappedX, mappedY, ok := camera.MapRayToPixel(ray)

			if !ok {
				t.Fatalf("Failed to map %s ray to pixel", tc.name)
			}

			if abs(mappedX-tc.pixelX) > tc.tolerance || abs(mappedY-tc.pixelY) > tc.tolerance {
				t.Errorf("%s ray mapping error. Expected (~%d, ~%d), got (%d, %d)",
					tc.name, tc.pixelX, tc.pixelY, mappedX, mappedY)
			}
		})
	}
}

// TestMapRayToPixelOutOfBounds tests rays that don't hit the image plane
func TestMapRayToPixelOutOfBounds(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.0,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)

	// Test ray pointing away from camera
	rayAway := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1))
	x, y, ok := camera.MapRayToPixel(rayAway)

	if ok {
		t.Errorf("Expected failure for ray pointing away from camera, got (%d, %d)", x, y)
	}

	// Test ray pointing far off to the side (outside field of view)
	rayOffSide := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(10, 10, -1))
	x, y, ok = camera.MapRayToPixel(rayOffSide)

	if ok {
		t.Errorf("Expected failure for ray outside field of view, got (%d, %d)", x, y)
	}
}

// TestWeFunction tests the camera importance function
func TestWeFunction(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.1,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)

	// Test center ray (should have high importance)
	centerRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	centerWe := camera.EvaluateRayImportance(centerRay)

	if centerWe.Luminance() <= 0 {
		t.Error("Center ray should have positive importance")
	}

	// Test ray at edge of field of view (should have lower importance due to cos^4 term)
	edgeDirection := core.NewVec3(0.7, 0, -0.7).Normalize() // 45 degrees off center
	edgeRay := core.NewRay(core.NewVec3(0, 0, 0), edgeDirection)
	edgeWe := camera.EvaluateRayImportance(edgeRay)

	// Calculate cosTheta for debugging
	cameraForward := camera.GetCameraForward()
	centerCosTheta := centerRay.Direction.Dot(cameraForward)
	edgeCosTheta := edgeDirection.Dot(cameraForward)

	t.Logf("Camera forward: %v", cameraForward)
	t.Logf("Edge ray direction: %v", edgeDirection)
	t.Logf("Edge ray cosTheta: %f", edgeCosTheta)
	t.Logf("Edge ray importance: %v", edgeWe)
	t.Logf("Center ray direction: %v", centerRay.Direction)
	t.Logf("Center ray cosTheta: %f", centerCosTheta)
	t.Logf("Center ray importance: %v", centerWe)

	if edgeWe.Luminance() <= 0 {
		t.Error("Edge ray within FOV should have positive importance")
	}

	// Debug the cos^4 calculation
	centerCos4 := centerCosTheta * centerCosTheta * centerCosTheta * centerCosTheta
	edgeCos4 := edgeCosTheta * edgeCosTheta * edgeCosTheta * edgeCosTheta
	t.Logf("Center cos^4: %f", centerCos4)
	t.Logf("Edge cos^4: %f", edgeCos4)

	// PBRT importance formula: We = 1 / (A * lensArea * cos^4(theta))
	// So edge rays (smaller cos^4) have HIGHER importance than center rays
	expectedRatio := centerCos4 / edgeCos4 // Expected edge/center ratio = 1.0/0.25 = 4.0
	actualRatio := edgeWe.Luminance() / centerWe.Luminance()
	t.Logf("Expected edge/center importance ratio: %f", expectedRatio)
	t.Logf("Actual edge/center importance ratio: %f", actualRatio)

	// Edge rays should have higher importance than center due to cos^4 in denominator
	if edgeWe.Luminance() <= centerWe.Luminance() {
		t.Error("Edge ray should have higher importance than center ray (PBRT formula)")
	}

	// Check the ratio is approximately correct (allowing some tolerance)
	tolerance := 0.01
	if math.Abs(actualRatio-expectedRatio) > tolerance {
		t.Errorf("Importance ratio mismatch. Expected %.3f, got %.3f", expectedRatio, actualRatio)
	}

	// Test ray outside field of view (should have zero importance)
	outsideDirection := core.NewVec3(5, 0, -1).Normalize() // Way off to the side (more extreme)
	outsideRay := core.NewRay(core.NewVec3(0, 0, 0), outsideDirection)
	outsideWe := camera.EvaluateRayImportance(outsideRay)

	if outsideWe.Luminance() > 0 {
		t.Error("Ray outside field of view should have zero importance")
	}

	// Test ray pointing away from camera
	backwardRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1))
	backwardWe := camera.EvaluateRayImportance(backwardRay)

	if backwardWe.Luminance() > 0 {
		t.Error("Backward ray should have zero importance")
	}
}

// TestCameraSamplingConsistency tests that camera sampling and ray generation are consistent
func TestCameraSamplingConsistency(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          90,
		Aperture:      0.0, // Pinhole for deterministic testing
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Generate a reference point on the image plane
	refPoint := core.NewVec3(0.2, 0.1, -1.0)

	// Sample camera from this point
	sample := camera.SampleCameraFromPoint(refPoint, sampler.Get2D())
	if sample == nil {
		t.Fatal("Camera sampling failed")
	}

	// Map the resulting ray back to pixel coordinates
	x, y, ok := camera.MapRayToPixel(sample.Ray)
	if !ok {
		t.Fatal("Failed to map sampled ray back to pixels")
	}

	// Generate a ray for those pixel coordinates
	pixelRay := camera.GetRay(x, y, sampler.Get2D(), sampler.Get2D())

	// The pixel ray should point in approximately the same direction as our reference point
	pixelDirection := pixelRay.Direction.Normalize()
	refDirection := refPoint.Subtract(camera.config.Center).Normalize()

	dotProduct := pixelDirection.Dot(refDirection)
	if dotProduct < 0.95 { // Allow some tolerance
		t.Errorf("Inconsistent ray directions. Pixel ray: %v, Reference direction: %v (dot: %f)",
			pixelDirection, refDirection, dotProduct)
	}
}

// Helper function for absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
