package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// ImageTexture provides color from a 2D image
type ImageTexture struct {
	Width  int
	Height int
	Pixels []core.Vec3 // Row-major: Pixels[y*Width + x]
}

// NewImageTexture creates a new image texture
func NewImageTexture(width, height int, pixels []core.Vec3) *ImageTexture {
	return &ImageTexture{
		Width:  width,
		Height: height,
		Pixels: pixels,
	}
}

// Evaluate samples the texture at given UV coordinates using nearest-neighbor filtering
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
	// Wrap UV coordinates to [0, 1]
	u := uv.X - float64(int(uv.X))
	v := uv.Y - float64(int(uv.Y))
	if u < 0 {
		u += 1.0
	}
	if v < 0 {
		v += 1.0
	}

	// Convert to pixel coordinates
	// V=0 is bottom, V=1 is top (flip V for image coordinates where origin is top-left)
	x := int(u * float64(t.Width))
	y := int((1.0 - v) * float64(t.Height))

	// Clamp to image bounds
	if x >= t.Width {
		x = t.Width - 1
	}
	if y >= t.Height {
		y = t.Height - 1
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	return t.Pixels[y*t.Width+x]
}
