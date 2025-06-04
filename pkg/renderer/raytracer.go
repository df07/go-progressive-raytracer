package renderer

import (
	"image"
	"image/color"

	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/math"
)

// Raytracer handles the rendering process
type Raytracer struct {
	scene  Scene
	width  int
	height int
}

// Scene interface to avoid circular imports
type Scene interface {
	GetCamera() *Camera
	GetBackgroundColors() (topColor, bottomColor math.Vec3)
	GetShapes() []geometry.Shape
}

// NewRaytracer creates a new raytracer
func NewRaytracer(scene Scene, width, height int) *Raytracer {
	return &Raytracer{
		scene:  scene,
		width:  width,
		height: height,
	}
}

// hitWorld checks if a ray hits any object in the scene
func (rt *Raytracer) hitWorld(ray math.Ray, tMin, tMax float64) (*geometry.HitRecord, bool) {
	var closestHit *geometry.HitRecord
	closestSoFar := tMax
	hitAnything := false

	for _, shape := range rt.scene.GetShapes() {
		if hit, isHit := shape.Hit(ray, tMin, closestSoFar); isHit {
			hitAnything = true
			closestSoFar = hit.T
			closestHit = hit
		}
	}

	return closestHit, hitAnything
}

// backgroundGradient returns a gradient color based on ray direction
func (rt *Raytracer) backgroundGradient(r math.Ray) math.Vec3 {
	// Get colors from the scene
	topColor, bottomColor := rt.scene.GetBackgroundColors()

	// Normalize the ray direction to get consistent results
	unitDirection := r.Direction.Normalize()

	// Use the y-component to create a gradient (map from -1,1 to 0,1)
	t := 0.5 * (unitDirection.Y + 1.0)

	// Linear interpolation: (1-t)*bottom + t*top
	return bottomColor.Multiply(1.0 - t).Add(topColor.Multiply(t))
}

// rayColor returns the color for a given ray
func (rt *Raytracer) rayColor(r math.Ray) color.RGBA {
	// Check for intersections with objects
	if hit, isHit := rt.hitWorld(r, 0.001, 1000.0); isHit {
		// Simple shading: use normal as color (for now)
		// Map normal from [-1,1] to [0,1] and use as RGB
		normalColor := hit.Normal.Add(math.NewVec3(1, 1, 1)).Multiply(0.5)
		// Clamp to valid color range
		normalColor = normalColor.Clamp(0.0, 1.0)
		// Apply gamma correction (gamma = 2.0)
		normalColor = normalColor.GammaCorrect(2.0)

		return color.RGBA{
			R: uint8(255 * normalColor.X),
			G: uint8(255 * normalColor.Y),
			B: uint8(255 * normalColor.Z),
			A: 255,
		}
	}

	// No intersection, use background gradient
	colorVec := rt.backgroundGradient(r)
	// Clamp to valid color range
	colorVec = colorVec.Clamp(0.0, 1.0)
	// Apply gamma correction (gamma = 2.0)
	colorVec = colorVec.GammaCorrect(2.0)

	// Convert to RGBA (0-255 range)
	return color.RGBA{
		R: uint8(255 * colorVec.X),
		G: uint8(255 * colorVec.Y),
		B: uint8(255 * colorVec.Z),
		A: 255,
	}
}

// RenderPass renders a single pass and returns an image
func (rt *Raytracer) RenderPass() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, rt.width, rt.height))
	camera := rt.scene.GetCamera()

	for j := rt.height - 1; j >= 0; j-- {
		for i := 0; i < rt.width; i++ {
			// Convert pixel coordinates to normalized coordinates (0-1)
			s := float64(i) / float64(rt.width-1)
			t := float64(j) / float64(rt.height-1)

			// Get the ray for this pixel
			ray := camera.GetRay(s, t)

			// Calculate the color
			pixelColor := rt.rayColor(ray)

			// Set the pixel
			img.SetRGBA(i, rt.height-1-j, pixelColor)
		}
	}

	return img
}
