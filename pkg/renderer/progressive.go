package renderer

import (
	"context"
	"fmt"
	"image"
	"math/rand"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// DefaultLogger implements core.Logger by writing to stdout
type DefaultLogger struct{}

func (dl *DefaultLogger) Printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// NewDefaultLogger creates a new default logger
func NewDefaultLogger() core.Logger {
	return &DefaultLogger{}
}

// ProgressiveConfig contains configuration for progressive rendering
type ProgressiveConfig struct {
	TileSize           int // Size of each tile (64x64 recommended)
	InitialSamples     int // Samples for first pass (1 recommended)
	MaxSamplesPerPixel int // Maximum total samples per pixel
	MaxPasses          int // Maximum number of passes
	NumWorkers         int // Number of parallel workers (0 = use CPU count)
}

// DefaultProgressiveConfig returns sensible default values
func DefaultProgressiveConfig() ProgressiveConfig {
	return ProgressiveConfig{
		TileSize:           64,
		InitialSamples:     1,
		MaxSamplesPerPixel: 50, // Match original raytracer max samples
		MaxPasses:          7,  // 1, 2, 4, 8, 16, 32, then adaptive up to 50
		NumWorkers:         0,  // Auto-detect CPU count
	}
}

// ProgressiveRaytracer manages progressive rendering with multiple passes
type ProgressiveRaytracer struct {
	scene         core.Scene
	width, height int
	config        ProgressiveConfig
	tiles         []*Tile        // Tile management
	currentPass   int            // Progressive state
	pixelStats    [][]PixelStats // Shared pixel statistics array (global image coordinates)
	raytracer     *Raytracer     // Base raytracer for actual rendering
	workerPool    *WorkerPool    // Worker pool for parallel processing
	logger        core.Logger    // Logger for rendering output
}

// NewProgressiveRaytracer creates a new progressive raytracer
func NewProgressiveRaytracer(scene core.Scene, width, height int, config ProgressiveConfig, logger core.Logger) *ProgressiveRaytracer {
	// Create base raytracer
	raytracer := NewRaytracer(scene, width, height)

	// Create tile grid
	tiles := NewTileGrid(width, height, config.TileSize)

	// Initialize shared pixel statistics array (global image coordinates)
	pixelStats := make([][]PixelStats, height)
	for y := range pixelStats {
		pixelStats[y] = make([]PixelStats, width)
	}

	// Create worker pool
	workerPool := NewWorkerPool(scene, width, height, config.TileSize, config.NumWorkers)

	return &ProgressiveRaytracer{
		scene:       scene,
		width:       width,
		height:      height,
		config:      config,
		tiles:       tiles,
		currentPass: 0,
		pixelStats:  pixelStats,
		raytracer:   raytracer,
		workerPool:  workerPool,
		logger:      logger,
	}
}

// getSamplesForPass calculates the target total samples for a given pass
func (pr *ProgressiveRaytracer) getSamplesForPass(passNumber int) int {
	// Special case: if only 1 pass, use all samples
	if pr.config.MaxPasses == 1 {
		return pr.config.MaxSamplesPerPixel
	}

	// For multiple passes: first pass is quick preview
	if passNumber == 1 {
		return pr.config.InitialSamples // Always 1 for quick preview
	}

	// Divide remaining samples evenly across remaining passes
	remainingSamples := pr.config.MaxSamplesPerPixel - pr.config.InitialSamples
	remainingPasses := pr.config.MaxPasses - 1
	samplesPerPass := remainingSamples / remainingPasses

	// Calculate target total samples for this pass
	targetSamples := pr.config.InitialSamples + (passNumber-1)*samplesPerPass

	// For the final pass, use all remaining samples
	if passNumber == pr.config.MaxPasses {
		targetSamples = pr.config.MaxSamplesPerPixel
	}

	return targetSamples
}

// RenderPass renders a single progressive pass using parallel processing
func (pr *ProgressiveRaytracer) RenderPass(passNumber int, tileCallback func(TileCompletionResult)) (*image.RGBA, RenderStats, error) {
	pr.currentPass = passNumber

	// Calculate target samples for this pass
	targetSamples := pr.getSamplesForPass(passNumber)

	pr.logger.Printf("Pass %d: Target %d samples per pixel (using %d workers)...\n",
		passNumber, targetSamples, pr.workerPool.GetNumWorkers())

	// Configure base raytracer for this pass (for shared pixel stats processing)
	pr.raytracer.MergeSamplingConfig(core.SamplingConfig{
		SamplesPerPixel: targetSamples,
	})

	// Start worker pool if not already started
	if passNumber == 1 {
		pr.workerPool.Start()
	}

	// No need to clear tile callbacks since we handle them internally now

	// Submit all tiles as tasks
	taskID := 0
	for _, tile := range pr.tiles {
		task := TileTask{
			Tile:          tile,
			PassNumber:    passNumber,
			TargetSamples: targetSamples,
			TaskID:        taskID,
			PixelStats:    pr.pixelStats, // Pass shared pixel stats array
		}
		pr.workerPool.SubmitTask(task)
		taskID++
	}

	// Wait for all tiles to complete and dispatch tile callbacks in thread-safe manner
	for i := 0; i < len(pr.tiles); i++ {
		result, ok := pr.workerPool.GetResult()
		if !ok {
			return nil, RenderStats{}, fmt.Errorf("worker pool closed unexpectedly")
		}
		if result.Error != nil {
			return nil, RenderStats{}, result.Error
		}

		// Increment completed passes for the corresponding tile
		tile := pr.tiles[result.TaskID]
		tile.PassesCompleted++

		// Dispatch tile completion callback if provided (thread-safe, single-threaded dispatch)
		if tileCallback != nil {
			// Extract tile image from the shared pixel stats
			tileImage := pr.extractTileImage(tile)
			tileX := tile.Bounds.Min.X / pr.config.TileSize
			tileY := tile.Bounds.Min.Y / pr.config.TileSize

			tileCallback(TileCompletionResult{
				TileX:      tileX,
				TileY:      tileY,
				TileImage:  tileImage,
				PassNumber: passNumber,

				// Progress information
				TileNumber:  i + 1,
				TotalTiles:  len(pr.tiles),
				TotalPasses: pr.config.MaxPasses,
			})
		}
	}

	// Assemble image and calculate final stats from actual pixel data
	img, stats := pr.assembleCurrentImage(targetSamples)

	return img, stats, nil
}

// extractTileImage extracts a tile image from the shared pixel stats array
func (pr *ProgressiveRaytracer) extractTileImage(tile *Tile) *image.RGBA {
	bounds := tile.Bounds
	tileWidth := bounds.Dx()
	tileHeight := bounds.Dy()

	// Create RGBA image for this tile
	tileImage := image.NewRGBA(image.Rect(0, 0, tileWidth, tileHeight))

	// Copy pixels from shared stats array to tile image
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			// Skip if coordinates are out of bounds
			if y >= len(pr.pixelStats) || x >= len(pr.pixelStats[y]) {
				continue
			}

			stats := &pr.pixelStats[y][x]
			if stats.SampleCount > 0 {
				// Get averaged color using existing method
				colorVec := stats.GetColor()
				pixelColor := pr.raytracer.vec3ToColor(colorVec)

				// Set pixel in tile image (relative coordinates)
				tileImage.SetRGBA(x-bounds.Min.X, y-bounds.Min.Y, pixelColor)
			}
		}
	}

	return tileImage
}

// PassResult contains the result of a single pass
type PassResult struct {
	PassNumber int
	Image      *image.RGBA
	Stats      RenderStats
	IsLast     bool
}

// TileCompletionResult contains information about a completed tile for callbacks
type TileCompletionResult struct {
	TileX      int // Tile coordinates (not pixel coordinates)
	TileY      int
	TileImage  *image.RGBA // Image data for just this tile
	PassNumber int         // Which pass this tile was rendered in

	// Progress information
	TileNumber  int // Current tile number in this pass (1-based)
	TotalTiles  int // Total number of tiles in the image
	TotalPasses int // Total number of passes planned
}

// RenderOptions configures progressive rendering behavior
type RenderOptions struct {
	TileUpdates bool // Whether to generate tile completion events
}

// RenderProgressive renders with channel-based communication (idiomatic Go)
// Returns channels for events. The caller should read from these channels in separate goroutines.
// If options.TileUpdates is false, the tile channel will be closed immediately and no tile events will be generated.
func (pr *ProgressiveRaytracer) RenderProgressive(ctx context.Context, options RenderOptions) (<-chan PassResult, <-chan TileCompletionResult, <-chan error) {
	passChan := make(chan PassResult, 1)
	tileChan := make(chan TileCompletionResult, 100) // Buffer for tiles
	errChan := make(chan error, 1)

	// If tile updates are disabled, close the channel immediately
	if !options.TileUpdates {
		close(tileChan)
	}

	go func() {
		defer close(passChan)
		if options.TileUpdates {
			defer close(tileChan)
		}
		defer close(errChan)
		defer pr.workerPool.Stop()

		pr.logger.Printf("Starting progressive rendering with %d passes...\n", pr.config.MaxPasses)

		for pass := 1; pass <= pr.config.MaxPasses; pass++ {
			// Check if client disconnected before starting this pass
			select {
			case <-ctx.Done():
				pr.logger.Printf("Rendering cancelled before pass %d\n", pass)
				errChan <- ctx.Err()
				return
			default:
			}

			startTime := time.Now()

			// Create tile callback only if tile updates are enabled
			var tileCallback func(TileCompletionResult)
			if options.TileUpdates {
				tileCallback = func(result TileCompletionResult) {
					select {
					case tileChan <- result:
					case <-ctx.Done():
						return
					default:
						// Channel full, could log this
					}
				}
			}

			img, stats, err := pr.RenderPass(pass, tileCallback)
			if err != nil {
				errChan <- err
				return
			}

			passTime := time.Since(startTime)
			actualSamples := int(stats.AverageSamples)

			pr.logger.Printf("Pass %d completed in %v (actual: %d samples/pixel)\n",
				pass, passTime, actualSamples)

			// Send pass completion event
			isLast := pass == pr.config.MaxPasses || actualSamples >= pr.config.MaxSamplesPerPixel
			result := PassResult{
				PassNumber: pass,
				Image:      img,
				Stats:      stats,
				IsLast:     isLast,
			}

			select {
			case passChan <- result:
			case <-ctx.Done():
				return
			}

			// Check if we've reached maximum samples
			if actualSamples >= pr.config.MaxSamplesPerPixel {
				pr.logger.Printf("Reached maximum samples per pixel (%d), stopping.\n", pr.config.MaxSamplesPerPixel)
				break
			}
		}
	}()

	return passChan, tileChan, errChan
}

// assembleCurrentImage creates an image from the current state of the shared pixel stats
// and calculates render statistics in a single pass
func (pr *ProgressiveRaytracer) assembleCurrentImage(targetSamples int) (*image.RGBA, RenderStats) {
	img := image.NewRGBA(image.Rect(0, 0, pr.width, pr.height))

	// Initialize statistics
	stats := RenderStats{
		TotalPixels:    pr.width * pr.height,
		TotalSamples:   0,
		AverageSamples: 0,
		MaxSamples:     targetSamples,
		MinSamples:     pr.config.MaxSamplesPerPixel, // Start high, will be reduced
		MaxSamplesUsed: 0,
	}

	// Single pass: create image and calculate stats
	for y := 0; y < pr.height; y++ {
		for x := 0; x < pr.width; x++ {
			pixel := &pr.pixelStats[y][x]

			// Create image pixel
			colorVec := pixel.GetColor()
			pixelColor := pr.raytracer.vec3ToColor(colorVec)
			img.SetRGBA(x, y, pixelColor)

			// Update statistics
			stats.TotalSamples += pixel.SampleCount
			stats.MinSamples = min(stats.MinSamples, pixel.SampleCount)
			stats.MaxSamplesUsed = max(stats.MaxSamplesUsed, pixel.SampleCount)
		}
	}

	// Finalize statistics
	stats.AverageSamples = float64(stats.TotalSamples) / float64(stats.TotalPixels)

	return img, stats
}

// Tile represents a rectangular region of the image to be rendered
type Tile struct {
	ID              int             // Unique tile identifier
	Bounds          image.Rectangle // Pixel bounds (x0,y0,x1,y1)
	PassesCompleted int             // Number of passes completed for this tile
	Random          *rand.Rand      // Tile-specific random generator for deterministic results
}

// NewTile creates a new tile with the specified bounds
func NewTile(id int, bounds image.Rectangle) *Tile {
	// Create deterministic random generator based on tile ID
	random := rand.New(rand.NewSource(int64(id + 42))) // +42 to avoid seed 0

	return &Tile{
		ID:              id,
		Bounds:          bounds,
		PassesCompleted: 0,
		Random:          random,
	}
}

// NewTileGrid creates a grid of tiles covering the entire image
func NewTileGrid(width, height, tileSize int) []*Tile {
	var tiles []*Tile
	tileID := 0

	// Calculate number of tiles in each dimension
	tilesX := (width + tileSize - 1) / tileSize // Ceiling division
	tilesY := (height + tileSize - 1) / tileSize

	for tileY := 0; tileY < tilesY; tileY++ {
		for tileX := 0; tileX < tilesX; tileX++ {
			// Calculate tile bounds
			x0 := tileX * tileSize
			y0 := tileY * tileSize
			x1 := min(x0+tileSize, width) // Don't exceed image bounds
			y1 := min(y0+tileSize, height)

			bounds := image.Rect(x0, y0, x1, y1)
			tile := NewTile(tileID, bounds)
			tiles = append(tiles, tile)
			tileID++
		}
	}

	return tiles
}
