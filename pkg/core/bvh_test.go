package core

import (
	"math"
	"testing"
)

// MockShape for testing
type MockShape struct {
	boundingBox AABB
	hitFn       func(ray Ray, tMin, tMax float64) (*HitRecord, bool)
}

func (m MockShape) Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
	return m.hitFn(ray, tMin, tMax)
}

func (m MockShape) BoundingBox() AABB {
	return m.boundingBox
}

func TestBVH_LeafThresholdBoundary(t *testing.T) {
	// Test behavior around the leaf threshold (8 shapes)

	// Create exactly leafThreshold shapes - should create single leaf
	shapes := make([]Shape, 8)
	for i := 0; i < 8; i++ {
		shapes[i] = MockShape{
			boundingBox: NewAABB(NewVec3(float64(i), 0, 0), NewVec3(float64(i)+1, 1, 1)),
			hitFn: func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
				return nil, false // Never hit for simplicity
			},
		}
	}

	bvh := NewBVH(shapes)
	stats := bvh.getStats()

	// Should have exactly 1 node (single leaf)
	if stats.totalNodes != 1 {
		t.Errorf("Expected 1 node for %d shapes, got %d", len(shapes), stats.totalNodes)
	}
	if stats.leafNodes != 1 {
		t.Errorf("Expected 1 leaf node for %d shapes, got %d", len(shapes), stats.leafNodes)
	}

	// Test with leafThreshold + 1 shapes - should split
	shapes = append(shapes, MockShape{
		boundingBox: NewAABB(NewVec3(8, 0, 0), NewVec3(9, 1, 1)),
		hitFn: func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
			return nil, false
		},
	})

	bvh = NewBVH(shapes)
	stats = bvh.getStats()

	// Should have more than 1 node (split occurred)
	if stats.totalNodes == 1 {
		t.Errorf("Expected split for %d shapes, but got single node", len(shapes))
	}
	if stats.leafNodes < 2 {
		t.Errorf("Expected at least 2 leaf nodes after split, got %d", stats.leafNodes)
	}
}

func TestBVH_EmptyAndSingleShape(t *testing.T) {
	// Test empty BVH
	bvh := NewBVH([]Shape{})
	if bvh.Root != nil {
		t.Error("Expected nil root for empty BVH")
	}

	ray := NewRay(NewVec3(0, 0, 0), NewVec3(1, 0, 0))
	hit, isHit := bvh.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Error("Expected no hit for empty BVH")
	}
	if hit != nil {
		t.Error("Expected nil hit record for empty BVH")
	}

	// Test single shape BVH
	shape := MockShape{
		boundingBox: NewAABB(NewVec3(0, 0, 0), NewVec3(1, 1, 1)),
		hitFn: func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
			return &HitRecord{T: 1.0}, true
		},
	}

	bvh = NewBVH([]Shape{shape})
	stats := bvh.getStats()

	if stats.totalNodes != 1 {
		t.Errorf("Expected 1 node for single shape, got %d", stats.totalNodes)
	}
	if stats.leafNodes != 1 {
		t.Errorf("Expected 1 leaf node for single shape, got %d", stats.leafNodes)
	}
}

func TestBVH_MultipleHitsInLeaf(t *testing.T) {
	// Test that BVH correctly finds closest hit when multiple shapes in leaf hit

	// Helper function to create hit function with specific t value
	makeHitFn := func(tValue float64) func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
		return func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
			if ray.Direction.X > 0 && tValue >= tMin && tValue <= tMax {
				return &HitRecord{T: tValue}, true
			}
			return nil, false
		}
	}

	// Create shapes that will be in same leaf (close together)
	shapes := []Shape{
		MockShape{
			boundingBox: NewAABB(NewVec3(0, 0, 0), NewVec3(1, 1, 1)),
			hitFn:       makeHitFn(2.0), // Hit at t = 2.0
		},
		MockShape{
			boundingBox: NewAABB(NewVec3(0.5, 0, 0), NewVec3(1.5, 1, 1)),
			hitFn:       makeHitFn(1.0), // Hit at t = 1.0 (closer)
		},
		MockShape{
			boundingBox: NewAABB(NewVec3(1.0, 0, 0), NewVec3(2.0, 1, 1)),
			hitFn:       makeHitFn(3.0), // Hit at t = 3.0 (farther)
		},
	}

	bvh := NewBVH(shapes)
	ray := NewRay(NewVec3(-1, 0.5, 0.5), NewVec3(1, 0, 0))

	hit, isHit := bvh.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit")
	}

	// Should return the closest hit (t = 1.0)
	if math.Abs(hit.T-1.0) > 1e-9 {
		t.Errorf("Expected closest hit at t=1.0, got t=%f", hit.T)
	}
}

func TestBVH_RayHitsBoundingBoxButMissesShapes(t *testing.T) {
	// Test case where ray hits the bounding box but misses all shapes inside

	shape := MockShape{
		boundingBox: NewAABB(NewVec3(0, 0, 0), NewVec3(2, 2, 2)),
		hitFn: func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
			// Shape occupies only a small part of its bounding box
			// Ray hits bounding box but misses actual shape
			return nil, false
		},
	}

	bvh := NewBVH([]Shape{shape})

	// Ray that goes through the bounding box but misses the shape
	ray := NewRay(NewVec3(-1, 1, 1), NewVec3(1, 0, 0))

	hit, isHit := bvh.Hit(ray, 0.001, 1000.0)
	if isHit {
		t.Error("Expected miss when ray hits bounding box but misses shape")
	}
	if hit != nil {
		t.Error("Expected nil hit record when no shapes are hit")
	}
}

func TestBVH_StatsCollection(t *testing.T) {
	// Test that BVH statistics are collected correctly

	// Create enough shapes to force multiple levels
	shapes := make([]Shape, 20)
	for i := 0; i < 20; i++ {
		shapes[i] = MockShape{
			boundingBox: NewAABB(NewVec3(float64(i), 0, 0), NewVec3(float64(i)+1, 1, 1)),
			hitFn: func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
				return nil, false
			},
		}
	}

	bvh := NewBVH(shapes)
	stats := bvh.getStats()

	// Verify basic properties
	if stats.totalShapes != 20 {
		t.Errorf("Expected 20 total shapes, got %d", stats.totalShapes)
	}

	if stats.leafNodes == 0 {
		t.Error("Expected at least one leaf node")
	}

	if stats.totalNodes < stats.leafNodes {
		t.Error("Total nodes should be >= leaf nodes")
	}

	if stats.maxDepth < 0 {
		t.Error("Max depth should be non-negative")
	}

	// For 20 shapes with leaf threshold 8, we should have multiple levels
	if stats.maxDepth == 0 {
		t.Error("Expected max depth > 0 for 20 shapes")
	}
}

func TestBVH_IdenticalBoundingBoxes(t *testing.T) {
	// Test edge case where multiple shapes have identical bounding boxes

	sameBoundingBox := NewAABB(NewVec3(0, 0, 0), NewVec3(1, 1, 1))
	shapes := make([]Shape, 5)

	// Helper function to create hit function with specific t value
	makeHitFn := func(tValue float64) func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
		return func(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
			if ray.Direction.X > 0 && tValue >= tMin && tValue <= tMax {
				return &HitRecord{T: tValue}, true
			}
			return nil, false
		}
	}

	for i := 0; i < 5; i++ {
		shapes[i] = MockShape{
			boundingBox: sameBoundingBox,
			hitFn:       makeHitFn(float64(i + 1)), // Each shape hits at different t values
		}
	}

	bvh := NewBVH(shapes)
	ray := NewRay(NewVec3(-1, 0.5, 0.5), NewVec3(1, 0, 0))

	hit, isHit := bvh.Hit(ray, 0.001, 1000.0)
	if !isHit {
		t.Fatal("Expected hit")
	}

	// Should return the closest hit (t = 1.0, from first shape)
	if math.Abs(hit.T-1.0) > 1e-9 {
		t.Errorf("Expected closest hit at t=1.0, got t=%f", hit.T)
	}
}
