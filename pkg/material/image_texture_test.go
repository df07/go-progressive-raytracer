package material

import (
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TestImageTextureEvaluate tests basic texture sampling
func TestImageTextureEvaluate(t *testing.T) {
	// Create a 2x2 checkerboard pattern
	// Layout:
	//   white black
	//   black white
	pixels := []core.Vec3{
		core.NewVec3(1, 1, 1), core.NewVec3(0, 0, 0), // Row 0 (top in image coords)
		core.NewVec3(0, 0, 0), core.NewVec3(1, 1, 1), // Row 1 (bottom in image coords)
	}
	texture := NewImageTexture(2, 2, pixels)

	white := core.NewVec3(1, 1, 1)
	black := core.NewVec3(0, 0, 0)

	// Test corner sampling (use values slightly inside to avoid wrapping ambiguity)
	// UV (0.1, 0.1) is bottom-left region, maps to image coords (0, 1) after V-flip
	// That's pixels[1*2 + 0] = black
	result := texture.Evaluate(core.NewVec2(0.1, 0.1), core.Vec3{})
	if !result.Equals(black) {
		t.Errorf("UV(0.1,0.1): expected %v, got %v", black, result)
	}

	// UV (0.9, 0.1) is bottom-right region, maps to image coords (1, 1) after V-flip
	// That's pixels[1*2 + 1] = white
	result = texture.Evaluate(core.NewVec2(0.9, 0.1), core.Vec3{})
	if !result.Equals(white) {
		t.Errorf("UV(0.9,0.1): expected %v, got %v", white, result)
	}

	// UV (0.1, 0.9) is top-left region, maps to image coords (0, 0) after V-flip
	// That's pixels[0*2 + 0] = white
	result = texture.Evaluate(core.NewVec2(0.1, 0.9), core.Vec3{})
	if !result.Equals(white) {
		t.Errorf("UV(0.1,0.9): expected %v, got %v", white, result)
	}

	// UV (0.9, 0.9) is top-right region, maps to image coords (1, 0) after V-flip
	// That's pixels[0*2 + 1] = black
	result = texture.Evaluate(core.NewVec2(0.9, 0.9), core.Vec3{})
	if !result.Equals(black) {
		t.Errorf("UV(0.9,0.9): expected %v, got %v", black, result)
	}
}

// TestImageTextureWrapping tests UV wrapping behavior
func TestImageTextureWrapping(t *testing.T) {
	// Simple 1x1 red texture
	pixels := []core.Vec3{core.NewVec3(1, 0, 0)}
	texture := NewImageTexture(1, 1, pixels)

	red := core.NewVec3(1, 0, 0)

	// Test that UVs outside [0,1] wrap correctly
	testCases := []core.Vec2{
		core.NewVec2(0.5, 0.5),   // Normal case
		core.NewVec2(1.5, 0.5),   // U wraps
		core.NewVec2(0.5, 1.5),   // V wraps
		core.NewVec2(-0.5, -0.5), // Negative wrap
		core.NewVec2(2.3, 3.7),   // Large values
	}

	for _, uv := range testCases {
		result := texture.Evaluate(uv, core.Vec3{})
		if !result.Equals(red) {
			t.Errorf("UV%v: expected %v, got %v", uv, red, result)
		}
	}
}

// TestImageTextureSampling tests that sampling selects correct pixels
func TestImageTextureSampling(t *testing.T) {
	// Create a 4x4 gradient
	pixels := make([]core.Vec3, 16)
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			// Each pixel gets a unique brightness based on position
			val := float64(y*4+x) / 15.0
			pixels[y*4+x] = core.NewVec3(val, val, val)
		}
	}
	texture := NewImageTexture(4, 4, pixels)

	// Sample at UV (0.125, 0.875) which should map to pixel (0, 0) in image coords
	// UV V=0.875 -> flipped to 0.125 -> pixel y=0
	// UV U=0.125 -> pixel x=0
	// Pixel (0, 0) has value 0/15
	result := texture.Evaluate(core.NewVec2(0.125, 0.875), core.Vec3{})
	expected := core.NewVec3(0, 0, 0)
	if !result.Equals(expected) {
		t.Errorf("Sample top-left: expected %v, got %v", expected, result)
	}

	// Sample at UV (0.875, 0.125) which should map to pixel (3, 3) in image coords
	// UV V=0.125 -> flipped to 0.875 -> pixel y=3
	// UV U=0.875 -> pixel x=3
	// Pixel (3, 3) has value 15/15 = 1.0
	result = texture.Evaluate(core.NewVec2(0.875, 0.125), core.Vec3{})
	expected = core.NewVec3(1, 1, 1)
	if !result.Equals(expected) {
		t.Errorf("Sample bottom-right: expected %v, got %v", expected, result)
	}
}

// TestSolidColor tests the backward compatibility solid color source
func TestSolidColor(t *testing.T) {
	color := core.NewVec3(0.7, 0.3, 0.1)
	solid := NewSolidColor(color)

	// Should return same color regardless of UV or position
	testCases := []struct {
		uv    core.Vec2
		point core.Vec3
	}{
		{core.NewVec2(0, 0), core.NewVec3(0, 0, 0)},
		{core.NewVec2(1, 1), core.NewVec3(5, 3, -2)},
		{core.NewVec2(0.5, 0.5), core.NewVec3(-1, -1, -1)},
	}

	for _, tc := range testCases {
		result := solid.Evaluate(tc.uv, tc.point)
		if !result.Equals(color) {
			t.Errorf("SolidColor at UV%v, Point%v: expected %v, got %v",
				tc.uv, tc.point, color, result)
		}
	}
}
