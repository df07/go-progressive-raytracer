package loaders

import (
	"strings"
	"testing"
)

// TestMultiLineParameterFormatting tests the multi-line parameter parsing functionality
func TestMultiLineParameterFormatting(t *testing.T) {
	tests := []struct {
		name           string
		pbrtContent    string
		expectedShapes int
		expectedMats   int
		expectedLights int
		expectError    bool
	}{
		{
			name: "single line parameters",
			pbrtContent: `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
Material "diffuse" "rgb reflectance" [0.5 0.5 0.5]
Shape "sphere" "float radius" 1.0
WorldEnd`,
			expectedShapes: 1,
			expectedMats:   1,
			expectedLights: 0,
			expectError:    false,
		},
		{
			name: "multi-line parameters",
			pbrtContent: `Camera "perspective"
    "float fov" 40
Film "rgb"
    "string filename" "test.png"
    "integer xresolution" 200
    "integer yresolution" 200
WorldBegin
Material "diffuse"
    "rgb reflectance" [0.5 0.5 0.5]
Shape "sphere"
    "float radius" 1.0
WorldEnd`,
			expectedShapes: 1,
			expectedMats:   1,
			expectedLights: 0,
			expectError:    false,
		},
		{
			name: "complex multi-line bilinearPatch",
			pbrtContent: `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
LightSource "infinite" "rgb L" [1 1 1]
Material "diffuse" "rgb reflectance" [0.5 0.5 0.5]
Shape "bilinearPatch"
    "point3 P00" [-10 -1 -10]
    "point3 P01" [10 -1 -10]
    "point3 P10" [-10 -1 10]
    "point3 P11" [10 -1 10]
Material "diffuse" "rgb reflectance" [0.8 0.2 0.2]
Shape "sphere" "float radius" 1.0
WorldEnd`,
			expectedShapes: 2,
			expectedMats:   2,
			expectedLights: 1,
			expectError:    false,
		},
		{
			name: "multi-line infinite-gradient light",
			pbrtContent: `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
LightSource "infinite-gradient"
    "rgb topColor" [0.4 0.6 1.0]
    "rgb bottomColor" [1.0 1.0 1.0]
Material "diffuse" "rgb reflectance" [0.5 0.5 0.5]
Shape "sphere" "float radius" 1.0
WorldEnd`,
			expectedShapes: 1,
			expectedMats:   1,
			expectedLights: 1,
			expectError:    false,
		},
		{
			name: "mixed single and multi-line",
			pbrtContent: `Camera "perspective" "float fov" 40
Film "rgb"
    "string filename" "test.png"
    "integer xresolution" 200
    "integer yresolution" 200
WorldBegin
LightSource "infinite" "rgb L" [1 1 1]
Material "diffuse"
    "rgb reflectance" [0.5 0.5 0.5]
Shape "sphere" "float radius" 1.0
Material "diffuse" "rgb reflectance" [0.8 0.2 0.2]
Shape "bilinearPatch"
    "point3 P00" [0 0 0]
    "point3 P01" [1 0 0]
    "point3 P10" [0 1 0]
    "point3 P11" [1 1 0]
WorldEnd`,
			expectedShapes: 2,
			expectedMats:   2,
			expectedLights: 1,
			expectError:    false,
		},
		{
			name: "empty scene",
			pbrtContent: `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
WorldEnd`,
			expectedShapes: 0,
			expectedMats:   0,
			expectedLights: 0,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test parsing using ParsePBRT with string reader
			reader := strings.NewReader(tt.pbrtContent)
			scene, err := ParsePBRT(reader)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Verify counts
			if len(scene.Shapes) != tt.expectedShapes {
				t.Errorf("Expected %d shapes, got %d", tt.expectedShapes, len(scene.Shapes))
			}
			if len(scene.Materials) != tt.expectedMats {
				t.Errorf("Expected %d materials, got %d", tt.expectedMats, len(scene.Materials))
			}
			if len(scene.LightSources) != tt.expectedLights {
				t.Errorf("Expected %d lights, got %d", tt.expectedLights, len(scene.LightSources))
			}
		})
	}
}

// TestMaterialIndexAssignment tests that shapes get correct material indices
func TestMaterialIndexAssignment(t *testing.T) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
Material "diffuse" "rgb reflectance" [1.0 0.0 0.0]
Shape "sphere" "float radius" 1.0
Material "diffuse" "rgb reflectance" [0.0 1.0 0.0]
Shape "sphere" "float radius" 0.5
Material "diffuse" "rgb reflectance" [0.0 0.0 1.0]
Shape "bilinearPatch"
    "point3 P00" [0 0 0]
    "point3 P01" [1 0 0]
    "point3 P10" [0 1 0]
    "point3 P11" [1 1 0]
WorldEnd`

	// Parse scene using ParsePBRT with string reader
	reader := strings.NewReader(pbrtContent)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify material assignments
	expectedMaterialIndices := []int{0, 1, 2}
	if len(scene.Shapes) != len(expectedMaterialIndices) {
		t.Fatalf("Expected %d shapes, got %d", len(expectedMaterialIndices), len(scene.Shapes))
	}

	for i, expectedIndex := range expectedMaterialIndices {
		if scene.Shapes[i].MaterialIndex != expectedIndex {
			t.Errorf("Shape %d: expected MaterialIndex %d, got %d",
				i, expectedIndex, scene.Shapes[i].MaterialIndex)
		}
	}
}

// TestStatementStartDetection tests the isStatementStart function
func TestStatementStartDetection(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		// Valid statement starts
		{"Camera \"perspective\"", true},
		{"Film \"rgb\"", true},
		{"Material \"diffuse\"", true},
		{"Shape \"sphere\"", true},
		{"LightSource \"infinite\"", true},
		{"AreaLightSource \"diffuse\"", true},
		{"Translate 1 2 3", true},
		{"Rotate 45 0 1 0", true},
		{"Scale 2 2 2", true},
		{"Transform 1 0 0 0 0 1 0 0 0 0 1 0 0 0 0 1", true},
		{"ReverseOrientation", true},
		{"Attribute \"shape\"", true},

		// Continuation lines (parameters)
		{"    \"float fov\" 40", false},
		{"\"rgb reflectance\" [0.5 0.5 0.5]", false},
		{"    \"point3 P00\" [0 0 0]", false},
		{"\"string filename\" \"test.png\"", false},

		// Comments and empty lines (should not start statements)
		{"# This is a comment", false},
		{"", false},
		{"   ", false},

		// Special directives
		{"WorldBegin", false},              // Handled separately
		{"WorldEnd", false},                // Handled separately
		{"AttributeBegin", false},          // Handled separately
		{"AttributeEnd", false},            // Handled separately
		{"LookAt 0 0 1 0 0 0 0 1 0", true}, // LookAt is a statement start

		// Invalid/unknown statements
		{"UnknownStatement \"test\"", false},
		{"SomeRandomText", false},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := isStatementStart(tt.line)
			if result != tt.expected {
				t.Errorf("isStatementStart(%q) = %v, want %v", tt.line, result, tt.expected)
			}
		})
	}
}

// TestPreWorldAndWorldSections tests proper routing of statements
func TestPreWorldAndWorldSections(t *testing.T) {
	pbrtContent := `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb"
    "string filename" "test.png"
    "integer xresolution" 200
    "integer yresolution" 200
Sampler "halton" "integer pixelsamples" 16
Integrator "volpath"
WorldBegin
Material "diffuse" "rgb reflectance" [0.5 0.5 0.5]
Shape "sphere" "float radius" 1.0
LightSource "infinite" "rgb L" [1 1 1]
WorldEnd`

	// Parse scene using ParsePBRT with string reader
	reader := strings.NewReader(pbrtContent)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify pre-world statements
	if scene.Camera == nil {
		t.Error("Camera should be set")
	} else if scene.Camera.Subtype != "perspective" {
		t.Errorf("Expected perspective camera, got %s", scene.Camera.Subtype)
	}

	if scene.Film == nil {
		t.Error("Film should be set")
	} else if scene.Film.Subtype != "rgb" {
		t.Errorf("Expected rgb film, got %s", scene.Film.Subtype)
	}

	if scene.Sampler == nil {
		t.Error("Sampler should be set")
	} else if scene.Sampler.Subtype != "halton" {
		t.Errorf("Expected halton sampler, got %s", scene.Sampler.Subtype)
	}

	if scene.Integrator == nil {
		t.Error("Integrator should be set")
	} else if scene.Integrator.Subtype != "volpath" {
		t.Errorf("Expected volpath integrator, got %s", scene.Integrator.Subtype)
	}

	// Verify world statements
	if len(scene.Materials) != 1 {
		t.Errorf("Expected 1 material, got %d", len(scene.Materials))
	}
	if len(scene.Shapes) != 1 {
		t.Errorf("Expected 1 shape, got %d", len(scene.Shapes))
	}
	if len(scene.LightSources) != 1 {
		t.Errorf("Expected 1 light source, got %d", len(scene.LightSources))
	}
}

// TestAttributeBlocksWithMultiLine tests multi-line parameters in attribute blocks
func TestAttributeBlocksWithMultiLine(t *testing.T) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
AttributeBegin
Material "diffuse"
    "rgb reflectance" [1.0 0.0 0.0]
AreaLightSource "diffuse"
    "rgb L" [10 10 10]
Shape "bilinearPatch"
    "point3 P00" [0 2 0]
    "point3 P01" [1 2 0]
    "point3 P10" [0 2 1]
    "point3 P11" [1 2 1]
AttributeEnd
Material "diffuse" "rgb reflectance" [0.0 1.0 0.0]
Shape "sphere" "float radius" 1.0
WorldEnd`

	// Parse scene using ParsePBRT with string reader
	reader := strings.NewReader(pbrtContent)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify attribute blocks are parsed
	if len(scene.Attributes) != 1 {
		t.Errorf("Expected 1 attribute block, got %d", len(scene.Attributes))
	}

	attrBlock := scene.Attributes[0]
	if len(attrBlock.Materials) != 1 {
		t.Errorf("Expected 1 material in attribute block, got %d", len(attrBlock.Materials))
	}
	if len(attrBlock.Shapes) != 1 {
		t.Errorf("Expected 1 shape in attribute block, got %d", len(attrBlock.Shapes))
	}
	if len(attrBlock.LightSources) != 1 {
		t.Errorf("Expected 1 light source in attribute block, got %d", len(attrBlock.LightSources))
	}

	// Verify global materials and shapes
	if len(scene.Materials) != 1 {
		t.Errorf("Expected 1 global material, got %d", len(scene.Materials))
	}
	if len(scene.Shapes) != 1 {
		t.Errorf("Expected 1 global shape, got %d", len(scene.Shapes))
	}
}

// TestComplexMultiLineScene tests a comprehensive scene with various multi-line elements
func TestComplexMultiLineScene(t *testing.T) {
	pbrtContent := `# Complex multi-line test scene
LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective"
    "float fov" 40
Film "rgb"
    "string filename" "complex-test.png"
    "integer xresolution" 400
    "integer yresolution" 400
Sampler "halton"
    "integer pixelsamples" 16
Integrator "volpath"

WorldBegin

# Gradient background light
LightSource "infinite-gradient"
    "rgb topColor" [0.5 0.7 1.0]
    "rgb bottomColor" [1.0 1.0 1.0]

# Ground plane
Material "diffuse"
    "rgb reflectance" [0.8 0.8 0.8]
Shape "bilinearPatch"
    "point3 P00" [-5 0 -5]
    "point3 P01" [5 0 -5]
    "point3 P10" [-5 0 5]
    "point3 P11" [5 0 5]

# Red sphere
Material "diffuse"
    "rgb reflectance" [0.8 0.2 0.2]
Shape "sphere"
    "float radius" 1.0

# Area light
AttributeBegin
    Material "diffuse"
        "rgb reflectance" [0.0 0.0 0.0]
    AreaLightSource "diffuse"
        "rgb L" [15 15 15]
    Shape "bilinearPatch"
        "point3 P00" [-1 3 -1]
        "point3 P01" [1 3 -1]
        "point3 P10" [-1 3 1]
        "point3 P11" [1 3 1]
AttributeEnd

WorldEnd`

	// Parse scene using ParsePBRT with string reader
	reader := strings.NewReader(pbrtContent)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify scene structure
	expectedCounts := map[string]int{
		"materials":    2, // Ground + red sphere materials
		"shapes":       2, // Ground plane + red sphere
		"lightSources": 1, // Infinite gradient light
		"attributes":   1, // Area light attribute block
	}

	if len(scene.Materials) != expectedCounts["materials"] {
		t.Errorf("Expected %d materials, got %d", expectedCounts["materials"], len(scene.Materials))
	}
	if len(scene.Shapes) != expectedCounts["shapes"] {
		t.Errorf("Expected %d shapes, got %d", expectedCounts["shapes"], len(scene.Shapes))
	}
	if len(scene.LightSources) != expectedCounts["lightSources"] {
		t.Errorf("Expected %d light sources, got %d", expectedCounts["lightSources"], len(scene.LightSources))
	}
	if len(scene.Attributes) != expectedCounts["attributes"] {
		t.Errorf("Expected %d attribute blocks, got %d", expectedCounts["attributes"], len(scene.Attributes))
	}

	// Verify material indices are correct
	if scene.Shapes[0].MaterialIndex != 0 {
		t.Errorf("First shape should have MaterialIndex 0, got %d", scene.Shapes[0].MaterialIndex)
	}
	if scene.Shapes[1].MaterialIndex != 1 {
		t.Errorf("Second shape should have MaterialIndex 1, got %d", scene.Shapes[1].MaterialIndex)
	}

	// Verify light type
	if scene.LightSources[0].Subtype != "infinite-gradient" {
		t.Errorf("Expected infinite-gradient light, got %s", scene.LightSources[0].Subtype)
	}

	// Verify attribute block contents
	attrBlock := scene.Attributes[0]
	if len(attrBlock.Materials) != 1 || len(attrBlock.Shapes) != 1 || len(attrBlock.LightSources) != 1 {
		t.Errorf("Attribute block should have 1 material, 1 shape, 1 light source, got %d, %d, %d",
			len(attrBlock.Materials), len(attrBlock.Shapes), len(attrBlock.LightSources))
	}
}

// BenchmarkMultiLineParsing benchmarks the multi-line parsing performance
func BenchmarkMultiLineParsing(b *testing.B) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "bench.png" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
LightSource "infinite-gradient"
    "rgb topColor" [0.4 0.6 1.0]
    "rgb bottomColor" [1.0 1.0 1.0]
Material "diffuse"
    "rgb reflectance" [0.5 0.5 0.5]
Shape "bilinearPatch"
    "point3 P00" [-10 -1 -10]
    "point3 P01" [10 -1 -10]
    "point3 P10" [-10 -1 10]
    "point3 P11" [10 -1 10]
Material "diffuse"
    "rgb reflectance" [0.8 0.2 0.2]
Shape "sphere"
    "float radius" 1.0
WorldEnd`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Parse using ParsePBRT with string reader
		reader := strings.NewReader(pbrtContent)
		_, err := ParsePBRT(reader)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}
