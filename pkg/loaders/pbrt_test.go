package loaders

import (
	"strings"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestTokenizePBRT(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple statement",
			input:    `Camera "perspective"`,
			expected: []string{`Camera`, `"perspective"`},
		},
		{
			name:     "statement with parameters",
			input:    `Camera "perspective" "float fov" 45`,
			expected: []string{`Camera`, `"perspective"`, `"float fov"`, `45`},
		},
		{
			name:     "statement with array",
			input:    `Material "diffuse" "rgb reflectance" [0.7 0.3 0.1]`,
			expected: []string{`Material`, `"diffuse"`, `"rgb reflectance"`, `[0.7 0.3 0.1]`},
		},
		{
			name:     "shape with multiple arrays",
			input:    `Shape "bilinearPatch" "point3 P00" [0 0 0] "point3 P01" [1 0 0]`,
			expected: []string{`Shape`, `"bilinearPatch"`, `"point3 P00"`, `[0 0 0]`, `"point3 P01"`, `[1 0 0]`},
		},
		{
			name:     "lookAt statement",
			input:    `LookAt 278 278 -800 278 278 0 0 1 0`,
			expected: []string{`LookAt`, `278`, `278`, `-800`, `278`, `278`, `0`, `0`, `1`, `0`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizePBRT(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("tokenizePBRT() length = %d, want %d", len(result), len(tt.expected))
				t.Errorf("got: %v", result)
				t.Errorf("want: %v", tt.expected)
				return
			}
			for i, token := range result {
				if token != tt.expected[i] {
					t.Errorf("tokenizePBRT()[%d] = %q, want %q", i, token, tt.expected[i])
				}
			}
		})
	}
}

func TestParseStatement(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedType  string
		expectedSub   string
		expectedParam string
		expectedValue []string
	}{
		{
			name:          "camera statement",
			input:         `Camera "perspective" "float fov" 45`,
			expectedType:  "Camera",
			expectedSub:   "perspective",
			expectedParam: "fov",
			expectedValue: []string{"45"},
		},
		{
			name:          "material with RGB",
			input:         `Material "diffuse" "rgb reflectance" [0.7 0.3 0.1]`,
			expectedType:  "Material",
			expectedSub:   "diffuse",
			expectedParam: "reflectance",
			expectedValue: []string{"0.7", "0.3", "0.1"},
		},
		{
			name:          "shape with point3",
			input:         `Shape "sphere" "float radius" 1.5`,
			expectedType:  "Shape",
			expectedSub:   "sphere",
			expectedParam: "radius",
			expectedValue: []string{"1.5"},
		},
		{
			name:          "light source",
			input:         `LightSource "point" "rgb I" [10 8 6]`,
			expectedType:  "LightSource",
			expectedSub:   "point",
			expectedParam: "I",
			expectedValue: []string{"10", "8", "6"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := parseStatement(tt.input)
			if err != nil {
				t.Fatalf("parseStatement() error = %v", err)
			}

			if stmt.Type != tt.expectedType {
				t.Errorf("parseStatement().Type = %v, want %v", stmt.Type, tt.expectedType)
			}
			if stmt.Subtype != tt.expectedSub {
				t.Errorf("parseStatement().Subtype = %v, want %v", stmt.Subtype, tt.expectedSub)
			}

			param, exists := stmt.Parameters[tt.expectedParam]
			if !exists {
				t.Errorf("parseStatement() missing parameter %v", tt.expectedParam)
				return
			}

			if len(param.Values) != len(tt.expectedValue) {
				t.Errorf("parseStatement().Parameters[%v].Values length = %d, want %d",
					tt.expectedParam, len(param.Values), len(tt.expectedValue))
				return
			}

			for i, val := range param.Values {
				if val != tt.expectedValue[i] {
					t.Errorf("parseStatement().Parameters[%v].Values[%d] = %v, want %v",
						tt.expectedParam, i, val, tt.expectedValue[i])
				}
			}
		})
	}
}

func TestParseLookAt(t *testing.T) {
	stmt, err := parseStatement("LookAt 1 2 3 4 5 6 7 8 9")
	if err != nil {
		t.Fatalf("parseStatement() error = %v", err)
	}

	scene := &PBRTScene{}
	err = parseLookAt(stmt, scene)
	if err != nil {
		t.Fatalf("parseLookAt() error = %v", err)
	}

	expectedEye := core.Vec3{X: 1, Y: 2, Z: 3}
	expectedAt := core.Vec3{X: 4, Y: 5, Z: 6}
	expectedUp := core.Vec3{X: 7, Y: 8, Z: 9}

	if *scene.LookAt != expectedEye {
		t.Errorf("parseLookAt() eye = %v, want %v", *scene.LookAt, expectedEye)
	}
	if *scene.LookAtTo != expectedAt {
		t.Errorf("parseLookAt() at = %v, want %v", *scene.LookAtTo, expectedAt)
	}
	if *scene.LookAtUp != expectedUp {
		t.Errorf("parseLookAt() up = %v, want %v", *scene.LookAtUp, expectedUp)
	}
}

func TestGetParameterMethods(t *testing.T) {
	// Test GetFloatParam
	stmt := &PBRTStatement{
		Parameters: map[string]PBRTParam{
			"fov": {Type: "float", Values: []string{"45.5"}},
		},
	}

	fov, ok := stmt.GetFloatParam("fov")
	if !ok {
		t.Error("GetFloatParam() should find fov parameter")
	}
	if fov != 45.5 {
		t.Errorf("GetFloatParam() = %v, want %v", fov, 45.5)
	}

	// Test GetRGBParam
	stmt.Parameters["color"] = PBRTParam{Type: "rgb", Values: []string{"0.8", "0.6", "0.4"}}
	color, ok := stmt.GetRGBParam("color")
	if !ok {
		t.Error("GetRGBParam() should find color parameter")
	}
	expected := core.Vec3{X: 0.8, Y: 0.6, Z: 0.4}
	if *color != expected {
		t.Errorf("GetRGBParam() = %v, want %v", *color, expected)
	}

	// Test GetPoint3Param
	stmt.Parameters["position"] = PBRTParam{Type: "point3", Values: []string{"1.0", "2.0", "3.0"}}
	pos, ok := stmt.GetPoint3Param("position")
	if !ok {
		t.Error("GetPoint3Param() should find position parameter")
	}
	expectedPos := core.Vec3{X: 1.0, Y: 2.0, Z: 3.0}
	if *pos != expectedPos {
		t.Errorf("GetPoint3Param() = %v, want %v", *pos, expectedPos)
	}

	// Test GetStringParam
	stmt.Parameters["filename"] = PBRTParam{Type: "string", Values: []string{"test.png"}}
	filename, ok := stmt.GetStringParam("filename")
	if !ok {
		t.Error("GetStringParam() should find filename parameter")
	}
	if filename != "test.png" {
		t.Errorf("GetStringParam() = %v, want %v", filename, "test.png")
	}
}

func TestLoadPBRTBasic(t *testing.T) {
	// Create a temporary PBRT file
	content := `# Test PBRT file
LookAt 0 0 1  0 0 0  0 1 0
Camera "perspective" "float fov" 45

Film "rgb" "string filename" "test.png" "integer xresolution" 400 "integer yresolution" 300

WorldBegin

Material "diffuse" "rgb reflectance" [0.7 0.7 0.7]

Shape "sphere" "float radius" 1.0

LightSource "infinite" "rgb L" [1 1 1]

WorldEnd
`

	// Test loading using ParsePBRT with string reader
	reader := strings.NewReader(content)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("LoadPBRT() error = %v", err)
	}

	// Check basic structure
	if scene.Camera == nil {
		t.Error("LoadPBRT() should have camera")
	}
	if scene.Camera.Subtype != "perspective" {
		t.Errorf("Camera subtype = %v, want %v", scene.Camera.Subtype, "perspective")
	}

	if scene.LookAt == nil {
		t.Error("LoadPBRT() should have LookAt position")
	}

	if scene.Film == nil {
		t.Error("LoadPBRT() should have film")
	}

	if len(scene.Materials) != 1 {
		t.Errorf("LoadPBRT() materials count = %d, want %d", len(scene.Materials), 1)
	}

	if len(scene.Shapes) != 1 {
		t.Errorf("LoadPBRT() shapes count = %d, want %d", len(scene.Shapes), 1)
	}

	if len(scene.LightSources) != 1 {
		t.Errorf("LoadPBRT() light sources count = %d, want %d", len(scene.LightSources), 1)
	}
}

func TestLoadPBRTWithAttributes(t *testing.T) {
	// Create a temporary PBRT file with attribute blocks
	content := `# Test PBRT file with attributes
LookAt 0 0 1  0 0 0  0 1 0
Camera "perspective" "float fov" 45

WorldBegin

# Top-level material
Material "diffuse" "rgb reflectance" [0.5 0.5 0.5]

AttributeBegin
    Material "diffuse" "rgb reflectance" [0.8 0.2 0.2]
    Shape "sphere" "float radius" 0.5
AttributeEnd

AttributeBegin
    Material "conductor" "rgb eta" [0.2 0.9 1.0]
    Shape "sphere" "float radius" 0.8
AttributeEnd

WorldEnd
`

	// Test loading using ParsePBRT with string reader
	reader := strings.NewReader(content)
	scene, err := ParsePBRT(reader)
	if err != nil {
		t.Fatalf("LoadPBRT() error = %v", err)
	}

	// Check attribute blocks
	if len(scene.Attributes) != 2 {
		t.Errorf("LoadPBRT() attribute blocks count = %d, want %d", len(scene.Attributes), 2)
	}

	// Check first attribute block
	if len(scene.Attributes[0].Materials) != 1 {
		t.Errorf("First attribute block materials count = %d, want %d", len(scene.Attributes[0].Materials), 1)
	}
	if len(scene.Attributes[0].Shapes) != 1 {
		t.Errorf("First attribute block shapes count = %d, want %d", len(scene.Attributes[0].Shapes), 1)
	}

	// Check material type
	mat1 := scene.Attributes[0].Materials[0]
	if mat1.Subtype != "diffuse" {
		t.Errorf("First attribute material subtype = %v, want %v", mat1.Subtype, "diffuse")
	}

	// Check second attribute block material type
	mat2 := scene.Attributes[1].Materials[0]
	if mat2.Subtype != "conductor" {
		t.Errorf("Second attribute material subtype = %v, want %v", mat2.Subtype, "conductor")
	}
}
