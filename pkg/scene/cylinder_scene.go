package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewCylinderTestScene creates a simple test scene with cylinders
func NewCylinderTestScene(cameraOverrides ...geometry.CameraConfig) *Scene {
	// Default camera configuration
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 1.5, 4), // Camera position
		LookAt:        core.NewVec3(0, 1, 0),   // Look at the cylinders
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
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.1)
	materialGlass := material.NewDielectric(1.5)

	// Create ground quad
	groundQuad := NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGray)

	// Create cylinders with different materials and orientations
	// Mix of capped and uncapped to showcase the difference

	// Center: Gold tube pointing toward camera - Metal, UNCAPPED
	// Angled so you can look directly down the hollow tube
	cylinderCenter := geometry.NewCylinder(
		core.NewVec3(-0.3, 1.0, -1.5), // base (further from camera)
		core.NewVec3(0, 1.2, 2.0),     // top (closer to camera)
		0.35,                          // radius
		false,                         // uncapped - open tube (look through it!)
		metalGold,
	)

	// Right: Tall cylinder (Y-axis aligned) - Red lambertian, CAPPED
	cylinderRight := geometry.NewCylinder(
		core.NewVec3(1.8, 0, 0), // base
		core.NewVec3(1.8, 2, 0), // top
		0.5,                     // radius
		true,                    // capped - can see top cap from above
		lambertianRed,
	)

	// Left: Horizontal cylinder (X-axis aligned) - Blue lambertian, CAPPED
	cylinderLeft := geometry.NewCylinder(
		core.NewVec3(-2.5, 0.3, 0), // base
		core.NewVec3(-1.5, 0.3, 0), // top
		0.3,                        // radius
		true,                       // capped - can see end caps
		lambertianBlue,
	)

	// Small glass cylinder (short, in front) - CAPPED
	cylinderGlass := geometry.NewCylinder(
		core.NewVec3(0.5, 0, 1),   // base
		core.NewVec3(0.5, 0.6, 1), // top
		0.2,                       // radius
		true,                      // capped
		materialGlass,
	)

	// Add objects to the scene
	s.Shapes = append(s.Shapes,
		groundQuad,
		cylinderCenter,
		cylinderLeft,
		cylinderRight,
		cylinderGlass,
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
