package renderer

import (
	"image"
	"image/color"
	"testing"
)

func TestCalculateAverageLuminance(t *testing.T) {
	// Create a 2x2 image
	// Top-left: Red (1, 0, 0) -> Lum = 0.2126
	// Top-right: Green (0, 1, 0) -> Lum = 0.7152
	// Bottom-left: Blue (0, 0, 1) -> Lum = 0.0722
	// Bottom-right: Black (0, 0, 0) -> Lum = 0.0

	// Expected average: (0.2126 + 0.7152 + 0.0722 + 0.0) / 4 = 1.0 / 4 = 0.25

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{0, 255, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})
	img.Set(1, 1, color.RGBA{0, 0, 0, 255})

	avgLum := CalculateAverageLuminance(img)
	expected := 0.25
	tolerance := 0.0001

	if avgLum < expected-tolerance || avgLum > expected+tolerance {
		t.Errorf("Expected average luminosity %f, got %f", expected, avgLum)
	}
}

func TestCalculateAverageLuminance_White(t *testing.T) {
	// 1x1 White pixel -> Lum = 1.0
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255})

	avgLum := CalculateAverageLuminance(img)
	expected := 1.0
	tolerance := 0.0001

	if avgLum < expected-tolerance || avgLum > expected+tolerance {
		t.Errorf("Expected average luminosity %f, got %f", expected, avgLum)
	}
}
