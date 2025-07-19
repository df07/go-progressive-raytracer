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
			initialRay:          core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0)),
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

			random := rand.New(rand.NewSource(42))
			integrator.extendPath(path, tt.initialRay, tt.initialBeta, tt.initialPdfFwd, tt.scene, random, tt.maxBounces, true)

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

	random := rand.New(rand.NewSource(42))
	path := integrator.generateCameraSubpath(ray, scene, random, 2)

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

	random := rand.New(rand.NewSource(42))
	path := integrator.generateLightSubpath(scene, random, 2)

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
// 2. LIGHT CALCULATION TESTS
// Test strategy evaluation and beta/throughput propagation
// ============================================================================

// TestEvaluatePathTracingStrategy tests s=0 strategies
func TestEvaluatePathTracingStrategy(t *testing.T) {
	// TODO: Test camera-only paths hitting lights
	// TODO: Test contribution calculation (emitted light * beta)
	t.Skip("TODO: Implement path tracing strategy tests")
}

// TestEvaluateDirectLightingStrategy tests s=1 strategies
func TestEvaluateDirectLightingStrategy(t *testing.T) {
	// TODO: Test direct light sampling from surface
	// TODO: Test BRDF evaluation and cosine factors
	t.Skip("TODO: Implement direct lighting strategy tests")
}

// TestEvaluateConnectionStrategy tests s>1,t>1 connection strategies
func TestEvaluateConnectionStrategy(t *testing.T) {
	// TODO: Test geometric term calculation
	// TODO: Test BRDF evaluation at both vertices
	t.Skip("TODO: Implement connection strategy tests")
}

// TestEvaluateLightTracingStrategy tests t=1 strategies
func TestEvaluateLightTracingStrategy(t *testing.T) {
	// TODO: Test light-to-camera connections
	// TODO: Test splat ray generation
	t.Skip("TODO: Implement light tracing strategy tests")
}

// TestBetaPropagation tests throughput calculation in path generation
func TestBetaPropagation(t *testing.T) {
	// TODO: Test beta calculation through diffuse bounces
	// TODO: Test beta calculation through specular bounces
	t.Skip("TODO: Implement beta propagation tests")
}

// ============================================================================
// 3. MIS CALCULATION TESTS
// Test MIS weights and PDF propagation
// ============================================================================

// TestVertexConvertSolidAngleToAreaPdf tests PDF conversion including infinite lights
func TestVertexConvertSolidAngleToAreaPdf(t *testing.T) {
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))

	tests := []struct {
		name            string
		fromPoint       core.Vec3
		fromNormal      core.Vec3
		toPoint         core.Vec3
		toNormal        core.Vec3
		toMaterial      core.Material
		isInfiniteLight bool
		solidAnglePdf   float64
		expectedPdf     float64
		tolerance       float64
	}{
		{
			name:          "SurfaceVertex_UnitDistance_DirectlyFacing",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(1, 0, 0),  // distance = 1
			toNormal:      core.NewVec3(-1, 0, 0), // cos(theta) = 1
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   1.0 * 1.0 / 1.0, // solidAngle * |cos| / dist²
			tolerance:     1e-10,
		},
		{
			name:          "SurfaceVertex_DistanceTwo_DirectlyFacing",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(2, 0, 0),  // distance = 2
			toNormal:      core.NewVec3(-1, 0, 0), // cos(theta) = 1
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   1.0 * 1.0 / 4.0, // solidAngle * |cos| / dist²
			tolerance:     1e-10,
		},
		{
			name:            "InfiniteLight_ShouldReturnOriginalPdf",
			fromPoint:       core.NewVec3(0, 0, 0),
			fromNormal:      core.NewVec3(0, 1, 0),
			toPoint:         core.NewVec3(1000, 1000, 1000),
			toNormal:        core.NewVec3(-1, -1, -1),
			toMaterial:      nil,
			isInfiniteLight: true,
			solidAnglePdf:   0.25,
			expectedPdf:     0.25, // unchanged for infinite lights
			tolerance:       1e-10,
		},
		{
			name:          "ZeroDistance_ShouldReturnZero",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(0, 0, 0), // same position
			toNormal:      core.NewVec3(0, 1, 0),
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   0.0, // zero distance case
			tolerance:     1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromVertex := createTestVertex(tt.fromPoint, tt.fromNormal, false, false, nil)
			toVertex := createTestVertex(tt.toPoint, tt.toNormal, false, false, tt.toMaterial)
			toVertex.IsInfiniteLight = tt.isInfiniteLight

			result := fromVertex.convertSolidAngleToAreaPdf(toVertex, tt.solidAnglePdf)

			if math.Abs(result-tt.expectedPdf) > tt.tolerance {
				t.Errorf("Expected PDF %.10f, got %.10f (diff: %.2e)",
					tt.expectedPdf, result, math.Abs(result-tt.expectedPdf))
			}
		})
	}
}

// TestCalculateMISWeight tests MIS weight computation
func TestCalculateMISWeight(t *testing.T) {
	// TODO: Test s+t==2 case (should return 1.0)
	// TODO: Test power heuristic calculation
	t.Skip("TODO: Implement MIS weight calculation tests")
}

// TestCalculateVertexPdf tests individual PDF calculations
func TestCalculateVertexPdf(t *testing.T) {
	// TODO: Test camera PDF calculation
	// TODO: Test material PDF calculation
	// TODO: Test light PDF calculation
	t.Skip("TODO: Implement vertex PDF calculation tests")
}

// TestPdfPropagation tests PDF forward/reverse calculation in path generation
func TestPdfPropagation(t *testing.T) {
	// TODO: Test forward PDF calculation during path extension
	// TODO: Test reverse PDF calculation during path extension
	t.Skip("TODO: Implement PDF propagation tests")
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// createSimpleTestScene creates a minimal scene for unit testing
func createSimpleTestScene() core.Scene {
	// Create a simple scene with one light and one diffuse surface
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, white)

	// Create a simple area light
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissive := material.NewEmissive(emission)
	light := geometry.NewSphereLight(core.NewVec3(0, 2, -1), 0.3, emissive)

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
		shapes:      []core.Shape{sphere, light.Sphere},
		lights:      []core.Light{light},
		topColor:    core.NewVec3(0.3, 0.3, 0.3),
		bottomColor: core.NewVec3(0.1, 0.1, 0.1),
		camera:      camera,
		config:      core.SamplingConfig{MaxDepth: 5},
	}
}

// createTestVertex creates a test vertex with specified properties
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
