package server

import (
	"fmt"
	"time"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// ConsoleMessage represents a console message with timestamp
type ConsoleMessage struct {
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "info", "warning", "error"
}

// WebLogger implements core.Logger by sending messages to a console channel
type WebLogger struct {
	renderID    string
	consoleChan chan<- ConsoleMessage
}

// NewWebLogger creates a new web logger for a specific render
func NewWebLogger(renderID string, consoleChan chan<- ConsoleMessage) core.Logger {
	return &WebLogger{
		renderID:    renderID,
		consoleChan: consoleChan,
	}
}

// Printf implements core.Logger interface
func (wl *WebLogger) Printf(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)

	// Also write to stdout for server logs
	fmt.Print(message)

	// Send to web console if channel is available (non-blocking)
	if wl.consoleChan != nil {
		select {
		case wl.consoleChan <- ConsoleMessage{
			Message:   message,
			Timestamp: time.Now(),
			Level:     "info",
		}:
		default:
			// Channel full, skip (don't block)
		}
	}
}
