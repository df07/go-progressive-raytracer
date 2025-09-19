package scene

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestConvertMaterial(t *testing.T) {
	tests := []struct {
		name     string
		stmt     *loaders.PBRTStatement
		expected string // Expected material type name
	}{
		{
			name: "diffuse material",
			stmt: &loaders.PBRTStatement{
				Type:    "Material",
				Subtype: "diffuse",
				Parameters: map[string]loaders.PBRTParam{
					"reflectance": {Type: "rgb", Values: []string{"0.8", "0.6", "0.4"}},
				},
			},
			expected: "*material.Lambertian",
		},
		{
			name: "conductor material",
			stmt: &loaders.PBRTStatement{
				Type:    "Material",
				Subtype: "conductor",
				Parameters: map[string]loaders.PBRTParam{
					"eta":       {Type: "rgb", Values: []string{"0.2", "0.9", "1.0"}},
					"roughness": {Type: "float", Values: []string{"0.1"}},
				},
			},
			expected: "*material.Metal",
		},
		{
			name: "dielectric material",
			stmt: &loaders.PBRTStatement{
				Type:    "Material",
				Subtype: "dielectric",
				Parameters: map[string]loaders.PBRTParam{
					"eta": {Type: "float", Values: []string{"1.5"}},
				},
			},
			expected: "*material.Dielectric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mat, err := convertMaterial(tt.stmt)
			if err != nil {
				t.Fatalf("convertMaterial() error = %v", err)
			}

			materialType := fmt.Sprintf("%T", mat)
			if materialType != tt.expected {
				t.Errorf("convertMaterial() type = %v, want %v", materialType, tt.expected)
			}
		})
	}
}

func TestConvertShape(t *testing.T) {
	// Test sphere conversion
	sphereStmt := &loaders.PBRTStatement{
		Type:    "Shape",
		Subtype: "sphere",
		Parameters: map[string]loaders.PBRTParam{
			"radius": {Type: "float", Values: []string{"2.5"}},
		},
	}

	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	shape, err := convertShape(sphereStmt, mat)
	if err != nil {
		t.Fatalf("convertShape(sphere) error = %v", err)
	}

	if fmt.Sprintf("%T", shape) != "*geometry.Sphere" {
		t.Errorf("convertShape(sphere) type = %T, want *geometry.Sphere", shape)
	}

	// Test bilinearPatch conversion
	patchStmt := &loaders.PBRTStatement{
		Type:    "Shape",
		Subtype: "bilinearPatch",
		Parameters: map[string]loaders.PBRTParam{
			"P00": {Type: "point3", Values: []string{"0", "0", "0"}},
			"P01": {Type: "point3", Values: []string{"1", "0", "0"}},
			"P10": {Type: "point3", Values: []string{"0", "1", "0"}},
			"P11": {Type: "point3", Values: []string{"1", "1", "0"}},
		},
	}

	shape, err = convertShape(patchStmt, mat)
	if err != nil {
		t.Fatalf("convertShape(bilinearPatch) error = %v", err)
	}

	if fmt.Sprintf("%T", shape) != "*geometry.Quad" {
		t.Errorf("convertShape(bilinearPatch) type = %T, want *geometry.Quad", shape)
	}
}

func TestConvertLight(t *testing.T) {
	tests := []struct {
		name     string
		stmt     *loaders.PBRTStatement
		expected string
	}{
		{
			name: "point light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "point",
				Parameters: map[string]loaders.PBRTParam{
					"I":    {Type: "rgb", Values: []string{"10", "8", "6"}},
					"from": {Type: "point3", Values: []string{"0", "5", "0"}},
				},
			},
			expected: "*lights.SphereLight",
		},
		{
			name: "distant light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "distant",
				Parameters: map[string]loaders.PBRTParam{
					"L":    {Type: "rgb", Values: []string{"3", "3", "3"}},
					"from": {Type: "point3", Values: []string{"0", "0", "0"}},
					"to":   {Type: "point3", Values: []string{"0", "0", "1"}},
				},
			},
			expected: "*lights.UniformInfiniteLight",
		},
		{
			name: "infinite light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "infinite",
				Parameters: map[string]loaders.PBRTParam{
					"L": {Type: "rgb", Values: []string{"1", "1", "1"}},
				},
			},
			expected: "*lights.UniformInfiniteLight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			light, err := convertLight(tt.stmt, nil)
			if err != nil {
				t.Fatalf("convertLight() error = %v", err)
			}

			lightType := fmt.Sprintf("%T", light)
			if lightType != tt.expected {
				t.Errorf("convertLight() type = %v, want %v", lightType, tt.expected)
			}
		})
	}
}

func TestConvertCamera(t *testing.T) {
	pbrtScene := &loaders.PBRTScene{
		LookAt:   &core.Vec3{X: 1, Y: 2, Z: 3},
		LookAtTo: &core.Vec3{X: 4, Y: 5, Z: 6},
		LookAtUp: &core.Vec3{X: 0, Y: 1, Z: 0},
		Camera: &loaders.PBRTStatement{
			Type:    "Camera",
			Subtype: "perspective",
			Parameters: map[string]loaders.PBRTParam{
				"fov": {Type: "float", Values: []string{"35"}},
			},
		},
		Film: &loaders.PBRTStatement{
			Type:    "Film",
			Subtype: "rgb",
			Parameters: map[string]loaders.PBRTParam{
				"xresolution": {Type: "integer", Values: []string{"800"}},
				"yresolution": {Type: "integer", Values: []string{"600"}},
			},
		},
	}

	scene := &Scene{
		SamplingConfig: createDefaultPBRTSamplingConfig(),
	}

	err := convertCamera(pbrtScene, scene)
	if err != nil {
		t.Fatalf("convertCamera() error = %v", err)
	}

	// Check camera position
	expectedCenter := core.Vec3{X: 1, Y: 2, Z: 3}
	if scene.CameraConfig.Center != expectedCenter {
		t.Errorf("Camera center = %v, want %v", scene.CameraConfig.Center, expectedCenter)
	}

	// Check camera target
	expectedLookAt := core.Vec3{X: 4, Y: 5, Z: 6}
	if scene.CameraConfig.LookAt != expectedLookAt {
		t.Errorf("Camera lookAt = %v, want %v", scene.CameraConfig.LookAt, expectedLookAt)
	}

	// Check FOV
	if scene.CameraConfig.VFov != 35.0 {
		t.Errorf("Camera VFov = %v, want %v", scene.CameraConfig.VFov, 35.0)
	}

	// Check image dimensions
	if scene.SamplingConfig.Width != 800 {
		t.Errorf("Image width = %v, want %v", scene.SamplingConfig.Width, 800)
	}
	if scene.SamplingConfig.Height != 600 {
		t.Errorf("Image height = %v, want %v", scene.SamplingConfig.Height, 600)
	}
}

func TestNewPBRTSceneIntegration(t *testing.T) {
	// Create a complete PBRT scene file for integration testing
	content := `# Integration test PBRT scene
LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40

Film "rgb" "string filename" "test.png" "integer xresolution" 200 "integer yresolution" 200

Sampler "halton" "integer pixelsamples" 16
Integrator "volpath"

WorldBegin

# White material
Material "diffuse" "rgb reflectance" [0.8 0.8 0.8]

# Floor quad
Shape "bilinearPatch" "point3 P00" [-2 -1 -2] "point3 P01" [2 -1 -2] "point3 P10" [-2 -1 2] "point3 P11" [2 -1 2]

# Test attribute block with different material and shape
AttributeBegin
    Material "conductor" "rgb eta" [0.2 0.9 1.0] "float roughness" 0.1
    Shape "sphere" "float radius" 0.5
AttributeEnd

# Test light
LightSource "infinite" "rgb L" [2 2 2]

# Test area light
AttributeBegin
    Material "diffuse" "rgb reflectance" [0 0 0]
    AreaLightSource "diffuse" "rgb L" [15 12 8]
    Shape "bilinearPatch" "point3 P00" [-0.5 2 -0.5] "point3 P01" [0.5 2 -0.5] "point3 P10" [-0.5 2 0.5] "point3 P11" [0.5 2 0.5]
AttributeEnd

WorldEnd
`

	// Parse PBRT content and create scene
	reader := strings.NewReader(content)
	pbrtScene, err := loaders.ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse PBRT content: %v", err)
	}

	// Test scene creation
	scene, err := NewPBRTScene(pbrtScene)
	if err != nil {
		t.Fatalf("NewPBRTScene() error = %v", err)
	}

	// Verify scene structure
	if scene == nil {
		t.Fatal("NewPBRTScene() returned nil scene")
	}

	// Check camera
	if scene.Camera == nil {
		t.Error("Scene should have camera")
	}

	// Check sampling config
	if scene.SamplingConfig.Width != 200 {
		t.Errorf("Scene width = %d, want %d", scene.SamplingConfig.Width, 200)
	}
	if scene.SamplingConfig.Height != 200 {
		t.Errorf("Scene height = %d, want %d", scene.SamplingConfig.Height, 200)
	}

	// Check shapes (should have floor + sphere + area light = 3 shapes)
	if len(scene.Shapes) < 2 {
		t.Errorf("Scene should have at least 2 shapes, got %d", len(scene.Shapes))
	}

	// Check lights (infinite light)
	if len(scene.Lights) < 1 {
		t.Errorf("Scene should have at least 1 light, got %d", len(scene.Lights))
	}

	// Test scene preprocessing (should not error)
	err = scene.Preprocess()
	if err != nil {
		t.Errorf("Scene.Preprocess() error = %v", err)
	}

	// Check BVH was created
	if scene.BVH == nil {
		t.Error("Scene preprocessing should create BVH")
	}

	// Check light sampler was created
	if scene.LightSampler == nil {
		t.Error("Scene preprocessing should create light sampler")
	}
}

func TestPBRTSceneErrorHandling(t *testing.T) {
	// Test with invalid PBRT content
	content := `# Invalid PBRT - missing WorldBegin
LookAt 0 0 1  0 0 0  0 1 0
Shape "sphere" "float radius" 1.0
`

	// Parse PBRT content
	reader := strings.NewReader(content)
	pbrtScene, err := loaders.ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse PBRT content: %v", err)
	}

	// Should still work (shapes outside WorldBegin are ignored)
	scene, err := NewPBRTScene(pbrtScene)
	if err != nil {
		t.Fatalf("NewPBRTScene() error = %v", err)
	}

	// Should have no shapes since they were outside WorldBegin
	if len(scene.Shapes) != 0 {
		t.Errorf("Scene should have 0 shapes for invalid content, got %d", len(scene.Shapes))
	}
}

func TestPBRTInputValidation(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		expectError bool
		errorMsg    string
	}{
		{
			name: "invalid FOV - too high",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 200
Film "rgb" "integer xresolution" 100 "integer yresolution" 100
WorldBegin
WorldEnd`,
			expectError: true,
			errorMsg:    "invalid camera FOV",
		},
		{
			name: "invalid FOV - negative",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" -10
Film "rgb" "integer xresolution" 100 "integer yresolution" 100
WorldBegin
WorldEnd`,
			expectError: true,
			errorMsg:    "invalid camera FOV",
		},
		{
			name: "invalid sphere radius - negative",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb" "integer xresolution" 100 "integer yresolution" 100
WorldBegin
Material "diffuse" "rgb reflectance" [0.7 0.7 0.7]
Shape "sphere" "float radius" -1.0
WorldEnd`,
			expectError: true,
			errorMsg:    "invalid sphere radius",
		},
		{
			name: "invalid IOR - negative",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb" "integer xresolution" 100 "integer yresolution" 100
WorldBegin
Material "dielectric" "float eta" -1.5
Shape "sphere" "float radius" 1.0
WorldEnd`,
			expectError: true,
			errorMsg:    "invalid dielectric IOR",
		},
		{
			name: "invalid image width - too large",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb" "integer xresolution" 10000 "integer yresolution" 100
WorldBegin
WorldEnd`,
			expectError: true,
			errorMsg:    "invalid image width",
		},
		{
			name: "valid parameters",
			content: `LookAt 0 0 5  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb" "integer xresolution" 200 "integer yresolution" 200
WorldBegin
Material "dielectric" "float eta" 1.5
Shape "sphere" "float radius" 1.0
WorldEnd`,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse PBRT content
			reader := strings.NewReader(tc.content)
			pbrtScene, err := loaders.ParsePBRT(reader)
			if err != nil {
				t.Fatalf("Failed to parse PBRT content: %v", err)
			}

			_, err = NewPBRTScene(pbrtScene)
			if tc.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got no error", tc.errorMsg)
				} else if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("Expected error containing '%s', but got: %v", tc.errorMsg, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, but got: %v", err)
				}
			}
		})
	}
}

func TestConvertLightTypes(t *testing.T) {
	tests := []struct {
		name         string
		stmt         *loaders.PBRTStatement
		expectedType string
	}{
		{
			name: "point light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "point",
				Parameters: map[string]loaders.PBRTParam{
					"I":    {Type: "rgb", Values: []string{"10", "8", "6"}},
					"from": {Type: "point3", Values: []string{"1", "2", "3"}},
				},
			},
			expectedType: "*lights.SphereLight",
		},
		{
			name: "distant light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "distant",
				Parameters: map[string]loaders.PBRTParam{
					"L": {Type: "rgb", Values: []string{"3", "3", "3"}},
				},
			},
			expectedType: "*lights.UniformInfiniteLight",
		},
		{
			name: "infinite light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "infinite",
				Parameters: map[string]loaders.PBRTParam{
					"L": {Type: "rgb", Values: []string{"1", "1", "1"}},
				},
			},
			expectedType: "*lights.UniformInfiniteLight",
		},
		{
			name: "infinite gradient light",
			stmt: &loaders.PBRTStatement{
				Type:    "LightSource",
				Subtype: "infinite-gradient",
				Parameters: map[string]loaders.PBRTParam{
					"topColor":    {Type: "rgb", Values: []string{"0.4", "0.6", "1.0"}},
					"bottomColor": {Type: "rgb", Values: []string{"1.0", "1.0", "1.0"}},
				},
			},
			expectedType: "*lights.GradientInfiniteLight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			light, err := convertLight(tt.stmt, nil)
			if err != nil {
				t.Fatalf("convertLight() error = %v", err)
			}

			actualType := fmt.Sprintf("%T", light)
			if actualType != tt.expectedType {
				t.Errorf("convertLight() = %v, want %v", actualType, tt.expectedType)
			}
		})
	}
}

func TestAreaLightProcessing(t *testing.T) {
	// Test that AreaLightSource creates both emissive shapes and QuadLight objects
	content := `# Area light test scene
LookAt 0 0 1  0 0 0  0 1 0
Camera "perspective" "float fov" 40
Film "rgb" "string filename" "test.png" "integer xresolution" 100 "integer yresolution" 100
WorldBegin

AttributeBegin
    Material "diffuse" "rgb reflectance" [0 0 0]
    AreaLightSource "diffuse" "rgb L" [15 12 8]
    Shape "bilinearPatch" "point3 P00" [0 2 0] "point3 P01" [1 2 0] "point3 P10" [0 2 1] "point3 P11" [1 2 1]
AttributeEnd

WorldEnd
`

	// Parse PBRT content and create scene
	reader := strings.NewReader(content)
	pbrtScene, err := loaders.ParsePBRT(reader)
	if err != nil {
		t.Fatalf("Failed to parse PBRT content: %v", err)
	}

	scene, err := NewPBRTScene(pbrtScene)
	if err != nil {
		t.Fatalf("NewPBRTScene() error = %v", err)
	}

	// Check that we have exactly one light (QuadLight)
	if len(scene.Lights) != 1 {
		t.Errorf("Expected 1 light, got %d", len(scene.Lights))
	}

	// Check light type
	lightType := fmt.Sprintf("%T", scene.Lights[0])
	if lightType != "*lights.QuadLight" {
		t.Errorf("Expected *lights.QuadLight, got %s", lightType)
	}

	// Check that we have exactly one shape (the emissive quad)
	if len(scene.Shapes) != 1 {
		t.Errorf("Expected 1 shape, got %d", len(scene.Shapes))
	}

	// Check shape type
	shapeType := fmt.Sprintf("%T", scene.Shapes[0])
	if shapeType != "*geometry.Quad" {
		t.Errorf("Expected *geometry.Quad, got %s", shapeType)
	}
}

func TestLightLoadingIntegration(t *testing.T) {
	// Test various light types in complete scenes
	tests := []struct {
		name           string
		scenePath      string
		expectedLights int
		expectedType   string
	}{
		{
			name:           "simple-sphere gradient light",
			scenePath:      "../../scenes/simple-sphere.pbrt",
			expectedLights: 1,
			expectedType:   "*lights.GradientInfiniteLight",
		},
		{
			name:           "test uniform infinite light",
			scenePath:      "../../scenes/test.pbrt",
			expectedLights: 1,
			expectedType:   "*lights.UniformInfiniteLight",
		},
		{
			name:           "cornell area light",
			scenePath:      "../../scenes/cornell.pbrt",
			expectedLights: 1,
			expectedType:   "*lights.QuadLight",
		},
		{
			name:           "cornell-empty area light",
			scenePath:      "../../scenes/cornell-empty.pbrt",
			expectedLights: 1,
			expectedType:   "*lights.QuadLight",
		},
		{
			name:           "cornell-boxes area light",
			scenePath:      "../../scenes/cornell-boxes.pbrt",
			expectedLights: 1,
			expectedType:   "*lights.QuadLight",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check if file exists
			if _, err := os.Stat(tt.scenePath); os.IsNotExist(err) {
				t.Skipf("Scene file %s does not exist, skipping test", tt.scenePath)
				return
			}

			pbrtScene, err := loaders.LoadPBRT(tt.scenePath)
			if err != nil {
				t.Fatalf("Failed to load PBRT file: %v", err)
			}
			scene, err := NewPBRTScene(pbrtScene)
			if err != nil {
				t.Fatalf("NewPBRTScene() error = %v", err)
			}

			// Check number of lights
			if len(scene.Lights) != tt.expectedLights {
				t.Errorf("Expected %d lights, got %d", tt.expectedLights, len(scene.Lights))
			}

			// Check light type
			if len(scene.Lights) > 0 {
				actualType := fmt.Sprintf("%T", scene.Lights[0])
				if actualType != tt.expectedType {
					t.Errorf("Expected light type %s, got %s", tt.expectedType, actualType)
				}
			}

			// Ensure no scene has zero lights (critical for rendering)
			if len(scene.Lights) == 0 {
				t.Errorf("Scene has no lights - this will cause poor rendering convergence")
			}
		})
	}
}
