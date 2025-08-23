package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
)

// TestSampler provides predetermined values for testing
type TestSampler struct {
	values1D []float64
	values2D []core.Vec2
	values3D []core.Vec3
	index1D  int
	index2D  int
	index3D  int
}

// NewTestSampler creates a sampler with predetermined values for each dimension
func NewTestSampler(values1D []float64, values2D []core.Vec2, values3D []core.Vec3) *TestSampler {
	return &TestSampler{
		values1D: values1D,
		values2D: values2D,
		values3D: values3D,
		index1D:  0,
		index2D:  0,
		index3D:  0,
	}
}

// Get1D returns the next predetermined 1D value
func (t *TestSampler) Get1D() float64 {
	if t.index1D >= len(t.values1D) {
		panic("TestSampler ran out of 1D values")
	}
	val := t.values1D[t.index1D]
	t.index1D++
	return val
}

// Get2D returns the next predetermined 2D value
func (t *TestSampler) Get2D() core.Vec2 {
	if t.index2D >= len(t.values2D) {
		panic("TestSampler ran out of 2D values")
	}
	val := t.values2D[t.index2D]
	t.index2D++
	return val
}

// Get3D returns the next predetermined 3D value
func (t *TestSampler) Get3D() core.Vec3 {
	if t.index3D >= len(t.values3D) {
		panic("TestSampler ran out of 3D values")
	}
	val := t.values3D[t.index3D]
	t.index3D++
	return val
}

// Reset resets the sampler to the beginning of all value sequences
func (t *TestSampler) Reset() {
	t.index1D = 0
	t.index2D = 0
	t.index3D = 0
}

// ============================================================================
// 1. BASIC PATH GENERATION TESTS
// Test ray bouncing physics, path shapes, intersections
// ============================================================================

// TestExtendPath tests path extension logic - the core ray bouncing mechanics
func TestExtendPath(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})

	tests := []struct {
		name                string
		scene               core.Scene
		initialRay          core.Ray
		initialBeta         core.Vec3
		initialPdfFwd       float64
		maxBounces          int
		expectedMinVertices int
		expectedMaxVertices int
		expectSurfaceHit    bool
		expectBackgroundHit bool
	}{
		{
			name:                "RayHittingSphere",
			scene:               createSimpleTestScene(),
			initialRay:          core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1)),
			initialBeta:         core.Vec3{X: 1, Y: 1, Z: 1},
			initialPdfFwd:       1.0,
			maxBounces:          3,
			expectedMinVertices: 2, // start vertex + at least the sphere hit
			expectedMaxVertices: 5, // start + sphere + light + background + maybe one more
			expectSurfaceHit:    true,
			expectBackgroundHit: false, // depends on bounces
		},
		{
			name:                "RayMissingScene",
			scene:               createSimpleTestScene(),
			initialRay:          core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, -1, 0)),
			initialBeta:         core.Vec3{X: 1, Y: 1, Z: 1},
			initialPdfFwd:       1.0,
			maxBounces:          3,
			expectedMinVertices: 2, // start vertex + background hit
			expectedMaxVertices: 2, // start + background terminates path
			expectSurfaceHit:    false,
			expectBackgroundHit: true,
		},
		{
			name:                "MaxBouncesZero",
			scene:               createSimpleTestScene(),
			initialRay:          core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1)),
			initialBeta:         core.Vec3{X: 1, Y: 1, Z: 1},
			initialPdfFwd:       1.0,
			maxBounces:          0,
			expectedMinVertices: 1, // just the start vertex, no bounces allowed
			expectedMaxVertices: 1,
			expectSurfaceHit:    false,
			expectBackgroundHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a path to extend with an initial starting vertex
			startVertex := createTestVertex(
				tt.initialRay.Origin,
				core.Vec3{X: 0, Y: 1, Z: 0}, // arbitrary normal
				false, false, nil,
			)
			startVertex.Beta = tt.initialBeta

			path := &Path{
				Vertices: []Vertex{startVertex},
				Length:   1,
			}

			sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
			integrator.extendPath(path, tt.initialRay, tt.initialBeta, tt.initialPdfFwd, tt.scene, sampler, tt.maxBounces, true)

			// Test path length
			if path.Length < tt.expectedMinVertices {
				t.Errorf("Expected at least %d vertices, got %d", tt.expectedMinVertices, path.Length)
			}
			if path.Length > tt.expectedMaxVertices {
				t.Errorf("Expected at most %d vertices, got %d", tt.expectedMaxVertices, path.Length)
			}

			// Test vertex properties
			foundSurface := false
			foundBackground := false

			for i, vertex := range path.Vertices {
				if vertex.Material != nil {
					foundSurface = true
				}
				if vertex.IsInfiniteLight {
					foundBackground = true
				}

				// Test that beta values are reasonable
				if vertex.Beta.X < 0 || vertex.Beta.Y < 0 || vertex.Beta.Z < 0 {
					t.Errorf("Vertex %d has negative beta: %v", i, vertex.Beta)
				}

				// Test that PDF values are reasonable
				if vertex.AreaPdfForward < 0 {
					t.Errorf("Vertex %d has negative forward PDF: %f", i, vertex.AreaPdfForward)
				}

				// Test that position is reasonable (not NaN/Inf)
				if math.IsNaN(vertex.Point.X) || math.IsInf(vertex.Point.X, 0) {
					t.Errorf("Vertex %d has invalid position: %v", i, vertex.Point)
				}
			}

			// Verify expectations
			if tt.expectSurfaceHit && !foundSurface {
				t.Error("Expected surface hit but none found")
			}
			if !tt.expectSurfaceHit && foundSurface {
				t.Error("Found unexpected surface hit")
			}
			if tt.expectBackgroundHit && !foundBackground {
				t.Error("Expected background hit but none found")
			}
		})
	}
}

// TestGenerateCameraSubpath tests camera vertex creation and initial ray
func TestGenerateCameraSubpath(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})
	scene := createSimpleTestScene()
	ray := core.NewRay(core.NewVec3(1, 2, 3), core.NewVec3(0, 0, -1))

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	path := integrator.generateCameraPath(ray, scene, sampler, 2)

	// Should have at least the camera vertex
	if path.Length == 0 {
		t.Fatal("Camera path should have at least the camera vertex")
	}

	// First vertex should be camera
	cameraVertex := path.Vertices[0]
	if !cameraVertex.IsCamera {
		t.Error("First vertex should be marked as camera")
	}
	if cameraVertex.Point != ray.Origin {
		t.Errorf("Camera vertex position should be %v, got %v", ray.Origin, cameraVertex.Point)
	}
	expectedNormal := ray.Direction.Multiply(-1)
	if cameraVertex.Normal != expectedNormal {
		t.Errorf("Camera vertex normal should be %v, got %v", expectedNormal, cameraVertex.Normal)
	}
}

// TestGenerateLightSubpath tests light emission sampling and initial vertex
func TestGenerateLightSubpath(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})
	scene := createSimpleTestScene()

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	path := integrator.generateLightPath(scene, sampler, 2)

	// Should have at least the light vertex
	if path.Length == 0 {
		t.Fatal("Light path should have at least the light vertex")
	}

	// First vertex should be light
	lightVertex := path.Vertices[0]
	if !lightVertex.IsLight {
		t.Error("First vertex should be marked as light")
	}
	if lightVertex.Light == nil {
		t.Error("Light vertex should have light reference")
	}
	if lightVertex.EmittedLight.Luminance() <= 0 {
		t.Error("Light vertex should have positive emission")
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// ExpectedVertex represents expected properties of a vertex in a path
type ExpectedVertex struct {
	index        int
	expectedBeta core.Vec3
	isLight      bool
	isCamera     bool
	isSpecular   bool
	tolerance    float64
}

// ExpectedPdfVertex represents expected PDF values for a vertex in a path
type ExpectedPdfVertex struct {
	index              int
	expectedForwardPdf float64
	expectedReversePdf float64
	tolerance          float64
	description        string
}

// createSimpleTestScene creates a minimal scene for unit testing
func createSimpleTestScene() core.Scene {
	// Create a simple scene with one light and one diffuse surface
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, white)

	// Create a simple quad light (easier to predict sampling)
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissive := material.NewEmissive(emission)
	// Quad light at y=2, centered at (0,2,0), size 1x1 in XZ plane
	light := geometry.NewQuadLight(
		core.NewVec3(-0.5, 2.0, -0.5), // corner
		core.NewVec3(1.0, 0.0, 0.0),   // u vector (X direction)
		core.NewVec3(0.0, 0.0, 1.0),   // v vector (Z direction)
		emissive,
	)

	// Use a real camera from the renderer package
	cameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         100,
		AspectRatio:   1.0,
		VFov:          45.0,
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := renderer.NewCamera(cameraConfig)

	return &MockScene{
		shapes: []core.Shape{sphere, light.Quad},
		lights: []core.Light{light},
		camera: camera,
		config: core.SamplingConfig{MaxDepth: 5},
	}
}

// Helper function to create a simple scene with a specific light
func createSceneWithLight(light core.Light) core.Scene {
	// Simple diffuse sphere (not used in our tests but needed for complete scene)
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -5), 0.5, white)

	var shapes []core.Shape
	switch l := light.(type) {
	case *geometry.QuadLight:
		shapes = []core.Shape{sphere, l.Quad}
	case *geometry.SphereLight:
		shapes = []core.Shape{sphere, l.Sphere}
	case *geometry.PointSpotLight:
		shapes = []core.Shape{sphere} // Point lights don't have geometry
	default:
		shapes = []core.Shape{sphere}
	}

	// Simple camera
	cameraConfig := renderer.CameraConfig{
		Center: core.NewVec3(0, 0, 0), LookAt: core.NewVec3(0, 0, -1), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := renderer.NewCamera(cameraConfig)

	return &MockScene{
		shapes: shapes, lights: []core.Light{light},
		camera: camera, config: core.SamplingConfig{MaxDepth: 5},
	}
}

// createTestVertex creates a test vertex with specified properties
func createTestAreaLight() core.Light {
	emissiveMaterial := material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0))
	return geometry.NewSphereLight(core.NewVec3(0, 1, 0), 0.1, emissiveMaterial)
}

func createGlancingTestSceneWithMaterial(mat core.Material) core.Scene {
	// Create sphere with the specified material - positioned for camera ray hit
	// Sphere is centered at (0, 0, -2) so camera at origin can hit it
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 1.0, mat)

	// Point light for simple lighting
	pointLight := geometry.NewPointSpotLight(
		core.NewVec3(0, 3, -2), core.NewVec3(0, -1, 0),
		core.NewVec3(3, 3, 3), 90.0, 1.0,
	)

	// Camera setup that matches createSimpleTestScene - at origin looking toward -Z
	cameraConfig := renderer.CameraConfig{
		Center: core.NewVec3(0, 0, 0), LookAt: core.NewVec3(0, 0, -1), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := renderer.NewCamera(cameraConfig)

	return &MockScene{
		shapes: []core.Shape{sphere},
		lights: []core.Light{pointLight},
		camera: camera, config: core.SamplingConfig{MaxDepth: 5},
	}
}

func createGlancingTestSceneAndRay(mat core.Material) (core.Scene, core.Ray) {
	scene := createGlancingTestSceneWithMaterial(mat)

	// Create the standard glancing ray that hits the sphere at an angle
	cameraOrigin := core.NewVec3(0, 0, 0)
	sphereGlancingPoint := core.NewVec3(0.5, 0, -1.5)
	ray := core.NewRayTo(cameraOrigin, sphereGlancingPoint)

	return scene, ray
}

func createLightSceneWithMaterial(mat core.Material) core.Scene {
	// Create a surface for light to bounce off of
	sphere := geometry.NewSphere(core.NewVec3(0, -1.5, 0), 0.8, mat)

	// create a bounding sphere to capture escaped rays so we can check final beta
	boundingSphere := geometry.NewSphere(core.NewVec3(0, 0, 0), 10.0, material.NewLambertian(core.NewVec3(0.0, 0.0, 0.0)))

	// Use quad light pointing downward - much more predictable than sphere light
	emissiveMaterial := material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0))
	// Create a horizontal quad at y=1.0, pointing downward (-Y direction)
	quadLight := geometry.NewQuadLight(
		core.NewVec3(-0.5, 1.0, -0.5), // corner
		core.NewVec3(1.0, 0, 0),       // u vector (width)
		core.NewVec3(0, 0, 1.0),       // v vector (height)
		emissiveMaterial,
	)

	cameraConfig := renderer.CameraConfig{
		Center: core.NewVec3(3, 0, 0), LookAt: core.NewVec3(0, 0, 0), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := renderer.NewCamera(cameraConfig)

	return &MockScene{
		shapes: []core.Shape{sphere, boundingSphere},
		lights: []core.Light{quadLight},
		camera: camera, config: core.SamplingConfig{MaxDepth: 5},
	}
}

func createTestVertex(point core.Vec3, normal core.Vec3, isLight bool, isCamera bool, material core.Material) Vertex {
	return Vertex{
		Point:             point,
		Normal:            normal,
		Material:          material,
		IsLight:           isLight,
		IsCamera:          isCamera,
		IsSpecular:        false,
		Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
		AreaPdfForward:    1.0,
		AreaPdfReverse:    1.0,
		EmittedLight:      core.Vec3{X: 0, Y: 0, Z: 0},
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 1},
	}
}

// ============================================================================
// HELPER FUNCTIONS FOR COMPREHENSIVE MIS TESTS
// ============================================================================

// createTestCameraPath creates a camera path with specified materials and positions
func createTestCameraPath(materials []core.Material, positions []core.Vec3) Path {
	if len(positions) != len(materials)+1 {
		panic("positions must be one more than materials")
	}

	vertices := make([]Vertex, len(positions))

	// First vertex is always camera
	vertices[0] = Vertex{
		Point:          positions[0],
		Normal:         core.NewVec3(0, 0, 1), // camera "normal"
		IsCamera:       true,
		Beta:           core.Vec3{X: 1, Y: 1, Z: 1},
		AreaPdfForward: 1.0,
	}

	// Subsequent vertices use provided materials
	for i := 1; i < len(positions); i++ {
		material := materials[i-1]
		// For test purposes, assume Metal and Dielectric are specular
		// This is a simplification - in reality we'd check PDF behavior
		isSpecular := false

		vertices[i] = Vertex{
			Point:          positions[i],
			Normal:         core.NewVec3(0, 1, 0), // upward normal
			Material:       material,
			IsSpecular:     isSpecular,
			Beta:           core.Vec3{X: 0.7, Y: 0.7, Z: 0.7}, // typical diffuse reflectance
			AreaPdfForward: 1.0 / math.Pi,                     // cosine-weighted hemisphere sampling
			AreaPdfReverse: 1.0 / math.Pi,
		}
	}

	return Path{
		Vertices: vertices,
		Length:   len(vertices),
	}
}

// createTestLightPath creates a light path with specified materials and positions
func createTestLightPath(materials []core.Material, positions []core.Vec3) Path {
	if len(positions) != len(materials) {
		panic("positions and materials must have same length")
	}

	vertices := make([]Vertex, len(positions))

	// First vertex is always a light source
	testLight := createTestAreaLight()
	vertices[0] = Vertex{
		Point:          positions[0],
		Normal:         core.NewVec3(0, -1, 0), // downward-facing light
		Material:       materials[0],
		IsLight:        true,
		Light:          testLight,
		Beta:           core.Vec3{X: 1, Y: 1, Z: 1}, // full light emission
		AreaPdfForward: 0.25 / math.Pi,              // area light sampling
		EmittedLight:   core.Vec3{X: 5, Y: 5, Z: 5},
	}

	// Subsequent vertices are bounces from the light
	for i := 1; i < len(positions); i++ {
		material := materials[i]
		// For test purposes, assume Metal and Dielectric are specular
		isSpecular := false

		vertices[i] = Vertex{
			Point:          positions[i],
			Normal:         core.NewVec3(0, 1, 0),
			Material:       material,
			IsSpecular:     isSpecular,
			Beta:           core.Vec3{X: 0.7, Y: 0.7, Z: 0.7},
			AreaPdfForward: 1.0 / math.Pi,
			AreaPdfReverse: 1.0 / math.Pi,
		}
	}

	return Path{
		Vertices: vertices,
		Length:   len(vertices),
	}
}

// createPathWithInfiniteLight creates a path that hits an infinite area light
func createPathWithInfiniteLight() Path {
	return Path{
		Vertices: []Vertex{
			{
				Point:    core.NewVec3(0, 0, 0),
				Normal:   core.NewVec3(0, 0, 1),
				IsCamera: true,
				Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
			},
			{
				Point:           core.NewVec3(0, 0, -1000), // Far away
				Normal:          core.NewVec3(0, 0, 1),
				IsInfiniteLight: true,
				IsLight:         true,
				Beta:            core.Vec3{X: 1, Y: 1, Z: 1},
				AreaPdfForward:  1.0 / (4.0 * math.Pi), // uniform sphere PDF
				EmittedLight:    core.Vec3{X: 1, Y: 1, Z: 1},
			},
		},
		Length: 2,
	}
}

// createMinimalCornellScene creates a Cornell scene with walls, floor and quad light
func createMinimalCornellScene(includeBoxes bool) core.Scene {
	// Create a basic scene
	scene := &TestScene{
		shapes: make([]core.Shape, 0),
		lights: make([]core.Light, 0),
	}

	// Create materials (same as real Cornell scene)
	white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
	red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
	green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

	// Add Cornell walls (same as pkg/scene/cornell.go)
	// Floor
	floor := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 0.0),
		core.NewVec3(556, 0.0, 0.0),
		core.NewVec3(0.0, 0.0, 556),
		white,
	)

	// Ceiling
	ceiling := geometry.NewQuad(
		core.NewVec3(0.0, 556, 0.0),
		core.NewVec3(556, 0.0, 0.0),
		core.NewVec3(0.0, 0.0, 556),
		white,
	)

	// Back wall
	backWall := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 556),
		core.NewVec3(556.0, 0.0, 0.0),
		core.NewVec3(0.0, 556, 0.0),
		white,
	)

	// Left wall (red)
	leftWall := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 0.0),
		core.NewVec3(0.0, 0.0, 556),
		core.NewVec3(0.0, 556, 0.0),
		red,
	)

	// Right wall (green)
	rightWall := geometry.NewQuad(
		core.NewVec3(556, 0.0, 0.0),
		core.NewVec3(0.0, 0.0, 556),
		core.NewVec3(0.0, 556, 0.0),
		green,
	)

	scene.shapes = append(scene.shapes, floor, ceiling, backWall, leftWall, rightWall)

	// Quad light - same as Cornell scene
	lightCorner := core.NewVec3(213.0, 556-0.001, 227.0)         // Slightly below ceiling
	lightU := core.NewVec3(130.0, 0, 0)                          // U vector (X direction)
	lightV := core.NewVec3(0, 0, 105.0)                          // V vector (Z direction)
	lightEmission := core.NewVec3(18.0, 15.0, 8.0).Multiply(2.5) // Warm yellowish white

	// Create emissive material and quad light
	emissiveMat := material.NewEmissive(lightEmission)
	quadLight := geometry.NewQuadLight(lightCorner, lightU, lightV, emissiveMat)
	scene.lights = append(scene.lights, quadLight)
	scene.shapes = append(scene.shapes, quadLight.Quad)

	// directly copied from pkg/scene/cornell.go
	if includeBoxes {
		white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
		// Mirror material for the tall block - highly reflective surface
		mirror := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0) // Very shiny mirror

		// Custom configuration: tall mirrored box on left, short white box on right
		// This should show the red wall reflection in the mirrored surface

		// Short box (white, diffuse) - positioned on the RIGHT side
		shortBoxCenter := core.NewVec3(370.0, 82.5, 169.0) // Right side, front
		shortBox := geometry.NewBox(
			shortBoxCenter,                     // center position
			core.NewVec3(82.5, 82.5, 82.5),     // size (half-extents: 165/2 for each dimension)
			core.NewVec3(0, 18*math.Pi/180, 0), // rotation (18 degrees around Y axis)
			white,                              // white lambertian material
		)

		// Tall box (mirrored) - positioned on the LEFT side
		tallBoxCenter := core.NewVec3(185.0, 165.0, 351.0) // Left side, back
		tallBox := geometry.NewBox(
			tallBoxCenter,                       // center position
			core.NewVec3(82.5, 165.0, 82.5),     // size (half-extents: 165/2, 330/2, 165/2)
			core.NewVec3(0, -20*math.Pi/180, 0), // rotation (-15 degrees) - angled to catch red wall reflection
			mirror,                              // mirror material
		)

		// Add boxes to scene
		scene.shapes = append(scene.shapes, shortBox, tallBox)
	}

	return scene
}

// TestScene is a minimal scene implementation for testing
type TestScene struct {
	shapes       []core.Shape
	lights       []core.Light
	bvh          *core.BVH
	camera       *renderer.Camera
	lightSampler core.LightSampler
}

func (s *TestScene) GetShapes() []core.Shape { return s.shapes }
func (s *TestScene) GetLights() []core.Light { return s.lights }
func (s *TestScene) GetBVH() *core.BVH {
	if s.bvh == nil {
		s.bvh = core.NewBVH(s.shapes)
	}
	return s.bvh
}
func (s *TestScene) GetLightSampler() core.LightSampler {
	if s.lightSampler == nil {
		sceneRadius := 10.0 // Default radius for testing
		if s.bvh != nil {
			sceneRadius = s.bvh.Radius
		}
		s.lightSampler = core.NewUniformLightSampler(s.lights, sceneRadius)
	}
	return s.lightSampler
}
func (s *TestScene) GetSamplingConfig() core.SamplingConfig {
	return core.SamplingConfig{MaxDepth: 5, SamplesPerPixel: 1}
}
func (s *TestScene) GetCamera() core.Camera {
	if s.camera == nil {
		// Create a realistic camera for PDF testing - matches Cornell box setup exactly
		config := renderer.CameraConfig{
			Center:        core.NewVec3(278, 278, -800), // Cornell box camera position
			LookAt:        core.NewVec3(278, 278, 0),    // Look at the center of the box
			Up:            core.NewVec3(0, 1, 0),        // Standard up direction
			Width:         400,
			AspectRatio:   1.0,  // Square aspect ratio for Cornell box
			VFov:          40.0, // Official Cornell field of view
			Aperture:      0.0,  // No depth of field for Cornell box
			FocusDistance: 0.0,  // Auto-calculate focus distance
		}
		s.camera = renderer.NewCamera(config)
	}

	return s.camera
}

func (s *TestScene) Preprocess() error {
	// Simple preprocessing for tests - just preprocess lights that need it
	for _, light := range s.lights {
		if preprocessor, ok := light.(core.Preprocessor); ok {
			if err := preprocessor.Preprocess(s); err != nil {
				return err
			}
		}
	}

	// Create light sampler
	sceneRadius := 10.0 // Default radius for testing
	if s.bvh != nil {
		sceneRadius = s.bvh.Radius
	}
	s.lightSampler = core.NewUniformLightSampler(s.lights, sceneRadius)

	return nil
}
