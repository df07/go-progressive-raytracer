package scene

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// CornellGeometryType represents the type of geometry to use in the Cornell box
type CornellGeometryType int

const (
	CornellSpheres CornellGeometryType = iota
	CornellBoxes
)

// NewCornellScene creates a classic Cornell box scene with quad walls and area lighting
func NewCornellScene(geometryType CornellGeometryType, cameraOverrides ...renderer.CameraConfig) *Scene {
	// Setup camera and basic scene configuration
	cameraConfig := setupCornellCamera(cameraOverrides...)
	camera := renderer.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.0, 0.0, 0.0), // Black background
		BottomColor:    core.NewVec3(0.0, 0.0, 0.0), // Black background
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: createCornellSamplingConfig(),
		CameraConfig:   cameraConfig,
	}

	// Add the Cornell box walls
	addCornellWalls(s)

	// Add ceiling light
	addCornellLight(s)

	// Add geometry based on type
	addCornellGeometry(s, geometryType)

	return s
}

// setupCornellCamera configures the camera for the Cornell box scene
func setupCornellCamera(cameraOverrides ...renderer.CameraConfig) renderer.CameraConfig {
	defaultCameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(278, 278, -800), // Position camera outside the box looking in
		LookAt:        core.NewVec3(278, 278, 0),    // Look at the center of the box
		Up:            core.NewVec3(0, 1, 0),        // Standard up direction
		Width:         400,
		AspectRatio:   1.0,  // Square aspect ratio for Cornell box
		VFov:          40.0, // Field of view
		Aperture:      0.0,  // No depth of field for Cornell box
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = renderer.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	return cameraConfig
}

// createCornellSamplingConfig creates the sampling configuration for Cornell box scenes
func createCornellSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{
		SamplesPerPixel:           150,
		MaxDepth:                  40,
		RussianRouletteMinBounces: 16,   // Need a lot of bounces for indirect lighting
		RussianRouletteMinSamples: 8,    // Fewer samples before RR
		AdaptiveMinSamples:        8,    // Lower minimum for simpler scene
		AdaptiveThreshold:         0.01, // Slightly higher threshold (1%) for faster convergence
		AdaptiveDarkThreshold:     1e-6, // Same absolute threshold for dark pixels
	}
}

// addCornellWalls adds the six walls of the Cornell box to the scene
func addCornellWalls(s *Scene) {
	// Create materials
	white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
	red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
	green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

	// Cornell box dimensions (standard 555x555x555 units)
	boxSize := 555.0

	// Create the six walls of the Cornell box using quads

	// Floor (white) - XZ plane at y=0
	floor := geometry.NewQuad(
		core.NewVec3(0, 0, 0),       // corner
		core.NewVec3(boxSize, 0, 0), // u vector (X direction)
		core.NewVec3(0, 0, boxSize), // v vector (Z direction)
		white,
	)

	// Ceiling (white) - XZ plane at y=boxSize
	ceiling := geometry.NewQuad(
		core.NewVec3(0, boxSize, 0), // corner
		core.NewVec3(boxSize, 0, 0), // u vector (X direction)
		core.NewVec3(0, 0, boxSize), // v vector (Z direction)
		white,
	)

	// Back wall (white) - XY plane at z=boxSize
	backWall := geometry.NewQuad(
		core.NewVec3(0, 0, boxSize), // corner
		core.NewVec3(boxSize, 0, 0), // u vector (X direction)
		core.NewVec3(0, boxSize, 0), // v vector (Y direction)
		white,
	)

	// Left wall (red) - YZ plane at x=0
	leftWall := geometry.NewQuad(
		core.NewVec3(0, 0, 0),       // corner
		core.NewVec3(0, 0, boxSize), // u vector (Z direction)
		core.NewVec3(0, boxSize, 0), // v vector (Y direction)
		red,
	)

	// Right wall (green) - YZ plane at x=boxSize
	rightWall := geometry.NewQuad(
		core.NewVec3(boxSize, 0, 0), // corner
		core.NewVec3(0, boxSize, 0), // u vector (Y direction)
		core.NewVec3(0, 0, boxSize), // v vector (Z direction)
		green,
	)

	// Add walls to scene
	s.Shapes = append(s.Shapes, floor, ceiling, backWall, leftWall, rightWall)
}

// addCornellLight adds the ceiling light to the Cornell box scene
func addCornellLight(s *Scene) {
	boxSize := 555.0
	lightSize := 130.0
	lightOffset := (boxSize - lightSize) / 2.0

	s.AddQuadLight(
		core.NewVec3(lightOffset, boxSize-1, lightOffset), // corner (slightly below ceiling)
		core.NewVec3(lightSize, 0, 0),                     // u vector (X direction)
		core.NewVec3(0, 0, lightSize),                     // v vector (Z direction)
		core.NewVec3(15.0, 15.0, 15.0),                    // bright white emission
	)
}

// addCornellGeometry adds the specified geometry type to the Cornell box scene
func addCornellGeometry(s *Scene, geometryType CornellGeometryType) {
	switch geometryType {
	case CornellSpheres:
		addCornellSpheres(s)
	case CornellBoxes:
		addCornellBoxes(s)
	}
}

// addCornellSpheres adds two spheres to the Cornell box scene
func addCornellSpheres(s *Scene) {
	// Left sphere (smaller, metallic)
	leftSphere := geometry.NewSphere(
		core.NewVec3(185, 82.5, 169), // position
		82.5,                         // radius
		material.NewMetal(core.NewVec3(0.8, 0.8, 0.9), 0.0), // shiny metal
	)

	// Right sphere (larger, glass)
	rightSphere := geometry.NewSphere(
		core.NewVec3(370, 90, 351),  // position
		90,                          // radius
		material.NewDielectric(1.5), // glass
	)

	// Add spheres to scene
	s.Shapes = append(s.Shapes, leftSphere, rightSphere)
}

// addCornellBoxes adds two boxes to the Cornell box scene (matching Ray Tracing in One Weekend tutorial)
func addCornellBoxes(s *Scene) {
	white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))

	// Box 1: 165×330×165, rotated 15°, translated to (265,0,295)
	// Calculate center position: translation + half the box size
	box1Center := core.NewVec3(265+82.5, 0+165, 295+82.5) // (347.5, 165, 377.5)
	box1 := geometry.NewBox(
		box1Center,                                          // center position
		core.NewVec3(82.5, 165, 82.5),                       // size (half-extents: 165/2, 330/2, 165/2)
		core.NewVec3(0, 15*math.Pi/180, 0),                  // rotation (15 degrees around Y axis)
		material.NewMetal(core.NewVec3(0.8, 0.8, 0.9), 0.0), // shiny metal
	)

	// Box 2: 165×165×165, rotated -18°, translated to (130,0,65)
	// Calculate center position: translation + half the box size
	box2Center := core.NewVec3(130+82.5, 0+82.5, 65+82.5) // (212.5, 82.5, 147.5)
	box2 := geometry.NewBox(
		box2Center,                          // center position
		core.NewVec3(82.5, 82.5, 82.5),      // size (half-extents: 165/2, 165/2, 165/2)
		core.NewVec3(0, -18*math.Pi/180, 0), // rotation (-18 degrees around Y axis)
		white,                               // white lambertian material
	)

	// Add boxes to scene
	s.Shapes = append(s.Shapes, box1, box2)
}
