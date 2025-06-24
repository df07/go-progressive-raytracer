package renderer

import (
	"runtime"
	"sync"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TileTask represents a tile rendering task for the worker pool
type TileTask struct {
	Tile          *Tile
	PassNumber    int
	TargetSamples int
	TaskID        int            // For deterministic ordering
	PixelStats    [][]PixelStats // Shared pixel stats array to write to
}

// TileResult contains the result from rendering a tile
type TileResult struct {
	TaskID int
	Stats  RenderStats
	Error  error
}

// WorkerPool manages parallel tile rendering
type WorkerPool struct {
	taskQueue   chan TileTask
	resultQueue chan TileResult
	workers     []*Worker
	numWorkers  int
	wg          sync.WaitGroup
	stopChan    chan bool
}

// Worker handles individual tile rendering tasks
type Worker struct {
	ID          int
	raytracer   *Raytracer
	taskQueue   chan TileTask
	resultQueue chan TileResult
	stopChan    chan bool
	pool        *WorkerPool // Reference to parent pool for callback access
}

// NewWorkerPool creates a worker pool with the specified number of workers
func NewWorkerPool(scene core.Scene, width, height, tileSize int, numWorkers int) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	// Calculate maximum number of tiles we might have
	// Assume worst case of 8x8 tile size for buffer calculation
	maxTiles := ((width + 7) / 8) * ((height + 7) / 8)

	wp := &WorkerPool{
		taskQueue:   make(chan TileTask, maxTiles),   // Buffer for all possible tiles
		resultQueue: make(chan TileResult, maxTiles), // Buffer for all possible results
		numWorkers:  numWorkers,
		stopChan:    make(chan bool),
	}

	// Create workers
	for i := 0; i < numWorkers; i++ {
		worker := &Worker{
			ID:          i,
			raytracer:   NewRaytracer(scene, width, height),
			taskQueue:   wp.taskQueue,
			resultQueue: wp.resultQueue,
			stopChan:    wp.stopChan,
			pool:        wp,
		}
		wp.workers = append(wp.workers, worker)
	}

	return wp
}

// Start begins all workers
func (wp *WorkerPool) Start() {
	for _, worker := range wp.workers {
		wp.wg.Add(1)
		go worker.run(&wp.wg)
	}
}

// Stop gracefully shuts down all workers
func (wp *WorkerPool) Stop() {
	close(wp.taskQueue) // No more tasks
	wp.wg.Wait()        // Wait for workers to finish
	close(wp.resultQueue)
}

// SubmitTask submits a tile task to the worker pool
func (wp *WorkerPool) SubmitTask(task TileTask) {
	wp.taskQueue <- task
}

// GetResult retrieves a completed tile result
func (wp *WorkerPool) GetResult() (TileResult, bool) {
	result, ok := <-wp.resultQueue
	return result, ok
}

// GetNumWorkers returns the number of workers in the pool
func (wp *WorkerPool) GetNumWorkers() int {
	return wp.numWorkers
}

// run is the main worker loop
func (w *Worker) run(wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range w.taskQueue {
		// Configure raytracer for this pass, merging only the target samples
		w.raytracer.MergeSamplingConfig(core.SamplingConfig{
			SamplesPerPixel: task.TargetSamples,
		})

		// Render the tile directly to the shared pixel stats array
		// Each tile has non-overlapping bounds, so this is thread-safe
		stats := w.raytracer.RenderBounds(task.Tile.Bounds, task.PixelStats, task.Tile.Random)

		// Send result back with just the stats
		result := TileResult{
			TaskID: task.TaskID,
			Stats:  stats,
			Error:  nil,
		}

		w.resultQueue <- result
	}
}
