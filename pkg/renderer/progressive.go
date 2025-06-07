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
}

// DefaultProgressiveConfig returns sensible default values
func DefaultProgressiveConfig() ProgressiveConfig {
	return ProgressiveConfig{
		TileSize:           64,
		InitialSamples:     1,
		MaxSamplesPerPixel: 50, // Match original raytracer max samples
		MaxPasses:          7,  // 1, 2, 4, 8, 16, 32, then adaptive up to 50
	}
}

// ProgressiveRaytracer manages progressive rendering with multiple passes
type ProgressiveRaytracer struct {
	scene         core.Scene
	width, height int
	config        ProgressiveConfig

	// Tile management
	tiles []*Tile

	// Progressive state
	currentPass int

	// Shared pixel statistics array (global image coordinates)
	pixelStats [][]PixelStats

	// Base raytracer for actual rendering
	raytracer *Raytracer
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

	return &ProgressiveRaytracer{
		scene:       scene,
		width:       width,
		height:      height,
		config:      config,
		tiles:       tiles,
		currentPass: 0,
		pixelStats:  pixelStats,
		raytracer:   raytracer,
	}
}

// getSamplesForPass calculates the target total samples for a given pass
func (pr *ProgressiveRaytracer) getSamplesForPass(passNumber int) int {
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

// RenderPass renders a single progressive pass
func (pr *ProgressiveRaytracer) RenderPass(passNumber int) (*image.RGBA, RenderStats, error) {
	pr.currentPass = passNumber

	// Calculate target samples for this pass
	targetSamples := pr.getSamplesForPass(passNumber)

	fmt.Printf("Pass %d: Target %d samples per pixel...\n", passNumber, targetSamples)

	// Configure raytracer for this pass
	pr.raytracer.SetSamplingConfig(SamplingConfig{
		SamplesPerPixel: targetSamples,
		MaxDepth:        25, // Keep consistent with main.go
	})

	// Render each tile
	for _, tile := range pr.tiles {
		// Render the tile bounds using the shared pixel stats with tile's random generator
		pr.raytracer.RenderBounds(tile.Bounds, pr.pixelStats, tile.Random)

		// Increment completed passes for this tile
		tile.PassesCompleted++
	}

	// Assemble image and calculate stats in a single pass
	img, stats := pr.assembleCurrentImage(targetSamples)

	return img, stats, nil
}

// RenderProgressive renders multiple progressive passes and returns all images
func (pr *ProgressiveRaytracer) RenderProgressive() ([]*image.RGBA, []RenderStats, error) {
	var images []*image.RGBA
	var allStats []RenderStats

	fmt.Printf("Starting progressive rendering with %d passes...\n", pr.config.MaxPasses)

	for pass := 1; pass <= pr.config.MaxPasses; pass++ {
		startTime := time.Now()

		img, stats, err := pr.RenderPass(pass)
		if err != nil {
			return images, allStats, err
		}

		passTime := time.Since(startTime)
		actualSamples := int(stats.AverageSamples)

		fmt.Printf("Pass %d completed in %v (actual: %d samples/pixel)\n",
			pass, passTime, actualSamples)

		images = append(images, img)
		allStats = append(allStats, stats)

		// Note: Image saving is now handled by the caller (main.go)

		// Check if we've reached maximum samples
		if actualSamples >= pr.config.MaxSamplesPerPixel {
			fmt.Printf("Reached maximum samples per pixel (%d), stopping.\n", pr.config.MaxSamplesPerPixel)
			break
		}
	}

	return images, allStats, nil
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
			if pixel.SampleCount < stats.MinSamples {
				stats.MinSamples = pixel.SampleCount
			}
			if pixel.SampleCount > stats.MaxSamplesUsed {
				stats.MaxSamplesUsed = pixel.SampleCount
			}
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
