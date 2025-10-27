package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewConeTestScene creates a simple test scene with cones and frustums
func NewConeTestScene(cameraOverrides ...geometry.CameraConfig) *Scene {
	// Default camera configuration
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 1.5, 4), // Camera position
		LookAt:        core.NewVec3(0, 1, 0),   // Look at the cones
		Up:            core.NewVec3(0, 1, 0),   // Standard up direction
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          50.0,
		Aperture:      0.0, // No depth of field
		FocusDistance: 0.0,
	}

	// Apply any overrides
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	camera := geometry.NewCamera(cameraConfig)

	samplingConfig := SamplingConfig{
		SamplesPerPixel:           200,
		MaxDepth:                  50,
		RussianRouletteMinBounces: 20,
		AdaptiveMinSamples:        0.15,
		AdaptiveThreshold:         0.01,
	}

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]geometry.Shape, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Create materials
	lambertianGray := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	lambertianRed := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
	lambertianBlue := material.NewLambertian(core.NewVec3(0.2, 0.2, 0.8))
	lambertianGreen := material.NewLambertian(core.NewVec3(0.2, 0.8, 0.2))
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.1)
	materialGlass := material.NewDielectric(1.5)

	// Create ground quad
	groundQuad := NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGray)

	// Create cones with different materials and types

	// Central tall pointed cone (Y-axis aligned) - Red lambertian
	coneCenter, err := geometry.NewCone(
		core.NewVec3(0, 0, 0), // base center
		0.5,                   // base radius
		core.NewVec3(0, 2, 0), // top center
		0.0,                   // top radius (pointed)
		lambertianRed,
	)
	if err != nil {
		panic(err)
	}

	// Left frustum (truncated cone) - Metal
	coneLeft, err := geometry.NewCone(
		core.NewVec3(-2, 0, 0),   // base center
		0.5,                      // base radius
		core.NewVec3(-2, 1.5, 0), // top center
		0.2,                      // top radius (frustum)
		metalGold,
	)
	if err != nil {
		panic(err)
	}

	// Right wide cone - Blue lambertian
	coneRight, err := geometry.NewCone(
		core.NewVec3(2, 0, 0),   // base center
		0.8,                     // base radius
		core.NewVec3(2, 1.2, 0), // top center
		0.0,                     // top radius (pointed)
		lambertianBlue,
	)
	if err != nil {
		panic(err)
	}

	// Small glass cone (front right)
	coneGlass, err := geometry.NewCone(
		core.NewVec3(1.2, 0, 0.8),   // base center
		0.25,                        // base radius
		core.NewVec3(1.2, 1.0, 0.8), // top center
		0.0,                         // top radius (pointed)
		materialGlass,
	)
	if err != nil {
		panic(err)
	}

	// Tilted green frustum (back left, more visible)
	coneTilted, err := geometry.NewCone(
		core.NewVec3(-1.5, 0, -0.5),   // base center
		0.4,                           // base radius
		core.NewVec3(-1.2, 1.2, -0.3), // top center (tilted)
		0.15,                          // top radius (frustum)
		lambertianGreen,
	)
	if err != nil {
		panic(err)
	}

	// Add objects to the scene
	s.Shapes = append(s.Shapes,
		groundQuad,
		coneCenter,
		coneLeft,
		coneRight,
		coneGlass,
		coneTilted,
	)

	// Add a sphere light
	s.AddSphereLight(
		core.NewVec3(3, 5, 3),          // position
		1.5,                            // radius
		core.NewVec3(10.0, 10.0, 10.0), // emission
	)

	// Add gradient infinite light
	s.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white)
	)

	return s
}
