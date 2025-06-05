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
}

// NewDefaultScene creates a default scene with gradient background and spheres with materials
func NewDefaultScene() *Scene {
	camera := renderer.NewCamera()

	// Create materials
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))
	lambertianBlue := material.NewLambertian(core.NewVec3(0.1, 0.2, 0.5))
	/*lambertianMix := material.NewMix(
		material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0),
		material.NewLambertian(core.NewVec3(0.7, 0.3, 0.3)),
		0.5,
	)*/
	metalSilver := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0)
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)

	// Create spheres with different materials
	sphereCenter := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertianBlue)
	sphereLeft := geometry.NewSphere(core.NewVec3(-1, 0, -1), 0.5, metalSilver)
	sphereRight := geometry.NewSphere(core.NewVec3(1, 0, -1), 0.5, metalGold)
	groundSphere := geometry.NewSphere(core.NewVec3(0, -100.5, -1), 100, lambertianGreen)

	return &Scene{
		Camera:      camera,
		TopColor:    core.NewVec3(0.5, 0.7, 1.0), // Blue
		BottomColor: core.NewVec3(1.0, 1.0, 1.0), // White
		Shapes:      []core.Shape{sphereCenter, sphereLeft, sphereRight, groundSphere},
	}
}

// GetCamera returns the scene's camera
func (s *Scene) GetCamera() *renderer.Camera {
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
