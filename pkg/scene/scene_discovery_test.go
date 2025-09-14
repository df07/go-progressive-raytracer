package scene

import (
	"os"
	"strings"
	"testing"
)

func TestTitleCase(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"cornell-empty", "Cornell Empty"},
		{"dragon_gold", "Dragon Gold"},
		{"my-custom-scene", "My Custom Scene"},
		{"simple", "Simple"},
		{"UPPER-case", "Upper Case"},
		{"", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := titleCase(tc.input)
			if result != tc.expected {
				t.Errorf("titleCase(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestParsePBRTMetadata(t *testing.T) {
	// Create temporary test files
	testCases := []struct {
		name     string
		content  string
		expected SceneInfo
	}{
		{
			name: "complete_metadata.pbrt",
			content: `# Scene: Cornell Box
# Variant: Empty Room
# Description: Classic Cornell box with no objects
# Group: Cornell Variants

LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40`,
			expected: SceneInfo{
				ID:          "pbrt:complete_metadata",
				Name:        "Cornell Box",
				DisplayName: "Cornell Box - Empty Room",
				Description: "Classic Cornell box with no objects",
				Group:       "Cornell Variants",
				Type:        "pbrt",
				Variant:     "Empty Room",
			},
		},
		{
			name: "partial_metadata.pbrt",
			content: `# Scene: Dragon
# Description: Dragon mesh scene

LookAt 0 0 5  0 0 0  0 1 0`,
			expected: SceneInfo{
				ID:          "pbrt:partial_metadata",
				Name:        "Dragon",
				DisplayName: "Dragon",
				Description: "Dragon mesh scene",
				Group:       "PBRT Scenes", // Default group
				Type:        "pbrt",
				Variant:     "",
			},
		},
		{
			name:    "no_metadata.pbrt",
			content: `LookAt 0 0 5  0 0 0  0 1 0`,
			expected: SceneInfo{
				ID:          "pbrt:no_metadata",
				Name:        "No Metadata", // From filename
				DisplayName: "No Metadata",
				Description: "",
				Group:       "PBRT Scenes", // Default group
				Type:        "pbrt",
				Variant:     "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", tc.name)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write content
			if _, err := tmpFile.WriteString(tc.content); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			// Parse metadata
			result, err := ParsePBRTMetadata(tmpFile.Name())
			if err != nil {
				t.Fatalf("ParsePBRTMetadata() error: %v", err)
			}

			// Update expected with actual file path
			tc.expected.FilePath = tmpFile.Name()

			// Compare results
			if result.ID != tc.expected.ID {
				t.Errorf("ID = %q, want %q", result.ID, tc.expected.ID)
			}
			if result.Name != tc.expected.Name {
				t.Errorf("Name = %q, want %q", result.Name, tc.expected.Name)
			}
			if result.DisplayName != tc.expected.DisplayName {
				t.Errorf("DisplayName = %q, want %q", result.DisplayName, tc.expected.DisplayName)
			}
			if result.Description != tc.expected.Description {
				t.Errorf("Description = %q, want %q", result.Description, tc.expected.Description)
			}
			if result.Group != tc.expected.Group {
				t.Errorf("Group = %q, want %q", result.Group, tc.expected.Group)
			}
			if result.Type != tc.expected.Type {
				t.Errorf("Type = %q, want %q", result.Type, tc.expected.Type)
			}
			if result.Variant != tc.expected.Variant {
				t.Errorf("Variant = %q, want %q", result.Variant, tc.expected.Variant)
			}
		})
	}
}

func TestListPBRTScenes_EmptyDirectory(t *testing.T) {
	// Test with non-existent scenes directory
	// We can't easily test this without changing the working directory,
	// so we'll test the basic functionality with existing files
	scenes, err := ListPBRTScenes()
	if err != nil {
		t.Errorf("ListPBRTScenes() error: %v", err)
	}

	// Should return empty list or found scenes - both are valid
	if scenes == nil {
		t.Error("ListPBRTScenes() returned nil, expected empty slice")
	}
}

func TestListAllScenes(t *testing.T) {
	response, err := ListAllScenes()
	if err != nil {
		t.Fatalf("ListAllScenes() error: %v", err)
	}

	// Should have at least the built-in scenes group
	if len(response.Groups) == 0 {
		t.Error("ListAllScenes() returned no groups")
	}

	// Find built-in scenes group
	var builtInGroup *SceneGroup
	for i, group := range response.Groups {
		if group.Name == "Built-in Scenes" {
			builtInGroup = &response.Groups[i]
			break
		}
	}

	if builtInGroup == nil {
		t.Error("ListAllScenes() did not include Built-in Scenes group")
	} else {
		// Should have expected built-in scenes
		expectedScenes := []string{"cornell-box", "basic", "sphere-grid", "triangle-mesh-sphere", "dragon", "caustic-glass"}
		if len(builtInGroup.Scenes) != len(expectedScenes) {
			t.Errorf("Built-in scenes count = %d, want %d", len(builtInGroup.Scenes), len(expectedScenes))
		}

		// Check that all expected scenes are present
		sceneIDs := make(map[string]bool)
		for _, scene := range builtInGroup.Scenes {
			sceneIDs[scene.ID] = true
		}

		for _, expectedID := range expectedScenes {
			if !sceneIDs[expectedID] {
				t.Errorf("Missing expected built-in scene: %s", expectedID)
			}
		}
	}
}

func TestListAllScenes_WithPBRTScenes(t *testing.T) {
	// This test will work with whatever PBRT scenes exist in the /scenes directory
	response, err := ListAllScenes()
	if err != nil {
		t.Fatalf("ListAllScenes() error: %v", err)
	}

	// Check that all scenes have required fields
	for _, group := range response.Groups {
		if group.Name == "" {
			t.Error("Found group with empty name")
		}

		for _, scene := range group.Scenes {
			if scene.ID == "" {
				t.Error("Found scene with empty ID")
			}
			if scene.DisplayName == "" {
				t.Error("Found scene with empty DisplayName")
			}
			if scene.Type != "builtin" && scene.Type != "pbrt" {
				t.Errorf("Invalid scene type: %s", scene.Type)
			}
			if scene.Type == "pbrt" && scene.FilePath == "" {
				t.Error("PBRT scene missing FilePath")
			}
			if scene.Type == "pbrt" && !strings.HasPrefix(scene.ID, "pbrt:") {
				t.Errorf("PBRT scene ID should start with 'pbrt:': %s", scene.ID)
			}
		}
	}
}

func TestParsePBRTMetadata_InvalidFile(t *testing.T) {
	// Test with non-existent file
	_, err := ParsePBRTMetadata("nonexistent.pbrt")
	// Should not return error, should use fallback values
	if err != nil {
		t.Errorf("ParsePBRTMetadata() should handle missing files gracefully")
	}
}

func TestParsePBRTMetadata_EdgeCases(t *testing.T) {
	testCases := []struct {
		name    string
		content string
	}{
		{
			name: "malformed_comments.pbrt",
			content: `#Scene: Missing space
#Variant:
# Description:   Extra spaces
#Group:

LookAt 0 0 5  0 0 0  0 1 0`,
		},
		{
			name: "mixed_content.pbrt",
			content: `# Scene: Test Scene
Some non-comment line
# This comment should be ignored
# Variant: Test Variant

LookAt 0 0 5  0 0 0  0 1 0`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", tc.name)
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write content
			if _, err := tmpFile.WriteString(tc.content); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			// Should not crash or return error
			result, err := ParsePBRTMetadata(tmpFile.Name())
			if err != nil {
				t.Errorf("ParsePBRTMetadata() should handle malformed metadata: %v", err)
			}

			// Should have basic fields populated
			if result.ID == "" || result.DisplayName == "" {
				t.Error("ParsePBRTMetadata() should populate basic fields even with malformed metadata")
			}
		})
	}
}
