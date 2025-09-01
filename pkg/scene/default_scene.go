package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewDefaultScene creates a default scene with spheres, ground, and camera
func NewDefaultScene(cameraOverrides ...geometry.CameraConfig) *Scene {
	// Default camera configuration
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 0.75, 2), // Position camera higher and farther back
		LookAt:        core.NewVec3(0, 0.5, -1), // Look at the sphere center
		Up:            core.NewVec3(0, 1, 0),    // Standard up direction
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          40.0, // Narrower field of view for focus effect
		Aperture:      0.05, // Strong depth of field blur
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	camera := geometry.NewCamera(cameraConfig)

	samplingConfig := SamplingConfig{
		SamplesPerPixel:           200,
		MaxDepth:                  50,
		RussianRouletteMinBounces: 20,   // Need a lot of bounces for complex glass
		AdaptiveMinSamples:        0.15, // 15% of max samples minimum for adaptive sampling
		AdaptiveThreshold:         0.01, // 1% relative error threshold
	}

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]geometry.Shape, 0),
		Lights:         make([]lights.Light, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Create materials
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0).Multiply(0.6))
	lambertianBlue := material.NewLambertian(core.NewVec3(0.1, 0.2, 0.5))
	lambertianRed := material.NewLambertian(core.NewVec3(0.65, 0.25, 0.2))
	metalSilver := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0)
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)
	materialGlass := material.NewDielectric(1.5)

	// Create layered material: glass coating over gold base
	coatedRed := material.NewLayered(materialGlass, lambertianRed)

	// Create spheres with different materials
	sphereCenter := geometry.NewSphere(core.NewVec3(0, 0.5, -1), 0.5, coatedRed)
	sphereLeft := geometry.NewSphere(core.NewVec3(-1, 0.5, -1), 0.5, metalSilver)
	sphereRight := geometry.NewSphere(core.NewVec3(1, 0.5, -1), 0.5, metalGold)
	solidGlassSphere := geometry.NewSphere(core.NewVec3(0.5, 0.25, -0.5), 0.25, materialGlass)

	// Create ground quad instead of infinite plane (large but finite for proper bounds)
	groundQuad := NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGreen)

	// Create hollow glass sphere with blue sphere inside
	hollowGlassOuter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.25, materialGlass)
	hollowGlassInner := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), -0.24, materialGlass)
	hollowGlassCenter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.20, lambertianBlue)

	// Add objects to the scene
	groundOnly := false
	if groundOnly {
		s.Shapes = append(s.Shapes, groundQuad)
	} else {
		// Add the specified light: pos [30, 30.5, 15], r: 10, emit: [15.0, 14.0, 13.0]
		s.AddSphereLight(
			core.NewVec3(30, 30.5, 15),     // position
			10,                             // radius
			core.NewVec3(15.0, 14.0, 13.0), // emission
		)
		s.Shapes = append(s.Shapes, sphereCenter, sphereLeft, sphereRight, groundQuad,
			solidGlassSphere, hollowGlassOuter, hollowGlassInner, hollowGlassCenter)
	}

	// Add gradient infinite light (replaces background gradient)
	s.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white ground)
	)

	return s
}
