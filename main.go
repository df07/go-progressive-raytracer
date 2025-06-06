package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

func main() {
	fmt.Println("Starting Progressive Raytracer...")

	// Create output directory
	outputDir := "output"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		return
	}

	// Create default scene
	defaultScene := scene.NewDefaultScene()

	// Create raytracer
	width := 400
	height := 225 // 16:9 aspect ratio
	raytracer := renderer.NewRaytracer(defaultScene, width, height)

	// Use fewer samples for faster rendering during development
	raytracer.SetSamplingConfig(renderer.SamplingConfig{
		SamplesPerPixel: 50, // Reduced for faster iteration
		MaxDepth:        25, // Reduced for faster iteration
	})

	// Render one pass
	startTime := time.Now()
	img := raytracer.RenderPass()
	renderTime := time.Since(startTime)
	fmt.Printf("Render completed in %v\n", renderTime)

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
