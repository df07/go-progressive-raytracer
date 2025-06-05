package renderer

import (
	"image"
	"image/color"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// SamplingConfig contains rendering configuration
type SamplingConfig struct {
	SamplesPerPixel int // Number of rays per pixel
	MaxDepth        int // Maximum ray bounce depth
}

// DefaultSamplingConfig returns sensible default values
func DefaultSamplingConfig() SamplingConfig {
	return SamplingConfig{
		SamplesPerPixel: 200,
		MaxDepth:        50,
	}
}

// Raytracer handles the rendering process
type Raytracer struct {
	scene  Scene
	width  int
	height int
	config SamplingConfig
	random *rand.Rand
}

// Scene interface to avoid circular imports
type Scene interface {
	GetCamera() *Camera
	GetBackgroundColors() (topColor, bottomColor core.Vec3)
	GetShapes() []core.Shape
}

// NewRaytracer creates a new raytracer
func NewRaytracer(scene Scene, width, height int) *Raytracer {
	return &Raytracer{
		scene:  scene,
		width:  width,
		height: height,
		config: DefaultSamplingConfig(),
		random: rand.New(rand.NewSource(42)), // Deterministic for testing
	}
}

// SetSamplingConfig updates the sampling configuration
func (rt *Raytracer) SetSamplingConfig(config SamplingConfig) {
	rt.config = config
}

// hitWorld checks if a ray hits any object in the scene
func (rt *Raytracer) hitWorld(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	var closestHit *core.HitRecord
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
func (rt *Raytracer) backgroundGradient(r core.Ray) core.Vec3 {
	// Get colors from the scene
	topColor, bottomColor := rt.scene.GetBackgroundColors()

	// Normalize the ray direction to get consistent results
	unitDirection := r.Direction.Normalize()

	// Use the y-component to create a gradient (map from -1,1 to 0,1)
	t := 0.5 * (unitDirection.Y + 1.0)

	// Linear interpolation: (1-t)*bottom + t*top
	return bottomColor.Multiply(1.0 - t).Add(topColor.Multiply(t))
}

// calculateSpecularColor handles specular material scattering
func (rt *Raytracer) calculateSpecularColor(scatter core.ScatterResult, depth int) core.Vec3 {
	return scatter.Attenuation.MultiplyVec(
		rt.rayColorRecursive(scatter.Scattered, depth-1))
}

// calculateDiffuseColor handles diffuse material scattering with proper Monte Carlo integration
func (rt *Raytracer) calculateDiffuseColor(scatter core.ScatterResult, hit *core.HitRecord, depth int) core.Vec3 {
	scatterDirection := scatter.Scattered.Direction.Normalize()
	cosine := scatterDirection.Dot(hit.Normal)
	if cosine < 0 {
		cosine = 0
	}

	if scatter.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Invalid PDF
	}

	// Monte Carlo estimator: (BRDF * incomingLight * cosine) / PDF
	// For lambertian: BRDF = albedo/π, PDF = cosθ/π
	// Result = (albedo/π * incomingLight * cosθ) / (cosθ/π) = albedo * incomingLight
	incomingLight := rt.rayColorRecursive(scatter.Scattered, depth-1)
	return scatter.Attenuation.Multiply(cosine / scatter.PDF).MultiplyVec(incomingLight)
}

// rayColorRecursive returns the color for a given ray with material support
func (rt *Raytracer) rayColorRecursive(r core.Ray, depth int) core.Vec3 {
	// If we've exceeded the ray bounce limit, no more light is gathered
	if depth <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check for intersections with objects
	hit, isHit := rt.hitWorld(r, 0.001, 1000.0)
	if !isHit {
		return rt.backgroundGradient(r)
	}

	// Try to scatter the ray
	scatter, didScatter := hit.Material.Scatter(r, *hit, rt.random)
	if !didScatter {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Material absorbed the ray
	}

	// Handle scattering based on material type
	if scatter.IsSpecular() {
		return rt.calculateSpecularColor(scatter, depth)
	} else {
		return rt.calculateDiffuseColor(scatter, hit, depth)
	}
}

// vec3ToColor converts a Vec3 color to RGBA with proper clamping and gamma correction
func (rt *Raytracer) vec3ToColor(colorVec core.Vec3) color.RGBA {
	// Apply gamma correction (gamma = 2.0)
	colorVec = colorVec.GammaCorrect(2.0)

	// Clamp to valid color range
	colorVec = colorVec.Clamp(0.0, 1.0)

	return color.RGBA{
		R: uint8(255 * colorVec.X),
		G: uint8(255 * colorVec.Y),
		B: uint8(255 * colorVec.Z),
		A: 255,
	}
}

// RenderPass renders a single pass with multi-sampling and returns an image
func (rt *Raytracer) RenderPass() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, rt.width, rt.height))
	camera := rt.scene.GetCamera()

	for j := rt.height - 1; j >= 0; j-- {
		for i := 0; i < rt.width; i++ {
			// Accumulate color from multiple samples
			colorAccum := core.Vec3{X: 0, Y: 0, Z: 0}

			for sample := 0; sample < rt.config.SamplesPerPixel; sample++ {
				// Convert pixel coordinates to normalized coordinates with jitter
				s := (float64(i) + rt.random.Float64()) / float64(rt.width)
				t := (float64(j) + rt.random.Float64()) / float64(rt.height)

				// Get the ray for this jittered pixel
				ray := camera.GetRay(s, t)

				// Calculate the color and accumulate
				colorAccum = colorAccum.Add(rt.rayColorRecursive(ray, rt.config.MaxDepth))
			}

			// Average the accumulated colors
			colorVec := colorAccum.Multiply(1.0 / float64(rt.config.SamplesPerPixel))
			pixelColor := rt.vec3ToColor(colorVec)

			// Set the pixel
			img.SetRGBA(i, rt.height-1-j, pixelColor)
		}
	}

	return img
}
