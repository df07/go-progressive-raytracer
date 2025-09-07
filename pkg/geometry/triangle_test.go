package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestTriangle_Hit(t *testing.T) {
	// Create a triangle in the XY plane
	v0 := core.NewVec3(0, 0, 0)
	v1 := core.NewVec3(1, 0, 0)
	v2 := core.NewVec3(0, 1, 0)
	triangle := NewTriangle(v0, v1, v2, material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5)))

	tests := []struct {
		name      string
		ray       core.Ray
		tMin      float64
		tMax      float64
		shouldHit bool
		expectedT float64
	}{
		{
			name: "Ray hits triangle center",
			ray: core.NewRay(
				core.NewVec3(0.25, 0.25, -1), // origin
				core.NewVec3(0, 0, 1),        // direction (toward +Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
		{
			name: "Ray hits triangle edge",
			ray: core.NewRay(
				core.NewVec3(0.5, 0, -1), // origin (on edge between v0 and v1)
				core.NewVec3(0, 0, 1),    // direction (toward +Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
		{
			name: "Ray misses triangle",
			ray: core.NewRay(
				core.NewVec3(1, 1, -1), // origin (outside triangle)
				core.NewVec3(0, 0, 1),  // direction (toward +Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: false,
		},
		{
			name: "Ray parallel to triangle",
			ray: core.NewRay(
				core.NewVec3(0.25, 0.25, 0), // origin (in triangle plane)
				core.NewVec3(1, 0, 0),       // direction (parallel to plane)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: false,
		},
		{
			name: "Ray hits from behind",
			ray: core.NewRay(
				core.NewVec3(0.25, 0.25, 1), // origin (behind triangle)
				core.NewVec3(0, 0, -1),      // direction (toward -Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, isHit := triangle.Hit(tt.ray, tt.tMin, tt.tMax)

			if isHit != tt.shouldHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.shouldHit, isHit)
				return
			}

			if tt.shouldHit {
				if hit == nil {
					t.Error("Expected hit record, got nil")
					return
				}

				if math.Abs(hit.T-tt.expectedT) > 1e-6 {
					t.Errorf("Expected t=%f, got t=%f", tt.expectedT, hit.T)
				}

				// Verify hit point is on the triangle plane
				expectedPoint := tt.ray.At(hit.T)
				if expectedPoint.Subtract(hit.Point).Length() > 1e-6 {
					t.Errorf("Hit point mismatch: expected %v, got %v", expectedPoint, hit.Point)
				}
			}
		})
	}
}

func TestTriangle_BoundingBox(t *testing.T) {
	v0 := core.NewVec3(0, 0, 0)
	v1 := core.NewVec3(2, 0, 0)
	v2 := core.NewVec3(1, 3, 0)
	triangle := NewTriangle(v0, v1, v2, material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5)))

	bbox := triangle.BoundingBox()

	expectedMin := core.NewVec3(0, 0, 0)
	expectedMax := core.NewVec3(2, 3, 0)

	const tolerance = 1e-9
	if bbox.Min.Subtract(expectedMin).Length() > tolerance {
		t.Errorf("Expected min %v, got %v", expectedMin, bbox.Min)
	}
	if bbox.Max.Subtract(expectedMax).Length() > tolerance {
		t.Errorf("Expected max %v, got %v", expectedMax, bbox.Max)
	}
}
