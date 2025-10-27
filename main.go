package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
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
	CPUProfile     string
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

	// Start CPU profiling if requested
	if config.CPUProfile != "" {
		f, err := os.Create(config.CPUProfile)
		if err != nil {
			fmt.Printf("Could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Printf("Could not start CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	fmt.Println("Starting Progressive Raytracer...")
	startTime := time.Now()

	sceneObj, err := createScene(config.SceneType)
	if err != nil {
		fmt.Printf("Error creating scene: %v\n", err)
		os.Exit(1)
	}
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
	flag.StringVar(&config.SceneType, "scene", "default", "Scene type or PBRT file path")
	flag.IntVar(&config.MaxPasses, "max-passes", 5, "Maximum number of progressive passes")
	flag.IntVar(&config.MaxSamples, "max-samples", 50, "Maximum samples per pixel")
	flag.IntVar(&config.NumWorkers, "workers", 0, "Number of parallel workers (0 = auto-detect CPU count)")
	flag.StringVar(&config.IntegratorType, "integrator", "path-tracing", "Integrator type: 'path-tracing' or 'bdpt'")
	flag.BoolVar(&config.Help, "help", false, "Show help information")
	flag.StringVar(&config.CPUProfile, "cpuprofile", "", "Write CPU profile to file")
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
	fmt.Println("Built-in scenes:")
	fmt.Println("  default      - Default scene with spheres and plane ground")
	fmt.Println("  cornell      - Cornell box scene with spheres")
	fmt.Println("  cornell-boxes - Cornell box scene with rotated boxes")
	fmt.Println("  cornell-pbrt - Cornell box scene loaded from PBRT file")
	fmt.Println("  spheregrid   - 10x10 grid of rainbow-colored metallic spheres (perfect for BVH testing)")
	fmt.Println("  trianglemesh - Scene showcasing triangle mesh geometry (boxes, pyramids, icosahedrons)")
	fmt.Println("  dragon       - Dragon PLY mesh from PBRT book")
	fmt.Println("  caustic-glass - Glass caustic geometry scene")
	fmt.Println()
	fmt.Println("PBRT scenes:")
	fmt.Println("  cornell-empty - Cornell box without objects (from scenes/cornell-empty.pbrt)")
	fmt.Println("  simple-sphere - Basic sphere scene (from scenes/simple-sphere.pbrt)")
	fmt.Println("  test         - Test scene (from scenes/test.pbrt)")
	fmt.Println("  Or use direct file path: scenes/my-custom-scene.pbrt")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  raytracer.exe --max-passes=5 --max-samples=100")
	fmt.Println("  raytracer.exe --scene=cornell --workers=4")
	fmt.Println("  raytracer.exe --scene=cornell-empty --max-samples=100")
	fmt.Println("  raytracer.exe --scene=scenes/simple-sphere.pbrt --integrator=bdpt")
	fmt.Println("  raytracer.exe --scene=caustic-glass --integrator=bdpt --max-samples=100")
	fmt.Println()
	fmt.Println("Output will be saved to output/<scene_type>/render_<timestamp>.png")
}

// createScene creates the appropriate scene based on scene type
func createScene(sceneType string) (*scene.Scene, error) {
	var sceneObj *scene.Scene

	// First, try to load as PBRT scene (direct path or scene name)
	if pbrtScene := tryLoadPBRTScene(sceneType); pbrtScene != nil {
		sceneObj = pbrtScene
	} else {
		// Fall back to built-in scenes
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
			sceneObj = scene.NewCausticGlassScene(true, lights.LightTypeArea, renderer.NewDefaultLogger())
		case "cylinder-test":
			fmt.Println("Using cylinder test scene...")
			sceneObj = scene.NewCylinderTestScene()
		case "cone-test":
			fmt.Println("Using cone test scene...")
			sceneObj = scene.NewConeTestScene()
		case "cornell-pbrt":
			fmt.Println("Using PBRT Cornell scene...")
			pbrtScene, err := loaders.LoadPBRT("scenes/cornell-empty.pbrt")
			if err != nil {
				return nil, fmt.Errorf("failed to load PBRT file: %v", err)
			}
			sceneObj, err = scene.NewPBRTScene(pbrtScene)
			if err != nil {
				return nil, fmt.Errorf("failed to create PBRT scene: %v", err)
			}
		case "default":
			fmt.Println("Using default scene...")
			sceneObj = scene.NewDefaultScene()
		default:
			return nil, fmt.Errorf("unknown scene type: %s", sceneType)
		}
	}

	// Get the width and height from the scene's camera configuration
	width := sceneObj.CameraConfig.Width
	height := int(float64(width) / sceneObj.CameraConfig.AspectRatio)

	sceneObj.SamplingConfig.Height = height
	sceneObj.SamplingConfig.Width = width

	return sceneObj, nil
}

// tryLoadPBRTScene attempts to load a PBRT scene from various possible paths
func tryLoadPBRTScene(sceneType string) *scene.Scene {
	// List of possible PBRT file paths to try
	possiblePaths := []string{
		sceneType, // Direct path (e.g., "scenes/my-scene.pbrt")
		filepath.Join("scenes", sceneType+".pbrt"), // Scene name + .pbrt (e.g., "cornell-empty" → "scenes/cornell-empty.pbrt")
		filepath.Join("scenes", sceneType),         // Scene name as direct file (e.g., "scenes/cornell-empty")
	}

	for _, path := range possiblePaths {
		// Check if file exists and has .pbrt extension
		if !strings.HasSuffix(path, ".pbrt") {
			continue
		}

		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Loading PBRT scene: %s...\n", path)
			pbrtScene, err := loaders.LoadPBRT(path)
			if err != nil {
				fmt.Printf("Failed to load PBRT file '%s': %v\n", path, err)
				continue
			}
			sceneObj, err := scene.NewPBRTScene(pbrtScene)
			if err != nil {
				fmt.Printf("Failed to create PBRT scene '%s': %v\n", path, err)
				continue
			}
			return sceneObj
		}
	}

	return nil
}

// createOutputDir creates the output directory for the scene type
func createOutputDir(sceneType string) string {
	// Extract a clean directory name from the scene type
	dirName := sceneType

	// If it's a file path, extract the filename without extension
	if strings.Contains(sceneType, "/") || strings.HasSuffix(sceneType, ".pbrt") {
		base := filepath.Base(sceneType)
		dirName = strings.TrimSuffix(base, ".pbrt")
	}

	// Use known scene types or default
	knownScenes := []string{"cornell", "cornell-boxes", "default", "spheregrid", "trianglemesh", "dragon", "caustic-glass", "cornell-pbrt", "cornell-empty", "simple-sphere", "test"}
	found := false
	for _, known := range knownScenes {
		if dirName == known {
			found = true
			break
		}
	}

	if !found {
		// Use the extracted name for PBRT scenes or fall back to "pbrt-scene"
		if dirName == sceneType {
			dirName = "pbrt-scene"
		}
	}

	outputDir := filepath.Join("output", dirName)
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
	var selectedIntegrator integrator.Integrator
	switch config.IntegratorType {
	case "bdpt":
		fmt.Println("Using BDPT integrator...")
		selectedIntegrator = integrator.NewBDPTIntegrator(sceneObj.SamplingConfig)
	case "path-tracing":
		fmt.Println("Using path tracing integrator...")
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.SamplingConfig)
	default:
		fmt.Printf("Unknown integrator type: %s. Using path tracing.\n", config.IntegratorType)
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.SamplingConfig)
	}

	progressiveRT, err := renderer.NewProgressiveRaytracer(sceneObj, progressiveConfig, selectedIntegrator, renderer.NewDefaultLogger())
	if err != nil {
		fmt.Printf("Error creating progressive raytracer: %v\n", err)
		os.Exit(1)
	}

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
