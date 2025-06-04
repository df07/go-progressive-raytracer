package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/math"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// Scene contains all the elements needed for rendering
type Scene struct {
	Camera      *renderer.Camera
	TopColor    math.Vec3        // Color at top of gradient
	BottomColor math.Vec3        // Color at bottom of gradient
	Shapes      []geometry.Shape // Objects in the scene
}

// NewDefaultScene creates a default scene with gradient background and a sphere
func NewDefaultScene() *Scene {
	camera := renderer.NewCamera()

	// Create a sphere in front of the camera
	sphere := geometry.NewSphere(math.NewVec3(0, 0, -1), 0.5)

	return &Scene{
		Camera:      camera,
		TopColor:    math.NewVec3(0.5, 0.7, 1.0), // Blue
		BottomColor: math.NewVec3(1.0, 1.0, 1.0), // White
		Shapes:      []geometry.Shape{sphere},
	}
}

// GetCamera returns the scene's camera
func (s *Scene) GetCamera() *renderer.Camera {
	return s.Camera
}

// GetBackgroundColors returns the top and bottom colors for the background gradient
func (s *Scene) GetBackgroundColors() (topColor, bottomColor math.Vec3) {
	return s.TopColor, s.BottomColor
}

// GetShapes returns all shapes in the scene
func (s *Scene) GetShapes() []geometry.Shape {
	return s.Shapes
}
