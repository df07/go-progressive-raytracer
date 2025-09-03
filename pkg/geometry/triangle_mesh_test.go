package geometry

import (
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

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

func TestTriangleMesh_WithCustomNormals(t *testing.T) {
	// Create a simple triangle with custom normal
	vertices := []core.Vec3{
		core.NewVec3(0, 0, 0),
		core.NewVec3(1, 0, 0),
		core.NewVec3(0, 1, 0),
	}

	faces := []int{0, 1, 2}

	// Custom normal pointing in negative Z direction
	customNormal := core.NewVec3(0, 0, -1)
	options := &TriangleMeshOptions{
		Normals: []core.Vec3{customNormal},
	}

	mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, options)

	if mesh.GetTriangleCount() != 1 {
		t.Errorf("Expected 1 triangle, got %d", mesh.GetTriangleCount())
	}

	// Test that ray hits the triangle from the custom normal direction
	ray := core.NewRay(
		core.NewVec3(0.3, 0.3, 1), // origin in positive Z
		core.NewVec3(0, 0, -1),    // direction toward negative Z
	)

	hit, isHit := mesh.Hit(ray, 0.001, 10.0)
	if !isHit {
		t.Error("Expected hit with custom normal")
	}

	// Verify the normal is the custom one we specified
	if hit != nil && hit.Normal.Subtract(customNormal.Multiply(-1)).Length() > 1e-6 {
		t.Errorf("Expected hit normal %v, got %v", customNormal.Multiply(-1), hit.Normal)
	}
}

func TestTriangleMesh_WithPerTriangleMaterials(t *testing.T) {
	// Create a quad with two different materials
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

	material1 := MockTriangleMaterial{}
	material2 := MockTriangleMaterial{}

	options := &TriangleMeshOptions{
		Materials: []material.Material{material1, material2},
	}

	mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, options)

	if mesh.GetTriangleCount() != 2 {
		t.Errorf("Expected 2 triangles, got %d", mesh.GetTriangleCount())
	}

	// Test hitting the first triangle
	ray1 := core.NewRay(
		core.NewVec3(0.8, 0.1, -1), // hits first triangle
		core.NewVec3(0, 0, 1),
	)

	hit1, isHit1 := mesh.Hit(ray1, 0.001, 10.0)
	if !isHit1 {
		t.Error("Expected hit on first triangle")
	}

	// Test hitting the second triangle
	ray2 := core.NewRay(
		core.NewVec3(0.1, 0.8, -1), // hits second triangle
		core.NewVec3(0, 0, 1),
	)

	hit2, isHit2 := mesh.Hit(ray2, 0.001, 10.0)
	if !isHit2 {
		t.Error("Expected hit on second triangle")
	}

	// Both should have valid hit records with materials
	if hit1 == nil || hit1.Material == nil {
		t.Error("Expected valid hit record with material for first triangle")
	}
	if hit2 == nil || hit2.Material == nil {
		t.Error("Expected valid hit record with material for second triangle")
	}
}

func TestTriangleMesh_GetTriangles(t *testing.T) {
	// Create a simple quad mesh
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

	triangles := mesh.GetTriangles()
	if len(triangles) != 2 {
		t.Errorf("Expected 2 triangles from GetTriangles(), got %d", len(triangles))
	}

	// Verify each returned shape is actually a triangle
	for i, shape := range triangles {
		if _, ok := shape.(*Triangle); !ok {
			t.Errorf("Triangle %d is not a Triangle type", i)
		}
	}
}

func TestTriangleMesh_ComplexGeometry(t *testing.T) {
	// Create a more complex mesh - a pyramid
	vertices := []core.Vec3{
		core.NewVec3(0, 0, 0),     // 0 - base corner
		core.NewVec3(1, 0, 0),     // 1 - base corner
		core.NewVec3(1, 0, 1),     // 2 - base corner
		core.NewVec3(0, 0, 1),     // 3 - base corner
		core.NewVec3(0.5, 1, 0.5), // 4 - apex
	}

	faces := []int{
		// Base (2 triangles)
		0, 1, 2,
		0, 2, 3,
		// Sides (4 triangles)
		0, 4, 1,
		1, 4, 2,
		2, 4, 3,
		3, 4, 0,
	}

	mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, nil)

	if mesh.GetTriangleCount() != 6 {
		t.Errorf("Expected 6 triangles in pyramid, got %d", mesh.GetTriangleCount())
	}

	// Test bounding box includes all vertices
	bbox := mesh.BoundingBox()
	if bbox.Min.X > 0 || bbox.Min.Y > 0 || bbox.Min.Z > 0 {
		t.Errorf("Bounding box min should be at origin, got %v", bbox.Min)
	}
	if bbox.Max.X < 1 || bbox.Max.Y < 1 || bbox.Max.Z < 1 {
		t.Errorf("Bounding box max should include all vertices, got %v", bbox.Max)
	}

	// Test ray hitting the pyramid from different angles
	testRays := []struct {
		name      string
		ray       core.Ray
		shouldHit bool
	}{
		{
			name: "Ray hits base from below",
			ray: core.NewRay(
				core.NewVec3(0.5, -1, 0.5),
				core.NewVec3(0, 1, 0),
			),
			shouldHit: true,
		},
		{
			name: "Ray hits side face",
			ray: core.NewRay(
				core.NewVec3(0.5, 0.5, -1),
				core.NewVec3(0, 0, 1),
			),
			shouldHit: true,
		},
		{
			name: "Ray misses pyramid completely",
			ray: core.NewRay(
				core.NewVec3(2, 0.5, 0.5),
				core.NewVec3(1, 0, 0),
			),
			shouldHit: false,
		},
	}

	for _, tt := range testRays {
		t.Run(tt.name, func(t *testing.T) {
			hit, isHit := mesh.Hit(tt.ray, 0.001, 10.0)

			if isHit != tt.shouldHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.shouldHit, isHit)
			}

			if tt.shouldHit && hit == nil {
				t.Error("Expected hit record, got nil")
			}

			if tt.shouldHit && hit != nil {
				// Verify hit point is reasonable
				if hit.T <= 0 {
					t.Errorf("Expected positive t value, got %f", hit.T)
				}
			}
		})
	}
}

func TestTriangleMesh_EdgeCases(t *testing.T) {
	t.Run("Empty mesh", func(t *testing.T) {
		// Test with empty face array
		vertices := []core.Vec3{
			core.NewVec3(0, 0, 0),
			core.NewVec3(1, 0, 0),
			core.NewVec3(0, 1, 0),
		}
		faces := []int{} // Empty faces

		mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, nil)

		if mesh.GetTriangleCount() != 0 {
			t.Errorf("Expected 0 triangles for empty faces, got %d", mesh.GetTriangleCount())
		}

		// Should not hit anything
		ray := core.NewRay(
			core.NewVec3(0.5, 0.5, -1),
			core.NewVec3(0, 0, 1),
		)

		hit, isHit := mesh.Hit(ray, 0.001, 10.0)
		if isHit {
			t.Error("Expected no hit for empty mesh")
		}
		if hit != nil {
			t.Error("Expected nil hit record for empty mesh")
		}
	})

	t.Run("Single triangle", func(t *testing.T) {
		vertices := []core.Vec3{
			core.NewVec3(0, 0, 0),
			core.NewVec3(1, 0, 0),
			core.NewVec3(0, 1, 0),
		}
		faces := []int{0, 1, 2}

		mesh := NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, nil)

		if mesh.GetTriangleCount() != 1 {
			t.Errorf("Expected 1 triangle, got %d", mesh.GetTriangleCount())
		}

		// Should hit the single triangle
		ray := core.NewRay(
			core.NewVec3(0.3, 0.3, -1),
			core.NewVec3(0, 0, 1),
		)

		hit, isHit := mesh.Hit(ray, 0.001, 10.0)
		if !isHit {
			t.Error("Expected hit for single triangle")
		}
		if hit == nil {
			t.Error("Expected valid hit record for single triangle")
		}
	})

	t.Run("Invalid options validation", func(t *testing.T) {
		vertices := []core.Vec3{
			core.NewVec3(0, 0, 0),
			core.NewVec3(1, 0, 0),
			core.NewVec3(0, 1, 0),
		}
		faces := []int{0, 1, 2}

		// Test with mismatched normals count
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for mismatched normals count")
			}
		}()

		options := &TriangleMeshOptions{
			Normals: []core.Vec3{
				core.NewVec3(0, 0, 1),
				core.NewVec3(0, 0, 1), // Too many normals for 1 triangle
			},
		}

		NewTriangleMesh(vertices, faces, MockTriangleMaterial{}, options)
	})
}
