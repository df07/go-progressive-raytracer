package material

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// ColorSource provides spatially-varying colors for materials
type ColorSource interface {
	// Evaluate returns color at given UV coordinates and 3D point
	// UV is used for image textures, point for procedural textures
	Evaluate(uv core.Vec2, point core.Vec3) core.Vec3
}

// SolidColor provides uniform color (backward compatibility)
type SolidColor struct {
	Color core.Vec3
}

// NewSolidColor creates a new solid color source
func NewSolidColor(color core.Vec3) *SolidColor {
	return &SolidColor{Color: color}
}

// Evaluate returns the solid color regardless of UV or position
func (s *SolidColor) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
	return s.Color
}
