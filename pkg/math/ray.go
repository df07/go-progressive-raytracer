package math

// Ray represents a ray with an origin and direction
type Ray struct {
	Origin    Vec3
	Direction Vec3
}

// NewRay creates a new ray
func NewRay(origin, direction Vec3) Ray {
	return Ray{Origin: origin, Direction: direction}
}

// At returns the point at parameter t along the ray
func (r Ray) At(t float64) Vec3 {
	return r.Origin.Add(r.Direction.Multiply(t))
} 