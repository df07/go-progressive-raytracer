package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"log"
	"net/http"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// TileUpdate represents a single tile update sent via SSE
type TileUpdate struct {
	TileX       int    `json:"tileX"`
	TileY       int    `json:"tileY"`
	ImageData   string `json:"imageData"` // Base64 encoded PNG of just this tile
	PassNumber  int    `json:"passNumber"`
	TileNumber  int    `json:"tileNumber"`  // Current tile number in this pass (1-based)
	TotalTiles  int    `json:"totalTiles"`  // Total number of tiles in the image
	TotalPasses int    `json:"totalPasses"` // Total number of passes planned
}

// SSEEvent represents a unified SSE event for thread-safe writing
type SSEEvent struct {
	Type string `json:"type"` // "console", "tile", "passComplete", "error", "complete"
	Data string `json:"data"` // JSON-encoded data
}

// RenderingPipeline contains the configured scene and raytracer
type RenderingPipeline struct {
	Scene     *scene.Scene
	Raytracer *renderer.ProgressiveRaytracer
}

// handleRender handles progressive rendering with real-time tile streaming via SSE
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	// Set SSE headers
	s.setSSEHeaders(w)

	ctx := r.Context()

	// Create unified SSE event channel for thread-safe writing
	sseEventChan := make(chan SSEEvent, 100)

	// Start single SSE writer goroutine
	go s.writeSSEEvents(w, ctx, sseEventChan)

	// Parse and validate request
	req, err := s.parseRenderRequest(r)
	if err != nil {
		s.handleError(ctx, sseEventChan, fmt.Sprintf("Invalid request: %v", err))
		return
	}

	// Setup console logging and streaming
	consoleChan, webLogger := s.setupConsoleLogging()
	go s.streamConsoleMessages(ctx, consoleChan, sseEventChan)

	pipeline, err := s.setupRenderingPipeline(req, webLogger)
	if err != nil {
		s.handleError(ctx, sseEventChan, err.Error())
		return
	}

	// Start rendering and stream events
	startTime := time.Now()
	renderOptions := renderer.RenderOptions{TileUpdates: true}
	passChan, tileChan, errChan := pipeline.Raytracer.RenderProgressive(ctx, renderOptions)

	// Handle rendering events and send to unified channel
	s.handleRenderingEvents(ctx, sseEventChan, passChan, tileChan, errChan, pipeline.Scene, req, startTime)
}

// setSSEHeaders sets the required headers for Server-Sent Events
func (s *Server) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

// setupConsoleLogging creates console channel and web logger for a render
func (s *Server) setupConsoleLogging() (chan ConsoleMessage, core.Logger) {
	consoleChan := make(chan ConsoleMessage, 50)
	renderID := fmt.Sprintf("render-%d", time.Now().UnixNano())
	webLogger := NewWebLogger(renderID, consoleChan)
	return consoleChan, webLogger
}

// writeSSEEvents handles writing all SSE events in a single goroutine (thread-safe)
func (s *Server) writeSSEEvents(w http.ResponseWriter, ctx context.Context, sseEventChan chan SSEEvent) {
	defer func() {
		if r := recover(); r != nil {
			// Client disconnected during write, this is expected behavior when stopping renders
			log.Printf("SSE writer recovered from panic (client disconnected): %v", r)
		}
	}()

	for {
		select {
		case event, ok := <-sseEventChan:
			if !ok {
				// Channel closed
				return
			}

			// Check if client is still connected before writing
			select {
			case <-ctx.Done():
				// Client disconnected, stop sending messages
				return
			default:
			}

			// Write SSE event
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
			if err != nil {
				// Client disconnected during write
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

		case <-ctx.Done():
			// Client disconnected
			return
		}
	}
}

// streamConsoleMessages handles the console message streaming goroutine
func (s *Server) streamConsoleMessages(ctx context.Context, consoleChan chan ConsoleMessage, sseEventChan chan SSEEvent) {
	for {
		select {
		case consoleMsg, ok := <-consoleChan:
			if !ok {
				// Channel closed
				return
			}

			// Send console message as SSE event
			data, err := json.Marshal(consoleMsg)
			if err != nil {
				log.Printf("Error marshaling console message: %v", err)
				continue
			}

			// Check if client is still connected before writing
			select {
			case <-ctx.Done():
				// Client disconnected, stop sending messages
				return
			default:
			}

			// Send to unified SSE channel
			select {
			case sseEventChan <- SSEEvent{Type: "console", Data: string(data)}:
			case <-ctx.Done():
				return
			default:
				// Channel full, skip message to avoid blocking
			}

		case <-ctx.Done():
			// Client disconnected
			return
		}
	}
}

// setupRenderingPipeline creates and configures the scene and raytracer
func (s *Server) setupRenderingPipeline(req *RenderRequest, logger core.Logger) (*RenderingPipeline, error) {
	// Create scene (logging will now go through WebLogger)
	sceneObj := s.createScene(req, false, logger)
	if sceneObj == nil {
		return nil, fmt.Errorf("Unknown scene: %s", req.Scene)
	}

	// Override sampling settings
	sceneObj.SamplingConfig.Width = req.Width
	sceneObj.SamplingConfig.Height = req.Height
	sceneObj.SamplingConfig.RussianRouletteMinBounces = req.RRMinBounces
	sceneObj.SamplingConfig.AdaptiveMinSamples = req.AdaptiveMinSamples
	sceneObj.SamplingConfig.AdaptiveThreshold = req.AdaptiveThreshold

	// Create progressive raytracer
	config := renderer.ProgressiveConfig{
		TileSize:           DefaultTileSize,
		InitialSamples:     1,
		MaxSamplesPerPixel: req.MaxSamples,
		MaxPasses:          req.MaxPasses,
		NumWorkers:         0, // Auto-detect
	}

	// Create the appropriate integrator based on request
	var selectedIntegrator integrator.Integrator
	switch req.Integrator {
	case "bdpt":
		selectedIntegrator = integrator.NewBDPTIntegrator(sceneObj.SamplingConfig)
	case "path-tracing":
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.SamplingConfig)
	default:
		// Default to path tracing for unknown integrator types
		selectedIntegrator = integrator.NewPathTracingIntegrator(sceneObj.SamplingConfig)
	}

	raytracer, err := renderer.NewProgressiveRaytracer(sceneObj, config, selectedIntegrator, logger)
	if err != nil {
		return nil, fmt.Errorf("error creating progressive raytracer: %w", err)
	}
	return &RenderingPipeline{
		Scene:     sceneObj,
		Raytracer: raytracer,
	}, nil
}

// handleRenderingEvents processes the main rendering event loop
func (s *Server) handleRenderingEvents(ctx context.Context, sseEventChan chan SSEEvent,
	passChan <-chan renderer.PassResult, tileChan <-chan renderer.TileCompletionResult, errChan <-chan error,
	scene *scene.Scene, req *RenderRequest, startTime time.Time) {

renderLoop:
	for {
		select {
		case passResult, ok := <-passChan:
			if !ok {
				passChan = nil // Channel closed
				continue
			}
			s.handlePassComplete(ctx, sseEventChan, passResult, req, scene, startTime)

		case tileResult, ok := <-tileChan:
			if !ok {
				tileChan = nil // Channel closed
				continue
			}
			s.handleTileUpdate(ctx, sseEventChan, tileResult)

		case err := <-errChan:
			if err != nil {
				s.handleError(ctx, sseEventChan, fmt.Sprintf("Rendering failed: %v", err))
				return
			}
			// errChan closed, rendering completed successfully
			s.drainRemainingChannels(ctx, sseEventChan, passChan, tileChan, req, scene, startTime)
			break renderLoop

		case <-ctx.Done():
			// Client disconnected
			return
		}
	}

	// Send completion event
	select {
	case sseEventChan <- SSEEvent{Type: "complete", Data: "Rendering completed"}:
		// Give the SSE writer time to send the event before the handler returns
		time.Sleep(50 * time.Millisecond)
	case <-ctx.Done():
		// Client disconnected
	}
}

// handlePassComplete processes and sends pass completion events
func (s *Server) handlePassComplete(ctx context.Context, sseEventChan chan SSEEvent, passResult renderer.PassResult, req *RenderRequest, scene *scene.Scene, startTime time.Time) {
	// Check if client is still connected
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Create pass completion data
	elapsed := time.Since(startTime)
	primitiveCount := scene.GetPrimitiveCount()

	// Calculate average luminance from the image
	avgLuminance := calculateAverageLuminance(passResult.Image)

	passUpdate := struct {
		Event            string  `json:"event"`
		PassNumber       int     `json:"passNumber"`
		TotalPasses      int     `json:"totalPasses"`
		ElapsedMs        int64   `json:"elapsedMs"`
		TotalPixels      int     `json:"totalPixels"`
		TotalSamples     int     `json:"totalSamples"`
		AverageSamples   float64 `json:"averageSamples"`
		MaxSamples       int     `json:"maxSamples"`
		MinSamples       int     `json:"minSamples"`
		MaxSamplesUsed   int     `json:"maxSamplesUsed"`
		PrimitiveCount   int     `json:"primitiveCount"`
		AverageLuminance float64 `json:"averageLuminance"`
	}{
		Event:            "passComplete",
		PassNumber:       passResult.PassNumber,
		TotalPasses:      req.MaxPasses,
		ElapsedMs:        elapsed.Milliseconds(),
		TotalPixels:      passResult.Stats.TotalPixels,
		TotalSamples:     passResult.Stats.TotalSamples,
		AverageSamples:   passResult.Stats.AverageSamples,
		MaxSamples:       passResult.Stats.MaxSamples,
		MinSamples:       passResult.Stats.MinSamples,
		MaxSamplesUsed:   passResult.Stats.MaxSamplesUsed,
		PrimitiveCount:   primitiveCount,
		AverageLuminance: avgLuminance,
	}

	data, err := json.Marshal(passUpdate)
	if err != nil {
		log.Printf("Error marshaling pass update: %v", err)
		return
	}

	select {
	case sseEventChan <- SSEEvent{Type: "passComplete", Data: string(data)}:
	case <-ctx.Done():
	}
}

// handleTileUpdate processes and sends tile update events
func (s *Server) handleTileUpdate(ctx context.Context, sseEventChan chan SSEEvent, tileResult renderer.TileCompletionResult) {
	// Check if client is still connected
	select {
	case <-ctx.Done():
		return
	default:
	}

	// Convert tile image to base64 PNG
	tileData, err := s.imageToBase64PNG(tileResult.TileImage)
	if err != nil {
		log.Printf("Error encoding tile image (%d, %d): %v", tileResult.TileX, tileResult.TileY, err)
		return
	}

	// Create and send tile update
	update := TileUpdate{
		TileX:       tileResult.TileX,
		TileY:       tileResult.TileY,
		ImageData:   tileData,
		PassNumber:  tileResult.PassNumber,
		TileNumber:  tileResult.TileNumber,
		TotalTiles:  tileResult.TotalTiles,
		TotalPasses: tileResult.TotalPasses,
	}

	data, err := json.Marshal(update)
	if err != nil {
		log.Printf("Error marshaling tile update: %v", err)
		return
	}

	select {
	case sseEventChan <- SSEEvent{Type: "tile", Data: string(data)}:
	case <-ctx.Done():
	}
}

// parseRenderRequest parses request parameters
func (s *Server) parseRenderRequest(r *http.Request) (*RenderRequest, error) {
	// Initialize request
	req := &RenderRequest{}

	// Parse common scene parameters using shared function
	if err := s.parseCommonSceneParams(r, req); err != nil {
		return nil, err
	}

	// Parse and validate render-specific parameters using helper functions
	var err error
	if req.MaxSamples, err = parseIntParam(r.URL.Query(), "maxSamples", 50, 1, 10000); err != nil {
		return nil, err
	}
	if req.MaxPasses, err = parseIntParam(r.URL.Query(), "maxPasses", 7, 1, 10000); err != nil {
		return nil, err
	}
	if req.RRMinBounces, err = parseIntParam(r.URL.Query(), "rrMinBounces", 5, 1, 1000); err != nil {
		return nil, err
	}
	if req.AdaptiveMinSamples, err = parseFloatParam(r.URL.Query(), "adaptiveMinSamples", 0.15, 0.01, 1.0); err != nil {
		return nil, err
	}
	if req.AdaptiveThreshold, err = parseFloatParam(r.URL.Query(), "adaptiveThreshold", 0.01, 0.001, 0.5); err != nil {
		return nil, err
	}

	// Performance warning
	if req.Width*req.Height > 800*600 && req.MaxSamples > 100 {
		log.Printf("Render warning: Large image with high samples may render slowly")
	}

	return req, nil
}

// imageToBase64PNG converts an image to base64-encoded PNG
func (s *Server) imageToBase64PNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// handleError sends an error event to the SSE channel
func (s *Server) handleError(ctx context.Context, sseEventChan chan SSEEvent, message string) {
	select {
	case sseEventChan <- SSEEvent{Type: "error", Data: message}:
	case <-ctx.Done():
		// Client disconnected, don't block
	}
}

// drainRemainingChannels processes any remaining data from passChan and tileChan
func (s *Server) drainRemainingChannels(ctx context.Context, sseEventChan chan SSEEvent,
	passChan <-chan renderer.PassResult, tileChan <-chan renderer.TileCompletionResult,
	req *RenderRequest, scene *scene.Scene, startTime time.Time) {

	timeout := time.After(100 * time.Millisecond)

	for passChan != nil || tileChan != nil {
		select {
		case passResult, ok := <-passChan:
			if !ok {
				passChan = nil
				continue
			}
			s.handlePassComplete(ctx, sseEventChan, passResult, req, scene, startTime)
		case tileResult, ok := <-tileChan:
			if !ok {
				tileChan = nil
				continue
			}
			s.handleTileUpdate(ctx, sseEventChan, tileResult)
		case <-timeout:
			// Timeout waiting for channels to close - this is normal for fast renders
			return
		case <-ctx.Done():
			return
		}
	}
}

// calculateAverageLuminance calculates the average luminance of an RGBA image
func calculateAverageLuminance(img *image.RGBA) float64 {
	bounds := img.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y
	totalPixels := width * height

	if totalPixels == 0 {
		return 0.0
	}

	var totalLuminance float64

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			// Convert from 16-bit to 8-bit values (0-255)
			r8 := float64(r>>8) / 255.0
			g8 := float64(g>>8) / 255.0
			b8 := float64(b>>8) / 255.0

			// Calculate luminance using standard RGB to luminance conversion
			// Y = 0.299*R + 0.587*G + 0.114*B
			luminance := 0.299*r8 + 0.587*g8 + 0.114*b8
			totalLuminance += luminance
		}
	}

	return totalLuminance / float64(totalPixels)
}
