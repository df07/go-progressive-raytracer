package geometry

import "github.com/df07/go-progressive-raytracer/pkg/math"

// HitRecord contains information about a ray-object intersection
type HitRecord struct {
	Point     math.Vec3 // Point of intersection
	Normal    math.Vec3 // Surface normal at intersection
	T         float64   // Parameter t along the ray
	FrontFace bool      // Whether ray hit the front face
}

// SetFaceNormal sets the normal vector and determines front/back face
func (h *HitRecord) SetFaceNormal(ray math.Ray, outwardNormal math.Vec3) {
	h.FrontFace = ray.Direction.Dot(outwardNormal) < 0
	if h.FrontFace {
		h.Normal = outwardNormal
	} else {
		h.Normal = outwardNormal.Multiply(-1)
	}
}

// Shape interface for objects that can be hit by rays
type Shape interface {
	Hit(ray math.Ray, tMin, tMax float64) (*HitRecord, bool)
}
