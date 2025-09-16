package loaders

import (
	"os"
	"testing"
)

// TestGraphicsStateStack tests that the graphics state stack properly handles material state
func TestGraphicsStateStack(t *testing.T) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200

WorldBegin

# Global material 1 (red)
Material "diffuse" "rgb reflectance" [1.0 0.0 0.0]
Shape "sphere" "float radius" 0.5

# Global material 2 (green)
Material "diffuse" "rgb reflectance" [0.0 1.0 0.0]

AttributeBegin
    # Local material within attribute block (blue)
    Material "diffuse" "rgb reflectance" [0.0 0.0 1.0]
    Shape "sphere" "float radius" 0.3
AttributeEnd

# This sphere should use the global green material (index 1)
Shape "sphere" "float radius" 0.7

WorldEnd`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_state_*.pbrt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pbrtContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Parse scene
	scene, err := LoadPBRT(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify global materials
	if len(scene.Materials) != 2 {
		t.Errorf("Expected 2 global materials, got %d", len(scene.Materials))
	}

	// Verify global shapes
	if len(scene.Shapes) != 2 {
		t.Errorf("Expected 2 global shapes, got %d", len(scene.Shapes))
	}

	// Check first global shape (should use material index 0 - red)
	if scene.Shapes[0].MaterialIndex != 0 {
		t.Errorf("First global shape should use material index 0, got %d", scene.Shapes[0].MaterialIndex)
	}

	// Check second global shape (should use material index 1 - green)
	if scene.Shapes[1].MaterialIndex != 1 {
		t.Errorf("Second global shape should use material index 1, got %d", scene.Shapes[1].MaterialIndex)
	}

	// Verify attribute block
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

	// Check attribute block shape (should use local material index 0 within the block)
	if attrBlock.Shapes[0].MaterialIndex != 0 {
		t.Errorf("Attribute block shape should use local material index 0, got %d", attrBlock.Shapes[0].MaterialIndex)
	}
}

// TestNestedAttributeBlocks tests nested AttributeBegin/AttributeEnd blocks
func TestNestedAttributeBlocks(t *testing.T) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200

WorldBegin

# Global material (red)
Material "diffuse" "rgb reflectance" [1.0 0.0 0.0]
Shape "sphere" "float radius" 1.0

AttributeBegin
    # Outer attribute block material (green)
    Material "diffuse" "rgb reflectance" [0.0 1.0 0.0]
    Shape "sphere" "float radius" 0.8

    AttributeBegin
        # Inner attribute block material (blue)
        Material "diffuse" "rgb reflectance" [0.0 0.0 1.0]
        Shape "sphere" "float radius" 0.6
    AttributeEnd

    # Back to outer attribute block - should still use green material
    Shape "sphere" "float radius" 0.4
AttributeEnd

# Back to global scope - should use red material
Shape "sphere" "float radius" 0.2

WorldEnd`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_nested_*.pbrt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pbrtContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Parse scene
	scene, err := LoadPBRT(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify global materials and shapes
	if len(scene.Materials) != 1 {
		t.Errorf("Expected 1 global material, got %d", len(scene.Materials))
	}
	if len(scene.Shapes) != 2 {
		t.Errorf("Expected 2 global shapes, got %d", len(scene.Shapes))
	}

	// Check global shapes use global material (index 0)
	for i, shape := range scene.Shapes {
		if shape.MaterialIndex != 0 {
			t.Errorf("Global shape %d should use material index 0, got %d", i, shape.MaterialIndex)
		}
	}

	// Verify attribute blocks (should have 2: outer and inner)
	if len(scene.Attributes) != 2 {
		t.Errorf("Expected 2 attribute blocks, got %d", len(scene.Attributes))
	}

	// Check inner attribute block (added first, so it's at index 0)
	innerBlock := scene.Attributes[0]
	if len(innerBlock.Materials) != 1 {
		t.Errorf("Expected 1 material in inner attribute block, got %d", len(innerBlock.Materials))
	}
	if len(innerBlock.Shapes) != 1 {
		t.Errorf("Expected 1 shape in inner attribute block, got %d", len(innerBlock.Shapes))
	}

	// Shape in inner block should use local material index 0 (blue)
	if innerBlock.Shapes[0].MaterialIndex != 0 {
		t.Errorf("Inner block shape should use local material index 0, got %d", innerBlock.Shapes[0].MaterialIndex)
	}

	// Check outer attribute block (added second, so it's at index 1)
	outerBlock := scene.Attributes[1]
	if len(outerBlock.Materials) != 1 {
		t.Errorf("Expected 1 material in outer attribute block, got %d", len(outerBlock.Materials))
	}
	if len(outerBlock.Shapes) != 2 {
		t.Errorf("Expected 2 shapes in outer attribute block, got %d", len(outerBlock.Shapes))
	}

	// Both shapes in outer block should use local material index 0 (green)
	for i, shape := range outerBlock.Shapes {
		if shape.MaterialIndex != 0 {
			t.Errorf("Outer block shape %d should use local material index 0, got %d", i, shape.MaterialIndex)
		}
	}
}

// TestMaterialStateRestoration tests that material state is properly restored after AttributeEnd
func TestMaterialStateRestoration(t *testing.T) {
	pbrtContent := `Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200

WorldBegin

# No global material defined initially
Shape "sphere" "float radius" 1.0  # Should have MaterialIndex -1

# Define global material (red)
Material "diffuse" "rgb reflectance" [1.0 0.0 0.0]
Shape "sphere" "float radius" 0.9  # Should have MaterialIndex 0

AttributeBegin
    # Local material in attribute block (green)
    Material "diffuse" "rgb reflectance" [0.0 1.0 0.0]
    Shape "sphere" "float radius" 0.8  # Should use local material index 0
AttributeEnd

# Back to global scope - should restore to global material index 0 (red)
Shape "sphere" "float radius" 0.7  # Should have MaterialIndex 0

# Define another global material (blue)
Material "diffuse" "rgb reflectance" [0.0 0.0 1.0]
Shape "sphere" "float radius" 0.6  # Should have MaterialIndex 1

WorldEnd`

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "test_restore_*.pbrt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(pbrtContent); err != nil {
		t.Fatalf("Failed to write test content: %v", err)
	}
	tmpFile.Close()

	// Parse scene
	scene, err := LoadPBRT(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse scene: %v", err)
	}

	// Verify global materials
	if len(scene.Materials) != 2 {
		t.Errorf("Expected 2 global materials, got %d", len(scene.Materials))
	}

	// Verify global shapes
	if len(scene.Shapes) != 4 {
		t.Errorf("Expected 4 global shapes, got %d", len(scene.Shapes))
	}

	// Check material indices of global shapes
	expectedIndices := []int{-1, 0, 0, 1}
	for i, shape := range scene.Shapes {
		if shape.MaterialIndex != expectedIndices[i] {
			t.Errorf("Global shape %d should have MaterialIndex %d, got %d", i, expectedIndices[i], shape.MaterialIndex)
		}
	}

	// Verify attribute block
	if len(scene.Attributes) != 1 {
		t.Errorf("Expected 1 attribute block, got %d", len(scene.Attributes))
	}

	attrBlock := scene.Attributes[0]
	if len(attrBlock.Shapes) != 1 {
		t.Errorf("Expected 1 shape in attribute block, got %d", len(attrBlock.Shapes))
	}

	// Attribute block shape should use local material index 0
	if attrBlock.Shapes[0].MaterialIndex != 0 {
		t.Errorf("Attribute block shape should use local material index 0, got %d", attrBlock.Shapes[0].MaterialIndex)
	}
}
