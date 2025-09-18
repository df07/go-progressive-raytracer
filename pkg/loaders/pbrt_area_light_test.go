package loaders

import (
	"os"
	"testing"
)

// Helper function to parse PBRT content from string for testing
func parsePBRTFromString(content string) (*PBRTScene, error) {
	// Create temporary file
	tmpFile, err := os.CreateTemp("", "pbrt_test_*.pbrt")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	// Write content to file
	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()

	// Parse using existing LoadPBRT function
	return LoadPBRT(tmpFile.Name())
}

func TestAreaLightParsing(t *testing.T) {
	tests := []struct {
		name                 string
		pbrtContent          string
		expectedLightSources int
		expectedShapes       int
		expectedAttributes   int
		description          string
	}{
		{
			name: "area light with sphere",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "rgb L" [ 10 8 6 ]
  Shape "sphere" "float radius" [ 1.0 ]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1, // AreaLightSource should be captured
			expectedShapes:       1, // Sphere should be captured
			expectedAttributes:   1, // Should be in one attribute block
			description:          "AreaLightSource followed by sphere in same attribute block",
		},
		{
			name: "area light with bilinear patch",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "rgb L" [ 15 12 8 ]
  Shape "bilinearPatch" "point3 P00" [0 2 0] "point3 P01" [1 2 0] "point3 P10" [0 2 1] "point3 P11" [1 2 1]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1,
			expectedShapes:       1,
			expectedAttributes:   1,
			description:          "AreaLightSource with bilinear patch (Cornell box style)",
		},
		{
			name: "area light with transform and shape",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "blackbody L" [ 6500 ] "float power" [ 100 ]
  Translate 0 10 0
  Shape "sphere" "float radius" [ 0.25 ]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1,
			expectedShapes:       1,
			expectedAttributes:   1,
			description:          "AreaLightSource with transform then shape (PBRT book example)",
		},
		{
			name: "multiple shapes after area light",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "rgb L" [ 5 5 5 ]
  Shape "sphere" "float radius" [ 1.0 ]
  Shape "sphere" "float radius" [ 0.5 ]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1,
			expectedShapes:       2, // Both spheres should get area light properties
			expectedAttributes:   1,
			description:          "AreaLightSource should apply to all subsequent shapes in block",
		},
		{
			name: "area light without shape",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "rgb L" [ 1 1 1 ]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1, // Should still parse the AreaLightSource
			expectedShapes:       0, // No shapes
			expectedAttributes:   1,
			description:          "AreaLightSource without shape should parse but not create lights",
		},
		{
			name: "shape before area light (wrong order)",
			pbrtContent: `
WorldBegin
AttributeBegin
  Shape "sphere" "float radius" [ 1.0 ]
  AreaLightSource "diffuse" "rgb L" [ 1 1 1 ]
AttributeEnd
WorldEnd`,
			expectedLightSources: 1,
			expectedShapes:       1,
			expectedAttributes:   1,
			description:          "AreaLightSource after shape should not affect the previous shape",
		},
		{
			name: "nested attribute blocks with area lights",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "rgb L" [ 2 2 2 ]
  AttributeBegin
    Shape "sphere" "float radius" [ 1.0 ]
  AttributeEnd
AttributeEnd
WorldEnd`,
			expectedLightSources: 1,
			expectedShapes:       1,
			expectedAttributes:   2, // Nested attribute blocks
			description:          "AreaLightSource should inherit into nested attribute blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pbrtScene, err := parsePBRTFromString(tt.pbrtContent)
			if err != nil {
				t.Fatalf("LoadPBRT() error = %v", err)
			}

			// Count light sources and shapes across all attribute blocks
			totalLightSources := 0
			totalShapes := 0
			for _, attr := range pbrtScene.Attributes {
				totalLightSources += len(attr.LightSources)
				totalShapes += len(attr.Shapes)
			}

			// Check number of light sources
			if totalLightSources != tt.expectedLightSources {
				t.Errorf("Expected %d light sources, got %d", tt.expectedLightSources, totalLightSources)
			}

			// Check number of shapes
			if totalShapes != tt.expectedShapes {
				t.Errorf("Expected %d shapes, got %d", tt.expectedShapes, totalShapes)
			}

			// Check number of attribute blocks
			if len(pbrtScene.Attributes) != tt.expectedAttributes {
				t.Errorf("Expected %d attribute blocks, got %d", tt.expectedAttributes, len(pbrtScene.Attributes))
			}

			// Verify the AreaLightSource is captured correctly
			if tt.expectedLightSources > 0 {
				found := false
				for _, attr := range pbrtScene.Attributes {
					for _, light := range attr.LightSources {
						if light.Type == "AreaLightSource" && light.Subtype == "diffuse" {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Expected to find AreaLightSource 'diffuse' in attribute blocks")
				}
			}

			t.Logf("Test '%s': %s", tt.name, tt.description)
		})
	}
}

func TestAreaLightStateInheritance(t *testing.T) {
	// Test that AreaLightSource state is properly tracked within attribute blocks
	pbrtContent := `
WorldBegin
AttributeBegin
  Material "diffuse" "rgb reflectance" [0.8 0.8 0.8]
  AreaLightSource "diffuse" "rgb L" [ 10 8 6 ]
  Shape "sphere" "float radius" [ 1.0 ]
  Shape "bilinearPatch" "point3 P00" [0 0 0] "point3 P01" [1 0 0] "point3 P10" [0 1 0] "point3 P11" [1 1 0]
AttributeEnd
WorldEnd`

	pbrtScene, err := parsePBRTFromString(pbrtContent)
	if err != nil {
		t.Fatalf("LoadPBRT() error = %v", err)
	}

	// Should have one attribute block
	if len(pbrtScene.Attributes) != 1 {
		t.Fatalf("Expected 1 attribute block, got %d", len(pbrtScene.Attributes))
	}

	attr := pbrtScene.Attributes[0]

	// Should have one area light source
	if len(attr.LightSources) != 1 {
		t.Errorf("Expected 1 light source in attribute block, got %d", len(attr.LightSources))
	}

	// Should have two shapes
	if len(attr.Shapes) != 2 {
		t.Errorf("Expected 2 shapes in attribute block, got %d", len(attr.Shapes))
	}

	// Should have one material
	if len(attr.Materials) != 1 {
		t.Errorf("Expected 1 material in attribute block, got %d", len(attr.Materials))
	}

	// The area light should be parsed correctly
	if len(attr.LightSources) > 0 {
		light := attr.LightSources[0]
		if light.Type != "AreaLightSource" {
			t.Errorf("Expected AreaLightSource, got %s", light.Type)
		}
		if light.Subtype != "diffuse" {
			t.Errorf("Expected diffuse subtype, got %s", light.Subtype)
		}

		// Check emission parameter
		if param, exists := light.Parameters["L"]; exists {
			if len(param.Values) != 3 {
				t.Errorf("Expected 3 emission values, got %d", len(param.Values))
			}
			if param.Values[0] != "10" || param.Values[1] != "8" || param.Values[2] != "6" {
				t.Errorf("Expected emission [10 8 6], got %v", param.Values)
			}
		} else {
			t.Errorf("Expected emission parameter 'L' in area light")
		}
	}
}

func TestAreaLightErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		pbrtContent string
		expectError bool
		description string
	}{
		{
			name: "area light outside attribute block",
			pbrtContent: `
WorldBegin
AreaLightSource "diffuse" "rgb L" [ 1 1 1 ]
Shape "sphere" "float radius" [ 1.0 ]
WorldEnd`,
			expectError: false, // Should parse but be at top level
			description: "AreaLightSource outside attribute block should be parsed",
		},
		{
			name: "malformed area light",
			pbrtContent: `
WorldBegin
AttributeBegin
  AreaLightSource "diffuse" "invalid parameter"
  Shape "sphere" "float radius" [ 1.0 ]
AttributeEnd
WorldEnd`,
			expectError: false, // Parser should be lenient about unknown parameters
			description: "Malformed area light should not crash parser",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePBRTFromString(tt.pbrtContent)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none for: %s", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for %s: %v", tt.description, err)
			}
		})
	}
}
