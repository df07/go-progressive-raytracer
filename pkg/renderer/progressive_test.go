package renderer

import (
	"image"
	"testing"
)

func TestProgressiveSampleCalculation(t *testing.T) {
	// Test the sample calculation logic without creating a full raytracer
	config := DefaultProgressiveConfig()
	config.InitialSamples = 1
	config.MaxSamplesPerPixel = 50
	config.MaxPasses = 7

	// Create a minimal progressive raytracer for testing
	pr := &ProgressiveRaytracer{
		config: config,
	}

	// Test sample progression with linear distribution
	// Pass 1: 1 sample
	// Pass 2-6: (50-1)/6 = 8.16 -> 8 samples per pass -> 1 + 8*1 = 9, 1 + 8*2 = 17, etc.
	// Pass 7: 50 (final pass gets all remaining)
	expectedTotalSamples := []int{1, 9, 17, 25, 33, 41, 50}

	for pass := 1; pass <= 7; pass++ {
		totalSamples := pr.getSamplesForPass(pass)

		if totalSamples != expectedTotalSamples[pass-1] {
			t.Errorf("Pass %d: expected %d total samples, got %d",
				pass, expectedTotalSamples[pass-1], totalSamples)
		}
	}
}

func TestProgressiveConfig(t *testing.T) {
	// Test default configuration
	config := DefaultProgressiveConfig()

	if config.TileSize != 64 {
		t.Errorf("Expected default tile size 64, got %d", config.TileSize)
	}

	if config.InitialSamples != 1 {
		t.Errorf("Expected default initial samples 1, got %d", config.InitialSamples)
	}

	if config.MaxSamplesPerPixel != 50 {
		t.Errorf("Expected default max samples 50, got %d", config.MaxSamplesPerPixel)
	}

	if config.MaxPasses != 7 {
		t.Errorf("Expected default max passes 7, got %d", config.MaxPasses)
	}

	// SaveIntermediateResults field removed - file I/O now handled by caller
}

// Note: Full integration test for ProgressiveRaytracer creation
// is tested in the main CLI integration tests to avoid import cycles

func TestNewTileGrid(t *testing.T) {
	// Test tile grid generation for a 400x225 image with 64x64 tiles
	width, height, tileSize := 400, 225, 64
	tiles := NewTileGrid(width, height, tileSize)

	// Calculate expected number of tiles
	expectedTilesX := (width + tileSize - 1) / tileSize   // 7 tiles
	expectedTilesY := (height + tileSize - 1) / tileSize  // 4 tiles
	expectedTotalTiles := expectedTilesX * expectedTilesY // 28 tiles

	if len(tiles) != expectedTotalTiles {
		t.Errorf("Expected %d tiles, got %d", expectedTotalTiles, len(tiles))
	}

	// Test that tiles cover the entire image without gaps or overlaps
	covered := make([][]bool, height)
	for y := range covered {
		covered[y] = make([]bool, width)
	}

	for _, tile := range tiles {
		for y := tile.Bounds.Min.Y; y < tile.Bounds.Max.Y; y++ {
			for x := tile.Bounds.Min.X; x < tile.Bounds.Max.X; x++ {
				if x >= width || y >= height {
					t.Errorf("Tile %d extends beyond image bounds at (%d,%d)", tile.ID, x, y)
				}
				if covered[y][x] {
					t.Errorf("Pixel (%d,%d) is covered by multiple tiles", x, y)
				}
				covered[y][x] = true
			}
		}
	}

	// Verify all pixels are covered
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if !covered[y][x] {
				t.Errorf("Pixel (%d,%d) is not covered by any tile", x, y)
			}
		}
	}
}

func TestTileDeterministicRandom(t *testing.T) {
	// Create two tiles with the same ID
	bounds := image.Rect(0, 0, 64, 64)
	tile1 := NewTile(42, bounds)
	tile2 := NewTile(42, bounds)

	// They should have the same random seed and produce the same sequence
	val1 := tile1.Sampler.Get1D()
	val2 := tile2.Sampler.Get1D()

	if val1 != val2 {
		t.Errorf("Tiles with same ID should produce same random values: %f != %f", val1, val2)
	}

	// Different tile IDs should produce different sequences
	tile3 := NewTile(43, bounds)
	val3 := tile3.Sampler.Get1D()

	if val1 == val3 {
		t.Error("Tiles with different IDs should produce different random values")
	}
}
