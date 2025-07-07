package geometry

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// MockMaterial for testing
type MockTriangleMaterial struct{}

func (m MockTriangleMaterial) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	return core.ScatterResult{}, false
}

func (m MockTriangleMaterial) EvaluateBRDF(incomingDir, outgoingDir, normal core.Vec3) core.Vec3 {
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

func (m MockTriangleMaterial) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	return 0.0, false
}

func TestTriangle_Hit(t *testing.T) {
	// Create a triangle in the XY plane
	v0 := core.NewVec3(0, 0, 0)
	v1 := core.NewVec3(1, 0, 0)
	v2 := core.NewVec3(0, 1, 0)
	triangle := NewTriangle(v0, v1, v2, MockTriangleMaterial{})

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
	triangle := NewTriangle(v0, v1, v2, MockTriangleMaterial{})

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

func TestTriangleMesh_Creation(t *testing.T) {
	// Create a simple quad mesh (2 triangles)
	vertices := []core.Vec3{
		core.NewVec3(0, 0, 0), // 0
		core.NewVec3(1, 0, 0), // 1
		core.NewVec3(1, 1, 0), // 2
		core.NewVec3(0, 1, 0), // 3
	}

	faces := []int{
		0, 1, 2, // first triangle
		0, 2, 3, // second triangle
	}

	mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, nil)

	if mesh.GetTriangleCount() != 2 {
		t.Errorf("Expected 2 triangles, got %d", mesh.GetTriangleCount())
	}

	// Test bounding box
	bbox := mesh.BoundingBox()
	expectedMin := core.NewVec3(0, 0, 0)
	expectedMax := core.NewVec3(1, 1, 0)

	const tolerance = 1e-9
	if bbox.Min.Subtract(expectedMin).Length() > tolerance {
		t.Errorf("Expected min %v, got %v", expectedMin, bbox.Min)
	}
	if bbox.Max.Subtract(expectedMax).Length() > tolerance {
		t.Errorf("Expected max %v, got %v", expectedMax, bbox.Max)
	}
}

func TestTriangleMesh_Hit(t *testing.T) {
	// Create a simple quad mesh (2 triangles)
	vertices := []core.Vec3{
		core.NewVec3(0, 0, 0), // 0
		core.NewVec3(1, 0, 0), // 1
		core.NewVec3(1, 1, 0), // 2
		core.NewVec3(0, 1, 0), // 3
	}

	faces := []int{
		0, 1, 2, // first triangle
		0, 2, 3, // second triangle
	}

	mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, nil)

	tests := []struct {
		name      string
		ray       core.Ray
		shouldHit bool
	}{
		{
			name: "Ray hits center of quad",
			ray: core.NewRay(
				core.NewVec3(0.5, 0.5, -1), // origin
				core.NewVec3(0, 0, 1),      // direction
			),
			shouldHit: true,
		},
		{
			name: "Ray hits corner",
			ray: core.NewRay(
				core.NewVec3(0, 0, -1), // origin
				core.NewVec3(0, 0, 1),  // direction
			),
			shouldHit: true,
		},
		{
			name: "Ray misses quad",
			ray: core.NewRay(
				core.NewVec3(2, 2, -1), // origin (outside quad)
				core.NewVec3(0, 0, 1),  // direction
			),
			shouldHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hit, isHit := mesh.Hit(tt.ray, 0.001, 10.0)

			if isHit != tt.shouldHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.shouldHit, isHit)
			}

			if tt.shouldHit && hit == nil {
				t.Error("Expected hit record, got nil")
			}
		})
	}
}

func TestTriangleMesh_ErrorHandling(t *testing.T) {
	vertices := []core.Vec3{
		core.NewVec3(0, 0, 0),
		core.NewVec3(1, 0, 0),
		core.NewVec3(0, 1, 0),
	}

	// Test invalid face count (not multiple of 3)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for invalid face count")
		}
	}()

	invalidFaces := []int{0, 1} // Only 2 indices, not a multiple of 3
	NewTriangleMesh(vertices, invalidFaces, MockTriangleMaterial{}, nil)
}
