package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// DummyMaterial for testing (same as in quad_test.go)
type DummyBoxMaterial struct{}

func (d DummyBoxMaterial) Scatter(rayIn core.Ray, hit material.HitRecord, sampler core.Sampler) (material.ScatterResult, bool) {
	return material.ScatterResult{}, false
}

func (d DummyBoxMaterial) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

func (d DummyBoxMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	return 0.0, false
}

func TestNewAxisAlignedBox(t *testing.T) {
	center := core.NewVec3(0, 0, 0)
	size := core.NewVec3(1, 1, 1)
	material := DummyBoxMaterial{}

	box := NewAxisAlignedBox(center, size, material)

	if box.Center != center {
		t.Errorf("Expected center %v, got %v", center, box.Center)
	}
	if box.Size != size {
		t.Errorf("Expected size %v, got %v", size, box.Size)
	}
	if box.Rotation.X != 0 || box.Rotation.Y != 0 || box.Rotation.Z != 0 {
		t.Errorf("Expected zero rotation, got %v", box.Rotation)
	}
}

func TestNewBox_WithRotation(t *testing.T) {
	center := core.NewVec3(1, 2, 3)
	size := core.NewVec3(0.5, 1, 1.5)
	rotation := core.NewVec3(math.Pi/4, math.Pi/6, math.Pi/3)
	material := DummyBoxMaterial{}

	box := NewBox(center, size, rotation, material)

	if box.Center != center {
		t.Errorf("Expected center %v, got %v", center, box.Center)
	}
	if box.Size != size {
		t.Errorf("Expected size %v, got %v", size, box.Size)
	}
	if box.Rotation != rotation {
		t.Errorf("Expected rotation %v, got %v", rotation, box.Rotation)
	}
}

func TestBox_Hit_AxisAligned(t *testing.T) {
	// Create a 2x2x2 box centered at origin
	box := NewAxisAlignedBox(
		core.NewVec3(0, 0, 0), // center
		core.NewVec3(1, 1, 1), // size (half-extents)
		DummyBoxMaterial{},
	)

	tests := []struct {
		name      string
		ray       core.Ray
		tMin      float64
		tMax      float64
		shouldHit bool
		expectedT float64
	}{
		{
			name: "Ray hits front face",
			ray: core.NewRay(
				core.NewVec3(0, 0, -3), // origin
				core.NewVec3(0, 0, 1),  // direction (toward +Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 2.0, // Distance from -3 to -1 (front face of box)
		},
		{
			name: "Ray hits right face",
			ray: core.NewRay(
				core.NewVec3(-3, 0, 0), // origin
				core.NewVec3(1, 0, 0),  // direction (toward +X)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 2.0, // Distance from -3 to -1 (left face of box)
		},
		{
			name: "Ray misses box",
			ray: core.NewRay(
				core.NewVec3(0, 3, -3), // origin (above box)
				core.NewVec3(0, 0, 1),  // direction (toward +Z)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: false,
		},
		{
			name: "Ray inside box",
			ray: core.NewRay(
				core.NewVec3(0, 0, 0), // origin (center of box)
				core.NewVec3(1, 0, 0), // direction (toward +X)
			),
			tMin:      0.001,
			tMax:      10.0,
			shouldHit: true,
			expectedT: 1.0, // Distance from center to right face
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit := &material.HitRecord{}
			isHit := box.Hit(tt.ray, tt.tMin, tt.tMax, hit)

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

				// Verify hit point is on the box surface
				expectedPoint := tt.ray.At(hit.T)
				if expectedPoint.Subtract(hit.Point).Length() > 1e-6 {
					t.Errorf("Hit point mismatch: expected %v, got %v", expectedPoint, hit.Point)
				}
			}
		})
	}
}

func TestBox_BoundingBox_AxisAligned(t *testing.T) {
	center := core.NewVec3(2, 3, 4)
	size := core.NewVec3(1, 2, 1.5)
	box := NewAxisAlignedBox(center, size, DummyBoxMaterial{})

	bbox := box.BoundingBox()

	expectedMin := core.NewVec3(1, 1, 2.5) // center - size
	expectedMax := core.NewVec3(3, 5, 5.5) // center + size

	const tolerance = 1e-9
	if bbox.Min.Subtract(expectedMin).Length() > tolerance {
		t.Errorf("Expected min %v, got %v", expectedMin, bbox.Min)
	}
	if bbox.Max.Subtract(expectedMax).Length() > tolerance {
		t.Errorf("Expected max %v, got %v", expectedMax, bbox.Max)
	}
}

func TestBox_BoundingBox_Rotated(t *testing.T) {
	// Create a box rotated 45 degrees around Y axis
	center := core.NewVec3(0, 0, 0)
	size := core.NewVec3(1, 1, 1)
	rotation := core.NewVec3(0, math.Pi/4, 0) // 45 degrees around Y
	box := NewBox(center, size, rotation, DummyBoxMaterial{})

	bbox := box.BoundingBox()

	// For a 45-degree rotation around Y, the bounding box should expand
	// The diagonal of the XZ face becomes the new extent
	expectedExtent := math.Sqrt(2) // sqrt(1^2 + 1^2)
	expectedMin := core.NewVec3(-expectedExtent, -1, -expectedExtent)
	expectedMax := core.NewVec3(expectedExtent, 1, expectedExtent)

	const tolerance = 1e-6
	if math.Abs(bbox.Min.X-expectedMin.X) > tolerance ||
		math.Abs(bbox.Min.Y-expectedMin.Y) > tolerance ||
		math.Abs(bbox.Min.Z-expectedMin.Z) > tolerance {
		t.Errorf("Expected min approximately %v, got %v", expectedMin, bbox.Min)
	}
	if math.Abs(bbox.Max.X-expectedMax.X) > tolerance ||
		math.Abs(bbox.Max.Y-expectedMax.Y) > tolerance ||
		math.Abs(bbox.Max.Z-expectedMax.Z) > tolerance {
		t.Errorf("Expected max approximately %v, got %v", expectedMax, bbox.Max)
	}
}

func TestBox_Hit_Rotated(t *testing.T) {
	// Create a box rotated 45 degrees around Y axis
	box := NewBox(
		core.NewVec3(0, 0, 0),         // center
		core.NewVec3(1, 1, 1),         // size
		core.NewVec3(0, math.Pi/4, 0), // 45 degree rotation around Y
		DummyBoxMaterial{},
	)

	// Ray that should hit the rotated box
	ray := core.NewRay(
		core.NewVec3(0, 0, -3), // origin
		core.NewVec3(0, 0, 1),  // direction (toward +Z)
	)

	hit := &material.HitRecord{}
	isHit := box.Hit(ray, 0.001, 10.0, hit)

	if !isHit {
		t.Error("Expected ray to hit rotated box")
		return
	}

	if hit == nil {
		t.Error("Expected hit record, got nil")
		return
	}

	// The hit should occur somewhere reasonable
	if hit.T <= 0 || hit.T >= 10 {
		t.Errorf("Expected reasonable t value, got %f", hit.T)
	}

	// Verify the hit point is on the ray
	expectedPoint := ray.At(hit.T)
	if expectedPoint.Subtract(hit.Point).Length() > 1e-6 {
		t.Errorf("Hit point not on ray: expected %v, got %v", expectedPoint, hit.Point)
	}
}
