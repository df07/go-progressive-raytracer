package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestPlane_Hit_BasicIntersection(t *testing.T) {
	// Create a horizontal plane at y=0
	plane := NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), DummyMaterial{})

	// Ray shooting down from above
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(0, -1, 0))

	hit, isHit := plane.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	expectedT := 1.0
	if math.Abs(hit.T-expectedT) > 1e-9 {
		t.Errorf("Expected t=%f, got t=%f", expectedT, hit.T)
	}

	expectedPoint := core.NewVec3(0, 0, 0)
	tolerance := 1e-9
	if math.Abs(hit.Point.X-expectedPoint.X) > tolerance ||
		math.Abs(hit.Point.Y-expectedPoint.Y) > tolerance ||
		math.Abs(hit.Point.Z-expectedPoint.Z) > tolerance {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestPlane_Hit_ParallelRay(t *testing.T) {
	// Create a horizontal plane at y=0
	plane := NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), DummyMaterial{})

	// Ray parallel to the plane
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(1, 0, 0))

	hit, isHit := plane.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Errorf("Expected miss for parallel ray, but got hit at t=%f", hit.T)
	}
}

func TestPlane_Hit_BehindRay(t *testing.T) {
	// Create a horizontal plane at y=0
	plane := NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), DummyMaterial{})

	// Ray shooting up from above (intersection behind ray origin)
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(0, 1, 0))

	hit, isHit := plane.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Errorf("Expected miss for intersection behind ray, but got hit at t=%f", hit.T)
	}
}

func TestPlane_Hit_FaceNormal(t *testing.T) {
	// Create a horizontal plane at y=0
	plane := NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), DummyMaterial{})

	tests := []struct {
		name           string
		rayOrigin      core.Vec3
		rayDirection   core.Vec3
		expectedFront  bool
		expectedNormal core.Vec3
	}{
		{
			name:           "front face hit (from above)",
			rayOrigin:      core.NewVec3(0, 1, 0),
			rayDirection:   core.NewVec3(0, -1, 0),
			expectedFront:  true,
			expectedNormal: core.NewVec3(0, 1, 0),
		},
		{
			name:           "back face hit (from below)",
			rayOrigin:      core.NewVec3(0, -1, 0),
			rayDirection:   core.NewVec3(0, 1, 0),
			expectedFront:  false,
			expectedNormal: core.NewVec3(0, -1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.rayOrigin, tt.rayDirection)
			hit, isHit := plane.Hit(ray, 0.001, 1000.0)

			if !isHit {
				t.Fatal("Expected hit, but got miss")
			}

			if hit.FrontFace != tt.expectedFront {
				t.Errorf("Expected front face %t, got %t", tt.expectedFront, hit.FrontFace)
			}

			tolerance := 1e-9
			if math.Abs(hit.Normal.X-tt.expectedNormal.X) > tolerance ||
				math.Abs(hit.Normal.Y-tt.expectedNormal.Y) > tolerance ||
				math.Abs(hit.Normal.Z-tt.expectedNormal.Z) > tolerance {
				t.Errorf("Expected normal %v, got %v", tt.expectedNormal, hit.Normal)
			}
		})
	}
}
