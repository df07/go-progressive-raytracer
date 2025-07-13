package renderer

import (
	"image"
	"sync"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// SplatXY represents a splat with pre-computed pixel coordinates
type SplatXY struct {
	X, Y  int       // Pixel coordinates (computed when enqueuing)
	Color core.Vec3 // Color contribution
}

// SplatQueue provides thread-safe accumulation of splat contributions for BDPT t=1 strategies
type SplatQueue struct {
	splats []SplatXY
	mutex  sync.Mutex
}

// NewSplatQueue creates a new splat queue with pre-allocated buffer
func NewSplatQueue() *SplatQueue {
	return &SplatQueue{
		splats: make([]SplatXY, 0, 1000), // Pre-allocate buffer to reduce allocations
	}
}

// AddSplat adds a splat contribution to the queue in a thread-safe manner
func (sq *SplatQueue) AddSplat(x, y int, color core.Vec3) {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	sq.splats = append(sq.splats, SplatXY{X: x, Y: y, Color: color})
}

// ExtractSplatsForTile removes and returns splats affecting this tile
// This is called by tile renderers to collect splats that need to be applied to their tile
func (sq *SplatQueue) ExtractSplatsForTile(bounds image.Rectangle) []SplatXY {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()

	var tileSplats []SplatXY
	var remaining []SplatXY

	for _, splat := range sq.splats {
		if splat.X >= bounds.Min.X && splat.X < bounds.Max.X &&
			splat.Y >= bounds.Min.Y && splat.Y < bounds.Max.Y {
			tileSplats = append(tileSplats, splat)
		} else {
			remaining = append(remaining, splat)
		}
	}

	sq.splats = remaining
	return tileSplats
}

// GetSplatCount returns the current number of pending splats (for debugging/monitoring)
func (sq *SplatQueue) GetSplatCount() int {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	return len(sq.splats)
}

// Clear removes all pending splats (useful for cleanup between frames)
func (sq *SplatQueue) Clear() {
	sq.mutex.Lock()
	defer sq.mutex.Unlock()
	sq.splats = sq.splats[:0] // Keep allocated capacity but reset length
}
