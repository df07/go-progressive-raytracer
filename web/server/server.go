package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

const (
	// Streaming configuration constants
	DefaultTileSize          = 64   // Size of each tile in pixels
	TileUpdateChannelBuffer  = 100  // Buffer size for tile update channel
	MaxConcurrentTileUpdates = 1000 // Maximum tiles that can be queued
)

// Server handles web requests for the progressive raytracer
type Server struct {
	port int
}

// NewServer creates a new web server
func NewServer(port int) *Server {
	return &Server{port: port}
}

// RenderRequest represents a render request from the client
type RenderRequest struct {
	Scene              string  `json:"scene"`              // Scene name (e.g., "cornell-box")
	Width              int     `json:"width"`              // Image width
	Height             int     `json:"height"`             // Image height
	MaxSamples         int     `json:"maxSamples"`         // Maximum samples per pixel
	MaxPasses          int     `json:"maxPasses"`          // Maximum number of passes
	RRMinBounces       int     `json:"rrMinBounces"`       // Russian Roulette minimum bounces
	AdaptiveMinSamples float64 `json:"adaptiveMinSamples"` // Adaptive sampling minimum samples as percentage (0.0-1.0)
	AdaptiveThreshold  float64 `json:"adaptiveThreshold"`  // Adaptive sampling relative error threshold
	Integrator         string  `json:"integrator"`         // Integrator type: "path-tracing" or "bdpt"

	// Scene-specific configuration
	CornellGeometry      string           `json:"cornellGeometry"`      // Cornell box geometry type: "spheres", "boxes", "empty"
	SphereGridSize       int              `json:"sphereGridSize"`       // Sphere grid size (e.g., 10, 20, 100)
	MaterialFinish       string           `json:"materialFinish"`       // Material finish for sphere grid: "metallic", "matte", "glossy", "glass", "mirror", "mixed"
	SphereComplexity     int              `json:"sphereComplexity"`     // Triangle mesh sphere complexity
	DragonMaterialFinish string           `json:"dragonMaterialFinish"` // Dragon material finish: "gold", "plastic", "matte", "mirror", "glass", "copper"
	LightType            lights.LightType `json:"lightType"`            // Light type: "area", "point"
}

// Stats represents render statistics
type Stats struct {
	TotalPixels    int     `json:"totalPixels"`
	TotalSamples   int64   `json:"totalSamples"`
	AverageSamples float64 `json:"averageSamples"`
	MaxSamples     int     `json:"maxSamples"`
	MinSamples     int     `json:"minSamples"`
	MaxSamplesUsed int     `json:"maxSamplesUsed"`
	PrimitiveCount int     `json:"primitiveCount"`
}

// Start starts the web server
func (s *Server) Start() error {
	// Serve static files with no-cache headers during development
	fileServer := http.FileServer(http.Dir("static/"))
	http.Handle("/", noCacheMiddleware(fileServer))

	// API endpoints
	http.HandleFunc("/api/render", s.handleRender) // Real-time tile streaming
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/scene-config", s.handleSceneConfig)
	http.HandleFunc("/api/scenes", s.handleScenes) // Scene discovery
	http.HandleFunc("/api/inspect", s.handleInspect)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting web server on http://localhost%s", addr)
	return http.ListenAndServe(addr, nil)
}

// noCacheMiddleware adds cache control headers to prevent browser caching during development
func noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Disable caching for all static files during development
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}

// handleHealth provides a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleScenes returns all available scenes (built-in and PBRT)
func (s *Server) handleScenes(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get all scenes using the scene discovery service
	scenes, err := scene.ListAllScenes()
	if err != nil {
		log.Printf("Error listing scenes: %v", err)
		http.Error(w, "Failed to list scenes", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(scenes)
}

// parseIntParam parses an integer parameter from URL query with validation
func parseIntParam(values url.Values, key string, defaultValue, min, max int) (int, error) {
	if value := values.Get(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %s", key, value)
		}
		if parsed < min || parsed > max {
			return 0, fmt.Errorf("%s must be between %d and %d, got: %d", key, min, max, parsed)
		}
		return parsed, nil
	}
	return defaultValue, nil
}

// parseFloatParam parses a float parameter from URL query with validation
func parseFloatParam(values url.Values, key string, defaultValue, min, max float64) (float64, error) {
	if value := values.Get(key); value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid %s: %s", key, value)
		}
		if parsed < min || parsed > max {
			return 0, fmt.Errorf("%s must be between %f and %f, got: %f", key, min, max, parsed)
		}
		return parsed, nil
	}
	return defaultValue, nil
}

// parseCommonSceneParams parses all common scene parameters (basic + scene-specific)
func (s *Server) parseCommonSceneParams(r *http.Request, req *RenderRequest) error {
	var err error

	// Parse scene name
	if scene := r.URL.Query().Get("scene"); scene != "" {
		req.Scene = scene
	} else {
		req.Scene = "cornell-box" // Default scene
	}

	// Parse width and height
	if req.Width, err = parseIntParam(r.URL.Query(), "width", 400, 100, 2000); err != nil {
		return err
	}
	if req.Height, err = parseIntParam(r.URL.Query(), "height", 400, 100, 2000); err != nil {
		return err
	}

	// Parse Cornell geometry type
	req.CornellGeometry = r.URL.Query().Get("cornellGeometry")
	if req.CornellGeometry == "" {
		req.CornellGeometry = "boxes" // Default
	}

	// Parse sphere grid size
	if req.SphereGridSize, err = parseIntParam(r.URL.Query(), "sphereGridSize", 20, 5, 200); err != nil {
		return err
	}

	// Parse material finish
	req.MaterialFinish = r.URL.Query().Get("materialFinish")
	if req.MaterialFinish == "" {
		req.MaterialFinish = "metallic" // Default
	}

	// Parse dragon material finish
	req.DragonMaterialFinish = r.URL.Query().Get("dragonMaterialFinish")
	if req.DragonMaterialFinish == "" {
		req.DragonMaterialFinish = "gold" // Default
	}

	// Parse light type
	sLightType := r.URL.Query().Get("lightType")
	switch sLightType {
	case "area":
		req.LightType = lights.LightTypeArea
	case "point":
		req.LightType = lights.LightTypePoint
	default:
		req.LightType = lights.LightTypeArea // Default
	}

	// Parse sphere complexity parameter
	if req.SphereComplexity, err = parseIntParam(r.URL.Query(), "sphereComplexity", 32, 4, 512); err != nil {
		return err
	}

	// Parse integrator type
	req.Integrator = r.URL.Query().Get("integrator")
	if req.Integrator == "" {
		req.Integrator = "path-tracing" // Default integrator
	}

	return nil
}

// createScene creates a scene based on the scene name and optionally updates camera for requested dimensions
func (s *Server) createScene(req *RenderRequest, configOnly bool, logger core.Logger) *scene.Scene {
	// Use default logger if none provided
	if logger == nil {
		logger = renderer.NewDefaultLogger()
	}
	// Create camera override config (empty if width/height are 0, which means use defaults)
	var cameraOverride geometry.CameraConfig
	if req.Width > 0 && req.Height > 0 {
		cameraOverride = geometry.CameraConfig{
			Width:       req.Width,
			AspectRatio: float64(req.Width) / float64(req.Height),
		}
	}

	// Handle PBRT scenes first (they start with "pbrt:")
	if strings.HasPrefix(req.Scene, "pbrt:") {
		// Security: Validate scene ID against known PBRT scenes to prevent path traversal
		pbrtScenes, err := scene.ListPBRTScenes()
		if err != nil {
			log.Printf("Failed to list PBRT scenes for validation: %v", err)
			return nil
		}

		// Find the scene with matching ID
		var scenePath string
		for _, pbrtScene := range pbrtScenes {
			if pbrtScene.ID == req.Scene {
				scenePath = pbrtScene.FilePath
				break
			}
		}

		if scenePath == "" {
			log.Printf("Invalid PBRT scene ID: %s", req.Scene)
			return nil // Unknown scene ID
		}

		// Load actual PBRT scene with camera override using validated path
		parsedScene, err := loaders.LoadPBRT(scenePath)
		if err != nil {
			log.Printf("Failed to load PBRT file %s: %v", scenePath, err)
			return nil // Return nil to trigger proper error response
		}
		pbrtScene, err := scene.NewPBRTScene(parsedScene, cameraOverride)
		if err != nil {
			log.Printf("Failed to create PBRT scene %s: %v", scenePath, err)
			return nil // Return nil to trigger proper error response
		}
		return pbrtScene
	}

	// Single switch statement for built-in scenes - pass override (which may be empty for defaults)
	switch req.Scene {
	case "cornell-box":
		// Parse Cornell geometry type
		var geometryType scene.CornellGeometryType
		switch req.CornellGeometry {
		case "boxes":
			geometryType = scene.CornellBoxes
		case "empty":
			geometryType = scene.CornellEmpty
		default: // "spheres" or any other value
			geometryType = scene.CornellSpheres
		}
		return scene.NewCornellScene(geometryType, cameraOverride)
	case "basic":
		return scene.NewDefaultScene(cameraOverride)
	case "sphere-grid":
		return scene.NewSphereGridScene(req.SphereGridSize, req.MaterialFinish, cameraOverride)
	case "triangle-mesh-sphere":
		return scene.NewTriangleMeshScene(req.SphereComplexity, cameraOverride)
	case "dragon":
		loadMesh := !configOnly
		return scene.NewDragonScene(loadMesh, req.DragonMaterialFinish, logger, cameraOverride)
	case "caustic-glass":
		loadMesh := !configOnly
		return scene.NewCausticGlassScene(loadMesh, req.LightType, logger, cameraOverride)
	case "cylinder-test":
		return scene.NewCylinderTestScene(cameraOverride)
	case "cone-test":
		return scene.NewConeTestScene(cameraOverride)
	case "cornell-pbrt":
		if configOnly {
			// For config-only requests, return a basic cornell scene for dimensions
			return scene.NewCornellScene(scene.CornellEmpty, cameraOverride)
		}
		// Load actual PBRT scene
		parsedScene, err := loaders.LoadPBRT("scenes/cornell-empty.pbrt")
		if err != nil {
			log.Printf("Failed to load PBRT file: %v", err)
			return scene.NewCornellScene(scene.CornellEmpty, cameraOverride)
		}
		pbrtScene, err := scene.NewPBRTScene(parsedScene, cameraOverride)
		if err != nil {
			log.Printf("Failed to create PBRT scene: %v", err)
			return scene.NewCornellScene(scene.CornellEmpty, cameraOverride)
		}
		return pbrtScene
	default:
		return nil
	}
}

// handleSceneConfig returns the default configuration for a scene
func (s *Server) handleSceneConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sceneName := r.URL.Query().Get("scene")
	if sceneName == "" {
		sceneName = "cornell-box" // Default scene
	}

	// Create scene with default camera settings to get sampling config and default dimensions
	defaultReq := &RenderRequest{
		Scene:           sceneName,
		Width:           0,
		Height:          0,
		CornellGeometry: "boxes", // Default
		SphereGridSize:  20,      // Default
	}
	sceneObj := s.createScene(defaultReq, true, nil)
	if sceneObj == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown scene: " + sceneName})
		return
	}

	// Get default width and height from the scene's camera
	defaultWidth := sceneObj.CameraConfig.Width
	defaultHeight := int(float64(defaultWidth) / sceneObj.CameraConfig.AspectRatio)

	// Return the scene's sampling configuration with validation limits
	config := sceneObj.SamplingConfig

	// Set web-specific defaults for samples and passes
	webMaxSamples := config.SamplesPerPixel
	webMaxPasses := 10 // Default for most scenes

	// Override defaults for Cornell Box scene to show off the lighting better
	if sceneName == "cornell-box" {
		webMaxSamples = 800
		webMaxPasses = 40
	}

	// Override defaults for Caustic Glass scene for better quality
	if sceneName == "caustic-glass" {
		webMaxSamples = 10000 // Higher samples for caustics
		webMaxPasses = 1000   // More passes for progressive feedback
	}

	response := map[string]interface{}{
		"scene": sceneName,
		"defaults": map[string]interface{}{
			"width":                     defaultWidth,
			"height":                    defaultHeight,
			"samplesPerPixel":           webMaxSamples,
			"maxPasses":                 webMaxPasses,
			"maxDepth":                  config.MaxDepth,
			"russianRouletteMinBounces": config.RussianRouletteMinBounces,
			"adaptiveMinSamples":        config.AdaptiveMinSamples,
			"adaptiveThreshold":         config.AdaptiveThreshold,
			"cornellGeometry":           "boxes",
			"sphereGridSize":            20,
			"materialFinish":            "metallic",
			"sphereComplexity":          32,
			"dragonMaterialFinish":      "gold",
			"lightType":                 "area",
		},
		"limits": map[string]interface{}{
			"width": map[string]int{
				"min": 100,
				"max": 2000,
			},
			"height": map[string]int{
				"min": 100,
				"max": 2000,
			},
			"maxSamples": map[string]int{
				"min": 1,
				"max": 10000,
			},
			"maxPasses": map[string]int{
				"min": 1,
				"max": 10000,
			},
			"russianRouletteMinBounces": map[string]int{
				"min": 1,
				"max": 1000,
			},
			"adaptiveMinSamples": map[string]float64{
				"min": 0.01,
				"max": 1.0,
			},
			"adaptiveThreshold": map[string]float64{
				"min": 0.001,
				"max": 0.5,
			},
			"sphereGridSize": map[string]int{
				"min": 5,
				"max": 200,
			},
			"sphereComplexity": map[string]int{
				"min": 4,
				"max": 512,
			},
		},
	}

	// Add scene-specific configuration options
	switch sceneName {
	case "cornell-box":
		response["sceneOptions"] = map[string]interface{}{
			"cornellGeometry": map[string]interface{}{
				"type":    "select",
				"options": []string{"spheres", "boxes", "empty"},
				"default": "boxes",
			},
		}
	case "sphere-grid":
		response["sceneOptions"] = map[string]interface{}{
			"sphereGridSize": map[string]interface{}{
				"type":    "number",
				"min":     5,
				"max":     200,
				"default": 20,
			},
			"materialFinish": map[string]interface{}{
				"type":    "select",
				"options": []string{"metallic", "matte", "glossy", "mirror", "glass", "mixed"},
				"default": "metallic",
			},
		}
	case "triangle-mesh-sphere":
		response["sceneOptions"] = map[string]interface{}{
			"sphereComplexity": map[string]interface{}{
				"type":    "number",
				"min":     4,
				"max":     512,
				"default": 32,
				"label":   "Sphere Complexity",
			},
		}
	case "dragon":
		response["sceneOptions"] = map[string]interface{}{
			"dragonMaterialFinish": map[string]interface{}{
				"type":    "select",
				"options": []string{"gold", "plastic", "matte", "mirror", "glass", "copper"},
				"default": "gold",
			},
		}
	case "caustic-glass":
		response["sceneOptions"] = map[string]interface{}{
			"lightType": map[string]interface{}{
				"type":    "select",
				"options": []string{"area", "point"},
				"default": "area",
			},
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
