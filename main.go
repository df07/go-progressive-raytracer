package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// Config holds all the configuration for the raytracer
type Config struct {
	SceneType      string
	MaxPasses      int
	MaxSamples     int
	NumWorkers     int
	IntegratorType string
	Help           bool
}

// RenderResult holds the final image and statistics
type RenderResult struct {
	Image     *image.RGBA
	Stats     renderer.RenderStats
	Timestamp string
}

func main() {
	config := parseFlags()
	if config.Help {
		showHelp()
		return
	}

	fmt.Println("Starting Progressive Raytracer...")
	startTime := time.Now()

	sceneObj := createScene(config.SceneType)
	outputDir := createOutputDir(config.SceneType)
	result := renderProgressive(config, sceneObj)

	renderTime := time.Since(startTime)
	fmt.Printf("Render completed in %v\n", renderTime)
	fmt.Printf("Samples per pixel: %.1f (range %d - %d)\n",
		result.Stats.AverageSamples, result.Stats.MinSamples, result.Stats.MaxSamplesUsed)
	fmt.Printf("Render saved as %s\n", filepath.Join(outputDir, fmt.Sprintf("render_%s.png", result.Timestamp)))
}

// parseFlags parses command line flags and returns configuration
func parseFlags() Config {
	config := Config{}
	flag.StringVar(&config.SceneType, "scene", "default", "Scene type: 'default', 'cornell', 'spheregrid', 'trianglemesh', 'dragon', or 'caustic-glass'")
	flag.IntVar(&config.MaxPasses, "max-passes", 5, "Maximum number of progressive passes")
	flag.IntVar(&config.MaxSamples, "max-samples", 50, "Maximum samples per pixel")
	flag.IntVar(&config.NumWorkers, "workers", 0, "Number of parallel workers (0 = auto-detect CPU count)")
	flag.StringVar(&config.IntegratorType, "integrator", "path-tracing", "Integrator type: 'path-tracing' or 'bdpt'")
	flag.BoolVar(&config.Help, "help", false, "Show help information")
	flag.Parse()
	return config
}

// showHelp displays help information
func showHelp() {
	fmt.Println("Progressive Raytracer")
	fmt.Println("Usage: raytracer.exe [options]")
	fmt.Println()
	fmt.Println("Options:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("Available scenes:")
	fmt.Println("  default      - Default scene with spheres and plane ground")
	fmt.Println("  cornell      - Cornell box scene with spheres")
	fmt.Println("  cornell-boxes - Cornell box scene with rotated boxes")
	fmt.Println("  spheregrid   - 10x10 grid of rainbow-colored metallic spheres (perfect for BVH testing)")
	fmt.Println("  trianglemesh - Scene showcasing triangle mesh geometry (boxes, pyramids, icosahedrons)")
	fmt.Println("  dragon       - Dragon PLY mesh from PBRT book")
	fmt.Println("  caustic-glass - Glass caustic geometry scene")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  raytracer.exe --max-passes=5 --max-samples=100")
	fmt.Println("  raytracer.exe --scene=cornell --workers=4")
	fmt.Println("  raytracer.exe --scene=caustic-glass --max-passes=1 --max-samples=25")
	fmt.Println("  raytracer.exe --scene=caustic-glass --integrator=bdpt --max-samples=100")
	fmt.Println()
	fmt.Println("Output will be saved to output/<scene_type>/render_<timestamp>.png")
}

// createScene creates the appropriate scene based on scene type
func createScene(sceneType string) *scene.Scene {
	var sceneObj *scene.Scene

	switch sceneType {
	case "cornell":
		fmt.Println("Using Cornell scene...")
		sceneObj = scene.NewCornellScene(scene.CornellSpheres)
	case "cornell-boxes":
		fmt.Println("Using Cornell scene with boxes...")
		sceneObj = scene.NewCornellScene(scene.CornellBoxes)
	case "spheregrid":
		fmt.Println("Using sphere grid scene...")
		sceneObj = scene.NewSphereGridScene(20, "metallic") // Default grid size and material
	case "trianglemesh":
		fmt.Println("Using triangle mesh scene...")
		sceneObj = scene.NewTriangleMeshScene(32) // Default complexity
	case "dragon":
		fmt.Println("Using dragon PLY mesh scene...")
		sceneObj = scene.NewDragonScene(true, "gold", renderer.NewDefaultLogger()) // Default to gold material
	case "caustic-glass":
		fmt.Println("Using caustic glass scene...")
		sceneObj = scene.NewCausticGlassScene(true, core.LightTypeArea, renderer.NewDefaultLogger())
	case "default":
		fmt.Println("Using default scene...")
		sceneObj = scene.NewDefaultScene()
	default:
		fmt.Printf("Unknown scene type: %s. Using default scene.\n", sceneType)
		sceneObj = scene.NewDefaultScene()
	}

	// Get the width and height from the scene's camera configuration
	width := sceneObj.CameraConfig.Width
	height := int(float64(width) / sceneObj.CameraConfig.AspectRatio)

	sceneObj.SamplingConfig.Height = height
	sceneObj.SamplingConfig.Width = width

	return sceneObj
}

// createOutputDir creates the output directory for the scene type
func createOutputDir(sceneType string) string {
	// Normalize scene type
	if sceneType != "cornell" && sceneType != "cornell-boxes" && sceneType != "default" && sceneType != "spheregrid" && sceneType != "trianglemesh" && sceneType != "dragon" && sceneType != "caustic-glass" {
		sceneType = "default"
	}

	outputDir := filepath.Join("output", sceneType)
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		fmt.Printf("Error creating output directory: %v\n", err)
		os.Exit(1)
	}
	return outputDir
}

// renderProgressive handles progressive rendering with immediate file saving
func renderProgressive(config Config, sceneObj *scene.Scene) RenderResult {
	timestamp := time.Now().Format("20060102_150405")
	fmt.Println("Using progressive rendering...")

	progressiveConfig := renderer.DefaultProgressiveConfig()
	progressiveConfig.MaxPasses = config.MaxPasses
	progressiveConfig.MaxSamplesPerPixel = config.MaxSamples
	progressiveConfig.NumWorkers = config.NumWorkers

	// Create the appropriate integrator based on config
	var selectedIntegrator core.Integrator
	switch config.IntegratorType {
	case "bdpt":
		fmt.Println("Using BDPT integrator...")
		selectedIntegrator = integrator.NewBDPTIntegrator(sceneObj.GetSamplingConfig())
	case "path-tracing":
		fmt.Println("Using path tracing integrator...")
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.GetSamplingConfig())
	default:
		fmt.Printf("Unknown integrator type: %s. Using path tracing.\n", config.IntegratorType)
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.GetSamplingConfig())
	}

	progressiveRT := renderer.NewProgressiveRaytracer(sceneObj, progressiveConfig, selectedIntegrator, renderer.NewDefaultLogger())

	// Create output directory
	outputDir := createOutputDir(config.SceneType)
	baseFilename := fmt.Sprintf("render_%s", timestamp)

	var finalImage *image.RGBA
	var finalStats renderer.RenderStats

	// Start rendering and get event channels (disable tile updates for command-line)
	renderOptions := renderer.RenderOptions{TileUpdates: false}
	passChan, _, errChan := progressiveRT.RenderProgressive(context.Background(), renderOptions)

	// Listen to events from channels
renderLoop:
	for {
		select {
		case passResult, ok := <-passChan:
			if !ok {
				passChan = nil // Channel closed
				continue
			}

			// Save intermediate passes (not the final one)
			filename := filepath.Join(outputDir, fmt.Sprintf("%s.png", baseFilename))
			if !passResult.IsLast {
				filename = filepath.Join(outputDir, fmt.Sprintf("%s_pass_%02d.png", baseFilename, passResult.PassNumber))
			}
			if err := saveImageToFile(passResult.Image, filename); err != nil {
				fmt.Printf("Error saving final image: %v\n", err)
				os.Exit(1)
			}

			// Keep track of final result
			finalImage = passResult.Image
			finalStats = passResult.Stats

		case err := <-errChan:
			if err != nil {
				fmt.Printf("Error during progressive rendering: %v\n", err)
				os.Exit(1)
			}
			// errChan closed, rendering completed successfully
			break renderLoop
		}

		// If all channels are closed, we're done
		if passChan == nil {
			break renderLoop
		}
	}

	if finalImage == nil {
		fmt.Println("No images were rendered")
		os.Exit(1)
	}

	return RenderResult{
		Image:     finalImage,
		Stats:     finalStats,
		Timestamp: timestamp,
	}
}

// saveImageToFile saves an image to the specified file path
func saveImageToFile(img *image.RGBA, filename string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	// Create and save file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
