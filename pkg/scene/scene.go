package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// Scene contains all the elements needed for rendering
type Scene struct {
	Camera         *renderer.Camera
	TopColor       core.Vec3    // Color at top of gradient
	BottomColor    core.Vec3    // Color at bottom of gradient
	Shapes         []core.Shape // Objects in the scene
	Lights         []core.Light // Lights in the scene
	SamplingConfig core.SamplingConfig
	CameraConfig   renderer.CameraConfig
}

// NewDefaultScene creates a default scene with spheres, ground, and camera
func NewDefaultScene(cameraOverrides ...renderer.CameraConfig) *Scene {
	// Default camera configuration
	defaultCameraConfig := renderer.CameraConfig{
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
		cameraConfig = renderer.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	camera := renderer.NewCamera(cameraConfig)

	samplingConfig := core.SamplingConfig{
		SamplesPerPixel:           200,
		MaxDepth:                  50,
		RussianRouletteMinBounces: 16,   // Need a lot of bounces for complex glass
		RussianRouletteMinSamples: 8,    // More samples before RR due to caustics
		AdaptiveMinSamples:        8,    // Standard minimum for adaptive sampling
		AdaptiveThreshold:         0.01, // 1% relative error threshold
		AdaptiveDarkThreshold:     1e-6, // Low absolute threshold for dark pixels
	}

	s := &Scene{
		Camera:         camera,
		TopColor:       core.NewVec3(0.5, 0.7, 1.0), // Blue
		BottomColor:    core.NewVec3(1.0, 1.0, 1.0), // White
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Add the specified light: pos [30, 30.5, 15], r: 10, emit: [15.0, 14.0, 13.0]
	s.AddSphereLight(
		core.NewVec3(30, 30.5, 15),     // position
		10,                             // radius
		core.NewVec3(15.0, 14.0, 13.0), // emission
	)

	// Create materials
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))
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

	// Create ground plane instead of sphere
	groundPlane := geometry.NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), lambertianGreen)

	// Create hollow glass sphere with blue sphere inside
	hollowGlassOuter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.25, materialGlass)
	hollowGlassInner := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), -0.24, materialGlass)
	hollowGlassCenter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.20, lambertianBlue)

	// Add objects to the scene
	s.Shapes = append(s.Shapes, sphereCenter, sphereLeft, sphereRight, groundPlane,
		solidGlassSphere, hollowGlassOuter, hollowGlassInner, hollowGlassCenter)

	return s
}

// GetCamera returns the scene's camera
func (s *Scene) GetCamera() core.Camera {
	return s.Camera
}

// GetBackgroundColors returns the top and bottom colors for the background gradient
func (s *Scene) GetBackgroundColors() (topColor, bottomColor core.Vec3) {
	return s.TopColor, s.BottomColor
}

// GetShapes returns all shapes in the scene
func (s *Scene) GetShapes() []core.Shape {
	return s.Shapes
}

// GetLights returns all lights in the scene
func (s *Scene) GetLights() []core.Light {
	return s.Lights
}

// GetSamplingConfig returns the scene's sampling configuration
func (s *Scene) GetSamplingConfig() core.SamplingConfig {
	return s.SamplingConfig
}

// GetPrimitiveCount returns the total number of primitive objects in the scene
func (s *Scene) GetPrimitiveCount() int {
	count := 0
	for _, shape := range s.Shapes {
		count += s.countPrimitivesInShape(shape)
	}
	return count
}

// countPrimitivesInShape counts primitives in a single shape, handling complex objects
func (s *Scene) countPrimitivesInShape(shape core.Shape) int {
	switch obj := shape.(type) {
	case *geometry.TriangleMesh:
		// Triangle meshes contain multiple triangles
		return obj.GetTriangleCount()
	default:
		// Regular shapes count as 1 primitive each
		return 1
	}
}

// AddSphereLight adds a spherical light to the scene
func (s *Scene) AddSphereLight(center core.Vec3, radius float64, emission core.Vec3) {
	emissiveMat := material.NewEmissive(emission)
	sphereLight := geometry.NewSphereLight(center, radius, emissiveMat)
	s.Lights = append(s.Lights, sphereLight)
	s.Shapes = append(s.Shapes, sphereLight.Sphere)
}

// AddQuadLight adds a rectangular area light to the scene
func (s *Scene) AddQuadLight(corner, u, v core.Vec3, emission core.Vec3) {
	emissiveMat := material.NewEmissive(emission)
	quadLight := geometry.NewQuadLight(corner, u, v, emissiveMat)
	s.Lights = append(s.Lights, quadLight)
	s.Shapes = append(s.Shapes, quadLight.Quad)
}
