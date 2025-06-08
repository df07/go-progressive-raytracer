package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// NewCornellScene creates a classic Cornell box scene with quad walls and area lighting
func NewCornellScene() *Scene {
	config := renderer.CameraConfig{
		Center:        core.NewVec3(278, 278, -800), // Position camera outside the box looking in
		LookAt:        core.NewVec3(278, 278, 0),    // Look at the center of the box
		Up:            core.NewVec3(0, 1, 0),        // Standard up direction
		Width:         400,
		AspectRatio:   1.0,  // Square aspect ratio for Cornell box
		VFov:          40.0, // Field of view
		Aperture:      0.0,  // No depth of field for Cornell box
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	samplingConfig := core.SamplingConfig{
		SamplesPerPixel:           150,
		MaxDepth:                  40,
		RussianRouletteMinBounces: 4, // More aggressive - fewer complex caustics
		RussianRouletteMinSamples: 6, // Fewer samples before RR
	}

	camera := renderer.NewCamera(config)

	// Create the scene
	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.0, 0.0, 0.0), // Black background
		BottomColor:    core.NewVec3(0.0, 0.0, 0.0), // Black background
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: samplingConfig,
	}

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

	// Add ceiling light (smaller quad in the center of the ceiling)
	lightSize := 130.0
	lightOffset := (boxSize - lightSize) / 2.0
	s.AddQuadLight(
		core.NewVec3(lightOffset, boxSize-1, lightOffset), // corner (slightly below ceiling)
		core.NewVec3(lightSize, 0, 0),                     // u vector (X direction)
		core.NewVec3(0, 0, lightSize),                     // v vector (Z direction)
		core.NewVec3(15.0, 15.0, 15.0),                    // bright white emission
	)

	// Add two spheres in the box

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

	return s
}
