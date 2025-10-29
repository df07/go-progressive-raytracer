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

	// Central tall pointed cone (Y-axis aligned) - Red lambertian, CAPPED
	coneCenter, err := geometry.NewCone(
		core.NewVec3(0, 0, 0), // base center
		0.5,                   // base radius
		core.NewVec3(0, 2, 0), // top center
		0.0,                   // top radius (pointed)
		true,                  // capped - shows base cap
		lambertianRed,
	)
	if err != nil {
		panic(err)
	}

	// Left frustum (truncated cone) - Metal, CAPPED
	// Tilted backward so base cap faces toward camera
	coneLeft, err := geometry.NewCone(
		core.NewVec3(-2, 0.8, -0.8), // base center (elevated and back)
		0.5,                         // base radius
		core.NewVec3(-2, 0.2, 0.5),  // top center (lower and forward - tilted back)
		0.2,                         // top radius (frustum)
		true,                        // capped - base cap should be visible
		metalGold,
	)
	if err != nil {
		panic(err)
	}

	// Right: Wide short frustum - Blue lambertian, CAPPED
	// Short and wide so the top cap is clearly visible
	coneRight, err := geometry.NewCone(
		core.NewVec3(2, 0, 0),   // base center
		0.8,                     // base radius (wide)
		core.NewVec3(2, 0.6, 0), // top center (short)
		0.5,                     // top radius (wide frustum - clear top cap)
		true,                    // capped - top cap should be very visible
		lambertianBlue,
	)
	if err != nil {
		panic(err)
	}

	// Glass cone sitting on top of blue frustum - creates visual continuation
	// Positioned to perfectly align with blue frustum top
	coneGlass, err := geometry.NewCone(
		core.NewVec3(2, 0.6, 0), // base center (at blue frustum top)
		0.5,                     // base radius (matches blue frustum top radius)
		core.NewVec3(2, 1.8, 0), // top center (pointed, extends upward)
		0.0,                     // top radius (pointed)
		true,                    // capped - base cap sits on blue frustum
		materialGlass,
	)
	if err != nil {
		panic(err)
	}

	// Tilted green frustum (back left) - UNCAPPED
	coneTilted, err := geometry.NewCone(
		core.NewVec3(-1.5, 0, -0.5),   // base center
		0.4,                           // base radius
		core.NewVec3(-1.2, 1.2, -0.3), // top center (tilted)
		0.15,                          // top radius (frustum)
		false,                         // uncapped - open at both ends
		lambertianGreen,
	)
	if err != nil {
		panic(err)
	}

	// Small glass cone TOUCHING the ground (front left) - CAPPED
	glassConeTouching, err := geometry.NewCone(
		core.NewVec3(-0.8, 0, 1.2),   // base center (on ground)
		0.3,                          // base radius
		core.NewVec3(-0.8, 0.8, 1.2), // top center
		0.0,                          // pointed
		true,                         // capped - base cap touches ground
		materialGlass,
	)
	if err != nil {
		panic(err)
	}

	// Small glass cone FLOATING above ground (front right) - CAPPED
	glassConeFLoating, err := geometry.NewCone(
		core.NewVec3(0.8, 0.2, 1.2), // base center (0.2 units above ground)
		0.3,                         // base radius
		core.NewVec3(0.8, 1.0, 1.2), // top center
		0.0,                         // pointed
		true,                        // capped - base cap NOT touching anything
		materialGlass,
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
		glassConeTouching,
		glassConeFLoating,
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
