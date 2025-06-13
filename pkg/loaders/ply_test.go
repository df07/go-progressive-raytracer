package loaders

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// createTestPLY creates a simple test PLY file for testing
func createTestPLY(t *testing.T, filename string, includeNormals bool, includeColors bool) {
	var buf bytes.Buffer

	// Write PLY header
	buf.WriteString("ply\n")
	buf.WriteString("format binary_little_endian 1.0\n")
	buf.WriteString("element vertex 4\n")
	buf.WriteString("property float x\n")
	buf.WriteString("property float y\n")
	buf.WriteString("property float z\n")

	if includeNormals {
		buf.WriteString("property float nx\n")
		buf.WriteString("property float ny\n")
		buf.WriteString("property float nz\n")
	}

	if includeColors {
		buf.WriteString("property uchar red\n")
		buf.WriteString("property uchar green\n")
		buf.WriteString("property uchar blue\n")
	}

	buf.WriteString("element face 2\n")
	buf.WriteString("property list uchar int vertex_indices\n")
	buf.WriteString("end_header\n")

	// Write vertex data (4 vertices forming a square)
	vertices := []struct {
		x, y, z    float32
		nx, ny, nz float32
		r, g, b    uint8
	}{
		{0.0, 0.0, 0.0, 0.0, 0.0, 1.0, 255, 0, 0},   // red
		{1.0, 0.0, 0.0, 0.0, 0.0, 1.0, 0, 255, 0},   // green
		{1.0, 1.0, 0.0, 0.0, 0.0, 1.0, 0, 0, 255},   // blue
		{0.0, 1.0, 0.0, 0.0, 0.0, 1.0, 255, 255, 0}, // yellow
	}

	for _, v := range vertices {
		binary.Write(&buf, binary.LittleEndian, v.x)
		binary.Write(&buf, binary.LittleEndian, v.y)
		binary.Write(&buf, binary.LittleEndian, v.z)

		if includeNormals {
			binary.Write(&buf, binary.LittleEndian, v.nx)
			binary.Write(&buf, binary.LittleEndian, v.ny)
			binary.Write(&buf, binary.LittleEndian, v.nz)
		}

		if includeColors {
			binary.Write(&buf, binary.LittleEndian, v.r)
			binary.Write(&buf, binary.LittleEndian, v.g)
			binary.Write(&buf, binary.LittleEndian, v.b)
		}
	}

	// Write face data (2 triangles)
	faces := []struct {
		count      uint8
		v1, v2, v3 int32
	}{
		{3, 0, 1, 2}, // First triangle
		{3, 0, 2, 3}, // Second triangle
	}

	for _, f := range faces {
		binary.Write(&buf, binary.LittleEndian, f.count)
		binary.Write(&buf, binary.LittleEndian, f.v1)
		binary.Write(&buf, binary.LittleEndian, f.v2)
		binary.Write(&buf, binary.LittleEndian, f.v3)
	}

	// Write to file
	err := os.WriteFile(filename, buf.Bytes(), 0644)
	if err != nil {
		t.Fatalf("Failed to create test PLY file: %v", err)
	}
}

func TestLoadPLY_Basic(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_basic.ply")
	createTestPLY(t, testFile, false, false)
	defer os.Remove(testFile)

	// Load PLY data
	data, err := LoadPLY(testFile)
	if err != nil {
		t.Fatalf("Failed to load PLY: %v", err)
	}

	// Verify vertices
	expectedVertices := []core.Vec3{
		core.NewVec3(0.0, 0.0, 0.0),
		core.NewVec3(1.0, 0.0, 0.0),
		core.NewVec3(1.0, 1.0, 0.0),
		core.NewVec3(0.0, 1.0, 0.0),
	}

	if len(data.Vertices) != len(expectedVertices) {
		t.Fatalf("Expected %d vertices, got %d", len(expectedVertices), len(data.Vertices))
	}

	for i, expected := range expectedVertices {
		if !data.Vertices[i].Equals(expected) {
			t.Errorf("Vertex %d: expected %v, got %v", i, expected, data.Vertices[i])
		}
	}

	// Verify faces
	expectedFaces := []int{0, 1, 2, 0, 2, 3}
	if len(data.Faces) != len(expectedFaces) {
		t.Fatalf("Expected %d face indices, got %d", len(expectedFaces), len(data.Faces))
	}

	for i, expected := range expectedFaces {
		if data.Faces[i] != expected {
			t.Errorf("Face index %d: expected %d, got %d", i, expected, data.Faces[i])
		}
	}

	// Should have no normals
	if len(data.Normals) != 0 {
		t.Errorf("Expected no normals, got %d", len(data.Normals))
	}
}

func TestLoadPLY_WithNormals(t *testing.T) {
	// Create temporary test file with normals
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_normals.ply")
	createTestPLY(t, testFile, true, false)
	defer os.Remove(testFile)

	// Load PLY data
	data, err := LoadPLY(testFile)
	if err != nil {
		t.Fatalf("Failed to load PLY: %v", err)
	}

	// Verify normals
	expectedNormals := []core.Vec3{
		core.NewVec3(0.0, 0.0, 1.0),
		core.NewVec3(0.0, 0.0, 1.0),
		core.NewVec3(0.0, 0.0, 1.0),
		core.NewVec3(0.0, 0.0, 1.0),
	}

	if len(data.Normals) != len(expectedNormals) {
		t.Fatalf("Expected %d normals, got %d", len(expectedNormals), len(data.Normals))
	}

	for i, expected := range expectedNormals {
		if !data.Normals[i].Equals(expected) {
			t.Errorf("Normal %d: expected %v, got %v", i, expected, data.Normals[i])
		}
	}
}

func TestLoadPLY_WithColors(t *testing.T) {
	// Create temporary test file with colors
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_colors.ply")
	createTestPLY(t, testFile, false, true)
	defer os.Remove(testFile)

	// Load PLY data
	data, err := LoadPLY(testFile)
	if err != nil {
		t.Fatalf("Failed to load PLY: %v", err)
	}

	// Verify colors (normalized to [0,1])
	expectedColors := []core.Vec3{
		core.NewVec3(1.0, 0.0, 0.0), // red
		core.NewVec3(0.0, 1.0, 0.0), // green
		core.NewVec3(0.0, 0.0, 1.0), // blue
		core.NewVec3(1.0, 1.0, 0.0), // yellow
	}

	if len(data.Colors) != len(expectedColors) {
		t.Fatalf("Expected %d colors, got %d", len(expectedColors), len(data.Colors))
	}

	for i, expected := range expectedColors {
		if !data.Colors[i].Equals(expected) {
			t.Errorf("Color %d: expected %v, got %v", i, expected, data.Colors[i])
		}
	}
}

func TestLoadPLY_NonExistentFile(t *testing.T) {
	_, err := LoadPLY("nonexistent.ply")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestParsePLYHeader(t *testing.T) {
	// Create a simple PLY header
	headerContent := `ply
format binary_little_endian 1.0
comment Test PLY file
element vertex 100
property float x
property float y
property float z
property float nx
property float ny
property float nz
property uchar red
property uchar green
property uchar blue
element face 50
property list uchar int vertex_indices
end_header
`

	// Create temporary file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_header.ply")
	err := os.WriteFile(testFile, []byte(headerContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Open file and parse header
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	header, headerSize, err := parsePLYHeader(file)
	if err != nil {
		t.Fatalf("Failed to parse header: %v", err)
	}

	// Verify header
	if header.Format != "binary_little_endian" {
		t.Errorf("Expected format 'binary_little_endian', got '%s'", header.Format)
	}

	if header.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", header.Version)
	}

	if header.VertexCount != 100 {
		t.Errorf("Expected 100 vertices, got %d", header.VertexCount)
	}

	if header.FaceCount != 50 {
		t.Errorf("Expected 50 faces, got %d", header.FaceCount)
	}

	if !header.HasNormals {
		t.Error("Expected normals to be detected")
	}

	if len(header.VertexProps) != 9 {
		t.Errorf("Expected 9 vertex properties, got %d", len(header.VertexProps))
	}

	if len(header.FaceProps) != 1 {
		t.Errorf("Expected 1 face property, got %d", len(header.FaceProps))
	}

	if headerSize <= 0 {
		t.Errorf("Expected positive header size, got %d", headerSize)
	}
}

func TestGetTypeSize(t *testing.T) {
	tests := []struct {
		dataType string
		expected int
	}{
		{"float", 4},
		{"float32", 4},
		{"int", 4},
		{"int32", 4},
		{"uint", 4},
		{"uint32", 4},
		{"double", 8},
		{"float64", 8},
		{"short", 2},
		{"int16", 2},
		{"ushort", 2},
		{"uint16", 2},
		{"char", 1},
		{"int8", 1},
		{"uchar", 1},
		{"uint8", 1},
		{"unknown", 4}, // default
	}

	for _, test := range tests {
		result := getTypeSize(test.dataType)
		if result != test.expected {
			t.Errorf("getTypeSize(%s): expected %d, got %d", test.dataType, test.expected, result)
		}
	}
}

func TestCalculateVertexSize(t *testing.T) {
	props := []PLYProperty{
		{Name: "x", Type: "float"},
		{Name: "y", Type: "float"},
		{Name: "z", Type: "float"},
		{Name: "nx", Type: "float"},
		{Name: "ny", Type: "float"},
		{Name: "nz", Type: "float"},
		{Name: "red", Type: "uchar"},
		{Name: "green", Type: "uchar"},
		{Name: "blue", Type: "uchar"},
	}

	expected := 6*4 + 3*1 // 6 floats (4 bytes each) + 3 uchars (1 byte each)
	result := calculateVertexSize(props)

	if result != expected {
		t.Errorf("calculateVertexSize: expected %d, got %d", expected, result)
	}
}
