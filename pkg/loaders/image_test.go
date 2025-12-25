package loaders

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TestLoadImage creates a test PNG and verifies loading
func TestLoadImage(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.png")

	// Create a simple 2x2 test image
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	// Set pixel colors (RGBA with max value 65535 when using RGBA())
	// Top-left: white
	img.Set(0, 0, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	// Top-right: red
	img.Set(1, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	// Bottom-left: green
	img.Set(0, 1, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	// Bottom-right: blue
	img.Set(1, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})

	// Save as PNG
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("Failed to encode PNG: %v", err)
	}
	f.Close()

	// Load the image
	imageData, err := LoadImage(testFile)
	if err != nil {
		t.Fatalf("LoadImage failed: %v", err)
	}

	// Verify dimensions
	if imageData.Width != 2 || imageData.Height != 2 {
		t.Errorf("Expected 2x2 image, got %dx%d", imageData.Width, imageData.Height)
	}

	// Verify pixel count
	if len(imageData.Pixels) != 4 {
		t.Errorf("Expected 4 pixels, got %d", len(imageData.Pixels))
	}

	// Helper function to check color with tolerance for precision
	checkColor := func(name string, got, expected core.Vec3) {
		const tolerance = 0.01
		if abs(got.X-expected.X) > tolerance ||
			abs(got.Y-expected.Y) > tolerance ||
			abs(got.Z-expected.Z) > tolerance {
			t.Errorf("%s: expected %v, got %v", name, expected, got)
		}
	}

	// Verify colors (row-major order)
	white := core.NewVec3(1.0, 1.0, 1.0)
	red := core.NewVec3(1.0, 0.0, 0.0)
	green := core.NewVec3(0.0, 1.0, 0.0)
	blue := core.NewVec3(0.0, 0.0, 1.0)

	checkColor("Top-left (white)", imageData.Pixels[0], white)
	checkColor("Top-right (red)", imageData.Pixels[1], red)
	checkColor("Bottom-left (green)", imageData.Pixels[2], green)
	checkColor("Bottom-right (blue)", imageData.Pixels[3], blue)
}

// TestLoadImageNotFound verifies error handling for missing files
func TestLoadImageNotFound(t *testing.T) {
	_, err := LoadImage("nonexistent.png")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
