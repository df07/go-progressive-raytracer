package main

import (
	"testing"
)

func TestCreateScene(t *testing.T) {
	tests := []struct {
		name        string
		sceneType   string
		expectError bool
	}{
		// Built-in scenes
		{"default scene", "default", false},
		{"cornell scene", "cornell", false},
		{"cornell-boxes scene", "cornell-boxes", false},
		{"spheregrid scene", "spheregrid", false},
		{"trianglemesh scene", "trianglemesh", false},
		{"dragon scene", "dragon", false},
		{"caustic-glass scene", "caustic-glass", false},
		{"cornell-pbrt scene", "cornell-pbrt", false},

		// PBRT scenes (by name)
		{"cornell-empty PBRT", "cornell-empty", false},
		{"simple-sphere PBRT", "simple-sphere", false},
		{"test PBRT", "test", false},

		// PBRT scenes (by path)
		{"direct PBRT path", "scenes/cornell-empty.pbrt", false},
		{"direct PBRT path 2", "scenes/simple-sphere.pbrt", false},

		// Invalid scenes
		{"unknown scene", "nonexistent", true},
		{"invalid PBRT path", "scenes/nonexistent.pbrt", true},
		{"empty scene name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene, err := createScene(tt.sceneType)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for scene type '%s', but got none", tt.sceneType)
				}
				if scene != nil {
					t.Errorf("Expected nil scene for invalid scene type '%s', got %T", tt.sceneType, scene)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for scene type '%s': %v", tt.sceneType, err)
				}
				if scene == nil {
					t.Errorf("Expected scene for valid scene type '%s', got nil", tt.sceneType)
				}

				// Verify scene has required properties
				if scene != nil {
					if scene.CameraConfig.Width <= 0 {
						t.Errorf("Scene camera width should be positive, got %d", scene.CameraConfig.Width)
					}
					if scene.SamplingConfig.Height <= 0 {
						t.Errorf("Scene sampling height should be positive, got %d", scene.SamplingConfig.Height)
					}
					if scene.SamplingConfig.Width <= 0 {
						t.Errorf("Scene sampling width should be positive, got %d", scene.SamplingConfig.Width)
					}
				}
			}
		})
	}
}

func TestTryLoadPBRTScene(t *testing.T) {
	tests := []struct {
		name       string
		sceneType  string
		expectLoad bool
	}{
		// Existing PBRT files (should load)
		{"cornell-empty by name", "cornell-empty", true},
		{"simple-sphere by name", "simple-sphere", true},
		{"cornell-empty by path", "scenes/cornell-empty.pbrt", true},

		// Non-existent files (should not load)
		{"nonexistent PBRT", "nonexistent", false},
		{"invalid path", "scenes/nonexistent.pbrt", false},
		{"built-in scene name", "cornell", false}, // Built-in scenes shouldn't load as PBRT
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := tryLoadPBRTScene(tt.sceneType)

			if tt.expectLoad {
				if scene == nil {
					t.Errorf("Expected PBRT scene to load for '%s', got nil", tt.sceneType)
				}
			} else {
				if scene != nil {
					t.Errorf("Expected PBRT scene not to load for '%s', got %T", tt.sceneType, scene)
				}
			}
		})
	}
}

func TestCreateOutputDir(t *testing.T) {
	tests := []struct {
		name         string
		sceneType    string
		expectedBase string
	}{
		// Built-in scenes
		{"default scene", "default", "default"},
		{"cornell scene", "cornell", "cornell"},

		// PBRT scenes by name
		{"cornell-empty PBRT", "cornell-empty", "cornell-empty"},
		{"simple-sphere PBRT", "simple-sphere", "simple-sphere"},

		// PBRT scenes by path
		{"PBRT file path", "scenes/cornell-empty.pbrt", "cornell-empty"},
		{"nested PBRT path", "scenes/subdir/my-scene.pbrt", "my-scene"},

		// Unknown scenes
		{"unknown scene", "unknown", "pbrt-scene"},
		{"custom PBRT", "my-custom-scene", "pbrt-scene"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputDir := createOutputDir(tt.sceneType)

			if outputDir == "" {
				t.Errorf("Expected non-empty output directory for scene '%s'", tt.sceneType)
			}

			// Check that the directory contains the expected base name
			if !containsSubstring(outputDir, tt.expectedBase) {
				t.Errorf("Expected output directory to contain '%s', got '%s'", tt.expectedBase, outputDir)
			}

			// Check that it starts with "output/"
			if !containsSubstring(outputDir, "output") {
				t.Errorf("Expected output directory to contain 'output', got '%s'", outputDir)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsMiddle(s, substr))))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
