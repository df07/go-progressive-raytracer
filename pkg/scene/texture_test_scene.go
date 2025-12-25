package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewTextureTestScene creates a scene demonstrating texture mapping on various geometry
func NewTextureTestScene(cameraOverrides ...geometry.CameraConfig) *Scene {
	// Camera configuration
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 2, 10),
		LookAt:        core.NewVec3(0, 1, 0),
		Up:            core.NewVec3(0, 1, 0),
		Width:         800,
		AspectRatio:   16.0 / 9.0,
		VFov:          50.0, // Wider FOV to see all shapes
		Aperture:      0.0,  // No DOF for texture clarity
		FocusDistance: 0.0,
	}

	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	camera := geometry.NewCamera(cameraConfig)

	samplingConfig := SamplingConfig{
		SamplesPerPixel:           100,
		MaxDepth:                  10,
		RussianRouletteMinBounces: 5,
		AdaptiveMinSamples:        0.15,
		AdaptiveThreshold:         0.01,
	}

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]geometry.Shape, 0),
		Lights:         make([]lights.Light, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Create procedural textures
	checkerboard := material.NewCheckerboardTexture(256, 256, 32,
		core.NewVec3(0.9, 0.9, 0.9), // White
		core.NewVec3(0.2, 0.2, 0.8), // Blue
	)

	redGreenGradient := material.NewGradientTexture(256, 256,
		core.NewVec3(1.0, 0.2, 0.2), // Red (top)
		core.NewVec3(0.2, 1.0, 0.2), // Green (bottom)
	)

	uvDebug := material.NewUVDebugTexture(256, 256)

	fineBrickPattern := material.NewCheckerboardTexture(512, 512, 16,
		core.NewVec3(0.7, 0.3, 0.1),  // Orange
		core.NewVec3(0.5, 0.2, 0.05), // Dark brown
	)

	// Create textured materials
	checkerMat := material.NewTexturedLambertian(checkerboard)
	gradientMat := material.NewTexturedLambertian(redGreenGradient)
	uvDebugMat := material.NewTexturedLambertian(uvDebug)
	brickMat := material.NewTexturedLambertian(fineBrickPattern)

	// All shapes in a single row, left to right
	// Sphere with checkerboard
	sphere := geometry.NewSphere(core.NewVec3(-6, 1, 0), 1.0, checkerMat)

	// Cylinder with gradient
	cylinder := geometry.NewCylinder(
		core.NewVec3(-4, 0, 0), // base center
		core.NewVec3(-4, 2, 0), // top center
		0.6,                    // radius
		true,                   // capped
		gradientMat,
	)

	// Cone with UV debug
	cone, _ := geometry.NewCone(
		core.NewVec3(-2, 0, 0), // base center
		0.8,                    // base radius
		core.NewVec3(-2, 2, 0), // top center
		0.2,                    // top radius (frustum)
		true,                   // capped
		uvDebugMat,
	)

	// Box with brick pattern
	box := geometry.NewAxisAlignedBox(
		core.NewVec3(0, 1, 0),       // center
		core.NewVec3(0.8, 0.8, 0.8), // half-extents
		brickMat,
	)

	// Disc with checkerboard (vertical, facing camera)
	disc := geometry.NewDisc(
		core.NewVec3(2, 1.2, 0), // center
		core.NewVec3(0, 0, 1),   // normal (facing forward)
		0.9,                     // radius
		checkerMat,
	)

	// Quad with gradient (vertical, slightly rotated)
	quad := geometry.NewQuad(
		core.NewVec3(3.5, 0, 0.2),  // corner
		core.NewVec3(1.5, 0, -0.3), // u vector (tilted)
		core.NewVec3(0, 2, 0),      // v vector (vertical)
		gradientMat,
	)

	// Triangle with UV debug (make it visible as a standalone shape)
	triangle := geometry.NewTriangle(
		core.NewVec3(5.5, 0, 0),  // bottom left
		core.NewVec3(7, 0, 0),    // bottom right
		core.NewVec3(6.25, 2, 0), // top
		uvDebugMat,
	)

	// Ground with subtle brick pattern
	ground := geometry.NewQuad(
		core.NewVec3(-10, 0, -5), // corner
		core.NewVec3(20, 0, 0),   // u vector
		core.NewVec3(0, 0, 15),   // v vector
		brickMat,
	)

	// Add shapes to scene
	s.Shapes = append(s.Shapes,
		// All geometry types in a row
		sphere,
		cylinder,
		cone,
		box,
		disc,
		quad,
		triangle,
		// Walls and ground
		ground,
	)

	// Add area light
	s.AddSphereLight(
		core.NewVec3(0, 8, 5),    // position
		2.0,                      // radius
		core.NewVec3(20, 20, 20), // emission (bright white)
	)

	// Add ambient infinite light
	s.AddGradientInfiniteLight(
		core.NewVec3(0.3, 0.4, 0.6), // topColor (subtle blue)
		core.NewVec3(0.2, 0.2, 0.2), // bottomColor (dim gray)
	)

	return s
}
