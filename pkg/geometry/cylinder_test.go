package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestNewCylinder(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	baseCenter := core.NewVec3(0, 0, 0)
	topCenter := core.NewVec3(0, 2, 0)
	radius := 1.0

	cyl := NewCylinder(baseCenter, topCenter, radius, false, mat)

	if cyl == nil {
		t.Fatal("NewCylinder returned nil")
	}

	// Check that axis is normalized and points in the right direction
	expectedAxis := core.NewVec3(0, 1, 0)
	if !cyl.axis.Equals(expectedAxis) {
		t.Errorf("Expected axis %v, got %v", expectedAxis, cyl.axis)
	}

	// Check height
	expectedHeight := 2.0
	if math.Abs(cyl.height-expectedHeight) > 1e-9 {
		t.Errorf("Expected height %f, got %f", expectedHeight, cyl.height)
	}
}

func TestCylinder_BoundingBox(t *testing.T) {
	tests := []struct {
		name       string
		baseCenter core.Vec3
		topCenter  core.Vec3
		radius     float64
		wantMin    core.Vec3
		wantMax    core.Vec3
	}{
		{
			name:       "axis-aligned Y",
			baseCenter: core.NewVec3(0, 0, 0),
			topCenter:  core.NewVec3(0, 2, 0),
			radius:     1.0,
			wantMin:    core.NewVec3(-1, 0, -1),
			wantMax:    core.NewVec3(1, 2, 1),
		},
		{
			name:       "axis-aligned Z",
			baseCenter: core.NewVec3(0, 0, 0),
			topCenter:  core.NewVec3(0, 0, 3),
			radius:     0.5,
			wantMin:    core.NewVec3(-0.5, -0.5, 0),
			wantMax:    core.NewVec3(0.5, 0.5, 3),
		},
		{
			name:       "offset position",
			baseCenter: core.NewVec3(5, 5, 5),
			topCenter:  core.NewVec3(5, 8, 5),
			radius:     2.0,
			wantMin:    core.NewVec3(3, 5, 3),
			wantMax:    core.NewVec3(7, 8, 7),
		},
	}

	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cyl := NewCylinder(tt.baseCenter, tt.topCenter, tt.radius, false, mat)
			bbox := cyl.BoundingBox()

			tolerance := 1e-9
			if !approxEqual(bbox.Min.X, tt.wantMin.X, tolerance) ||
				!approxEqual(bbox.Min.Y, tt.wantMin.Y, tolerance) ||
				!approxEqual(bbox.Min.Z, tt.wantMin.Z, tolerance) {
				t.Errorf("Expected min %v, got %v", tt.wantMin, bbox.Min)
			}

			if !approxEqual(bbox.Max.X, tt.wantMax.X, tolerance) ||
				!approxEqual(bbox.Max.Y, tt.wantMax.Y, tolerance) ||
				!approxEqual(bbox.Max.Z, tt.wantMax.Z, tolerance) {
				t.Errorf("Expected max %v, got %v", tt.wantMax, bbox.Max)
			}
		})
	}
}

// Helper function for approximate equality
func approxEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestCylinder_Hit_SimpleHit(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Cylinder along Y axis, from Y=0 to Y=2, radius 1
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	// Ray from outside hitting the cylinder perpendicular to axis
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	// Should hit at X=1 (surface of cylinder)
	expectedPoint := core.NewVec3(1, 1, 0)
	tolerance := 1e-6
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in +X direction (radially outward)
	expectedNormal := core.NewVec3(1, 0, 0)
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}

	if !hit.FrontFace {
		t.Error("Expected front face hit")
	}
}

func TestCylinder_Hit_Miss(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	// Ray that misses the cylinder entirely
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if isHit {
		t.Errorf("Expected miss, but got hit at %v", hit.Point)
	}
}

func TestCylinder_Hit_HeightBounds(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Cylinder from Y=0 to Y=2
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	tests := []struct {
		name    string
		origin  core.Vec3
		dir     core.Vec3
		wantHit bool
	}{
		{
			name:    "hit within bounds",
			origin:  core.NewVec3(2, 1, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: true,
		},
		{
			name:    "miss above cylinder",
			origin:  core.NewVec3(2, 3, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: false,
		},
		{
			name:    "miss below cylinder",
			origin:  core.NewVec3(2, -1, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cyl.Hit(ray, 0.001, 1000.0)

			if isHit != tt.wantHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.wantHit, isHit)
			}
		})
	}
}

func TestCylinder_Hit_FromInside(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	// Ray from inside the cylinder shooting outward
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(1, 0, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	// Should be a back face hit
	if hit.FrontFace {
		t.Error("Expected back face hit when shooting from inside")
	}

	// Normal should point inward (flipped by SetFaceNormal)
	expectedNormal := core.NewVec3(-1, 0, 0)
	tolerance := 1e-6
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCylinder_Hit_ArbitraryOrientation(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Cylinder along X axis
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(3, 0, 0),
		1.0,
		false,
		mat,
	)

	// Ray hitting from the side
	ray := core.NewRay(core.NewVec3(1.5, 2, 0), core.NewVec3(0, -1, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	// Should hit at Y=1 (surface of cylinder)
	expectedPoint := core.NewVec3(1.5, 1, 0)
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in +Y direction
	expectedNormal := core.NewVec3(0, 1, 0)
	tolerance := 1e-6
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCylinder_Hit_TwoIntersections(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	// Ray passing through the cylinder (should hit closer intersection)
	ray := core.NewRay(core.NewVec3(-2, 1, 0), core.NewVec3(1, 0, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	// Should hit the closer intersection at X=-1
	expectedT := 1.0
	tolerance := 1e-6
	if math.Abs(hit.T-expectedT) > tolerance {
		t.Errorf("Expected t=%f, got t=%f", expectedT, hit.T)
	}

	expectedPoint := core.NewVec3(-1, 1, 0)
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestCylinder_Hit_TBounds(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false,
		mat,
	)

	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))

	// Test tMax bound - intersections at t=1 and t=3, but tMax=0.5
	hit, isHit := cyl.Hit(ray, 0.001, 0.5)
	if isHit {
		t.Errorf("Expected miss due to tMax bound, but got hit at t=%f", hit.T)
	}

	// Test tMin bound - intersections at t=1 and t=3, but tMin=3.5
	hit, isHit = cyl.Hit(ray, 3.5, 1000.0)
	if isHit {
		t.Errorf("Expected miss due to tMin bound, but got hit at t=%f", hit.T)
	}

	// Test that we get the closer intersection when both are in range
	hit, isHit = cyl.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit")
	}
	expectedT := 1.0
	if math.Abs(hit.T-expectedT) > 1e-6 {
		t.Errorf("Expected closer intersection at t=%f, got t=%f", expectedT, hit.T)
	}
}

func TestCylinder_Capped_HitTopCap(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		true, // capped
		mat,
	)

	// Ray from above hitting top cap
	ray := core.NewRay(core.NewVec3(0.5, 3, 0), core.NewVec3(0, -1, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on top cap, but got miss")
	}

	// Should hit at Y=2 (top of cylinder)
	expectedPoint := core.NewVec3(0.5, 2, 0)
	tolerance := 1e-6
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in +Y direction (upward)
	expectedNormal := core.NewVec3(0, 1, 0)
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}

	if !hit.FrontFace {
		t.Error("Expected front face hit on top cap")
	}
}

func TestCylinder_Capped_HitBottomCap(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		true, // capped
		mat,
	)

	// Ray from below hitting bottom cap
	ray := core.NewRay(core.NewVec3(0.3, -1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on bottom cap, but got miss")
	}

	// Should hit at Y=0 (bottom of cylinder)
	expectedPoint := core.NewVec3(0.3, 0, 0)
	tolerance := 1e-6
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in -Y direction (downward)
	expectedNormal := core.NewVec3(0, -1, 0)
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}

	if !hit.FrontFace {
		t.Error("Expected front face hit on bottom cap")
	}
}

func TestCylinder_Capped_MissOutsideCapRadius(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		true, // capped
		mat,
	)

	tests := []struct {
		name   string
		origin core.Vec3
		dir    core.Vec3
	}{
		{
			name:   "miss top cap outside radius",
			origin: core.NewVec3(1.5, 3, 0),
			dir:    core.NewVec3(0, -1, 0),
		},
		{
			name:   "miss bottom cap outside radius",
			origin: core.NewVec3(-1.5, -1, 0),
			dir:    core.NewVec3(0, 1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cyl.Hit(ray, 0.001, 1000.0)

			if isHit {
				t.Error("Expected miss outside cap radius, but got hit")
			}
		})
	}
}

func TestCylinder_Capped_HitBodyNotCap(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		true, // capped
		mat,
	)

	// Ray hitting the side (body), not the caps
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on cylinder body")
	}

	// Should hit the body at X=1, not a cap
	expectedPoint := core.NewVec3(1, 1, 0)
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should be radial (in X direction), not axial
	expectedNormal := core.NewVec3(1, 0, 0)
	tolerance := 1e-6
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected radial normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCylinder_Capped_FromInside(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		true, // capped
		mat,
	)

	// Ray from inside shooting upward toward top cap
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cyl.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on top cap from inside")
	}

	// Should be a back face hit
	if hit.FrontFace {
		t.Error("Expected back face hit when shooting from inside")
	}

	// Normal should point inward (downward)
	expectedNormal := core.NewVec3(0, -1, 0)
	tolerance := 1e-6
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected inward normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCylinder_Uncapped_NoCapHits(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cyl := NewCylinder(
		core.NewVec3(0, 0, 0),
		core.NewVec3(0, 2, 0),
		1.0,
		false, // uncapped
		mat,
	)

	tests := []struct {
		name   string
		origin core.Vec3
		dir    core.Vec3
	}{
		{
			name:   "ray through top opening",
			origin: core.NewVec3(0.5, 3, 0),
			dir:    core.NewVec3(0, -1, 0),
		},
		{
			name:   "ray through bottom opening",
			origin: core.NewVec3(0.3, -1, 0),
			dir:    core.NewVec3(0, 1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cyl.Hit(ray, 0.001, 1000.0)

			// Should pass through without hitting
			if isHit {
				t.Error("Expected miss on uncapped cylinder, but got hit")
			}
		})
	}
}
