package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// CornellGeometryType represents the type of geometry to use in the Cornell box
type CornellGeometryType int

const (
	CornellSpheres CornellGeometryType = iota
	CornellBoxes
	CornellEmpty
)

// NewCornellScene creates a classic Cornell box scene with quad walls and area lighting
func NewCornellScene(geometryType CornellGeometryType, cameraOverrides ...geometry.CameraConfig) *Scene {
	// Setup camera and basic scene configuration
	cameraConfig := setupCornellCamera(cameraOverrides...)
	camera := geometry.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]geometry.Shape, 0),
		Lights:         make([]lights.Light, 0),
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
func setupCornellCamera(cameraOverrides ...geometry.CameraConfig) geometry.CameraConfig {
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(278, 278, -800), // Centered camera position
		LookAt:        core.NewVec3(278, 278, 0),    // Look at the center of the box
		Up:            core.NewVec3(0, 1, 0),        // Standard up direction
		Width:         400,
		AspectRatio:   1.0,  // Square aspect ratio for Cornell box
		VFov:          40.0, // Official Cornell field of view
		Aperture:      0.0,  // No depth of field for Cornell box
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	return cameraConfig
}

// createCornellSamplingConfig creates the sampling configuration for Cornell box scenes
func createCornellSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{
		SamplesPerPixel:           150,
		MaxDepth:                  5,
		RussianRouletteMinBounces: 16,    // Need a lot of bounces for indirect lighting
		AdaptiveMinSamples:        0.20,  // 20% of max samples minimum to avoid black pixels on front box
		AdaptiveThreshold:         0.005, // Lower threshold (0.5%)
	}
}

// addCornellWalls adds the six walls of the Cornell box to the scene
func addCornellWalls(s *Scene) {
	// Create materials
	white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
	red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
	green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

	// Cornell box dimensions from official data (slightly irregular, not perfect 555x555x555)
	// Using the actual measured dimensions from Cornell

	// Floor (white) - XZ plane at y=0
	// Extended to meet all walls properly
	floor := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 0.0), // corner
		core.NewVec3(556, 0.0, 0.0), // u vector (X direction) - extended to match other walls
		core.NewVec3(0.0, 0.0, 556), // v vector (Z direction)
		white,
	)

	// Ceiling (white) - XZ plane at y=548.8
	// From Cornell data: 556.0 548.8 0.0, 556.0 548.8 559.2, 0.0 548.8 559.2, 0.0 548.8 0.0
	ceiling := geometry.NewQuad(
		core.NewVec3(0.0, 556, 0.0), // corner
		core.NewVec3(556, 0.0, 0.0), // u vector (X direction)
		core.NewVec3(0.0, 0.0, 556), // v vector (Z direction)
		white,
	)

	// Back wall (white) - XY plane at z=559.2
	// Extended to meet the left wall properly
	backWall := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 556),   // corner
		core.NewVec3(556.0, 0.0, 0.0), // u vector (X direction) - extended to match ceiling
		core.NewVec3(0.0, 556, 0.0),   // v vector (Y direction)
		white,
	)

	// Left wall (red) - YZ plane at x=0
	// From Cornell data: 0.0 0.0 559.2, 0.0 0.0 0.0, 0.0 548.8 0.0, 0.0 548.8 559.2
	leftWall := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 0.0), // corner
		core.NewVec3(0.0, 0.0, 556), // u vector (Z direction)
		core.NewVec3(0.0, 556, 0.0), // v vector (Y direction)
		red,
	)

	// Right wall (green) - YZ plane at x=556.0
	// Simplified to use consistent X coordinate
	rightWall := geometry.NewQuad(
		core.NewVec3(556, 0.0, 0.0), // corner
		core.NewVec3(0.0, 0.0, 556), // u vector (Z direction)
		core.NewVec3(0.0, 556, 0.0), // v vector (Y direction)
		green,
	)

	// Add walls to scene
	s.Shapes = append(s.Shapes, floor, ceiling, backWall, leftWall, rightWall)
}

// addCornellLight adds the ceiling light to the Cornell box scene
func addCornellLight(s *Scene) {
	// Cornell box light specifications from official data
	// Light position: 343.0 548.8 227.0 to 213.0 548.8 332.0
	// This gives us a 130x105 light (343-213=130, 332-227=105)
	lightCorner := core.NewVec3(213.0, 556-0.001, 227.0) // Slightly below ceiling
	lightU := core.NewVec3(130.0, 0, 0)                  // U vector (X direction)
	lightV := core.NewVec3(0, 0, 105.0)                  // V vector (Z direction)

	// Warmer, more yellowish light based on Cornell emission spectrum
	// The spectrum shows higher values at longer wavelengths (more yellow/orange)
	lightEmission := core.NewVec3(18.0, 15.0, 8.0).Multiply(2.5) // Warm yellowish white

	s.AddQuadLight(lightCorner, lightU, lightV, lightEmission)
}

// addCornellGeometry adds the specified geometry type to the Cornell box scene
func addCornellGeometry(s *Scene, geometryType CornellGeometryType) {
	switch geometryType {
	case CornellSpheres:
		addCornellSpheres(s)
	case CornellBoxes:
		addCornellBoxes(s)
	case CornellEmpty:
		// No geometry added - just the empty Cornell box
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

// addCornellBoxes adds two boxes to the Cornell box scene (custom configuration)
func addCornellBoxes(s *Scene) {
	white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
	// Mirror material for the tall block - highly reflective surface
	mirror := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0) // Very shiny mirror

	// Custom configuration: tall mirrored box on left, short white box on right
	// This should show the red wall reflection in the mirrored surface

	// Short box (white, diffuse) - positioned on the RIGHT side
	shortBoxCenter := core.NewVec3(370.0, 82.5, 169.0) // Right side, front
	shortBox := geometry.NewBox(
		shortBoxCenter,                     // center position
		core.NewVec3(82.5, 82.5, 82.5),     // size (half-extents: 165/2 for each dimension)
		core.NewVec3(0, 18*math.Pi/180, 0), // rotation (18 degrees around Y axis)
		white,                              // white lambertian material
	)

	// Tall box (mirrored) - positioned on the LEFT side
	tallBoxCenter := core.NewVec3(185.0, 165.0, 351.0) // Left side, back
	tallBox := geometry.NewBox(
		tallBoxCenter,                       // center position
		core.NewVec3(82.5, 165.0, 82.5),     // size (half-extents: 165/2, 330/2, 165/2)
		core.NewVec3(0, -20*math.Pi/180, 0), // rotation (-15 degrees) - angled to catch red wall reflection
		mirror,                              // mirror material
	)

	// Add boxes to scene
	s.Shapes = append(s.Shapes, shortBox, tallBox)
}
