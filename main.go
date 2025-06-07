package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

func main() {
	// Parse command line flags
	sceneType := flag.String("scene", "default", "Scene type: 'default' or 'cornell'")
	help := flag.Bool("help", false, "Show help information")
	flag.Parse()

	// Show help if requested
	if *help {
		fmt.Println("Progressive Raytracer")
		fmt.Println("Usage: raytracer.exe [options]")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Available scenes:")
		fmt.Println("  default - Default scene with spheres and plane ground")
		fmt.Println("  cornell - Cornell box scene with quad walls and area lighting")
		fmt.Println()
		fmt.Println("Output will be saved to output/<scene_type>/render_<timestamp>.png")
		return
	}

	fmt.Println("Starting Progressive Raytracer...")

	// Create scene based on command line argument
	var selectedScene *scene.Scene
	var width, height int

	switch *sceneType {
	case "cornell":
		fmt.Println("Using Cornell scene...")
		selectedScene = scene.NewCornellScene()
		width = 400
		height = 400 // Square aspect ratio for Cornell box
	case "default":
		fmt.Println("Using default scene...")
		selectedScene = scene.NewDefaultScene()
		width = 400
		height = 225 // 16:9 aspect ratio
	default:
		fmt.Printf("Unknown scene type: %s. Using default scene.\n", *sceneType)
		selectedScene = scene.NewDefaultScene()
		width = 400
		height = 225
		*sceneType = "default" // Normalize the scene type for directory creation
	}

	// Create output directory for this scene type
	outputDir := filepath.Join("output", *sceneType)
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Create raytracer
	raytracer := renderer.NewRaytracer(selectedScene, width, height)

	// Use fewer samples for faster rendering during development
	raytracer.SetSamplingConfig(renderer.SamplingConfig{
		SamplesPerPixel: 50, // Reduced for faster iteration
		MaxDepth:        25, // Reduced for faster iteration
	})

	// Render one pass
	startTime := time.Now()
	img, stats := raytracer.RenderPass()
	renderTime := time.Since(startTime)

	fmt.Printf("Render completed in %v\n", renderTime)
	fmt.Printf("Samples per pixel: %.1f (range %d - %d)\n",
		stats.AverageSamples, stats.MinSamples, stats.MaxSamplesUsed)

	// Create timestamped filename
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("render_%s.png", timestamp))

	file, err := os.Create(filename)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		return
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		fmt.Printf("Error saving PNG: %v\n", err)
		return
	}

	fmt.Printf("Render saved as %s\n", filename)
}
