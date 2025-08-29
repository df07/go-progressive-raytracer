package integrator

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// SplatRay represents a ray-based color contribution that needs to be mapped to pixels
type SplatRay struct {
	Ray   core.Ray  // Ray that should contribute to some pixel
	Color core.Vec3 // Color contribution
}

// Integrator defines the interface for light transport algorithms
type Integrator interface {
	// RayColor computes color for a ray, with support for ray-based splatting
	// Returns (pixel color, splat rays)
	// Note: Uses interface{} to avoid circular import (actual type is *scene.Scene)
	RayColor(ray core.Ray, scene *scene.Scene, sampler core.Sampler) (core.Vec3, []SplatRay)
}
