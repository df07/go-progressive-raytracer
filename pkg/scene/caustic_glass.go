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

// NewCausticGlassScene creates a scene with glass caustic geometry
// Based on the glass.pbrt scene configuration
// If loadMesh is false, creates the scene structure without loading the PLY files
func NewCausticGlassScene(loadMesh bool, logger core.Logger, cameraOverrides ...renderer.CameraConfig) *Scene {
	// Setup camera based on PBRT scene configuration
	cameraConfig := setupCausticGlassCamera(cameraOverrides...)
	camera := renderer.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.2, 0.2, 0.2), // Dark background like PBRT infinite light
		BottomColor:    core.NewVec3(0.2, 0.2, 0.2), // Consistent dark background
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: createCausticGlassSamplingConfig(),
		CameraConfig:   cameraConfig,
	}

	// Add lighting based on PBRT scene
	addCausticGlassLighting(s)

	// Load and add meshes only if requested
	if loadMesh {
		addCausticGlassMeshes(s, logger)
	} else {
		logger.Printf("Caustic glass scene created without meshes for configuration\n")
	}

	return s
}

// setupCausticGlassCamera configures the camera based on PBRT scene
// LookAt -5.5 7 -5.5, -4.75 2.25 0, 0 1 0
// Camera "perspective" "float fov" [ 30 ]
// Film resolution 1050x1500 with scale 1.5
// PBRT scale parameter affects the effective field of view - scale > 1 means zoom out
func setupCausticGlassCamera(cameraOverrides ...renderer.CameraConfig) renderer.CameraConfig {
	defaultCameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(-5.5, 7, -5.5),  // PBRT camera position
		LookAt:        core.NewVec3(-4.75, 2.25, 0), // PBRT look at point
		Up:            core.NewVec3(0, 1, 0),        // Y-up coordinate system
		Width:         525,                          // PBRT film resolution width/2
		AspectRatio:   525.0 / 750.0,                // PBRT aspect ratio (width/height)
		VFov:          30.0 * 1.5,                   // PBRT FOV * scale (1.5 = zoom out)
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

// createCausticGlassSamplingConfig creates sampling configuration optimized for glass caustics
// Based on PBRT scene using 8192 samples and max depth 20
func createCausticGlassSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{
		SamplesPerPixel:           8192,  // Match PBRT pixelsamples
		MaxDepth:                  20,    // Match PBRT maxdepth
		RussianRouletteMinBounces: 10,    // Conservative for caustics
		RussianRouletteMinSamples: 16,    // More samples before RR for caustics
		AdaptiveMinSamples:        0.2,   // 20% minimum samples for complex caustics
		AdaptiveThreshold:         0.005, // Tighter threshold for caustic quality
	}
}

// addCausticGlassLighting adds lighting based on PBRT scene
// LightSource "spot" "point from" [ 0 5 9 ] "point to" [ -5 2.7500000000 0 ]
// "rgb I" [ 139.8113403320 118.6366500854 105.3887557983 ]
// LightSource "infinite" "rgb L" [ 0.1000000015 0.1000000015 0.1000000015 ]
func addCausticGlassLighting(s *Scene) {
	// Main spot light using exact PBRT parameters
	spotFrom := core.NewVec3(0, 5, 9)
	spotIntensity := core.NewVec3(139.8113403320, 118.6366500854, 105.3887557983)

	// Use disc spot light
	spotTo := core.NewVec3(-5, 2.7500000000, 0)
	s.AddSpotLight(spotFrom, spotTo, spotIntensity, 30.0, 5.0, 0.7)

	// Infinite light is handled by the background colors (already set to 0.1, 0.1, 0.1)
}

// addCausticGlassMeshes loads the PLY files and adds them to the scene
func addCausticGlassMeshes(s *Scene, logger core.Logger) {
	// Try multiple possible paths for the PLY files
	possibleBasePaths := []string{
		"models/caustic-glass/geometry/",    // From project root (command line)
		"../models/caustic-glass/geometry/", // From web/ directory (web server)
	}

	// Mesh 1: Glass material with index 1.25
	// Material "glass" "float index" [ 1.2500000000 ]
	// Shape "plymesh" "string filename" "geometry/mesh_00001.ply"
	addCausticGlassMesh(s, "mesh_00001.ply", "glass", 1.25, possibleBasePaths, logger)

	// Mesh 2: Uber material (rough diffuse with some specular)
	// Material "uber" "float roughness" [ 0.0104080001 ] "float index" [ 1 ]
	// "rgb Kd" [ 0.6399999857 0.6399999857 0.6399999857 ]
	// "rgb Ks" [ 0.1000000015 0.1000000015 0.1000000015 ]
	addCausticGlassMesh(s, "mesh_00002.ply", "uber", 1.0, possibleBasePaths, logger)
}

// addCausticGlassMesh loads a single PLY mesh and adds it to the scene
func addCausticGlassMesh(s *Scene, filename, materialType string, refractiveIndex float64, possibleBasePaths []string, logger core.Logger) {
	var meshPath string
	var found bool

	// Find the first path that exists
	for _, basePath := range possibleBasePaths {
		path := basePath + filename
		if _, err := os.Stat(path); err == nil {
			meshPath = path
			found = true
			break
		}
	}

	if !found {
		logger.Printf("Warning: %s not found at any of these locations:\n", filename)
		for _, basePath := range possibleBasePaths {
			logger.Printf("  - %s\n", basePath+filename)
		}
		return
	}

	// Create material based on type
	var meshMaterial core.Material
	switch materialType {
	case "glass":
		meshMaterial = material.NewDielectric(refractiveIndex)
	case "uber":
		// Approximate PBRT uber material as lambertian (matches rendered appearance better)
		// Use the diffuse component: "rgb Kd" [ 0.6399999857 0.6399999857 0.6399999857 ]
		diffuseColor := core.NewVec3(0.6399999857, 0.6399999857, 0.6399999857)
		meshMaterial = material.NewLambertian(diffuseColor)
	default:
		// Fallback to lambertian
		meshMaterial = material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	}

	// Load the PLY data
	logger.Printf("Loading %s from %s...\n", filename, meshPath)
	plyStart := time.Now()
	plyData, err := loaders.LoadPLY(meshPath)
	plyLoadTime := time.Since(plyStart)
	if err != nil {
		logger.Printf("Error loading %s: %v\n", filename, err)
		return
	}

	logger.Printf("PLY data loaded: %d vertices, %d triangles in %v\n",
		len(plyData.Vertices), len(plyData.Faces)/3, plyLoadTime)

	// Create mesh options (no rotation needed - using PBRT coordinates as-is)
	// Note: PLY normals are per-vertex, but TriangleMesh expects per-triangle normals
	// So we skip the normals and let the mesh calculate them automatically
	var meshOptions *geometry.TriangleMeshOptions

	// Create triangle mesh
	logger.Printf("Creating triangle mesh with %d vertices, %d triangles...\n", len(plyData.Vertices), len(plyData.Faces)/3)
	meshStart := time.Now()
	mesh := geometry.NewTriangleMesh(plyData.Vertices, plyData.Faces, meshMaterial, meshOptions)
	logger.Printf("Triangle mesh created in %v\n", time.Since(meshStart))

	logger.Printf("Successfully loaded %s with %d triangles\n", filename, mesh.GetTriangleCount())

	s.Shapes = append(s.Shapes, mesh)
}
