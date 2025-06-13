package scene

import (
	"fmt"
	"os"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// NewDragonScene creates a scene with the dragon PLY mesh
// If loadMesh is false, creates the scene structure without loading the PLY file
// This is useful for getting scene configuration without the expensive mesh loading
func NewDragonScene(loadMesh bool, cameraOverrides ...renderer.CameraConfig) *Scene {
	// Setup camera for dragon viewing
	cameraConfig := setupDragonCamera(cameraOverrides...)
	camera := renderer.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.5, 0.7, 1.0), // Light blue sky
		BottomColor:    core.NewVec3(1.0, 1.0, 1.0), // White horizon
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: createDragonSamplingConfig(),
		CameraConfig:   cameraConfig,
	}

	// Add lighting
	addDragonLighting(s)

	// Add ground plane
	addDragonGround(s)

	// Load and add dragon mesh only if requested
	if loadMesh {
		addDragonMesh(s)
	} else {
		// Add a placeholder for configuration purposes
		fmt.Println("Dragon scene created without mesh for configuration")
	}

	return s
}

// setupDragonCamera configures the camera for viewing the dragon (based on PBRT scene)
func setupDragonCamera(cameraOverrides ...renderer.CameraConfig) renderer.CameraConfig {
	// Use exact PBRT scene coordinates: LookAt 277 -240 250  0 60 -30 0 0 1
	defaultCameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(277, -240, 250), // Exact PBRT camera position
		LookAt:        core.NewVec3(0, 60, -30),     // Exact PBRT look at point
		Up:            core.NewVec3(0, 0, 1),        // Z-up coordinate system from PBRT
		Width:         900,                          // Match PBRT resolution (900x900)
		AspectRatio:   1.0,                          // PBRT uses 900x900
		VFov:          33.0,                         // FOV from PBRT scene
		Aperture:      0.0,                          // No depth of field
		FocusDistance: 0.0,                          // Auto-calculate focus distance
	}

	// Apply any overrides
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = renderer.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	return cameraConfig
}

// createDragonSamplingConfig creates sampling configuration optimized for complex mesh
func createDragonSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{
		SamplesPerPixel:           200,  // Higher samples for quality
		MaxDepth:                  50,   // Deep bounces for complex geometry
		RussianRouletteMinBounces: 15,   // More bounces before Russian Roulette
		RussianRouletteMinSamples: 8,    // More samples before RR
		AdaptiveMinSamples:        8,    // Higher minimum for complex geometry
		AdaptiveThreshold:         0.01, // Lower threshold for better quality
		AdaptiveDarkThreshold:     1e-6, // Standard threshold for dark pixels
	}
}

// addDragonLighting adds dramatic lighting for the dragon
func addDragonLighting(s *Scene) {
	// Redesign lighting for better shadows and no intrusion in camera view
	// Reduce overall lighting for more dramatic shadows
	// Remember: Z-up coordinate system, camera at (277, -240, 250) looking at (0, 60, -30)

	// Main key light - position away from camera view (higher Z, more positive Y)
	s.AddSphereLight(
		core.NewVec3(350, 200, 300),    // position (right, behind dragon, high up)
		15.0,                           // smaller radius for sharper shadows
		core.NewVec3(15.0, 14.0, 12.0), // reduced intensity
	)

	// Soft fill light - position on opposite side, away from camera
	s.AddSphereLight(
		core.NewVec3(-250, 150, 200), // position (left, behind dragon, elevated)
		25.0,                         // larger radius for soft fill
		core.NewVec3(2.0, 2.5, 3.0),  // much dimmer fill
	)

	// Rim light - well behind dragon and high up
	s.AddSphereLight(
		core.NewVec3(100, 300, 400), // position (behind dragon, very high up)
		10.0,                        // small radius for sharp rim
		core.NewVec3(6.0, 5.0, 4.0), // slightly dimmer rim light
	)
}

// addDragonGround adds a ground plane (matching PBRT scene at Z = -40)
func addDragonGround(s *Scene) {
	groundMaterial := material.NewLambertian(core.NewVec3(0.6, 0.6, 0.6))
	// PBRT ground: Translate 0 0 -40 (exact coordinates)
	groundPlane := geometry.NewPlane(
		core.NewVec3(0, 0, -40), // Ground at Z = -40 exactly like PBRT
		core.NewVec3(0, 0, 1),   // Z-up normal exactly like PBRT
		groundMaterial,
	)
	s.Shapes = append(s.Shapes, groundPlane)
}

// addDragonMesh loads the dragon PLY file and adds it to the scene
func addDragonMesh(s *Scene) {
	// Try multiple possible paths for the dragon PLY file
	// This allows the scene to work from both command line and web server contexts
	possiblePaths := []string{
		"models/dragon_remeshed.ply",    // From project root (command line)
		"../models/dragon_remeshed.ply", // From web/ directory (web server)
	}

	var dragonPath string
	var found bool

	// Find the first path that exists
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			dragonPath = path
			found = true
			break
		}
	}

	if !found {
		fmt.Printf("Warning: Dragon PLY file not found at any of these locations:\n")
		for _, path := range possiblePaths {
			fmt.Printf("  - %s\n", path)
		}
		fmt.Println("Please ensure dragon_remeshed.ply is in the models/ directory")

		// Add a placeholder sphere instead
		dragonMaterial := material.NewMetal(core.NewVec3(0.7, 0.5, 0.2), 0.002) // Darker gold color with very low roughness
		placeholder := geometry.NewSphere(
			core.NewVec3(0, 1, 0), // center
			1.0,                   // radius
			dragonMaterial,
		)
		s.Shapes = append(s.Shapes, placeholder)
		fmt.Println("Added placeholder sphere instead of dragon mesh")
		return
	}

	// Create dragon material - mirror-like gold metal matching PBRT
	// PBRT uses "float roughness" [.002] for very shiny metal
	// Reduce brightness and adjust color to be more realistic
	dragonMaterial := material.NewMetal(core.NewVec3(0.7, 0.5, 0.2), 0.002) // Darker gold color with very low roughness

	// Load the PLY data
	fmt.Printf("Loading dragon mesh from %s...\n", dragonPath)
	plyData, err := loaders.LoadPLY(dragonPath)
	if err != nil {
		fmt.Printf("Error loading dragon PLY data: %v\n", err)
		fmt.Println("Adding placeholder sphere instead")

		// Add placeholder sphere
		placeholder := geometry.NewSphere(
			core.NewVec3(0, 1, 0), // center
			1.0,                   // radius
			dragonMaterial,
		)
		s.Shapes = append(s.Shapes, placeholder)
		return
	}

	// Create triangle mesh with rotation
	// Apply the exact rotation from PBRT scene: "Rotate -53 0 1 0"
	// This means -53 degrees around Y axis (0 1 0)
	rotationY := -53.0 * 3.14159265359 / 180.0 // -53 degrees in radians
	rotation := core.NewVec3(0, rotationY, 0)  // Rotate around Y axis exactly like PBRT
	center := core.NewVec3(0, 0, 0)            // Rotate around origin

	// Create mesh options
	var meshOptions *geometry.TriangleMeshOptions
	if len(plyData.Normals) > 0 {
		meshOptions = &geometry.TriangleMeshOptions{
			Normals:  plyData.Normals,
			Rotation: &rotation,
			Center:   &center,
		}
	} else {
		meshOptions = &geometry.TriangleMeshOptions{
			Rotation: &rotation,
			Center:   &center,
		}
	}

	dragonMesh := geometry.NewTriangleMesh(plyData.Vertices, plyData.Faces, dragonMaterial, meshOptions)

	fmt.Printf("Successfully loaded dragon mesh with %d triangles\n", dragonMesh.GetTriangleCount())

	// Print bounding box for debugging
	bbox := dragonMesh.BoundingBox()
	fmt.Printf("Dragon bounding box: min(%.2f, %.2f, %.2f) max(%.2f, %.2f, %.2f)\n",
		bbox.Min.X, bbox.Min.Y, bbox.Min.Z, bbox.Max.X, bbox.Max.Y, bbox.Max.Z)

	// Calculate mesh size
	size := bbox.Max.Subtract(bbox.Min)
	center = bbox.Min.Add(bbox.Max).Multiply(0.5)
	fmt.Printf("Dragon size: %.2f x %.2f x %.2f, center: (%.2f, %.2f, %.2f)\n",
		size.X, size.Y, size.Z, center.X, center.Y, center.Z)

	s.Shapes = append(s.Shapes, dragonMesh)
}
