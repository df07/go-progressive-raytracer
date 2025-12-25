package loaders

import (
	"fmt"
	"image"
	_ "image/jpeg" // JPEG decoder
	_ "image/png"  // PNG decoder
	"os"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// ImageData contains loaded image data as Vec3 color array
type ImageData struct {
	Width  int
	Height int
	Pixels []core.Vec3
}

// LoadImage loads a PNG or JPEG image and converts it to Vec3 color array
func LoadImage(filename string) (*ImageData, error) {
	// Open file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// Decode image (auto-detects PNG/JPEG from file header)
	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Log the detected format for debugging
	_ = format // PNG or JPEG

	// Convert to Vec3 array
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	pixels := make([]core.Vec3, width*height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			// RGBA returns uint32 in [0, 65535], convert to [0, 1]
			pixels[y*width+x] = core.NewVec3(
				float64(r)/65535.0,
				float64(g)/65535.0,
				float64(b)/65535.0,
			)
		}
	}

	return &ImageData{
		Width:  width,
		Height: height,
		Pixels: pixels,
	}, nil
}
