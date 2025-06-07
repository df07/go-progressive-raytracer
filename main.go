package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// Config holds all the configuration for the raytracer
type Config struct {
	SceneType  string
	RenderMode string
	MaxPasses  int
	NumWorkers int
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
	raytracer := setupRaytracer(sceneInfo)

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
	flag.StringVar(&config.SceneType, "scene", "default", "Scene type: 'default' or 'cornell'")
	flag.StringVar(&config.RenderMode, "mode", "normal", "Render mode: 'normal' or 'progressive'")
	flag.IntVar(&config.MaxPasses, "max-passes", 5, "Maximum number of progressive passes")
	flag.IntVar(&config.NumWorkers, "workers", 0, "Number of parallel workers (0 = auto-detect CPU count)")
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
	fmt.Println("  default - Default scene with spheres and plane ground")
	fmt.Println("  cornell - Cornell box scene with quad walls and area lighting")
	fmt.Println()
	fmt.Println("Available modes:")
	fmt.Println("  normal      - Standard single-threaded rendering")
	fmt.Println("  progressive - Progressive multi-pass parallel rendering")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  raytracer.exe --mode=progressive --max-passes=5")
	fmt.Println("  raytracer.exe --scene=cornell --mode=progressive --workers=4")
	fmt.Println("  raytracer.exe --mode=normal")
	fmt.Println()
	fmt.Println("Output will be saved to output/<scene_type>/render_<timestamp>.png")
}

// createScene creates the appropriate scene based on scene type
func createScene(sceneType string) SceneInfo {
	switch sceneType {
	case "cornell":
		fmt.Println("Using Cornell scene...")
		return SceneInfo{
			Scene:  scene.NewCornellScene(),
			Width:  400,
			Height: 400, // Square aspect ratio for Cornell box
		}
	case "default":
		fmt.Println("Using default scene...")
		return SceneInfo{
			Scene:  scene.NewDefaultScene(),
			Width:  400,
			Height: 225, // 16:9 aspect ratio
		}
	default:
		fmt.Printf("Unknown scene type: %s. Using default scene.\n", sceneType)
		return SceneInfo{
			Scene:  scene.NewDefaultScene(),
			Width:  400,
			Height: 225,
		}
	}
}

// setupRaytracer creates and configures a raytracer
func setupRaytracer(sceneInfo SceneInfo) *renderer.Raytracer {
	raytracer := renderer.NewRaytracer(sceneInfo.Scene, sceneInfo.Width, sceneInfo.Height)
	raytracer.SetSamplingConfig(renderer.SamplingConfig{
		SamplesPerPixel: 50, // Reduced for faster iteration
		MaxDepth:        25, // Reduced for faster iteration
	})
	return raytracer
}

// createOutputDir creates the output directory for the scene type
func createOutputDir(sceneType string) string {
	// Normalize scene type
	if sceneType != "cornell" && sceneType != "default" {
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
		return renderNormal(raytracer, timestamp)
	}
}

// renderProgressive handles progressive rendering
func renderProgressive(config Config, sceneInfo SceneInfo, timestamp string) RenderResult {
	fmt.Println("Using progressive rendering...")

	progressiveConfig := renderer.DefaultProgressiveConfig()
	progressiveConfig.MaxPasses = config.MaxPasses
	progressiveConfig.NumWorkers = config.NumWorkers

	progressiveRT := renderer.NewProgressiveRaytracer(sceneInfo.Scene, sceneInfo.Width, sceneInfo.Height, progressiveConfig)

	images, allStats, err := progressiveRT.RenderProgressive()
	if err != nil {
		fmt.Printf("Error during progressive rendering: %v\n", err)
		os.Exit(1)
	}

	if len(images) == 0 {
		fmt.Println("No images were rendered")
		os.Exit(1)
	}

	return RenderResult{
		Image:     images[len(images)-1],
		Stats:     allStats[len(allStats)-1],
		Images:    images,
		AllStats:  allStats,
		Timestamp: timestamp,
	}
}

// renderNormal handles normal rendering
func renderNormal(raytracer *renderer.Raytracer, timestamp string) RenderResult {
	img, stats := raytracer.RenderPass()

	return RenderResult{
		Image:     img,
		Stats:     stats,
		Timestamp: timestamp,
	}
}

// saveResults saves all the rendered images
func saveResults(config Config, result RenderResult, outputDir string) {
	// Save progressive pass images if applicable
	if config.RenderMode == "progressive" {
		saveProgressiveImages(result, outputDir)
	}

	// Save final image
	filename := filepath.Join(outputDir, fmt.Sprintf("render_%s.png", result.Timestamp))
	err := saveImageToFile(result.Image, filename)
	if err != nil {
		fmt.Printf("Error saving PNG: %v\n", err)
		os.Exit(1)
	}
}

// saveProgressiveImages saves intermediate progressive images
func saveProgressiveImages(result RenderResult, outputDir string) {
	baseFilename := fmt.Sprintf("render_%s", result.Timestamp)

	for i, passImg := range result.Images {
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
