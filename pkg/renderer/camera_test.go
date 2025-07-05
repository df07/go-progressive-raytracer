package renderer

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestCameraGetCameraForward(t *testing.T) {
	config := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(config)

	forward := camera.GetCameraForward()
	expected := core.NewVec3(0, 0, -1)

	if math.Abs(forward.X-expected.X) > 1e-6 ||
		math.Abs(forward.Y-expected.Y) > 1e-6 ||
		math.Abs(forward.Z-expected.Z) > 1e-6 {
		t.Errorf("Expected forward direction %v, got %v", expected, forward)
	}
}

func TestCameraCalculateRayPDFs_AreaPDF(t *testing.T) {
	config := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(config)

	// Generate a ray pointing straight forward
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	areaPDF, directionPDF := camera.CalculateRayPDFs(ray)

	// Area PDF should be 1 / total_sensor_area
	// For a 400x400 image, we need to calculate the actual sensor area in world units
	imageHeight := int(float64(config.Width) / config.AspectRatio)
	expectedPixels := float64(config.Width * imageHeight)

	if areaPDF <= 0 {
		t.Errorf("Area PDF should be positive, got %f", areaPDF)
	}

	if directionPDF <= 0 {
		t.Errorf("Direction PDF should be positive, got %f", directionPDF)
	}

	// Area PDF should be proportional to 1 / pixel_count
	// The exact value depends on the world-space size of pixels
	t.Logf("Area PDF: %f, Direction PDF: %f, Expected pixels: %f", areaPDF, directionPDF, expectedPixels)
}

func TestCameraCalculateRayPDFs_DirectionPDF(t *testing.T) {
	config := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(config)

	// Test ray pointing straight forward (cosTheta = 1)
	straightRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	_, straightDirectionPDF := camera.CalculateRayPDFs(straightRay)

	// Test ray pointing at an angle (cosTheta < 1)
	angleRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0.5, 0, -1).Normalize())
	_, angleDirectionPDF := camera.CalculateRayPDFs(angleRay)

	// Direction PDF should be higher for rays closer to the optical axis (cos^3 term)
	if straightDirectionPDF <= angleDirectionPDF {
		t.Errorf("Straight ray should have higher direction PDF than angled ray. Straight: %f, Angled: %f",
			straightDirectionPDF, angleDirectionPDF)
	}

	t.Logf("Straight ray direction PDF: %f", straightDirectionPDF)
	t.Logf("Angled ray direction PDF: %f", angleDirectionPDF)
}

func TestCameraCalculateRayPDFs_InvalidRay(t *testing.T) {
	config := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(config)

	// Test ray pointing backwards (cosTheta <= 0)
	backwardRay := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1))
	areaPDF, directionPDF := camera.CalculateRayPDFs(backwardRay)

	if areaPDF != 0 || directionPDF != 0 {
		t.Errorf("Backward ray should have zero PDFs, got area: %f, direction: %f", areaPDF, directionPDF)
	}
}

func TestCameraCalculateRayPDFs_CornellBoxRealistic(t *testing.T) {
	// Test with realistic Cornell box camera settings
	config := CameraConfig{
		Center:        core.NewVec3(278, 278, -800),
		LookAt:        core.NewVec3(278, 278, 0),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   1.0,
		VFov:          40.0,
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := NewCamera(config)

	// Generate a typical ray toward the Cornell box
	ray := core.NewRay(
		core.NewVec3(278, 278, -800),
		core.NewVec3(0, 0, 1), // Forward into the box
	)

	areaPDF, directionPDF := camera.CalculateRayPDFs(ray)

	// Both PDFs should be positive and reasonable
	if areaPDF <= 0 {
		t.Errorf("Cornell box camera area PDF should be positive, got %f", areaPDF)
	}
	if directionPDF <= 0 {
		t.Errorf("Cornell box camera direction PDF should be positive, got %f", directionPDF)
	}

	// Area PDF should be in a reasonable range (not too big or too small)
	if areaPDF > 1.0 {
		t.Errorf("Area PDF seems too large: %f", areaPDF)
	}
	if areaPDF < 1e-10 {
		t.Errorf("Area PDF seems too small: %f", areaPDF)
	}

	t.Logf("Cornell box camera - Area PDF: %e, Direction PDF: %e", areaPDF, directionPDF)
	t.Logf("Combined PDF: %e", areaPDF*directionPDF)
}

func TestCameraCalculateRayPDFs_ConsistencyWithGeneration(t *testing.T) {
	// Test that PDF calculations are consistent with ray generation
	config := CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       100, // Smaller for faster test
		AspectRatio: 1.0,
		VFov:        45.0,
	}
	camera := NewCamera(config)

	random := rand.New(rand.NewSource(42))

	// Generate several rays and check their PDFs
	numSamples := 10
	for i := 0; i < numSamples; i++ {
		// Generate a ray from a random pixel
		pixelI := random.Intn(config.Width)
		pixelJ := random.Intn(config.Width) // Square image

		ray := camera.GetRay(pixelI, pixelJ, random)
		areaPDF, directionPDF := camera.CalculateRayPDFs(ray)

		// Check that PDFs are positive for forward-facing rays
		forward := camera.GetCameraForward()
		cosTheta := ray.Direction.Normalize().Dot(forward)

		if cosTheta > 0 {
			if areaPDF <= 0 {
				t.Errorf("Generated ray should have positive area PDF, got %f", areaPDF)
			}
			if directionPDF <= 0 {
				t.Errorf("Generated ray should have positive direction PDF, got %f", directionPDF)
			}
		}

		t.Logf("Pixel (%d,%d): cosTheta=%f, areaPDF=%e, directionPDF=%e",
			pixelI, pixelJ, cosTheta, areaPDF, directionPDF)
	}
}

func TestCameraCalculateRayPDFs_ScaleConsistency(t *testing.T) {
	// Test that PDF scales are consistent with expected BDPT usage
	config := CameraConfig{
		Center:        core.NewVec3(278, 278, -800),
		LookAt:        core.NewVec3(278, 278, 0),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   1.0,
		VFov:          40.0,
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := NewCamera(config)

	ray := core.NewRay(
		core.NewVec3(278, 278, -800),
		core.NewVec3(0, 0, 1),
	)

	areaPDF, _ := camera.CalculateRayPDFs(ray)

	// Area PDF should be in the same order of magnitude as light area PDFs
	// Cornell box quad light is 130x105 = 13650 units^2
	// So light area PDF ≈ 1/13650 ≈ 7.3e-5
	lightAreaPDF := 1.0 / (130.0 * 105.0)

	// Camera area PDF should be in a similar range (maybe 1-2 orders of magnitude different)
	ratio := areaPDF / lightAreaPDF

	t.Logf("Camera area PDF: %e", areaPDF)
	t.Logf("Light area PDF: %e", lightAreaPDF)
	t.Logf("Ratio (camera/light): %f", ratio)

	// The ratio should be reasonable (not 1000x different)
	if ratio < 0.001 || ratio > 1000.0 {
		t.Errorf("Camera area PDF scale seems inconsistent with light area PDF. Ratio: %f", ratio)
	}
}
