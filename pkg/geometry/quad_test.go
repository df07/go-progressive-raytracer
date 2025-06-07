package geometry

import (
	"fmt"
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestQuad_Hit_BasicIntersection(t *testing.T) {
	// Create a 1x1 quad in the XZ plane at y=0
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0) // X direction
	v := core.NewVec3(0, 0, 1) // Z direction
	quad := NewQuad(corner, u, v, DummyMaterial{})

	// Ray shooting down at the center of the quad
	ray := core.NewRay(core.NewVec3(0.5, 1, 0.5), core.NewVec3(0, -1, 0))

	hit, isHit := quad.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	expectedT := 1.0
	if math.Abs(hit.T-expectedT) > 1e-9 {
		t.Errorf("Expected t=%f, got t=%f", expectedT, hit.T)
	}

	expectedPoint := core.NewVec3(0.5, 0, 0.5)
	tolerance := 1e-9
	if math.Abs(hit.Point.X-expectedPoint.X) > tolerance ||
		math.Abs(hit.Point.Y-expectedPoint.Y) > tolerance ||
		math.Abs(hit.Point.Z-expectedPoint.Z) > tolerance {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestQuad_Hit_OutsideBounds(t *testing.T) {
	// Create a 1x1 quad in the XZ plane at y=0
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0) // X direction
	v := core.NewVec3(0, 0, 1) // Z direction
	quad := NewQuad(corner, u, v, DummyMaterial{})

	tests := []struct {
		name      string
		rayOrigin core.Vec3
		rayDir    core.Vec3
	}{
		{
			name:      "outside X bounds (negative)",
			rayOrigin: core.NewVec3(-0.5, 1, 0.5),
			rayDir:    core.NewVec3(0, -1, 0),
		},
		{
			name:      "outside X bounds (positive)",
			rayOrigin: core.NewVec3(1.5, 1, 0.5),
			rayDir:    core.NewVec3(0, -1, 0),
		},
		{
			name:      "outside Z bounds (negative)",
			rayOrigin: core.NewVec3(0.5, 1, -0.5),
			rayDir:    core.NewVec3(0, -1, 0),
		},
		{
			name:      "outside Z bounds (positive)",
			rayOrigin: core.NewVec3(0.5, 1, 1.5),
			rayDir:    core.NewVec3(0, -1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.rayOrigin, tt.rayDir)
			hit, isHit := quad.Hit(ray, 0.001, 1000.0)
			if isHit {
				t.Errorf("Expected miss for ray outside bounds, but got hit at t=%f", hit.T)
			}
		})
	}
}

func TestQuad_Hit_CornerHits(t *testing.T) {
	// Create a 1x1 quad in the XZ plane at y=0
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0) // X direction
	v := core.NewVec3(0, 0, 1) // Z direction
	quad := NewQuad(corner, u, v, DummyMaterial{})

	corners := []core.Vec3{
		{0, 0, 0}, // corner
		{1, 0, 0}, // corner + u
		{0, 0, 1}, // corner + v
		{1, 0, 1}, // corner + u + v
	}

	for i, cornerPoint := range corners {
		t.Run(fmt.Sprintf("corner_%d", i), func(t *testing.T) {
			ray := core.NewRay(cornerPoint.Add(core.NewVec3(0, 1, 0)), core.NewVec3(0, -1, 0))
			_, isHit := quad.Hit(ray, 0.001, 1000.0)
			if !isHit {
				t.Errorf("Expected hit at corner %v, but got miss", cornerPoint)
			}
		})
	}
}

func TestQuad_Hit_ParallelRay(t *testing.T) {
	// Create a 1x1 quad in the XZ plane at y=0
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0) // X direction
	v := core.NewVec3(0, 0, 1) // Z direction
	quad := NewQuad(corner, u, v, DummyMaterial{})

	// Ray parallel to the quad
	ray := core.NewRay(core.NewVec3(0.5, 1, 0.5), core.NewVec3(1, 0, 0))

	_, isHit := quad.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Error("Expected miss for parallel ray, but got hit")
	}
}
