package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// DummyMaterial for testing - doesn't actually scatter
type DummyMaterial struct{}

func (d DummyMaterial) Scatter(rayIn core.Ray, hit core.HitRecord, sampler core.Sampler) (core.ScatterResult, bool) {
	return core.ScatterResult{}, false
}

func (d DummyMaterial) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

func (d DummyMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	return 0.0, false
}

func TestSphere_Hit_Miss(t *testing.T) {
	sphere := NewSphere(core.NewVec3(0, 0, 0), 1.0, DummyMaterial{})
	ray := core.NewRay(core.NewVec3(2, 0, 0), core.NewVec3(0, 1, 0))

	hit, isHit := sphere.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Errorf("Expected miss, but got hit at t=%f", hit.T)
	}
}

func TestSphere_Hit_FrontAndBackFace(t *testing.T) {
	sphere := NewSphere(core.NewVec3(0, 0, 0), 1.0, DummyMaterial{})

	tests := []struct {
		name           string
		rayOrigin      core.Vec3
		rayDirection   core.Vec3
		expectedT      float64
		expectedFront  bool
		expectedNormal core.Vec3
	}{
		{
			name:           "front face hit",
			rayOrigin:      core.NewVec3(0, 0, 2),
			rayDirection:   core.NewVec3(0, 0, -1),
			expectedT:      1.0,
			expectedFront:  true,
			expectedNormal: core.NewVec3(0, 0, 1),
		},
		{
			name:           "back face hit",
			rayOrigin:      core.NewVec3(0, 0, 0),
			rayDirection:   core.NewVec3(0, 0, 1),
			expectedT:      1.0,
			expectedFront:  false,
			expectedNormal: core.NewVec3(0, 0, -1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.rayOrigin, tt.rayDirection)
			hit, isHit := sphere.Hit(ray, 0.001, 1000.0)

			if !isHit {
				t.Fatal("Expected hit, but got miss")
			}

			if math.Abs(hit.T-tt.expectedT) > 1e-9 {
				t.Errorf("Expected t=%f, got t=%f", tt.expectedT, hit.T)
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

func TestSphere_Hit_GlancingHit(t *testing.T) {
	sphere := NewSphere(core.NewVec3(0, 0, 0), 1.0, DummyMaterial{})
	ray := core.NewRay(core.NewVec3(1, 0, 2), core.NewVec3(0, 0, -1))

	hit, isHit := sphere.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected glancing hit, but got miss")
	}

	expectedPoint := core.NewVec3(1, 0, 0)
	tolerance := 1e-9
	if math.Abs(hit.Point.X-expectedPoint.X) > tolerance ||
		math.Abs(hit.Point.Y-expectedPoint.Y) > tolerance ||
		math.Abs(hit.Point.Z-expectedPoint.Z) > tolerance {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestSphere_Hit_Bounds(t *testing.T) {
	sphere := NewSphere(core.NewVec3(0, 0, 0), 1.0, DummyMaterial{})
	ray := core.NewRay(core.NewVec3(0, 0, 2), core.NewVec3(0, 0, -1))

	// Test tMax bound
	hit, isHit := sphere.Hit(ray, 0.001, 0.5)
	if isHit {
		t.Errorf("Expected miss due to tMax bound, but got hit at t=%f", hit.T)
	}

	// Test tMin bound
	hit, isHit = sphere.Hit(ray, 3.5, 1000.0)
	if isHit {
		t.Errorf("Expected miss due to tMin bound, but got hit at t=%f", hit.T)
	}
}

func TestSphere_Hit_ClosestIntersection(t *testing.T) {
	sphere := NewSphere(core.NewVec3(0, 0, 0), 1.0, DummyMaterial{})
	ray := core.NewRay(core.NewVec3(0, 0, 2), core.NewVec3(0, 0, -1))

	hit, isHit := sphere.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	expectedT := 1.0
	if math.Abs(hit.T-expectedT) > 1e-9 {
		t.Errorf("Expected closest intersection at t=%f, got t=%f", expectedT, hit.T)
	}

	if !hit.FrontFace {
		t.Error("Expected closest intersection to be front face")
	}
}
