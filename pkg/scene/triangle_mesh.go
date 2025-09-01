package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// TriangleMeshGeometryType represents the type of geometry to use in the triangle mesh scene
type TriangleMeshGeometryType int

const (
	TriangleMeshSphere TriangleMeshGeometryType = iota
)

// NewTriangleMeshScene creates a scene showcasing triangle mesh geometry
func NewTriangleMeshScene(complexity int, cameraOverrides ...geometry.CameraConfig) *Scene {
	return NewTriangleMeshSceneWithComplexity(complexity, cameraOverrides...)
}

// NewTriangleMeshSceneWithComplexity creates a scene with configurable sphere complexity
func NewTriangleMeshSceneWithComplexity(complexity int, cameraOverrides ...geometry.CameraConfig) *Scene {
	// Setup camera and basic scene configuration
	cameraConfig := setupTriangleMeshCamera(cameraOverrides...)
	camera := geometry.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]geometry.Shape, 0),
		Lights:         make([]lights.Light, 0),
		SamplingConfig: createTriangleMeshSamplingConfig(),
		CameraConfig:   cameraConfig,
	}

	// Add lighting
	addTriangleMeshLighting(s)

	// Add ground plane
	addTriangleMeshGround(s)

	// Add sphere comparison geometry
	addSphereTriangleMeshGeometryWithComplexity(s, complexity)

	return s
}

// setupTriangleMeshCamera configures the camera for the triangle mesh scene
func setupTriangleMeshCamera(cameraOverrides ...geometry.CameraConfig) geometry.CameraConfig {
	defaultCameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 2, 6), // Position camera to see the meshes
		LookAt:        core.NewVec3(0, 1, 0), // Look at the center of the scene
		Up:            core.NewVec3(0, 1, 0), // Standard up direction
		Width:         600,
		AspectRatio:   16.0 / 9.0,
		VFov:          45.0, // Good field of view for showcasing
		Aperture:      0.02, // Slight depth of field
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	return cameraConfig
}

// createTriangleMeshSamplingConfig creates the sampling configuration for triangle mesh scenes
func createTriangleMeshSamplingConfig() SamplingConfig {
	return SamplingConfig{
		SamplesPerPixel:           150,
		MaxDepth:                  40,
		RussianRouletteMinBounces: 10,    // Good balance for triangle mesh scenes
		AdaptiveMinSamples:        0.10,  // 10% of max samples minimum for efficient rendering
		AdaptiveThreshold:         0.015, // Slightly higher threshold for faster convergence
	}
}

// addTriangleMeshLighting adds symmetrical lighting to the scene
func addTriangleMeshLighting(s *Scene) {
	// Main overhead light - centered above both spheres
	s.AddSphereLight(
		core.NewVec3(0, 6, 0),          // position (centered above spheres)
		1.5,                            // radius
		core.NewVec3(15.0, 15.0, 15.0), // bright white emission
	)

	// Left fill light - symmetrical to right
	s.AddSphereLight(
		core.NewVec3(-4, 4, 3),      // position (left side)
		0.8,                         // radius
		core.NewVec3(8.0, 8.0, 8.0), // neutral white emission
	)

	// Right fill light - symmetrical to left
	s.AddSphereLight(
		core.NewVec3(4, 4, 3),       // position (right side, mirrored)
		0.8,                         // radius
		core.NewVec3(8.0, 8.0, 8.0), // neutral white emission (same as left)
	)

	// Add gradient infinite light (replaces background gradient)
	s.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (light blue)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white)
	)
}

// addTriangleMeshGround adds a ground plane to the scene
func addTriangleMeshGround(s *Scene) {
	groundMaterial := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	groundQuad := NewGroundQuad(
		core.NewVec3(0, 0, 0),
		10000.0,
		groundMaterial,
	)
	s.Shapes = append(s.Shapes, groundQuad)
}

// addSphereTriangleMeshGeometryWithComplexity adds triangle mesh and regular spheres for comparison
func addSphereTriangleMeshGeometryWithComplexity(s *Scene, complexity int) {
	// Create material - use a nice gold metal that shows off the mesh structure
	goldMetal := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.05)

	// Calculate latitude subdivisions proportionally (3/4 of longitude for good sphere proportions)
	latitudeSubdivisions := (complexity * 3) / 4
	if latitudeSubdivisions < 3 {
		latitudeSubdivisions = 3
	}

	// Triangle mesh sphere on the left
	triangleMeshSphere := createSphereMesh(
		core.NewVec3(-1.5, 1, 0), // center (left side, elevated above ground)
		1.0,                      // radius
		complexity,               // longitude subdivisions
		latitudeSubdivisions,     // latitude subdivisions
		goldMetal,
	)
	s.Shapes = append(s.Shapes, triangleMeshSphere)

	// Regular sphere on the right for comparison
	regularSphere := geometry.NewSphere(
		core.NewVec3(1.5, 1, 0), // center (right side, elevated above ground)
		1.0,                     // radius (same as triangle mesh)
		goldMetal,               // same material
	)
	s.Shapes = append(s.Shapes, regularSphere)
}

// createSphereMesh creates a triangle mesh representing a sphere using UV sphere generation
// center: center point of the sphere
// radius: radius of the sphere
// longitudeSubdivisions: number of subdivisions around the sphere (longitude)
// latitudeSubdivisions: number of subdivisions from pole to pole (latitude)
// material: material for the sphere
func createSphereMesh(center core.Vec3, radius float64, longitudeSubdivisions, latitudeSubdivisions int, material material.Material) *geometry.TriangleMesh {
	vertices := make([]core.Vec3, 0)
	faces := make([]int, 0)

	// Generate vertices using spherical coordinates
	for lat := 0; lat <= latitudeSubdivisions; lat++ {
		// Latitude angle from 0 (north pole) to π (south pole)
		theta := float64(lat) * math.Pi / float64(latitudeSubdivisions)
		sinTheta := math.Sin(theta)
		cosTheta := math.Cos(theta)

		for lon := 0; lon <= longitudeSubdivisions; lon++ {
			// Longitude angle from 0 to 2π
			phi := float64(lon) * 2.0 * math.Pi / float64(longitudeSubdivisions)
			sinPhi := math.Sin(phi)
			cosPhi := math.Cos(phi)

			// Convert spherical to cartesian coordinates
			x := radius * sinTheta * cosPhi
			y := radius * cosTheta
			z := radius * sinTheta * sinPhi

			vertex := center.Add(core.NewVec3(x, y, z))
			vertices = append(vertices, vertex)
		}
	}

	// Generate faces (triangles)
	for lat := 0; lat < latitudeSubdivisions; lat++ {
		for lon := 0; lon < longitudeSubdivisions; lon++ {
			// Calculate vertex indices for the current quad
			current := lat*(longitudeSubdivisions+1) + lon
			next := current + longitudeSubdivisions + 1

			// Create two triangles for each quad
			// Triangle 1: current, next, current+1
			faces = append(faces, current, next, current+1)
			// Triangle 2: current+1, next, next+1
			faces = append(faces, current+1, next, next+1)
		}
	}

	return geometry.NewTriangleMesh(vertices, faces, material, nil)
}
