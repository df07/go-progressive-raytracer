# Progressive Parallel Raytracer Architecture Spec

## Overview

This spec outlines the architecture for transforming our raytracer into a progressive, parallel system capable of:
- **Progressive Rendering**: Multiple passes with increasing quality
- **Parallel Processing**: Efficient utilization of multiple CPU cores
- **Tile-Based Rendering**: Image divided into tiles for better parallelization
- **Work Stealing**: Dynamic load balancing across worker threads
- **Adaptive Quality**: More samples in areas with higher error/variance

## Goals

1. **Progressive Enhancement**: Quickly display a low-quality preview, then progressively improve
2. **Parallel Efficiency**: Scale rendering performance with available CPU cores
3. **Responsive UI**: Allow intermediate results to be displayed/saved during rendering
4. **Memory Efficiency**: Minimize memory usage while maintaining quality
5. **Deterministic Results**: Same scene configuration produces identical results
6. **Backward Compatibility**: Existing scene/material system remains unchanged

## Key Architectural Changes

### 1. Tile-Based Rendering System

**Current**: Single-threaded, pixel-by-pixel rendering
**New**: Divide image into tiles, render tiles in parallel

```
Image: 400x400 pixels
Tile Size: 64x64 pixels  
Result: 7x7 = 49 tiles total
```

**Tile Structure**:
```go
type Tile struct {
    ID          int           // Unique tile identifier
    Bounds      image.Rectangle // Pixel bounds (x0,y0,x1,y1)
    PassNumber  int           // Current pass number for this tile
    PixelStats  [][]PixelStats // Per-pixel statistics
    Random      *rand.Rand    // Tile-specific random generator for deterministic results
    IsCompleted bool          // Has this tile been rendered at least once?
    LastRender  time.Time     // When this tile was last completed
}
```

### 2. Progressive Pass System

**Pass Strategy**: Each pass doubles the sample count
- Pass 1: 1 sample per pixel (fast preview)
- Pass 2: 2 samples per pixel (4 total)
- Pass 3: 4 samples per pixel (8 total)  
- Pass 4: 8 samples per pixel (16 total)
- Continue until quality threshold or max samples reached

**Progressive Accumulation**: Each pass adds to previous samples, maintaining all statistical data for proper Monte Carlo integration.

**Image Completion**: A complete image is returned only when ALL tiles have finished their current pass. Tiles that have never been rendered get maximum priority to ensure the first complete image is generated quickly.

### 3. Worker Pool Architecture

**Components**:
```go
type WorkerPool struct {
    Workers     []*Worker        // Pool of rendering workers
    TileQueue   chan *TileTask   // Queue of tiles waiting to be rendered
    ResultQueue chan *TileResult // Completed tile results
    NumWorkers  int              // Number of concurrent workers
    StopChan    chan bool        // Graceful shutdown signal
}

type Worker struct {
    ID          int              // Worker identifier
    Raytracer   *Raytracer      // Thread-local raytracer instance - wraps existing raytracer
}

type TileTask struct {
    Tile        *Tile           // Tile to render
    PassNumber  int             // Which pass to execute
    SamplesPerPixel int         // Samples to add this pass
}

type TileResult struct {
    Tile        *Tile           // Completed tile
    PassNumber  int             // Pass that was completed
    RenderTime  time.Duration   // Time taken to render
    SampleStats RenderStats     // Statistics for this tile
}
```

### 4. Progressive Raytracer Manager

**New Main Component**:
```go
type ProgressiveRaytracer struct {
    scene           core.Scene
    width, height   int
    tileSize        int
    
    // Tile management
    tiles           []*Tile
    tileGrid        [][]*Tile        // 2D grid for easy access
    
    // Parallel processing
    workerPool      *WorkerPool
    
    // Progressive state
    currentPass     int
    maxPasses       int
    qualityTarget   float64          // Stop when average error below this
    
    // Result management
    currentImage    *image.RGBA      // Current best image
    imageUpdated    chan bool        // Signal when image is updated
    
    // Configuration
    config          ProgressiveConfig
}

type ProgressiveConfig struct {
    TileSize                int     // Size of each tile (64x64 recommended)
    MaxWorkers              int     // Number of worker threads (0 = auto-detect)
    InitialSamples          int     // Samples for first pass (1 recommended)
    MaxSamplesPerPixel      int     // Maximum total samples per pixel
    MaxPasses               int     // Maximum number of passes
    AdaptiveThresholdStart  float64 // Initial adaptive sampling threshold (relaxed)
    AdaptiveThresholdEnd    float64 // Final adaptive sampling threshold (strict)
    SaveIntermediateResults bool    // Save images after each pass
}
```

## Detailed Component Design

### 1. Tile Management

**Simplified Tile Queuing**:
```go
func (pr *ProgressiveRaytracer) QueueTilesForPass(passNumber int) {
    // Queue incomplete tiles first (max priority)
    for _, tile := range pr.tiles {
        if !tile.IsCompleted {
            pr.workerPool.TileQueue <- &TileTask{
                Tile:            tile,
                PassNumber:      passNumber,
                SamplesPerPixel: pr.getSamplesForPass(passNumber),
            }
        }
    }
    
    // Then queue completed tiles for additional passes
    for _, tile := range pr.tiles {
        if tile.IsCompleted && tile.PassNumber < passNumber {
            pr.workerPool.TileQueue <- &TileTask{
                Tile:            tile,
                PassNumber:      passNumber,
                SamplesPerPixel: pr.getSamplesForPass(passNumber),
            }
        }
    }
}
```

**Tile Queue Management**:
- Simple queue - incomplete tiles first, then completed tiles
- No complex priority calculations (let adaptive sampling handle convergence)
- Deterministic ordering for consistent results

### 2. Worker Thread Design

**Thread Safety Considerations**:
- Each tile has its own `*rand.Rand` instance with deterministic seed based on tile ID
- Scene data is read-only, safe for concurrent access
- Tile data is owned by single worker during rendering
- Results communicated through Go channels (thread-safe message passing)

**Worker Render Loop**:
```go
func (w *Worker) Run(taskQueue <-chan *TileTask, resultQueue chan<- *TileResult) {
    for task := range taskQueue {
        startTime := time.Now()
        
        // Use the tile's random generator for deterministic results
        w.Raytracer.SetRandom(task.Tile.Random)
        
        // Render the tile for this pass using existing raytracer
        w.renderTilePass(task.Tile, task.PassNumber, task.SamplesPerPixel)
        
        // Mark tile as completed after first pass
        if task.PassNumber == 1 {
            task.Tile.IsCompleted = true
        }
        
        // Send result back via channel
        result := &TileResult{
            Tile:       task.Tile,
            PassNumber: task.PassNumber,
            RenderTime: time.Since(startTime),
        }
        
        resultQueue <- result
    }
}
```

### 3. Progressive Image Assembly

**Image Update Strategy**:
- Maintain single `*image.RGBA` for current best result
- Update pixels when tile completes
- Signal UI/saver when image is updated
- Thread-safe updates using mutex

**Simplified Convergence Detection**:
```go
func (pr *ProgressiveRaytracer) CheckPassComplete() bool {
    // A pass is complete when all tiles have finished their current pass
    for _, tile := range pr.tiles {
        if tile.PassNumber < pr.currentPass {
            return false // This tile hasn't finished the current pass
        }
    }
    return true
}

func (pr *ProgressiveRaytracer) CheckGlobalConvergence() bool {
    // Simple convergence: check if we've reached max passes or max samples
    samplesPerPixel := pr.getSamplesForPass(pr.currentPass)
    return pr.currentPass >= pr.config.MaxPasses || 
           samplesPerPixel >= pr.config.MaxSamplesPerPixel
}
```

## API Design

### 1. Main Entry Points

```go
// Create progressive raytracer
func NewProgressiveRaytracer(scene core.Scene, width, height int, config ProgressiveConfig) *ProgressiveRaytracer

// Start rendering with callback for intermediate results
func (pr *ProgressiveRaytracer) StartRendering(callback func(*image.RGBA, int, RenderStats))

// Get current best image (thread-safe)
func (pr *ProgressiveRaytracer) GetCurrentImage() *image.RGBA

// Stop rendering gracefully
func (pr *ProgressiveRaytracer) Stop()

// Block until rendering complete or stopped
func (pr *ProgressiveRaytracer) Wait() (*image.RGBA, RenderStats)
```

### 2. Callback System

**Progressive Updates**: Callback function called after each pass completion
```go
type ProgressCallback func(image *image.RGBA, passNumber int, stats RenderStats)

// Example usage:
raytracer.StartRendering(func(img *image.RGBA, pass int, stats RenderStats) {
    filename := fmt.Sprintf("progressive_pass_%02d.png", pass)
    saveImage(img, filename)
    
    fmt.Printf("Pass %d complete: %.1f avg samples, %.2f%% error\n", 
        pass, stats.AverageSamples, stats.AverageError*100)
})
```

### 3. Configuration Examples

**Fast Preview Mode**:
```go
config := ProgressiveConfig{
    TileSize:                64,
    MaxWorkers:              runtime.NumCPU(),
    InitialSamples:          1,
    MaxSamplesPerPixel:      32,
    QualityThreshold:        0.05, // 5% error acceptable
    SaveIntermediateResults: true,
}
```

**High Quality Mode**:
```go
config := ProgressiveConfig{
    TileSize:                32,  // Smaller tiles for better load balancing
    MaxWorkers:              runtime.NumCPU() * 2, // Hyperthreading
    InitialSamples:          2,
    MaxSamplesPerPixel:      500,
    QualityThreshold:        0.005, // 0.5% error
    SaveIntermediateResults: false,
}
```

## Implementation Plan (Incremental & Testable)

### Phase 1: Single-Threaded Tile System
**Goal**: Render image using tiles sequentially, produce identical results to current raytracer

**Deliverables**:
```go
// New files: pkg/renderer/tile.go
type Tile struct { /* basic structure */ }
func NewTileGrid(width, height, tileSize int) []*Tile
func (rt *Raytracer) RenderTile(tile *Tile, samplesPerPixel int) error

// Modified: pkg/renderer/raytracer.go  
func (rt *Raytracer) RenderWithTiles() (*image.RGBA, RenderStats)
```

**Testing**:
- Tile boundary calculations are correct
- Same scene renders identically with tiles vs without tiles
- All edge cases handled (partial tiles at image boundaries)
- Performance comparable to original (no significant overhead)

**CLI**: `go run main.go --mode=tiles` produces identical images

---

### Phase 2: Progressive Passes (Single-Threaded)
**Goal**: Add multiple passes with sample accumulation, still single-threaded

**Deliverables**:
```go
// New: pkg/renderer/progressive.go (basic version)
type ProgressiveRaytracer struct { /* simplified structure */ }
func (pr *ProgressiveRaytracer) RenderProgressive() []*image.RGBA // Returns slice of images
func (pr *ProgressiveRaytracer) RenderPass(passNumber int) (*image.RGBA, RenderStats)

// Enhanced: pkg/renderer/tile.go
func (t *Tile) AccumulateSamples(newSamples []core.Vec3)
func (t *Tile) GetCurrentImage() []color.RGBA
```

**Testing**:
- Pass 1 (1 sample) produces fast, noisy preview
- Pass 2 (4 total samples) shows improvement
- Pass 3 (8 total samples) shows further improvement
- Sample accumulation is mathematically correct
- Progressive images saved as `output/scene/pass_01.png`, `pass_02.png`, etc.

**CLI**: `go run main.go --mode=progressive --max-passes=3` produces sequence of improving images

---

### Phase 3: Multi-Threaded Worker Pool
**Goal**: Add parallelism while maintaining identical results

**Deliverables**:
```go
// New: pkg/renderer/worker.go
type WorkerPool struct { /* channel-based design */ }
type Worker struct { /* wraps existing raytracer */ }
func NewWorkerPool(numWorkers int, scene core.Scene) *WorkerPool
func (wp *WorkerPool) RenderTilesParallel(tiles []*Tile, pass int) error

// Enhanced: pkg/renderer/progressive.go
func (pr *ProgressiveRaytracer) EnableParallelism(numWorkers int)
```

**Testing**:
- Same deterministic results as Phase 2 (different tile random seeds)
- Performance scales with worker count (2x workers ≈ 2x speed)
- No race conditions or crashes under stress
- Worker cleanup handles graceful shutdown

**CLI**: `go run main.go --mode=progressive --workers=4` runs in parallel

---

### Phase 4: Adaptive Sampling Integration  
**Goal**: Variable adaptive thresholds across passes for optimal quality/speed

**Deliverables**:
```go
// Enhanced: pkg/renderer/raytracer.go
func (rt *Raytracer) SetAdaptiveThreshold(threshold float64)
func (rt *Raytracer) RenderTileAdaptive(tile *Tile, maxSamples int) (avgSamples float64)

// Enhanced: pkg/renderer/progressive.go  
func (pr *ProgressiveRaytracer) SetProgressiveThresholds(start, end float64)
```

**Testing**:
- Early passes use fewer samples per pixel (relaxed threshold)
- Later passes add samples only where needed (strict threshold)
- Overall sample count is reasonable (not excessive in smooth areas)
- Quality improves noticeably with each pass

**CLI**: `go run main.go --mode=progressive --adaptive-start=0.1 --adaptive-end=0.01`

---

### Phase 5: Production Polish
**Goal**: CLI integration, progress display, intermediate saves, error handling

**Deliverables**:
```go
// Enhanced: main.go
// Add --progressive flag with full configuration options
// Real-time progress display showing pass completion
// Automatic intermediate image saving
// Performance statistics and timing

// New: pkg/renderer/progress.go
type ProgressDisplay struct { /* real-time stats */ }
func (pd *ProgressDisplay) UpdateTileComplete(tileID int, samples float64)
func (pd *ProgressDisplay) UpdatePassComplete(pass int, totalTime time.Duration)
```

**Testing**:
- Graceful handling of Ctrl+C (saves current progress)
- Clear progress indicators show tile and pass completion
- Intermediate images saved with proper naming
- Memory usage stays reasonable for large images
- All error cases handled with helpful messages

**CLI**: Full-featured progressive raytracer with rich CLI options

## Testing Strategy for Each Phase

### Automated Tests
```bash
# After each phase, run full test suite
go test ./...

# Performance regression test  
go test -bench=. ./pkg/renderer/

# Memory leak detection
go test -race ./pkg/renderer/
```

### Visual Verification
```bash
# Render test scenes and compare
go run main.go --scene=cornell --output=phase1_cornell.png
go run main.go --scene=default --output=phase1_default.png

# Compare with reference images (manual inspection)
# Should be pixel-perfect identical until Phase 3
```

### Performance Benchmarks
```bash
# Measure performance impact of each phase
time go run main.go --scene=cornell --samples=50
# Phase 1: Should be within 10% of original
# Phase 2: May be slightly slower (more bookkeeping)  
# Phase 3: Should be significantly faster with multiple cores
```

This incremental approach ensures we can stop at any phase with working code, making debugging much easier and giving us confidence in each architectural layer.

## File Structure Changes

```
pkg/
├── renderer/
│   ├── raytracer.go              # Existing single-threaded raytracer
│   ├── progressive.go            # New ProgressiveRaytracer
│   ├── tile.go                   # Tile management
│   ├── worker.go                 # Worker pool implementation  
│   ├── priority_queue.go         # Priority queue for tiles
│   └── progressive_test.go       # Progressive rendering tests
```

## Testing Strategy

### 1. Unit Tests
- Tile boundary calculations
- Priority queue operations
- Worker thread safety
- Sample accumulation correctness

### 2. Integration Tests  
- Multi-pass convergence
- Image consistency across different tile sizes
- Worker pool scaling behavior
- Deterministic results with same seed

### 3. Performance Tests
- Scaling efficiency with worker count
- Memory usage under different configurations
- Tile size optimization
- Progressive vs single-pass comparison

## Benefits of This Architecture

1. **Immediate Visual Feedback**: Users see results within seconds
2. **Scalable Performance**: Automatically uses all available CPU cores
3. **Adaptive Quality**: Focuses compute power where it's needed most  
4. **Flexible Stopping**: Can stop at any quality level
5. **Memory Efficient**: Only stores necessary statistics, not full sample history
6. **Production Ready**: Handles edge cases, errors, and resource cleanup

## Integration with Existing Raytracer

### Wrapper Architecture
The `ProgressiveRaytracer` acts as an orchestrator that wraps the existing `Raytracer`:

```go
// Enhanced existing raytracer to support tile rendering
func (rt *Raytracer) RenderTile(bounds image.Rectangle, samplesPerPixel int) (*TileStats, error) {
    // Render only pixels within bounds
    // Use adaptive sampling with progressive threshold
    // Return tile-specific statistics
}

func (rt *Raytracer) SetRandom(random *rand.Rand) {
    rt.random = random // Allow deterministic seeding per tile
}

func (rt *Raytracer) SetAdaptiveThreshold(threshold float64) {
    // Adjust adaptive sampling threshold for progressive passes
}
```

### Go Channels Explained
Go channels provide thread-safe communication between goroutines (threads):

```go
// Channel creation
taskQueue := make(chan *TileTask, 100)    // Buffered channel, holds up to 100 tasks
resultQueue := make(chan *TileResult, 100) // Buffered channel for results

// Sending (blocks if channel is full)
taskQueue <- &TileTask{...}

// Receiving (blocks until data available)
task := <-taskQueue

// Non-blocking receive
select {
case result := <-resultQueue:
    // Process result
default:
    // No result available, continue
}
```

### Progressive Adaptive Sampling
The adaptive sampling threshold changes across passes:

```go
func (pr *ProgressiveRaytracer) getAdaptiveThreshold(passNumber int) float64 {
    // Linear interpolation from relaxed to strict threshold
    progress := float64(passNumber-1) / float64(pr.config.MaxPasses-1)
    return pr.config.AdaptiveThresholdStart * (1-progress) + 
           pr.config.AdaptiveThresholdEnd * progress
}
```

Early passes use relaxed thresholds (0.1 = 10% error OK), later passes use strict thresholds (0.01 = 1% error).

### Main Rendering Loop
```go
func (pr *ProgressiveRaytracer) StartRendering(callback ProgressCallback) {
    go func() { // Run in separate goroutine
        for pass := 1; pass <= pr.config.MaxPasses; pass++ {
            pr.currentPass = pass
            
            // Set adaptive threshold for this pass
            threshold := pr.getAdaptiveThreshold(pass)
            for _, worker := range pr.workerPool.Workers {
                worker.Raytracer.SetAdaptiveThreshold(threshold)
            }
            
            // Queue all tiles for this pass
            pr.QueueTilesForPass(pass)
            
            // Wait for all tiles to complete
            pr.waitForPassCompletion()
            
            // Assemble complete image and call callback
            image := pr.assembleImage()
            stats := pr.calculateStats()
            callback(image, pass, stats)
            
            // Check if we should stop
            if pr.CheckGlobalConvergence() {
                break
            }
        }
    }()
}
```

## Backward Compatibility

- Existing `Raytracer.RenderPass()` remains unchanged for single-pass rendering
- All scene, material, and geometry code works without changes  
- New CLI flags are optional, defaults to current behavior
- Progressive mode is opt-in, doesn't affect existing functionality
- Can mix and match: use ProgressiveRaytracer for previews, regular Raytracer for final renders

This architecture transforms our raytracer from a basic educational tool into a production-capable progressive renderer while maintaining clean separation of concerns and preserving all existing functionality. 