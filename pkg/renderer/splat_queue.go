package renderer

import (
	"sync"
	"sync/atomic"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// SplatXY represents a splat with pre-computed pixel coordinates
type SplatXY struct {
	X, Y  int       // Pixel coordinates (computed when enqueuing)
	Color core.Vec3 // Color contribution
}

// SplatQueue provides mostly lock-free accumulation of splat contributions for BDPT t=1 strategies
type SplatQueue struct {
	splats []SplatXY  // Pre-allocated buffer for lock-free appends
	length int64      // Atomic counter for current length
	mu     sync.Mutex // Mutex only for growing the buffer when full
}

// NewSplatQueue creates a new splat queue with pre-allocated buffer
func NewSplatQueue() *SplatQueue {
	return &SplatQueue{
		splats: make([]SplatXY, 50000), // Start with reasonable buffer size
		length: 0,
	}
}

// AddSplat adds a splat contribution to the queue with lock-free fast path
func (sq *SplatQueue) AddSplat(x, y int, color core.Vec3) {
	// Fast path: try to append lock-free
	index := atomic.AddInt64(&sq.length, 1) - 1

	if int(index) < len(sq.splats) {
		// Fast path: write directly to pre-allocated buffer
		sq.splats[index] = SplatXY{X: x, Y: y, Color: color}
	} else {
		// Slow path: buffer is full, need to grow
		sq.mu.Lock()
		defer sq.mu.Unlock()

		// Double-check after acquiring lock (another thread might have grown it)
		if int(index) >= len(sq.splats) {
			// Grow buffer by 2x
			newSize := len(sq.splats) * 2
			newSplats := make([]SplatXY, newSize)
			copy(newSplats, sq.splats)
			sq.splats = newSplats
		}

		// Now write to the buffer
		sq.splats[index] = SplatXY{X: x, Y: y, Color: color}
	}
}

// GetAllSplats returns a copy of all pending splats without removing them
func (sq *SplatQueue) GetAllSplats() []SplatXY {
	// Need to lock to ensure consistent view of length and slice
	sq.mu.Lock()
	defer sq.mu.Unlock()

	currentLength := atomic.LoadInt64(&sq.length)
	result := make([]SplatXY, currentLength)
	copy(result, sq.splats[:currentLength])
	return result
}

// GetSplatCount returns the current number of pending splats (for debugging/monitoring)
func (sq *SplatQueue) GetSplatCount() int {
	return int(atomic.LoadInt64(&sq.length))
}

// Clear removes all pending splats (useful for cleanup between frames)
func (sq *SplatQueue) Clear() {
	atomic.StoreInt64(&sq.length, 0) // Reset length atomically
	// Note: We don't need to zero out the array since length controls access
}
