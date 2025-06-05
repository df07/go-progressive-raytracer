package renderer

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Camera generates rays for rendering
type Camera struct {
	origin          core.Vec3
	lowerLeftCorner core.Vec3
	horizontal      core.Vec3
	vertical        core.Vec3
}

// NewCamera creates a simple camera
func NewCamera() *Camera {
	aspectRatio := 16.0 / 9.0
	viewportHeight := 2.0
	viewportWidth := aspectRatio * viewportHeight
	focalLength := 1.0

	origin := core.NewVec3(0, 0, 0)
	horizontal := core.NewVec3(viewportWidth, 0, 0)
	vertical := core.NewVec3(0, viewportHeight, 0)
	lowerLeftCorner := origin.Subtract(horizontal.Multiply(0.5)).
		Subtract(vertical.Multiply(0.5)).
		Subtract(core.NewVec3(0, 0, focalLength))

	return &Camera{
		origin:          origin,
		horizontal:      horizontal,
		vertical:        vertical,
		lowerLeftCorner: lowerLeftCorner,
	}
}

// GetRay generates a ray for screen coordinates (s, t) where 0 <= s,t <= 1
func (c *Camera) GetRay(s, t float64) core.Ray {
	direction := c.lowerLeftCorner.
		Add(c.horizontal.Multiply(s)).
		Add(c.vertical.Multiply(t)).
		Subtract(c.origin)

	return core.NewRay(c.origin, direction)
}
