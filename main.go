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
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// Config holds all the configuration for the raytracer
type Config struct {
	SceneType  string
	RenderMode string
	MaxPasses  int
	MaxSamples int
	NumWorkers int
	Profile    string // CPU profile output file
	Help       bool
}

// SceneInfo holds scene and its dimensions
type SceneInfo struct {
	Scene  *scene.Scene
	Width  int
	Height int
}

// RenderResult holds the final image and statistics
type RenderResult struct {
	Image     *image.RGBA
	Stats     renderer.RenderStats
	Images    []*image.RGBA // For progressive mode
	AllStats  []renderer.RenderStats
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

	sceneInfo := createScene(config.SceneType)
	raytracer := renderer.NewRaytracer(sceneInfo.Scene, sceneInfo.Width, sceneInfo.Height)

	outputDir := createOutputDir(config.SceneType)
	result := renderImage(config, sceneInfo, raytracer)

	saveResults(config, result, outputDir)

	renderTime := time.Since(startTime)
	fmt.Printf("Render completed in %v\n", renderTime)
	fmt.Printf("Samples per pixel: %.1f (range %d - %d)\n",
		result.Stats.AverageSamples, result.Stats.MinSamples, result.Stats.MaxSamplesUsed)
	fmt.Printf("Render saved as %s\n", filepath.Join(outputDir, fmt.Sprintf("render_%s.png", result.Timestamp)))
}

// parseFlags parses command line flags and returns configuration
func parseFlags() Config {
	config := Config{}
	flag.StringVar(&config.SceneType, "scene", "default", "Scene type: 'default', 'cornell', or 'spheregrid'")
	flag.StringVar(&config.RenderMode, "mode", "normal", "Render mode: 'normal' or 'progressive'")
	flag.IntVar(&config.MaxPasses, "max-passes", 5, "Maximum number of progressive passes")
	flag.IntVar(&config.MaxSamples, "max-samples", 50, "Maximum samples per pixel")
	flag.IntVar(&config.NumWorkers, "workers", 0, "Number of parallel workers (0 = auto-detect CPU count)")
	flag.StringVar(&config.Profile, "profile", "", "CPU profile output file")
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
	fmt.Println()
	fmt.Println("Available modes:")
	fmt.Println("  normal      - Standard single-threaded rendering")
	fmt.Println("  progressive - Progressive multi-pass parallel rendering")
	fmt.Println()
	fmt.Println("Profiling:")
	fmt.Println("  Use --profile=cpu.prof to generate CPU profile for normal mode")
	fmt.Println("  Analyze with: go tool pprof cpu.prof")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  raytracer.exe --mode=progressive --max-passes=5 --max-samples=100")
	fmt.Println("  raytracer.exe --scene=cornell --mode=progressive --workers=4")
	fmt.Println("  raytracer.exe --mode=normal --max-samples=200")
	fmt.Println("  raytracer.exe --mode=normal --max-samples=100 --profile=cpu.prof")
	fmt.Println("  raytracer.exe --mode=progressive --max-passes=1 --max-samples=25")
	fmt.Println()
	fmt.Println("Output will be saved to output/<scene_type>/render_<timestamp>.png")
}

// createScene creates the appropriate scene based on scene type
func createScene(sceneType string) SceneInfo {
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
		sceneObj = scene.NewDragonScene(true)
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

	return SceneInfo{
		Scene:  sceneObj,
		Width:  width,
		Height: height,
	}
}

// createOutputDir creates the output directory for the scene type
func createOutputDir(sceneType string) string {
	// Normalize scene type
	if sceneType != "cornell" && sceneType != "cornell-boxes" && sceneType != "default" && sceneType != "spheregrid" && sceneType != "trianglemesh" && sceneType != "dragon" {
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

// renderImage performs the actual rendering based on the mode
func renderImage(config Config, sceneInfo SceneInfo, raytracer *renderer.Raytracer) RenderResult {
	timestamp := time.Now().Format("20060102_150405")

	switch config.RenderMode {
	case "progressive":
		return renderProgressive(config, sceneInfo, timestamp)
	default:
		return renderNormal(config, raytracer, timestamp)
	}
}

// renderProgressive handles progressive rendering with immediate file saving
func renderProgressive(config Config, sceneInfo SceneInfo, timestamp string) RenderResult {
	fmt.Println("Using progressive rendering...")

	progressiveConfig := renderer.DefaultProgressiveConfig()
	progressiveConfig.MaxPasses = config.MaxPasses
	progressiveConfig.MaxSamplesPerPixel = config.MaxSamples
	progressiveConfig.NumWorkers = config.NumWorkers

	progressiveRT := renderer.NewProgressiveRaytracer(sceneInfo.Scene, sceneInfo.Width, sceneInfo.Height, progressiveConfig)

	// Create output directory
	outputDir := createOutputDir(config.SceneType)
	baseFilename := fmt.Sprintf("render_%s", timestamp)

	var finalImage *image.RGBA
	var finalStats renderer.RenderStats

	// Use callback to save images immediately as they complete
	err := progressiveRT.RenderProgressive(context.Background(), func(result renderer.PassResult) error {
		// Save intermediate passes (not the final one)
		if !result.IsLast {
			passFilename := filepath.Join(outputDir, fmt.Sprintf("%s_pass_%02d.png", baseFilename, result.PassNumber))
			if err := saveImageToFile(result.Image, passFilename); err != nil {
				fmt.Printf("Warning: Failed to save pass %d image: %v\n", result.PassNumber, err)
			}
		} else {
			// Save final image
			finalFilename := filepath.Join(outputDir, fmt.Sprintf("%s.png", baseFilename))
			if err := saveImageToFile(result.Image, finalFilename); err != nil {
				return fmt.Errorf("failed to save final image: %v", err)
			}
		}

		// Keep track of final result
		finalImage = result.Image
		finalStats = result.Stats

		return nil
	})

	if err != nil {
		fmt.Printf("Error during progressive rendering: %v\n", err)
		os.Exit(1)
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

// renderNormal handles normal rendering
func renderNormal(config Config, raytracer *renderer.Raytracer, timestamp string) RenderResult {
	// Start CPU profiling if requested
	if config.Profile != "" {
		fmt.Printf("Starting CPU profiling, output: %s\n", config.Profile)
		f, err := os.Create(config.Profile)
		if err != nil {
			fmt.Printf("Error creating profile file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Printf("Error starting CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer pprof.StopCPUProfile()
	}

	// Update raytracer config to use CLI max samples
	raytracer.MergeSamplingConfig(core.SamplingConfig{
		SamplesPerPixel: config.MaxSamples,
		// Leave other settings (MaxDepth, Russian Roulette) from previous config
	})

	fmt.Println("Starting single-threaded render...")
	img, stats := raytracer.RenderPass()

	if config.Profile != "" {
		fmt.Printf("CPU profiling complete. Analyze with: go tool pprof %s\n", config.Profile)
	}

	return RenderResult{
		Image:     img,
		Stats:     stats,
		Timestamp: timestamp,
	}
}

// saveResults saves all the rendered images
func saveResults(config Config, result RenderResult, outputDir string) {
	// Progressive mode handles its own file saving in the callback
	if config.RenderMode == "progressive" {
		return
	}

	// Save final image for normal mode
	filename := filepath.Join(outputDir, fmt.Sprintf("render_%s.png", result.Timestamp))
	err := saveImageToFile(result.Image, filename)
	if err != nil {
		fmt.Printf("Error saving PNG: %v\n", err)
		os.Exit(1)
	}
}

// saveProgressiveImages saves intermediate progressive images (excluding the final pass)
func saveProgressiveImages(result RenderResult, outputDir string) {
	baseFilename := fmt.Sprintf("render_%s", result.Timestamp)

	// Save all passes except the last one (which gets saved as the final image)
	for i := 0; i < len(result.Images)-1; i++ {
		passImg := result.Images[i]
		passNumber := i + 1
		passFilename := filepath.Join(outputDir, fmt.Sprintf("%s_pass_%02d.png", baseFilename, passNumber))

		err := saveImageToFile(passImg, passFilename)
		if err != nil {
			fmt.Printf("Warning: Failed to save pass %d image: %v\n", passNumber, err)
		}
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
