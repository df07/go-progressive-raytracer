package renderer

import (
	"fmt"
	"image"
	"math/rand"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

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
}

// NewProgressiveRaytracer creates a new progressive raytracer
func NewProgressiveRaytracer(scene core.Scene, width, height int, config ProgressiveConfig) *ProgressiveRaytracer {
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
	workerPool := NewWorkerPool(scene, width, height, config.NumWorkers)

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
func (pr *ProgressiveRaytracer) RenderPass(passNumber int) (*image.RGBA, RenderStats, error) {
	pr.currentPass = passNumber

	// Calculate target samples for this pass
	targetSamples := pr.getSamplesForPass(passNumber)

	fmt.Printf("Pass %d: Target %d samples per pixel (using %d workers)...\n",
		passNumber, targetSamples, pr.workerPool.GetNumWorkers())

	// Configure base raytracer for this pass (for shared pixel stats processing)
	pr.raytracer.MergeSamplingConfig(core.SamplingConfig{
		SamplesPerPixel: targetSamples,
	})

	// Start worker pool if not already started
	if passNumber == 1 {
		pr.workerPool.Start()
	}

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

	// Wait for all tiles to complete
	for i := 0; i < len(pr.tiles); i++ {
		result, ok := pr.workerPool.GetResult()
		if !ok {
			return nil, RenderStats{}, fmt.Errorf("worker pool closed unexpectedly")
		}
		if result.Error != nil {
			return nil, RenderStats{}, result.Error
		}

		// Increment completed passes for the corresponding tile
		pr.tiles[result.TaskID].PassesCompleted++
	}

	// Assemble image and calculate final stats from actual pixel data
	img, stats := pr.assembleCurrentImage(targetSamples)

	return img, stats, nil
}

// PassResult contains the result of a single pass
type PassResult struct {
	PassNumber int
	Image      *image.RGBA
	Stats      RenderStats
	IsLast     bool
}

// PassCallback is called after each pass completes
type PassCallback func(result PassResult) error

// RenderProgressiveWithCallback renders multiple progressive passes, calling the callback after each pass
func (pr *ProgressiveRaytracer) RenderProgressiveWithCallback(callback PassCallback) error {
	fmt.Printf("Starting progressive rendering with %d passes...\n", pr.config.MaxPasses)

	// Ensure worker pool is cleaned up when we're done
	defer pr.workerPool.Stop()

	for pass := 1; pass <= pr.config.MaxPasses; pass++ {
		startTime := time.Now()

		img, stats, err := pr.RenderPass(pass)
		if err != nil {
			return err
		}

		passTime := time.Since(startTime)
		actualSamples := int(stats.AverageSamples)

		fmt.Printf("Pass %d completed in %v (actual: %d samples/pixel)\n",
			pass, passTime, actualSamples)

		// Call the callback with this pass result
		isLast := pass == pr.config.MaxPasses || actualSamples >= pr.config.MaxSamplesPerPixel
		result := PassResult{
			PassNumber: pass,
			Image:      img,
			Stats:      stats,
			IsLast:     isLast,
		}

		if err := callback(result); err != nil {
			return fmt.Errorf("callback error: %v", err)
		}

		// Check if we've reached maximum samples
		if actualSamples >= pr.config.MaxSamplesPerPixel {
			fmt.Printf("Reached maximum samples per pixel (%d), stopping.\n", pr.config.MaxSamplesPerPixel)
			break
		}
	}

	return nil
}

// RenderProgressive renders multiple progressive passes and returns all images
func (pr *ProgressiveRaytracer) RenderProgressive() ([]*image.RGBA, []RenderStats, error) {
	var images []*image.RGBA
	var allStats []RenderStats

	// Use the callback version to collect all results
	err := pr.RenderProgressiveWithCallback(func(result PassResult) error {
		images = append(images, result.Image)
		allStats = append(allStats, result.Stats)
		return nil
	})

	return images, allStats, err
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
