package scene

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// TriangleMeshGeometryType represents the type of geometry to use in the triangle mesh scene
type TriangleMeshGeometryType int

const (
	TriangleMeshBasic TriangleMeshGeometryType = iota
)

// NewTriangleMeshScene creates a scene showcasing triangle mesh geometry
func NewTriangleMeshScene(geometryType TriangleMeshGeometryType, cameraOverrides ...renderer.CameraConfig) *Scene {
	// Setup camera and basic scene configuration
	cameraConfig := setupTriangleMeshCamera(cameraOverrides...)
	camera := renderer.NewCamera(cameraConfig)

	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.5, 0.7, 1.0), // Light blue
		BottomColor:    core.NewVec3(1.0, 1.0, 1.0), // White
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: createTriangleMeshSamplingConfig(),
		CameraConfig:   cameraConfig,
	}

	// Add lighting
	addTriangleMeshLighting(s)

	// Add ground plane
	addTriangleMeshGround(s)

	// Add geometry based on type
	addTriangleMeshGeometry(s, geometryType)

	return s
}

// setupTriangleMeshCamera configures the camera for the triangle mesh scene
func setupTriangleMeshCamera(cameraOverrides ...renderer.CameraConfig) renderer.CameraConfig {
	defaultCameraConfig := renderer.CameraConfig{
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
		cameraConfig = renderer.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	return cameraConfig
}

// createTriangleMeshSamplingConfig creates the sampling configuration for triangle mesh scenes
func createTriangleMeshSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{
		SamplesPerPixel:           150,
		MaxDepth:                  40,
		RussianRouletteMinBounces: 10,    // Good balance for triangle mesh scenes
		RussianRouletteMinSamples: 6,     // Fewer samples before RR
		AdaptiveMinSamples:        6,     // Lower minimum for efficient rendering
		AdaptiveThreshold:         0.015, // Slightly higher threshold for faster convergence
		AdaptiveDarkThreshold:     1e-6,  // Standard threshold for dark pixels
	}
}

// addTriangleMeshLighting adds lighting to the scene
func addTriangleMeshLighting(s *Scene) {
	// Main overhead light
	s.AddSphereLight(
		core.NewVec3(2, 6, 3),          // position
		1.5,                            // radius
		core.NewVec3(12.0, 11.0, 10.0), // warm white emission
	)

	// Secondary fill light
	s.AddSphereLight(
		core.NewVec3(-3, 4, 2),      // position
		0.8,                         // radius
		core.NewVec3(6.0, 7.0, 8.0), // cool blue emission
	)
}

// addTriangleMeshGround adds a ground plane to the scene
func addTriangleMeshGround(s *Scene) {
	groundMaterial := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	groundPlane := geometry.NewPlane(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 1, 0),
		groundMaterial,
	)
	s.Shapes = append(s.Shapes, groundPlane)
}

// addTriangleMeshGeometry adds the specified geometry type to the scene
func addTriangleMeshGeometry(s *Scene, geometryType TriangleMeshGeometryType) {
	switch geometryType {
	case TriangleMeshBasic:
		addBasicTriangleMeshGeometry(s)
	default:
		addBasicTriangleMeshGeometry(s)
	}
}

// addBasicTriangleMeshGeometry adds simple triangle mesh objects
func addBasicTriangleMeshGeometry(s *Scene) {
	// Create materials
	redMetal := material.NewMetal(core.NewVec3(0.8, 0.2, 0.2), 0.1)
	blueLambertian := material.NewLambertian(core.NewVec3(0.2, 0.3, 0.8))
	goldMetal := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.05) // Changed from glass to gold

	// Simple triangle mesh box - rotated to show multiple faces
	boxMesh := createBoxMesh(
		core.NewVec3(-2, 0.5, 0),      // center (sitting on ground)
		core.NewVec3(1, 1, 1),         // size
		core.NewVec3(0, math.Pi/6, 0), // rotation (30° around Y-axis)
		redMetal,
	)
	s.Shapes = append(s.Shapes, boxMesh)

	// Triangle mesh pyramid - rotated around Y-axis to show it's actually a pyramid
	pyramidMesh := createPyramidMesh(
		core.NewVec3(0, 1, 0),         // center
		1.5,                           // base size
		2.0,                           // height
		core.NewVec3(0, math.Pi/4, 0), // rotation (45° around Y-axis only)
		blueLambertian,
	)
	s.Shapes = append(s.Shapes, pyramidMesh)

	// Triangle mesh icosahedron - rotated to show its complex geometry
	icosahedronMesh := createIcosahedronMesh(
		core.NewVec3(2, 0.8, 0),       // center (sitting on ground)
		0.8,                           // radius
		core.NewVec3(0, math.Pi/3, 0), // rotation (60° around Y-axis)
		goldMetal,                     // Changed to gold metal
	)
	s.Shapes = append(s.Shapes, icosahedronMesh)
}

// Helper functions for creating triangle meshes (moved from geometry package)

// createBoxMesh creates a triangle mesh representing a box
func createBoxMesh(center, size core.Vec3, rotation core.Vec3, material core.Material) *geometry.TriangleMesh {
	// Calculate the 8 corners of the box
	halfSize := size.Multiply(0.5)
	vertices := []core.Vec3{
		center.Add(core.NewVec3(-halfSize.X, -halfSize.Y, -halfSize.Z)), // 0: left-bottom-back
		center.Add(core.NewVec3(+halfSize.X, -halfSize.Y, -halfSize.Z)), // 1: right-bottom-back
		center.Add(core.NewVec3(+halfSize.X, +halfSize.Y, -halfSize.Z)), // 2: right-top-back
		center.Add(core.NewVec3(-halfSize.X, +halfSize.Y, -halfSize.Z)), // 3: left-top-back
		center.Add(core.NewVec3(-halfSize.X, -halfSize.Y, +halfSize.Z)), // 4: left-bottom-front
		center.Add(core.NewVec3(+halfSize.X, -halfSize.Y, +halfSize.Z)), // 5: right-bottom-front
		center.Add(core.NewVec3(+halfSize.X, +halfSize.Y, +halfSize.Z)), // 6: right-top-front
		center.Add(core.NewVec3(-halfSize.X, +halfSize.Y, +halfSize.Z)), // 7: left-top-front
	}

	// Define the 12 triangles (2 per face, 6 faces)
	faces := []int{
		// Back face (Z-)
		0, 1, 2, 0, 2, 3,
		// Front face (Z+)
		4, 6, 5, 4, 7, 6,
		// Left face (X-)
		0, 3, 7, 0, 7, 4,
		// Right face (X+)
		1, 5, 6, 1, 6, 2,
		// Bottom face (Y-)
		0, 4, 5, 0, 5, 1,
		// Top face (Y+)
		3, 2, 6, 3, 6, 7,
	}

	if rotation.X == 0 && rotation.Y == 0 && rotation.Z == 0 {
		return geometry.NewTriangleMesh(vertices, faces, material)
	}
	return geometry.NewTriangleMeshWithRotation(vertices, faces, material, center, rotation)
}

// createPyramidMesh creates a triangle mesh representing a pyramid
func createPyramidMesh(center core.Vec3, baseSize, height float64, rotation core.Vec3, material core.Material) *geometry.TriangleMesh {
	halfBase := baseSize * 0.5
	halfHeight := height * 0.5

	vertices := []core.Vec3{
		// Base vertices (Y = center.Y - halfHeight)
		center.Add(core.NewVec3(-halfBase, -halfHeight, -halfBase)), // 0: left-back
		center.Add(core.NewVec3(+halfBase, -halfHeight, -halfBase)), // 1: right-back
		center.Add(core.NewVec3(+halfBase, -halfHeight, +halfBase)), // 2: right-front
		center.Add(core.NewVec3(-halfBase, -halfHeight, +halfBase)), // 3: left-front
		// Apex (Y = center.Y + halfHeight)
		center.Add(core.NewVec3(0, +halfHeight, 0)), // 4: apex
	}

	faces := []int{
		// Base (2 triangles)
		0, 2, 1, 0, 3, 2,
		// Side faces
		0, 1, 4, // back face
		1, 2, 4, // right face
		2, 3, 4, // front face
		3, 0, 4, // left face
	}

	if rotation.X == 0 && rotation.Y == 0 && rotation.Z == 0 {
		return geometry.NewTriangleMesh(vertices, faces, material)
	}
	return geometry.NewTriangleMeshWithRotation(vertices, faces, material, center, rotation)
}

// createIcosahedronMesh creates a triangle mesh representing an icosahedron (20-sided polyhedron)
func createIcosahedronMesh(center core.Vec3, radius float64, rotation core.Vec3, material core.Material) *geometry.TriangleMesh {
	// Golden ratio
	phi := (1.0 + 1.618033988749895) / 2.0 // (1 + sqrt(5)) / 2

	// Scale factor to achieve desired radius
	scale := radius / 1.618033988749895 // radius / phi

	// 12 vertices of icosahedron
	vertices := []core.Vec3{
		center.Add(core.NewVec3(-1, phi, 0).Multiply(scale)),  // 0
		center.Add(core.NewVec3(1, phi, 0).Multiply(scale)),   // 1
		center.Add(core.NewVec3(-1, -phi, 0).Multiply(scale)), // 2
		center.Add(core.NewVec3(1, -phi, 0).Multiply(scale)),  // 3
		center.Add(core.NewVec3(0, -1, phi).Multiply(scale)),  // 4
		center.Add(core.NewVec3(0, 1, phi).Multiply(scale)),   // 5
		center.Add(core.NewVec3(0, -1, -phi).Multiply(scale)), // 6
		center.Add(core.NewVec3(0, 1, -phi).Multiply(scale)),  // 7
		center.Add(core.NewVec3(phi, 0, -1).Multiply(scale)),  // 8
		center.Add(core.NewVec3(phi, 0, 1).Multiply(scale)),   // 9
		center.Add(core.NewVec3(-phi, 0, -1).Multiply(scale)), // 10
		center.Add(core.NewVec3(-phi, 0, 1).Multiply(scale)),  // 11
	}

	// 20 triangular faces
	faces := []int{
		// 5 faces around point 0
		0, 11, 5, 0, 5, 1, 0, 1, 7, 0, 7, 10, 0, 10, 11,
		// 5 adjacent faces
		1, 5, 9, 5, 11, 4, 11, 10, 2, 10, 7, 6, 7, 1, 8,
		// 5 faces around point 3
		3, 9, 4, 3, 4, 2, 3, 2, 6, 3, 6, 8, 3, 8, 9,
		// 5 adjacent faces
		4, 9, 5, 2, 4, 11, 6, 2, 10, 8, 6, 7, 9, 8, 1,
	}

	if rotation.X == 0 && rotation.Y == 0 && rotation.Z == 0 {
		return geometry.NewTriangleMesh(vertices, faces, material)
	}
	return geometry.NewTriangleMeshWithRotation(vertices, faces, material, center, rotation)
}
