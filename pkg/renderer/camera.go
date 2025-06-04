package renderer

import (
	"github.com/df07/go-progressive-raytracer/pkg/math"
)

// Camera generates rays for rendering
type Camera struct {
	origin          math.Vec3
	lowerLeftCorner math.Vec3
	horizontal      math.Vec3
	vertical        math.Vec3
}

// NewCamera creates a simple camera
func NewCamera() *Camera {
	aspectRatio := 16.0 / 9.0
	viewportHeight := 2.0
	viewportWidth := aspectRatio * viewportHeight
	focalLength := 1.0

	origin := math.NewVec3(0, 0, 0)
	horizontal := math.NewVec3(viewportWidth, 0, 0)
	vertical := math.NewVec3(0, viewportHeight, 0)
	lowerLeftCorner := origin.Subtract(horizontal.Multiply(0.5)).
		Subtract(vertical.Multiply(0.5)).
		Subtract(math.NewVec3(0, 0, focalLength))

	return &Camera{
		origin:          origin,
		horizontal:      horizontal,
		vertical:        vertical,
		lowerLeftCorner: lowerLeftCorner,
	}
}

// GetRay generates a ray for screen coordinates (s, t) where 0 <= s,t <= 1
func (c *Camera) GetRay(s, t float64) math.Ray {
	direction := c.lowerLeftCorner.
		Add(c.horizontal.Multiply(s)).
		Add(c.vertical.Multiply(t)).
		Subtract(c.origin)

	return math.NewRay(c.origin, direction)
}
