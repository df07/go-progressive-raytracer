package geometry

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Shape interface for objects that can be hit by rays
type Shape interface {
	Hit(ray core.Ray, tMin, tMax float64) (*material.SurfaceInteraction, bool)
	BoundingBox() AABB
}

// Preprocessor interface for objects that need scene preprocessing
type Preprocessor interface {
	Preprocess(worldCenter core.Vec3, worldRadius float64) error
}
