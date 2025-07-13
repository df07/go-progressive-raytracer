package renderer

import (
	"image"
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

	// Test tile extraction
	bounds := image.Rect(0, 0, 100, 100)
	tileSplats := queue.ExtractSplatsForTile(bounds)

	// Should extract 2 splats that fall within bounds (10,20) and (50,60)
	// (100,150) is outside bounds
	if len(tileSplats) != 2 {
		t.Errorf("Expected 2 splats in tile, got %d", len(tileSplats))
	}

	// Check remaining splats
	if count := queue.GetSplatCount(); count != 1 {
		t.Errorf("Expected 1 remaining splat, got %d", count)
	}

	// Extract remaining splats
	largeBounds := image.Rect(0, 0, 200, 200)
	remainingSplats := queue.ExtractSplatsForTile(largeBounds)

	if len(remainingSplats) != 1 {
		t.Errorf("Expected 1 remaining splat, got %d", len(remainingSplats))
	}

	// Queue should be empty
	if count := queue.GetSplatCount(); count != 0 {
		t.Errorf("Expected empty queue after extraction, got %d", count)
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
