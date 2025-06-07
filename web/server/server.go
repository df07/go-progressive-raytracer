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
	"strconv"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
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
	Scene      string `json:"scene"`      // Scene name (e.g., "cornell-box")
	Width      int    `json:"width"`      // Image width
	Height     int    `json:"height"`     // Image height
	MaxSamples int    `json:"maxSamples"` // Maximum samples per pixel
	MaxPasses  int    `json:"maxPasses"`  // Maximum number of passes
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
	http.Handle("/", http.FileServer(http.Dir("web/static/")))

	// API endpoints
	http.HandleFunc("/api/render", s.handleRender)
	http.HandleFunc("/api/health", s.handleHealth)

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

	// Create progressive raytracer
	config := renderer.ProgressiveConfig{
		TileSize:           64,
		InitialSamples:     1,
		MaxSamplesPerPixel: req.MaxSamples,
		MaxPasses:          req.MaxPasses,
		NumWorkers:         0, // Auto-detect
	}

	raytracer := renderer.NewProgressiveRaytracer(sceneObj, req.Width, req.Height, config)

	// Start progressive rendering with callback
	startTime := time.Now()

	err = raytracer.RenderProgressiveWithCallback(func(result renderer.PassResult) error {
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
	// Default values
	req := &RenderRequest{
		Scene:      "cornell-box",
		Width:      400,
		Height:     400,
		MaxSamples: 50,
		MaxPasses:  7,
	}

	var warnings []string

	// Parse query parameters with validation
	if scene := r.URL.Query().Get("scene"); scene != "" {
		req.Scene = scene
	}

	if width := r.URL.Query().Get("width"); width != "" {
		if w, err := strconv.Atoi(width); err != nil {
			return nil, fmt.Errorf("invalid width: %s", width)
		} else if w <= 0 || w > 2000 {
			return nil, fmt.Errorf("width must be between 1 and 2000, got: %d", w)
		} else {
			req.Width = w
		}
	}

	if height := r.URL.Query().Get("height"); height != "" {
		if h, err := strconv.Atoi(height); err != nil {
			return nil, fmt.Errorf("invalid height: %s", height)
		} else if h <= 0 || h > 2000 {
			return nil, fmt.Errorf("height must be between 1 and 2000, got: %d", h)
		} else {
			req.Height = h
		}
	}

	if maxSamples := r.URL.Query().Get("maxSamples"); maxSamples != "" {
		if ms, err := strconv.Atoi(maxSamples); err != nil {
			return nil, fmt.Errorf("invalid maxSamples: %s", maxSamples)
		} else if ms <= 0 || ms > 10000 {
			return nil, fmt.Errorf("maxSamples must be between 1 and 10000, got: %d", ms)
		} else {
			req.MaxSamples = ms
		}
	}

	if maxPasses := r.URL.Query().Get("maxPasses"); maxPasses != "" {
		if mp, err := strconv.Atoi(maxPasses); err != nil {
			return nil, fmt.Errorf("invalid maxPasses: %s", maxPasses)
		} else if mp <= 0 || mp > 100 {
			return nil, fmt.Errorf("maxPasses must be between 1 and 100, got: %d", mp)
		} else {
			req.MaxPasses = mp
		}
	}

	// Performance warning
	if req.Width*req.Height > 800*600 && req.MaxSamples > 100 {
		warnings = append(warnings, "Large image with high samples may render slowly")
	}

	// Log the warnings but don't fail the request
	if len(warnings) > 0 {
		log.Printf("Render warnings: %v", warnings)
	}

	return req, nil
}

// createScene creates a scene based on the scene name
func (s *Server) createScene(sceneName string) core.Scene {
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
