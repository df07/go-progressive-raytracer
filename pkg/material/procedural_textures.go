package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// NewCheckerboardTexture creates a procedural checkerboard pattern texture
func NewCheckerboardTexture(width, height, checkSize int, color1, color2 core.Vec3) *ImageTexture {
	pixels := make([]core.Vec3, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Determine which check we're in
			checkX := x / checkSize
			checkY := y / checkSize

			// Alternate colors based on check position
			var color core.Vec3
			if (checkX+checkY)%2 == 0 {
				color = color1
			} else {
				color = color2
			}

			pixels[y*width+x] = color
		}
	}

	return NewImageTexture(width, height, pixels)
}

// NewUVDebugTexture creates a texture showing UV coordinates as colors
// U maps to red channel, V maps to green channel
func NewUVDebugTexture(width, height int) *ImageTexture {
	pixels := make([]core.Vec3, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			u := float64(x) / float64(width-1)
			v := float64(y) / float64(height-1)
			pixels[y*width+x] = core.NewVec3(u, v, 0.0)
		}
	}

	return NewImageTexture(width, height, pixels)
}

// NewGradientTexture creates a vertical gradient from color1 (top) to color2 (bottom)
func NewGradientTexture(width, height int, color1, color2 core.Vec3) *ImageTexture {
	pixels := make([]core.Vec3, width*height)

	for y := 0; y < height; y++ {
		// Interpolate from top to bottom
		t := float64(y) / float64(height-1)
		color := color1.Multiply(1.0 - t).Add(color2.Multiply(t))

		for x := 0; x < width; x++ {
			pixels[y*width+x] = color
		}
	}

	return NewImageTexture(width, height, pixels)
}
