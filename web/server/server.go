package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
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
	Scene                 string  `json:"scene"`                 // Scene name (e.g., "cornell-box")
	Width                 int     `json:"width"`                 // Image width
	Height                int     `json:"height"`                // Image height
	MaxSamples            int     `json:"maxSamples"`            // Maximum samples per pixel
	MaxPasses             int     `json:"maxPasses"`             // Maximum number of passes
	RRMinBounces          int     `json:"rrMinBounces"`          // Russian Roulette minimum bounces
	RRMinSamples          int     `json:"rrMinSamples"`          // Russian Roulette minimum samples
	AdaptiveMinSamples    int     `json:"adaptiveMinSamples"`    // Adaptive sampling minimum samples
	AdaptiveThreshold     float64 `json:"adaptiveThreshold"`     // Adaptive sampling relative error threshold
	AdaptiveDarkThreshold float64 `json:"adaptiveDarkThreshold"` // Adaptive sampling dark pixel threshold
}

// ProgressUpdate represents a single progressive update sent via SSE
type ProgressUpdate struct {
	PassNumber  int    `json:"passNumber"`
	TotalPasses int    `json:"totalPasses"`
	ImageData   string `json:"imageData"` // Base64 encoded PNG
	Stats       Stats  `json:"stats"`
	IsComplete  bool   `json:"isComplete"`
	ElapsedMs   int64  `json:"elapsedMs"`
}

// Stats represents render statistics
type Stats struct {
	TotalPixels    int     `json:"totalPixels"`
	TotalSamples   int64   `json:"totalSamples"`
	AverageSamples float64 `json:"averageSamples"`
	MaxSamples     int     `json:"maxSamples"`
	MinSamples     int     `json:"minSamples"`
	MaxSamplesUsed int     `json:"maxSamplesUsed"`
}

// Start starts the web server
func (s *Server) Start() error {
	// Serve static files
	http.Handle("/", http.FileServer(http.Dir("static/")))

	// API endpoints
	http.HandleFunc("/api/render", s.handleRender)
	http.HandleFunc("/api/health", s.handleHealth)
	http.HandleFunc("/api/scene-config", s.handleSceneConfig)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting web server on http://localhost%s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleHealth provides a simple health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRender handles progressive rendering requests with SSE
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Parse request parameters
	req, err := s.parseRenderRequest(r)
	if err != nil {
		s.sendSSEError(w, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Create scene
	sceneObj := s.createScene(req.Scene)
	if sceneObj == nil {
		s.sendSSEError(w, "Unknown scene: "+req.Scene)
		return
	}

	// Override Russian Roulette settings from the web request
	// Modify the scene's sampling config directly
	sceneObj.SamplingConfig.RussianRouletteMinBounces = req.RRMinBounces
	sceneObj.SamplingConfig.RussianRouletteMinSamples = req.RRMinSamples
	sceneObj.SamplingConfig.AdaptiveMinSamples = req.AdaptiveMinSamples
	sceneObj.SamplingConfig.AdaptiveThreshold = req.AdaptiveThreshold
	sceneObj.SamplingConfig.AdaptiveDarkThreshold = req.AdaptiveDarkThreshold

	// Create progressive raytracer
	config := renderer.ProgressiveConfig{
		TileSize:           64,
		InitialSamples:     1,
		MaxSamplesPerPixel: req.MaxSamples,
		MaxPasses:          req.MaxPasses,
		NumWorkers:         0, // Auto-detect
	}

	raytracer := renderer.NewProgressiveRaytracer(sceneObj, req.Width, req.Height, config)

	// Use request context to detect client disconnection
	ctx := r.Context()

	// Start progressive rendering with callback
	startTime := time.Now()

	err = raytracer.RenderProgressive(ctx, func(result renderer.PassResult) error {
		// Convert image to base64 PNG
		imageData, err := s.imageToBase64PNG(result.Image)
		if err != nil {
			return fmt.Errorf("failed to encode image: %v", err)
		}

		// Create progress update
		update := ProgressUpdate{
			PassNumber:  result.PassNumber,
			TotalPasses: req.MaxPasses,
			ImageData:   imageData,
			Stats: Stats{
				TotalPixels:    result.Stats.TotalPixels,
				TotalSamples:   int64(result.Stats.TotalSamples),
				AverageSamples: result.Stats.AverageSamples,
				MaxSamples:     result.Stats.MaxSamples,
				MinSamples:     result.Stats.MinSamples,
				MaxSamplesUsed: result.Stats.MaxSamplesUsed,
			},
			IsComplete: result.IsLast,
			ElapsedMs:  time.Since(startTime).Milliseconds(),
		}

		// Send SSE event
		return s.sendSSEUpdate(w, update)
	})

	if err != nil {
		s.sendSSEError(w, fmt.Sprintf("Render error: %v", err))
		return
	}

	// Send completion event
	s.sendSSEEvent(w, "complete", "Rendering completed")
}

// parseRenderRequest parses request parameters
func (s *Server) parseRenderRequest(r *http.Request) (*RenderRequest, error) {
	// Initialize request with defaults handled by helper functions
	req := &RenderRequest{}

	// Parse scene name (string parameter, no validation needed)
	if scene := r.URL.Query().Get("scene"); scene != "" {
		req.Scene = scene
	} else {
		req.Scene = "cornell-box" // Default scene
	}

	// Parse and validate all parameters using helper functions
	var err error
	if req.Width, err = parseIntParam(r.URL.Query(), "width", 400, 100, 2000); err != nil {
		return nil, err
	}
	if req.Height, err = parseIntParam(r.URL.Query(), "height", 400, 100, 2000); err != nil {
		return nil, err
	}
	if req.MaxSamples, err = parseIntParam(r.URL.Query(), "maxSamples", 50, 1, 10000); err != nil {
		return nil, err
	}
	if req.MaxPasses, err = parseIntParam(r.URL.Query(), "maxPasses", 7, 1, 10000); err != nil {
		return nil, err
	}
	if req.RRMinBounces, err = parseIntParam(r.URL.Query(), "rrMinBounces", 5, 1, 1000); err != nil {
		return nil, err
	}
	if req.RRMinSamples, err = parseIntParam(r.URL.Query(), "rrMinSamples", 8, 1, 1000); err != nil {
		return nil, err
	}
	if req.AdaptiveMinSamples, err = parseIntParam(r.URL.Query(), "adaptiveMinSamples", 8, 1, 1000); err != nil {
		return nil, err
	}
	if req.AdaptiveThreshold, err = parseFloatParam(r.URL.Query(), "adaptiveThreshold", 0.01, 0.001, 0.5); err != nil {
		return nil, err
	}
	if req.AdaptiveDarkThreshold, err = parseFloatParam(r.URL.Query(), "adaptiveDarkThreshold", 1e-6, 1e-10, 1e-3); err != nil {
		return nil, err
	}

	// Performance warning
	if req.Width*req.Height > 800*600 && req.MaxSamples > 100 {
		log.Printf("Render warning: Large image with high samples may render slowly")
	}

	return req, nil
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

// createScene creates a scene based on the scene name
func (s *Server) createScene(sceneName string) *scene.Scene {
	switch sceneName {
	case "cornell-box":
		return scene.NewCornellScene()
	case "basic":
		return scene.NewDefaultScene()
	default:
		return nil
	}
}

// imageToBase64PNG converts an image to base64-encoded PNG
func (s *Server) imageToBase64PNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// sendSSEUpdate sends a progress update via SSE
func (s *Server) sendSSEUpdate(w http.ResponseWriter, update ProgressUpdate) error {
	data, err := json.Marshal(update)
	if err != nil {
		return err
	}
	return s.sendSSEEvent(w, "progress", string(data))
}

// sendSSEError sends an error via SSE
func (s *Server) sendSSEError(w http.ResponseWriter, message string) error {
	return s.sendSSEEvent(w, "error", message)
}

// sendSSEEvent sends a generic SSE event
func (s *Server) sendSSEEvent(w http.ResponseWriter, event, data string) error {
	if flusher, ok := w.(http.Flusher); ok {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
		return nil
	}
	return fmt.Errorf("streaming not supported")
}

// handleSceneConfig returns the default configuration for a scene
func (s *Server) handleSceneConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sceneName := r.URL.Query().Get("scene")
	if sceneName == "" {
		sceneName = "cornell-box" // Default scene
	}

	sceneObj := s.createScene(sceneName)
	if sceneObj == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown scene: " + sceneName})
		return
	}

	// Return the scene's sampling configuration with validation limits
	config := sceneObj.GetSamplingConfig()
	response := map[string]interface{}{
		"scene": sceneName,
		"defaults": map[string]interface{}{
			"samplesPerPixel":           config.SamplesPerPixel,
			"maxDepth":                  config.MaxDepth,
			"russianRouletteMinBounces": config.RussianRouletteMinBounces,
			"russianRouletteMinSamples": config.RussianRouletteMinSamples,
			"adaptiveMinSamples":        config.AdaptiveMinSamples,
			"adaptiveThreshold":         config.AdaptiveThreshold,
			"adaptiveDarkThreshold":     config.AdaptiveDarkThreshold,
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
			"russianRouletteMinSamples": map[string]int{
				"min": 1,
				"max": 1000,
			},
			"adaptiveMinSamples": map[string]int{
				"min": 1,
				"max": 1000,
			},
			"adaptiveThreshold": map[string]float64{
				"min": 0.001,
				"max": 0.5,
			},
			"adaptiveDarkThreshold": map[string]float64{
				"min": 1e-10,
				"max": 1e-3,
			},
		},
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
