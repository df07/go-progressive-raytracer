package scene

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// oklchToRGB converts OKLCH color values to RGB
// L: lightness (0-1), C: chroma (0-0.4+), H: hue (0-360 degrees)
func oklchToRGB(l, c, h float64) core.Vec3 {
	// Convert hue from degrees to radians
	hRad := h * math.Pi / 180.0

	// Convert from OKLCH to OKLAB
	a := c * math.Cos(hRad)
	b := c * math.Sin(hRad)

	// Convert from OKLAB to linear RGB
	// Using simplified approximation for OKLAB to RGB conversion
	// This is not perfectly accurate but good enough for our purposes

	// First convert to LMS
	l_ := l + 0.3963377774*a + 0.2158037573*b
	m_ := l - 0.1055613458*a - 0.0638541728*b
	s_ := l - 0.0894841775*a - 1.2914855480*b

	// Cube the values
	l_ = l_ * l_ * l_
	m_ = m_ * m_ * m_
	s_ = s_ * s_ * s_

	// Convert LMS to linear RGB
	r := +4.0767416621*l_ - 3.3077115913*m_ + 0.2309699292*s_
	g := -1.2684380046*l_ + 2.6097574011*m_ - 0.3413193965*s_
	blue := -0.0041960863*l_ - 0.7034186147*m_ + 1.7076147010*s_

	// Clamp to [0, 1] range
	r = math.Max(0, math.Min(1, r))
	g = math.Max(0, math.Min(1, g))
	blue = math.Max(0, math.Min(1, blue))

	return core.NewVec3(r, g, blue)
}

// NewSphereGridScene creates a scene with a 10x10 grid of spheres
func NewSphereGridScene(cameraOverrides ...renderer.CameraConfig) *Scene {
	// Default camera configuration for sphere grid
	defaultCameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(4.5, 6, 18),    // Position camera farther back and slightly lower
		LookAt:        core.NewVec3(4.5, 0.8, 4.5), // Look at center of grid, slightly lower
		Up:            core.NewVec3(0, 1, 0),       // Standard up direction
		Width:         800,
		AspectRatio:   16.0 / 9.0, // 16:9 aspect ratio
		VFov:          40.0,       // Slightly narrower field of view for better framing
		Aperture:      0.02,       // Small depth of field for some focus variation
		FocusDistance: 0.0,        // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = renderer.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	samplingConfig := core.SamplingConfig{
		SamplesPerPixel:           100,
		MaxDepth:                  40,
		RussianRouletteMinBounces: 12,    // Moderate bounces for metallic reflections
		RussianRouletteMinSamples: 6,     // Standard samples before RR
		AdaptiveMinSamples:        6,     // Standard minimum for adaptive sampling
		AdaptiveThreshold:         0.015, // 1.5% relative error threshold
		AdaptiveDarkThreshold:     1e-6,  // Standard absolute threshold
	}

	camera := renderer.NewCamera(cameraConfig)

	// Create the scene
	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.5, 0.7, 1.0), // Blue sky (same as default scene)
		BottomColor:    core.NewVec3(1.0, 1.0, 1.0), // White horizon (same as default scene)
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add environmental lighting - a bright sun-like light
	s.AddSphereLight(
		core.NewVec3(20, 25, 20),       // position (high and to the side)
		8,                              // radius
		core.NewVec3(12.0, 11.5, 10.0), // warm white emission
	)

	// Create ground plane (gray lambertian)
	groundPlane := geometry.NewPlane(
		core.NewVec3(0, 0, 0),                               // point on plane
		core.NewVec3(0, 1, 0),                               // normal vector (up)
		material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5)), // medium gray
	)
	s.Shapes = append(s.Shapes, groundPlane)

	// Create grid of spheres - automatically scale to fit in same visual space
	gridSize := 20

	// Calculate spacing and radius to fit grid in same visual area
	// Target area: roughly 9x9 units (to fit nicely in camera view)
	targetArea := 9.0
	spacing := targetArea / float64(gridSize-1)

	// Scale sphere radius based on spacing, but keep reasonable minimum/maximum
	sphereRadius := spacing * 0.35 // 35% of spacing
	minRadius := 0.02              // Minimum visible radius
	maxRadius := 0.35              // Maximum radius (original size)
	sphereRadius = math.Max(minRadius, math.Min(maxRadius, sphereRadius))

	// OKLCH parameters for color variation
	baseLightness := 0.65 // Keep lightness relatively constant for uniform appearance
	minChroma := 0.05     // Minimum chroma (near white/gray)
	maxChroma := 0.25     // Maximum chroma (vivid colors)

	for i := 0; i < gridSize; i++ {
		for j := 0; j < gridSize; j++ {
			// Position sphere in grid, centered around the camera's look-at point
			x := float64(i)*spacing - targetArea/2.0 + 4.5 // Center around x=4.5
			z := float64(j)*spacing - targetArea/2.0 + 4.5 // Center around z=4.5
			y := sphereRadius                              // Sphere sits on ground plane

			position := core.NewVec3(x, y, z)

			// Calculate OKLCH color based on grid position
			// Vary hue across X axis (0-360 degrees)
			hue := (float64(i) / float64(gridSize-1)) * 360.0

			// Vary chroma across Z axis (from desaturated to vivid)
			chroma := minChroma + (float64(j)/float64(gridSize-1))*(maxChroma-minChroma)

			// Slight lightness variation for more interest
			lightness := baseLightness + 0.1*math.Sin(float64(i+j)*0.5)

			// Convert OKLCH to RGB
			color := oklchToRGB(lightness, chroma, hue)

			// Create metallic material with the calculated color
			// Add slight roughness for more realistic metal appearance
			roughness := 0.05 + 0.1*float64((i+j)%3)/2.0 // Vary roughness slightly
			metalMaterial := material.NewMetal(color, roughness)

			// Create sphere
			sphere := geometry.NewSphere(position, sphereRadius, metalMaterial)
			s.Shapes = append(s.Shapes, sphere)
		}
	}

	return s
}
