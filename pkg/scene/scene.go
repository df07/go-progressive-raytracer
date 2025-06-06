package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// Scene contains all the elements needed for rendering
type Scene struct {
	Camera      *renderer.Camera
	TopColor    core.Vec3    // Color at top of gradient
	BottomColor core.Vec3    // Color at bottom of gradient
	Shapes      []core.Shape // Objects in the scene
	Lights      []core.Light // Lights in the scene
}

// NewDefaultScene creates a default scene with lighting, gradient background and spheres with materials
func NewDefaultScene() *Scene {
	config := renderer.CameraConfig{
		Center:        core.NewVec3(0, 0.75, 2), // Position camera higher and farther back
		LookAt:        core.NewVec3(0, 0.5, -1), // Look at the sphere center
		Up:            core.NewVec3(0, 1, 0),    // Standard up direction
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          40.0, // Narrower field of view for focus effect
		Aperture:      0.05, // Strong depth of field blur
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	camera := renderer.NewCamera(config)

	// Create the scene
	s := &Scene{
		Camera:      camera,
		TopColor:    core.NewVec3(0.5, 0.7, 1.0), // Blue
		BottomColor: core.NewVec3(1.0, 1.0, 1.0), // White
		Shapes:      make([]core.Shape, 0),
		Lights:      make([]core.Light, 0),
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
	metalSilver := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0)
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)

	// Create spheres with different materials
	sphereCenter := geometry.NewSphere(core.NewVec3(0, 0.5, -1), 0.5, lambertianBlue)
	sphereLeft := geometry.NewSphere(core.NewVec3(-1, 0.5, -1), 0.5, metalSilver)
	sphereRight := geometry.NewSphere(core.NewVec3(1, 0.5, -1), 0.5, metalGold)
	groundSphere := geometry.NewSphere(core.NewVec3(0, -100, -1), 100, lambertianGreen)

	// Add objects to the scene
	s.Shapes = append(s.Shapes, sphereCenter, sphereLeft, sphereRight, groundSphere)

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

// AddSphereLight adds a spherical light to the scene
func (s *Scene) AddSphereLight(center core.Vec3, radius float64, emission core.Vec3) {
	// Create emissive material
	emissiveMat := material.NewEmissive(emission)

	// Create sphere light for sampling
	sphereLight := geometry.NewSphereLight(center, radius, emissiveMat)

	// Add to light list for direct lighting
	s.Lights = append(s.Lights, sphereLight)

	// Add to scene as a regular object for ray intersections
	s.Shapes = append(s.Shapes, sphereLight.Sphere)
}
