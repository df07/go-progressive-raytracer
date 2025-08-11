package scene

import (
	"os"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// NewDragonScene creates a scene with the dragon PLY mesh
// If loadMesh is false, creates the scene structure without loading the PLY file
// This is useful for getting scene configuration without the expensive mesh loading
func NewDragonScene(loadMesh bool, materialFinish string, logger core.Logger, cameraOverrides ...renderer.CameraConfig) *Scene {
	// Setup camera for dragon viewing
	cameraConfig := setupDragonCamera(cameraOverrides...)
	camera := renderer.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
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
		addDragonMesh(s, materialFinish, logger)
	} else {
		// Add a placeholder for configuration purposes
		logger.Printf("Dragon scene created without mesh for configuration\n")
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
		AdaptiveMinSamples:        0.15, // 15% of max samples minimum for complex geometry
		AdaptiveThreshold:         0.01, // Lower threshold for better quality
	}
}

// addDragonLighting adds dramatic lighting for the dragon
func addDragonLighting(s *Scene) {
	// Redesign lighting for better shadows and no intrusion in camera view
	// Reduce overall lighting for more dramatic shadows
	// Remember: Z-up coordinate system, camera at (277, -240, 250) looking at (0, 60, -30)

	// Main key light - position away from camera view (higher Z, more positive Y)
	s.AddSphereLight(
		core.NewVec3(0, 200, 800), // position (right, behind dragon, high up)
		300.0,                     // smaller radius for sharper shadows
		core.NewVec3(15.0, 14.0, 12.0).Multiply(0.25), // reduced intensity
	)

	// Add gradient infinite light (replaces background gradient)
	s.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (light blue sky)
		core.NewVec3(0.0, 0.0, 0.0), // bottomColor (dark horizon)
	)
}

// addDragonGround adds a ground plane (matching PBRT scene at Z = -40)
func addDragonGround(s *Scene) {
	groundMaterial := material.NewLambertian(core.NewVec3(0.6, 0.6, 0.6))
	// Create ground quad at Z = -40 to match PBRT dragon scene
	// Note: This uses Z-up coordinate system (normal = (0,0,1))
	corner := core.NewVec3(-5000, -5000, -40)
	u := core.NewVec3(10000, 0, 0) // X direction
	v := core.NewVec3(0, 10000, 0) // Y direction
	// u × v = (10000,0,0) × (0,10000,0) = (0,0,10000²) which normalizes to (0,0,1)
	groundQuad := geometry.NewQuad(corner, u, v, groundMaterial)
	s.Shapes = append(s.Shapes, groundQuad)
}

// addDragonMesh loads the dragon PLY file and adds it to the scene
func addDragonMesh(s *Scene, materialFinish string, logger core.Logger) {
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
		logger.Printf("Warning: Dragon PLY file not found at any of these locations:\n")
		for _, path := range possiblePaths {
			logger.Printf("  - %s\n", path)
		}
		return
	}

	// Create dragon material based on finish type
	var dragonMaterial core.Material
	switch materialFinish {
	case "plastic":
		// Light purple shiny plastic - layered material with dielectric outer and lambertian inner
		purpleColor := core.NewVec3(0.7, 0.5, 0.8) // Light purple
		inner := material.NewLambertian(purpleColor)
		outerDielectric := material.NewDielectric(1.4) // Plastic-like refractive index
		dragonMaterial = material.NewLayered(outerDielectric, inner)
	case "matte":
		// Unfired pottery/ceramic lambertian material
		dragonMaterial = material.NewLambertian(core.NewVec3(0.75, 0.65, 0.55))
	case "mirror":
		// Perfect mirror - zero roughness metal with neutral color
		mirrorColor := core.NewVec3(0.9, 0.9, 0.9) // Slightly tinted white
		dragonMaterial = material.NewMetal(mirrorColor, 0.0)
	case "glass":
		// Clear glass dielectric
		dragonMaterial = material.NewDielectric(1.5) // Glass refractive index
	case "copper":
		// Copper metal with slight roughness
		copperColor := core.NewVec3(0.8, 0.4, 0.2) // Copper color
		dragonMaterial = material.NewMetal(copperColor, 0.1)
	default: // "gold" or any other value
		// Default: mirror-like gold metal matching PBRT
		// PBRT uses "float roughness" [.002] for very shiny metal
		dragonMaterial = material.NewMetal(core.NewVec3(0.7, 0.5, 0.2), 0.02) // Darker gold color with very low roughness
	}

	// Load the PLY data
	logger.Printf("Loading dragon mesh from %s...\n", dragonPath)
	plyStart := time.Now()
	plyData, err := loaders.LoadPLY(dragonPath)
	plyLoadTime := time.Since(plyStart)
	if err != nil {
		logger.Printf("Error loading dragon PLY data: %v\n", err)
		logger.Printf("Adding placeholder sphere instead\n")

		// Add placeholder sphere
		placeholder := geometry.NewSphere(
			core.NewVec3(0, 1, 0), // center
			1.0,                   // radius
			dragonMaterial,
		)
		s.Shapes = append(s.Shapes, placeholder)
		return
	}

	logger.Printf("PLY data loaded: %d vertices, %d triangles in %v\n",
		len(plyData.Vertices), len(plyData.Faces)/3, plyLoadTime)

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

	// Create triangle mesh with timing
	logger.Printf("Creating triangle mesh with %d vertices, %d triangles...\n", len(plyData.Vertices), len(plyData.Faces)/3)
	meshStart := time.Now()
	dragonMesh := geometry.NewTriangleMesh(plyData.Vertices, plyData.Faces, dragonMaterial, meshOptions)
	logger.Printf("Triangle mesh created in %v\n", time.Since(meshStart))

	logger.Printf("Successfully loaded dragon mesh with %d triangles\n", dragonMesh.GetTriangleCount())

	s.Shapes = append(s.Shapes, dragonMesh)
}
