package renderer

import (
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestSplatQueue(t *testing.T) {
	queue := NewSplatQueue()

	// Test initial state
	if count := queue.GetSplatCount(); count != 0 {
		t.Errorf("Expected empty queue, got %d splats", count)
	}

	// Add some splats
	queue.AddSplat(10, 20, core.Vec3{X: 0.5, Y: 0.3, Z: 0.1})
	queue.AddSplat(50, 60, core.Vec3{X: 0.8, Y: 0.2, Z: 0.4})
	queue.AddSplat(100, 150, core.Vec3{X: 0.1, Y: 0.9, Z: 0.6})

	// Check count
	if count := queue.GetSplatCount(); count != 3 {
		t.Errorf("Expected 3 splats, got %d", count)
	}

	// Test getting all splats (new post-processing workflow)
	allSplats := queue.GetAllSplats()

	// Should get all 3 splats
	if len(allSplats) != 3 {
		t.Errorf("Expected 3 splats from GetAllSplats, got %d", len(allSplats))
	}

	// Verify splat data
	expectedSplats := []SplatXY{
		{X: 10, Y: 20, Color: core.Vec3{X: 0.5, Y: 0.3, Z: 0.1}},
		{X: 50, Y: 60, Color: core.Vec3{X: 0.8, Y: 0.2, Z: 0.4}},
		{X: 100, Y: 150, Color: core.Vec3{X: 0.1, Y: 0.9, Z: 0.6}},
	}

	for i, expected := range expectedSplats {
		if i >= len(allSplats) {
			t.Errorf("Missing expected splat at index %d", i)
			continue
		}
		actual := allSplats[i]
		if actual.X != expected.X || actual.Y != expected.Y ||
			actual.Color.X != expected.Color.X || actual.Color.Y != expected.Color.Y || actual.Color.Z != expected.Color.Z {
			t.Errorf("Splat %d mismatch: expected %+v, got %+v", i, expected, actual)
		}
	}

	// GetAllSplats should not modify the queue
	if count := queue.GetSplatCount(); count != 3 {
		t.Errorf("Expected 3 splats after GetAllSplats, got %d", count)
	}

	// Clear should empty the queue
	queue.Clear()
	if count := queue.GetSplatCount(); count != 0 {
		t.Errorf("Expected empty queue after clear, got %d", count)
	}
}

func TestSplatQueueClear(t *testing.T) {
	queue := NewSplatQueue()

	// Add splats
	queue.AddSplat(10, 20, core.Vec3{X: 0.5, Y: 0.3, Z: 0.1})
	queue.AddSplat(50, 60, core.Vec3{X: 0.8, Y: 0.2, Z: 0.4})

	if count := queue.GetSplatCount(); count != 2 {
		t.Errorf("Expected 2 splats, got %d", count)
	}

	// Clear queue
	queue.Clear()

	if count := queue.GetSplatCount(); count != 0 {
		t.Errorf("Expected empty queue after clear, got %d", count)
	}
}

func TestSplatQueueConcurrency(t *testing.T) {
	queue := NewSplatQueue()

	// Test concurrent adds
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				queue.AddSplat(id*10+j, id*10+j, core.Vec3{X: float64(id), Y: float64(j), Z: 0.5})
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 100 splats total
	if count := queue.GetSplatCount(); count != 100 {
		t.Errorf("Expected 100 splats from concurrent adds, got %d", count)
	}
}
