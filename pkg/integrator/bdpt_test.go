package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/lights"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
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
	integrator := NewBDPTIntegrator(scene.SamplingConfig{MaxDepth: 5})

	tests := []struct {
		name                string
		scene               *scene.Scene
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
	integrator := NewBDPTIntegrator(scene.SamplingConfig{MaxDepth: 3})
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
	integrator := NewBDPTIntegrator(scene.SamplingConfig{MaxDepth: 3})
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
func createSimpleTestScene() *scene.Scene {
	// Create a simple scene with one light and one diffuse surface
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, white)

	// Create a simple quad light (easier to predict sampling)
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissive := material.NewEmissive(emission)
	// Quad light at y=2, centered at (0,2,0), size 1x1 in XZ plane
	light := lights.NewQuadLight(
		core.NewVec3(-0.5, 2.0, -0.5), // corner
		core.NewVec3(1.0, 0.0, 0.0),   // u vector (X direction)
		core.NewVec3(0.0, 0.0, 1.0),   // v vector (Z direction)
		emissive,
	)

	// Use a real camera from the renderer package
	cameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         100,
		AspectRatio:   1.0,
		VFov:          45.0,
		Aperture:      0.0,
		FocusDistance: 0.0,
	}
	camera := geometry.NewCamera(cameraConfig)
	ls := []lights.Light{light}

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere, light.Quad},
		Lights: ls, LightSampler: lights.NewUniformLightSampler(ls, 10),
		Camera: camera, SamplingConfig: scene.SamplingConfig{MaxDepth: 5},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()
	return scene
}

// Helper function to create a simple scene with specific lights
// If weights is provided, uses WeightedLightSampler; otherwise uses uniform sampling
func createSceneWithLightsAndWeights(ls []lights.Light, weights []float64) *scene.Scene {
	// Simple diffuse sphere (not used in our tests but needed for complete scene)
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -5), 0.5, white)

	var shapes []geometry.Shape = []geometry.Shape{sphere}

	// Add geometry for each light that has it
	for _, light := range ls {
		switch l := light.(type) {
		case *lights.QuadLight:
			shapes = append(shapes, l.Quad)
		case *lights.SphereLight:
			shapes = append(shapes, l.Sphere)
			// Point lights and infinite lights don't have geometry
		}
	}

	// Simple camera
	cameraConfig := geometry.CameraConfig{
		Center: core.NewVec3(0, 0, 0), LookAt: core.NewVec3(0, 0, -1), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := geometry.NewCamera(cameraConfig)

	scene := &scene.Scene{
		Shapes: shapes,
		Lights: ls, LightSampler: lights.NewUniformLightSampler(ls, 10),
		Camera: camera, SamplingConfig: scene.SamplingConfig{MaxDepth: 5},
	}

	if len(weights) > 0 {
		scene.LightSampler = lights.NewWeightedLightSampler(ls, weights, 10.0)
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()
	return scene
}

// Helper function to create a simple scene with a specific light (backwards compatibility)
func createSceneWithLight(light lights.Light) *scene.Scene {
	return createSceneWithLightsAndWeights([]lights.Light{light}, []float64{})
}

// createTestVertex creates a test vertex with specified properties
func createTestAreaLight() lights.Light {
	emissiveMaterial := material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0))
	return lights.NewSphereLight(core.NewVec3(0, 1, 0), 0.1, emissiveMaterial)
}

func createGlancingTestSceneWithMaterial(mat material.Material) *scene.Scene {
	// Create sphere with the specified material - positioned for camera ray hit
	// Sphere is centered at (0, 0, -2) so camera at origin can hit it
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 1.0, mat)

	// Point light for simple lighting
	pointLight := lights.NewPointSpotLight(
		core.NewVec3(0, 3, -2), core.NewVec3(0, -1, 0),
		core.NewVec3(3, 3, 3), 90.0, 1.0,
	)

	// Camera setup that matches createSimpleTestScene - at origin looking toward -Z
	cameraConfig := geometry.CameraConfig{
		Center: core.NewVec3(0, 0, 0), LookAt: core.NewVec3(0, 0, -1), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := geometry.NewCamera(cameraConfig)
	ls := []lights.Light{pointLight}

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere},
		Lights: ls, LightSampler: lights.NewUniformLightSampler(ls, 10),
		Camera: camera, SamplingConfig: scene.SamplingConfig{MaxDepth: 5},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()
	return scene
}

func createGlancingTestSceneAndRay(mat material.Material) (*scene.Scene, core.Ray) {
	scene := createGlancingTestSceneWithMaterial(mat)

	// Create the standard glancing ray that hits the sphere at an angle
	cameraOrigin := core.NewVec3(0, 0, 0)
	sphereGlancingPoint := core.NewVec3(0.5, 0, -1.5)
	ray := core.NewRayTo(cameraOrigin, sphereGlancingPoint)

	return scene, ray
}

func createLightSceneWithMaterial(mat material.Material) *scene.Scene {
	// Create a surface for light to bounce off of
	sphere := geometry.NewSphere(core.NewVec3(0, -1.5, 0), 0.8, mat)

	// create a bounding sphere to capture escaped rays so we can check final beta
	boundingSphere := geometry.NewSphere(core.NewVec3(0, 0, 0), 10.0, material.NewLambertian(core.NewVec3(0.0, 0.0, 0.0)))

	// Use quad light pointing downward - much more predictable than sphere light
	emissiveMaterial := material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0))
	// Create a horizontal quad at y=1.0, pointing downward (-Y direction)
	quadLight := lights.NewQuadLight(
		core.NewVec3(-0.5, 1.0, -0.5), // corner
		core.NewVec3(1.0, 0, 0),       // u vector (width)
		core.NewVec3(0, 0, 1.0),       // v vector (height)
		emissiveMaterial,
	)

	cameraConfig := geometry.CameraConfig{
		Center: core.NewVec3(3, 0, 0), LookAt: core.NewVec3(0, 0, 0), Up: core.NewVec3(0, 1, 0),
		Width: 100, AspectRatio: 1.0, VFov: 45.0,
	}
	camera := geometry.NewCamera(cameraConfig)
	ls := []lights.Light{quadLight}

	scene := &scene.Scene{
		Shapes: []geometry.Shape{sphere, boundingSphere},
		Lights: ls, LightSampler: lights.NewUniformLightSampler(ls, 10),
		Camera: camera, SamplingConfig: scene.SamplingConfig{MaxDepth: 5},
	}

	// Initialize BVH and preprocess lights
	scene.Preprocess()
	return scene
}

func createTestVertex(point core.Vec3, normal core.Vec3, isLight bool, isCamera bool, mat material.Material) Vertex {
	return Vertex{
		SurfaceInteraction: &material.SurfaceInteraction{
			Point:    point,
			Normal:   normal,
			Material: mat,
		},
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

// Misc integration tests

// TestLightPathDirectionAndIntersection verifies that light paths are generated correctly
func TestLightPathDirectionAndIntersection(t *testing.T) {
	testScene := createMinimalCornellScene(false)

	// Generate multiple light paths to test consistency
	seed := int64(42)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := scene.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	successfulPaths := 0
	totalPaths := 10

	for i := 0; i < totalPaths; i++ {
		lightPath := bdptIntegrator.generateLightPath(testScene, sampler, bdptConfig.MaxDepth)

		t.Logf("Light path %d: length=%d", i, lightPath.Length)

		if lightPath.Length == 0 {
			t.Logf("  No light path generated (no lights or invalid sample)")
			continue
		}

		// Check initial light vertex
		lightVertex := lightPath.Vertices[0]
		t.Logf("  Light vertex: pos=%v, normal=%v, IsLight=%v, EmittedLight=%v",
			lightVertex.Point, lightVertex.Normal, lightVertex.IsLight, lightVertex.EmittedLight)

		// Verify light vertex properties
		if !lightVertex.IsLight {
			t.Errorf("  First vertex should be marked as light")
		}
		if lightVertex.EmittedLight.Luminance() <= 0 {
			t.Errorf("  Light vertex should have positive emission: %v", lightVertex.EmittedLight)
		}

		// Check if light path hits the floor
		foundFloor := false
		for j, vertex := range lightPath.Vertices {
			t.Logf("  Light[%d]: pos=%v, material=%v, IsLight=%v",
				j, vertex.Point, vertex.Material != nil, vertex.IsLight)

			// Check if this vertex is on the floor (y ≈ 0)
			if vertex.Point.Y < 1.0 && vertex.Point.Y > -1.0 {
				foundFloor = true
				t.Logf("  Found floor hit at vertex %d: pos=%v", j, vertex.Point)

				// Verify the floor vertex has reasonable properties
				if vertex.Material == nil {
					t.Errorf("  Floor vertex should have a material")
				}
			}
		}

		if foundFloor {
			successfulPaths++
		} else {
			t.Logf("  Light path did not reach floor")
		}

		// Check that light path doesn't immediately hit the light geometry
		if lightPath.Length > 1 {
			secondVertex := lightPath.Vertices[1]
			// The second vertex should not be another light (self-intersection)
			if secondVertex.IsLight && secondVertex.Point.Y > 500 {
				t.Errorf("  Light path may be hitting light geometry itself at vertex 1: pos=%v", secondVertex.Point)
			}
		}
	}

	t.Logf("Successful paths (reached floor): %d/%d", successfulPaths, totalPaths)

	// At least some paths should reach the floor in a simple scene
	if successfulPaths == 0 {
		t.Errorf("No light paths reached the floor - this suggests directional issues")
	}
}

// TestBDPTCameraPathHitsLight tests that camera paths can find light sources
func TestBDPTCameraPathHitsLight(t *testing.T) {
	testScene := createMinimalCornellScene(false)

	// Create a ray that should hit the light directly
	rayToLight := core.NewRay(
		core.NewVec3(278, 400, 278), // Camera position below light
		core.NewVec3(0, 1, 0),       // Ray pointing straight up toward light
	)

	seed := int64(42)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := scene.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that should hit the light
	cameraPath := bdptIntegrator.generateCameraPath(rayToLight, testScene, sampler, bdptConfig.MaxDepth)

	// Assert: Camera path should have at least 2 vertices (camera + light hit)
	if cameraPath.Length < 2 {
		t.Fatalf("Camera path should have at least 2 vertices, got %d", cameraPath.Length)
	}

	// Assert: First vertex should be camera
	cameraVertex := cameraPath.Vertices[0]
	if !cameraVertex.IsCamera {
		t.Errorf("First vertex should be camera, got IsCamera=%v", cameraVertex.IsCamera)
	}
	if cameraVertex.IsLight {
		t.Errorf("Camera vertex should not be marked as light, got IsLight=%v", cameraVertex.IsLight)
	}

	// Assert: Some vertex should hit the light (have positive emission)
	foundLight := false
	for i, vertex := range cameraPath.Vertices {
		if vertex.EmittedLight.Luminance() > 0 {
			foundLight = true
			t.Logf("Found light hit at vertex %d: emission=%v", i, vertex.EmittedLight)

			// Assert: Light-hitting vertex should be marked as light
			if !vertex.IsLight {
				t.Errorf("Vertex %d hits light but IsLight=false", i)
			}
			break
		}
	}

	if !foundLight {
		t.Errorf("Camera path pointing at light should hit light source")
	}
}

// TestBDPTConnectionStrategy tests that BDPT can connect camera and light paths
func TestBDPTConnectionStrategy(t *testing.T) {
	testScene := createMinimalCornellScene(false)

	seed := int64(42)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := scene.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraPath(rayToFloor, testScene, sampler, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightPath(testScene, sampler, bdptConfig.MaxDepth)

	// Assert: Both paths should exist
	if cameraPath.Length == 0 {
		t.Fatalf("Camera path should have vertices")
	}
	if lightPath.Length == 0 {
		t.Fatalf("Light path should have vertices")
	}

	// Assert: Camera path should hit floor (material surface)
	foundFloorHit := false
	for i, vertex := range cameraPath.Vertices {
		if vertex.Material != nil && vertex.Point.Y < 1.0 {
			foundFloorHit = true
			t.Logf("Camera path hits floor at vertex %d: pos=%v", i, vertex.Point)
			break
		}
	}
	if !foundFloorHit {
		t.Errorf("Camera path should hit floor for connection test")
	}

	// Assert: Light path should have light source
	if !lightPath.Vertices[0].IsLight {
		t.Errorf("Light path should start with light vertex")
	}
	if lightPath.Vertices[0].EmittedLight.Luminance() <= 0 {
		t.Errorf("Light path should start with positive emission")
	}

	// Test connection strategy: light source to camera floor hit
	if cameraPath.Length >= 2 && lightPath.Length >= 1 {
		// s=1: light source (first vertex in light path)
		// t=2: floor hit (second vertex in camera path, first bounce from camera)
		contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 1, 2, testScene)
		t.Logf("Connection strategy (s=0, t=1) contribution: %v (luminance: %.6f)",
			contribution, contribution.Luminance())

		// Assert: Connection should produce some contribution (this is the key test!)
		if contribution.Luminance() <= 0 {
			t.Errorf("Connection strategy should produce positive contribution when connecting light source to floor hit")
		}
	}
}

// TestBDPTPathIndexing verifies how paths are indexed in our implementation
func TestBDPTPathIndexing(t *testing.T) {
	testScene := createMinimalCornellScene(false)

	seed := int64(42)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := scene.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraPath(rayToFloor, testScene, sampler, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightPath(testScene, sampler, bdptConfig.MaxDepth)

	t.Logf("=== CAMERA PATH (length %d) ===", cameraPath.Length)
	for i, vertex := range cameraPath.Vertices {
		t.Logf("  Vertex[%d]: pos=%v, IsCamera=%v, IsLight=%v, Material=%v",
			i, vertex.Point, vertex.IsCamera, vertex.IsLight, vertex.Material != nil)
	}

	t.Logf("=== LIGHT PATH (length %d) ===", lightPath.Length)
	for i, vertex := range lightPath.Vertices {
		t.Logf("  Vertex[%d]: pos=%v, IsCamera=%v, IsLight=%v, Material=%v",
			i, vertex.Point, vertex.IsCamera, vertex.IsLight, vertex.Material != nil)
	}

	// In standard BDPT:
	// Camera path: t=0 (camera), t=1 (first bounce), t=2 (second bounce), ...
	// Light path: s=0 (light), s=1 (first bounce), s=2 (second bounce), ...

	// So connecting s=0,t=1 should connect light source to first camera bounce
	if cameraPath.Length >= 2 && lightPath.Length >= 1 {
		t.Logf("=== Standard BDPT s=0,t=1 connection ===")
		t.Logf("s=0 should be light source: %v", lightPath.Vertices[0])
		t.Logf("t=1 should be first camera bounce: %v", cameraPath.Vertices[1])

		// Now using proper 0-based indexing
		contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, testScene)
		t.Logf("Connection contribution: %v (luminance: %.6f)", contribution, contribution.Luminance())
	}
}

// ============================================================================
// HELPER FUNCTIONS FOR COMPREHENSIVE MIS TESTS
// ============================================================================

// createTestCameraPath creates a camera path with specified materials and positions
func createTestCameraPath(materials []material.Material, positions []core.Vec3) Path {
	return createTestCameraPathWithLight(materials, positions, nil)
}

func createTestCameraPathWithLight(materials []material.Material, positions []core.Vec3, hitLight lights.Light) Path {
	if len(positions) != len(materials)+1 {
		panic("positions must be one more than materials")
	}

	vertices := make([]Vertex, len(positions))

	// First vertex is always camera
	vertices[0] = Vertex{
		SurfaceInteraction: &material.SurfaceInteraction{
			Point:  positions[0],
			Normal: core.NewVec3(0, 0, 1), // camera "normal"
		},
		IsCamera:       true,
		Beta:           core.Vec3{X: 1, Y: 1, Z: 1},
		AreaPdfForward: 1.0,
	}

	// Subsequent vertices use provided materials
	for i := 1; i < len(positions); i++ {
		mat := materials[i-1]

		// Check proper specular and emissive types
		_, isMetalSpecular := mat.(*material.Metal)
		_, isDielectricSpecular := mat.(*material.Dielectric)
		isSpecular := isMetalSpecular || isDielectricSpecular
		_, isEmissive := mat.(*material.Emissive)

		vertices[i] = Vertex{
			SurfaceInteraction: &material.SurfaceInteraction{
				Point:    positions[i],
				Normal:   core.NewVec3(0, 1, 0), // upward normal
				Material: mat,
			},
			IsSpecular:     isSpecular,
			IsLight:        isEmissive,
			Beta:           core.Vec3{X: 0.7, Y: 0.7, Z: 0.7}, // typical diffuse reflectance
			AreaPdfForward: 1.0 / math.Pi,                     // cosine-weighted hemisphere sampling
			AreaPdfReverse: 1.0 / math.Pi,
		}

		// If this is the last vertex, it's emissive, and we have a hitLight, set it up properly
		if i == len(positions)-1 && isEmissive && hitLight != nil {
			vertices[i].Light = hitLight
			vertices[i].LightIndex = 0 // Default to light index 0 for tests
			// Set EmittedLight using the same pattern as BDPT - use incoming ray
			incomingRay := core.NewRayTo(positions[i-1], positions[i])
			if emitter, isEmissive := mat.(material.Emitter); isEmissive {
				vertices[i].EmittedLight = emitter.Emit(incomingRay, vertices[i].SurfaceInteraction)
			}
		}
	}

	return Path{
		Vertices: vertices,
		Length:   len(vertices),
	}
}

// createTestLightPath creates a light path with specified materials and positions
func createTestLightPath(materials []material.Material, positions []core.Vec3) Path {
	if len(positions) != len(materials) {
		panic("positions and materials must have same length")
	}

	vertices := make([]Vertex, len(positions))

	// First vertex is always a light source
	testLight := createTestAreaLight()
	vertices[0] = Vertex{
		SurfaceInteraction: &material.SurfaceInteraction{
			Point:    positions[0],
			Normal:   core.NewVec3(0, -1, 0), // downward-facing light
			Material: materials[0],
		},
		IsLight:        true,
		Light:          testLight,
		LightIndex:     0,                           // Test light at index 0
		Beta:           core.Vec3{X: 1, Y: 1, Z: 1}, // full light emission
		AreaPdfForward: 0.25 / math.Pi,              // area light sampling
		EmittedLight:   core.Vec3{X: 5, Y: 5, Z: 5},
	}

	// Subsequent vertices are bounces from the light
	for i := 1; i < len(positions); i++ {
		mat := materials[i]
		// Check if material is specular (Metal or Dielectric)
		_, isMetalSpecular := mat.(*material.Metal)
		_, isDielectricSpecular := mat.(*material.Dielectric)
		isSpecular := isMetalSpecular || isDielectricSpecular

		vertices[i] = Vertex{
			SurfaceInteraction: &material.SurfaceInteraction{
				Point:    positions[i],
				Normal:   core.NewVec3(0, 1, 0),
				Material: mat,
			},
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
				SurfaceInteraction: &material.SurfaceInteraction{
					Point:  core.NewVec3(0, 0, 0),
					Normal: core.NewVec3(0, 0, 1),
				},
				IsCamera: true,
				Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
			},
			{
				SurfaceInteraction: &material.SurfaceInteraction{
					Point:  core.NewVec3(0, 0, -1000),
					Normal: core.NewVec3(0, 0, 1),
				},
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
func createMinimalCornellScene(includeBoxes bool) *scene.Scene {
	// Create a basic scene
	config := geometry.CameraConfig{
		Center:        core.NewVec3(278, 278, -800), // Cornell box camera position
		LookAt:        core.NewVec3(278, 278, 0),    // Look at the center of the box
		Up:            core.NewVec3(0, 1, 0),        // Standard up direction
		Width:         400,
		AspectRatio:   1.0,  // Square aspect ratio for Cornell box
		VFov:          40.0, // Official Cornell field of view
		Aperture:      0.0,  // No depth of field for Cornell box
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}
	camera := geometry.NewCamera(config)

	scene := &scene.Scene{
		Shapes: make([]geometry.Shape, 0),
		Lights: make([]lights.Light, 0),
		Camera: camera,
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

	scene.Shapes = append(scene.Shapes, floor, ceiling, backWall, leftWall, rightWall)

	// Quad light - same as Cornell scene
	lightCorner := core.NewVec3(213.0, 556-0.001, 227.0)         // Slightly below ceiling
	lightU := core.NewVec3(130.0, 0, 0)                          // U vector (X direction)
	lightV := core.NewVec3(0, 0, 105.0)                          // V vector (Z direction)
	lightEmission := core.NewVec3(18.0, 15.0, 8.0).Multiply(2.5) // Warm yellowish white

	// Create emissive material and quad light
	emissiveMat := material.NewEmissive(lightEmission)
	quadLight := lights.NewQuadLight(lightCorner, lightU, lightV, emissiveMat)
	scene.Lights = append(scene.Lights, quadLight)
	scene.Shapes = append(scene.Shapes, quadLight.Quad)

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
		scene.Shapes = append(scene.Shapes, shortBox, tallBox)
	}

	scene.LightSampler = lights.NewUniformLightSampler(scene.Lights, 10)

	// Initialize BVH and preprocess lights
	scene.Preprocess()

	return scene
}
