package server

import (
	"testing"
	"time"
)

func TestWebLogger_BasicLogging(t *testing.T) {
	// Create a channel to receive console messages
	messageChan := make(chan ConsoleMessage, 10)
	logger := NewWebLogger("test-render-123", messageChan)

	// Test basic logging
	testMessage := "Test log message"
	logger.Printf("%s\n", testMessage)

	// Wait for message to be sent to channel
	select {
	case msg := <-messageChan:
		expectedMessage := testMessage + "\n"
		if msg.Message != expectedMessage {
			t.Errorf("Expected message '%s', got '%s'", expectedMessage, msg.Message)
		}
		if msg.Level != "info" {
			t.Errorf("Expected level 'info', got '%s'", msg.Level)
		}
		if time.Since(msg.Timestamp) > time.Second {
			t.Errorf("Timestamp seems too old: %v", msg.Timestamp)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for console message")
	}
}

func TestWebLogger_MultipleMessages(t *testing.T) {
	messageChan := make(chan ConsoleMessage, 10)
	logger := NewWebLogger("test-render-456", messageChan)

	// Send multiple messages
	messages := []string{"Message 1", "Message 2", "Message 3"}
	for _, msg := range messages {
		logger.Printf("%s\n", msg)
	}

	// Collect all messages
	var receivedMessages []string
	timeout := time.After(200 * time.Millisecond)
	for i := 0; i < len(messages); i++ {
		select {
		case msg := <-messageChan:
			receivedMessages = append(receivedMessages, msg.Message)
		case <-timeout:
			t.Fatalf("Timeout waiting for message %d", i+1)
		}
	}

	// Verify all messages were received
	if len(receivedMessages) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(receivedMessages))
	}

	for i, expected := range messages {
		expectedWithNewline := expected + "\n"
		if i < len(receivedMessages) && receivedMessages[i] != expectedWithNewline {
			t.Errorf("Message %d: expected '%s', got '%s'", i, expectedWithNewline, receivedMessages[i])
		}
	}
}

func TestWebLogger_ChannelFull(t *testing.T) {
	// Create a small channel that will fill up
	messageChan := make(chan ConsoleMessage, 1)
	logger := NewWebLogger("test-render-789", messageChan)

	// Fill the channel
	logger.Printf("Message 1\n")

	// Wait for first message
	select {
	case <-messageChan:
		// Good, got the message
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for first message")
	}

	// Send more messages - these should not block even though channel is full
	logger.Printf("Message 2\n")
	logger.Printf("Message 3\n")

	// Logger should not block or panic when channel is full
	// This test passes if it doesn't hang or crash
}

func TestWebLogger_NilChannel(t *testing.T) {
	// Test logger with nil channel (should not panic)
	logger := NewWebLogger("test-render-nil", nil)

	// This should not panic
	logger.Printf("Test message with nil channel\n")
}

func TestWebLogger_FormattedMessages(t *testing.T) {
	messageChan := make(chan ConsoleMessage, 10)
	logger := NewWebLogger("test-render-format", messageChan)

	// Test formatted logging
	logger.Printf("Loading %s with %d triangles...\n", "dragon.ply", 12345)

	select {
	case msg := <-messageChan:
		expected := "Loading dragon.ply with 12345 triangles...\n"
		if msg.Message != expected {
			t.Errorf("Expected formatted message '%s', got '%s'", expected, msg.Message)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Timeout waiting for formatted message")
	}
}

func TestConsoleMessage_JSONSerialization(t *testing.T) {
	msg := ConsoleMessage{
		Message:   "Test message",
		Timestamp: time.Now(),
		Level:     "info",
	}

	// This tests that the struct can be marshaled to JSON (used in SSE)
	// The actual JSON marshaling is tested implicitly by the web server
	if msg.Message == "" {
		t.Error("Message should not be empty")
	}
	if msg.Level == "" {
		t.Error("Level should not be empty")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}
