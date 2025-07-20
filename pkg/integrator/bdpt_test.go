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
	path := integrator.generateCameraSubpath(ray, scene, sampler, 2)

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
	path := integrator.generateLightSubpath(scene, sampler, 2)

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
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})

	tests := []struct {
		name                   string
		pathVertices           []Vertex
		pathLength             int
		t                      int // strategy parameter
		expectedContribution   core.Vec3
		expectZeroContribution bool
	}{
		{
			name: "PathHittingEmissiveVertex",
			pathVertices: []Vertex{
				createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1), false, true, nil), // camera
				{
					Point:        core.NewVec3(0, 0, -1),
					Normal:       core.NewVec3(0, 0, 1),
					Material:     nil,
					IsLight:      true,
					Beta:         core.Vec3{X: 0.8, Y: 0.6, Z: 0.4}, // throughput from camera
					EmittedLight: core.Vec3{X: 2.0, Y: 1.5, Z: 1.0}, // emissive surface
				},
			},
			pathLength:           2,
			t:                    2,
			expectedContribution: core.Vec3{X: 1.6, Y: 0.9, Z: 0.4}, // EmittedLight * Beta
		},
		{
			name: "PathHittingNonEmissiveVertex",
			pathVertices: []Vertex{
				createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1), false, true, nil), // camera
				{
					Point:        core.NewVec3(0, 0, -1),
					Normal:       core.NewVec3(0, 0, 1),
					Material:     material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
					IsLight:      false,
					Beta:         core.Vec3{X: 0.8, Y: 0.6, Z: 0.4},
					EmittedLight: core.Vec3{X: 0, Y: 0, Z: 0}, // no emission
				},
			},
			pathLength:             2,
			t:                      2,
			expectZeroContribution: true,
		},
		{
			name: "PathHittingBackground",
			pathVertices: []Vertex{
				createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1), false, true, nil), // camera
				{
					Point:           core.NewVec3(0, 0, -1000),
					Normal:          core.NewVec3(0, 0, 1),
					Material:        nil,
					IsLight:         true,
					IsInfiniteLight: true,
					Beta:            core.Vec3{X: 1, Y: 1, Z: 1},       // no attenuation to background
					EmittedLight:    core.Vec3{X: 0.5, Y: 0.7, Z: 1.0}, // sky color
				},
			},
			pathLength:           2,
			t:                    2,
			expectedContribution: core.Vec3{X: 0.5, Y: 0.7, Z: 1.0}, // background emission
		},
		{
			name: "IncompletePathRequest",
			pathVertices: []Vertex{
				createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1), false, true, nil), // camera
				createTestVertex(core.NewVec3(0, 0, -1), core.NewVec3(0, 0, 1), false, false, material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))),
			},
			pathLength:             2,
			t:                      2, // asking for incomplete path
			expectZeroContribution: true,
		},
		{
			name: "ZeroBetaThroughput",
			pathVertices: []Vertex{
				createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1), false, true, nil), // camera
				{
					Point:        core.NewVec3(0, 0, -1),
					Normal:       core.NewVec3(0, 0, 1),
					Material:     nil,
					IsLight:      true,
					Beta:         core.Vec3{X: 0, Y: 0, Z: 0}, // zero throughput
					EmittedLight: core.Vec3{X: 2.0, Y: 1.5, Z: 1.0},
				},
			},
			pathLength:             2,
			t:                      2,
			expectZeroContribution: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := Path{
				Vertices: tt.pathVertices,
				Length:   tt.pathLength,
			}

			contribution := integrator.evaluatePathTracingStrategy(path, tt.t)

			if tt.expectZeroContribution {
				if contribution.Luminance() > 1e-6 {
					t.Errorf("Expected zero contribution, got %v", contribution)
				}
			} else {
				tolerance := 1e-6
				if math.Abs(contribution.X-tt.expectedContribution.X) > tolerance ||
					math.Abs(contribution.Y-tt.expectedContribution.Y) > tolerance ||
					math.Abs(contribution.Z-tt.expectedContribution.Z) > tolerance {
					t.Errorf("Expected contribution %v, got %v", tt.expectedContribution, contribution)
				}
			}
		})
	}
}

// Helper function to calculate expected direct lighting contribution
func calculateExpectedDirectLighting(
	vertex Vertex,
	lightPoint core.Vec3,
	emission core.Vec3,
) core.Vec3 {
	// Vector from surface to light
	lightDirection := lightPoint.Subtract(vertex.Point)
	distance := lightDirection.Length()
	normalizedLightDir := lightDirection.Normalize()

	// Cosine term (surface normal dot light direction)
	cosTheta := math.Max(0, vertex.Normal.Dot(normalizedLightDir))
	if cosTheta <= 0 {
		return core.Vec3{} // No contribution from backfacing
	}

	// BRDF evaluation for Lambertian materials
	if lambertian, ok := vertex.Material.(*material.Lambertian); ok {
		brdfValue := lambertian.Albedo.Multiply(1.0 / math.Pi)
		// Direct lighting: beta * brdf * emission * cosTheta / distance²
		return vertex.Beta.MultiplyVec(brdfValue.MultiplyVec(emission)).Multiply(cosTheta / (distance * distance))
	}

	// Other materials (metal, glass) typically don't contribute to direct lighting
	return core.Vec3{}
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
		topColor: core.NewVec3(0.3, 0.3, 0.3), bottomColor: core.NewVec3(0.1, 0.1, 0.1),
		camera: camera, config: core.SamplingConfig{MaxDepth: 5},
	}
}

// TestEvaluateDirectLightingStrategy tests s=1 strategies with various scenarios
func TestEvaluateDirectLightingStrategy(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})

	tests := []struct {
		name                   string
		cameraVertex           Vertex
		light                  core.Light
		sampler                *TestSampler
		expectedLightPoint     core.Vec3
		expectedContribution   core.Vec3
		expectZeroContribution bool
		tolerance              float64
		testDescription        string
	}{
		{
			name: "DiffuseSurfaceWithQuadLight",
			cameraVertex: Vertex{
				Point:             core.NewVec3(0, 0, 0),
				Normal:            core.NewVec3(0, 1, 0), // pointing up
				Material:          material.NewLambertian(core.NewVec3(0.7, 0.5, 0.3)),
				Beta:              core.Vec3{X: 0.8, Y: 0.6, Z: 0.4},
				IncomingDirection: core.NewVec3(0, 0, 1),
			},
			light: geometry.NewQuadLight(
				core.NewVec3(-0.5, 2.0, -0.5), // corner
				core.NewVec3(1.0, 0.0, 0.0),   // u vector
				core.NewVec3(0.0, 0.0, 1.0),   // v vector
				material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0)),
			),
			sampler: NewTestSampler(
				[]float64{0.0},                      // light selection
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // quad center
				[]core.Vec3{},
			),
			expectedLightPoint:   core.NewVec3(0, 2, 0),
			expectedContribution: core.NewVec3(0.223, 0.119, 0.0477), // calculated by helper
			tolerance:            1e-3,
			testDescription:      "Standard diffuse surface with quad light above",
		},
		{
			name: "DiffuseSurfaceWithPointSpotLightAway",
			cameraVertex: Vertex{
				Point:             core.NewVec3(0, 0, 0),
				Normal:            core.NewVec3(0, 1, 0), // pointing up
				Material:          material.NewLambertian(core.NewVec3(0.7, 0.5, 0.3)),
				Beta:              core.Vec3{X: 0.8, Y: 0.6, Z: 0.4},
				IncomingDirection: core.NewVec3(0, 0, 1),
			},
			light: geometry.NewPointSpotLight(
				core.NewVec3(0, 2, 0),       // light position above surface
				core.NewVec3(0, 2, -10),     // pointing away from surface
				core.NewVec3(5.0, 5.0, 5.0), // emission
				30.0, 5.0,                   // cone angle, falloff
			),
			sampler: NewTestSampler(
				[]float64{0.0},                      // light selection
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // dummy 2D value (not used by point lights)
				[]core.Vec3{},
			),
			expectedLightPoint:     core.NewVec3(0, 2, 0),
			expectedContribution:   core.Vec3{}, // zero because light points away
			expectZeroContribution: true,
			tolerance:              1e-6,
			testDescription:        "Point spot light pointing away from surface",
		},
		{
			name: "SurfaceWithNegativeCosine",
			cameraVertex: Vertex{
				Point:             core.NewVec3(0, 0, 0),
				Normal:            core.NewVec3(0, 1, 0), // pointing up
				Material:          material.NewLambertian(core.NewVec3(0.7, 0.5, 0.3)),
				Beta:              core.Vec3{X: 0.8, Y: 0.6, Z: 0.4},
				IncomingDirection: core.NewVec3(0, 0, 1),
			},
			light: geometry.NewQuadLight(
				core.NewVec3(-0.5, -2.0, -0.5), // corner below surface
				core.NewVec3(1.0, 0.0, 0.0),    // u vector
				core.NewVec3(0.0, 0.0, 1.0),    // v vector
				material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0)),
			),
			sampler: NewTestSampler(
				[]float64{0.0},                      // light selection
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // quad center
				[]core.Vec3{},
			),
			expectedLightPoint:     core.NewVec3(0, -2, 0),
			expectedContribution:   core.Vec3{}, // zero because cosTheta < 0
			expectZeroContribution: true,
			tolerance:              1e-6,
			testDescription:        "Light below surface (negative cosine)",
		},
		{
			name: "SpecularSurface",
			cameraVertex: Vertex{
				Point:      core.NewVec3(0, 0, 0),
				Normal:     core.NewVec3(0, 1, 0),
				Material:   material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0),
				Beta:       core.Vec3{X: 1, Y: 1, Z: 1},
				IsSpecular: true,
			},
			light: geometry.NewQuadLight(
				core.NewVec3(-0.5, 2.0, -0.5),
				core.NewVec3(1.0, 0.0, 0.0),
				core.NewVec3(0.0, 0.0, 1.0),
				material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0)),
			),
			sampler: NewTestSampler(
				[]float64{0.0},                      // light selection
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // quad center
				[]core.Vec3{},
			),
			expectedLightPoint:     core.NewVec3(0, 2, 0),
			expectedContribution:   core.Vec3{}, // zero for specular materials
			expectZeroContribution: true,
			tolerance:              1e-6,
			testDescription:        "Specular surfaces should not use direct lighting strategy",
		},
		{
			name: "ZeroBetaThroughput",
			cameraVertex: Vertex{
				Point:    core.NewVec3(0, 0, 0),
				Normal:   core.NewVec3(0, 1, 0),
				Material: material.NewLambertian(core.NewVec3(0.7, 0.5, 0.3)),
				Beta:     core.Vec3{X: 0, Y: 0, Z: 0}, // zero throughput
			},
			light: geometry.NewQuadLight(
				core.NewVec3(-0.5, 2.0, -0.5),
				core.NewVec3(1.0, 0.0, 0.0),
				core.NewVec3(0.0, 0.0, 1.0),
				material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0)),
			),
			sampler: NewTestSampler(
				[]float64{0.0},                      // light selection
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // quad center
				[]core.Vec3{},
			),
			expectedLightPoint:     core.NewVec3(0, 2, 0),
			expectedContribution:   core.Vec3{}, // zero because beta is zero
			expectZeroContribution: true,
			tolerance:              1e-6,
			testDescription:        "Zero beta should result in zero contribution",
		},
	}

	// Run all test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create path with camera and surface vertex
			path := Path{
				Vertices: []Vertex{
					createTestVertex(core.NewVec3(0, 0, 5), core.NewVec3(0, 0, -1), false, true, nil),
					tt.cameraVertex,
				},
				Length: 2,
			}

			// Create scene with the specific light for this test
			scene := createSceneWithLight(tt.light)

			// Run the test with deterministic sampler
			contribution, sampledVertex := integrator.evaluateDirectLightingStrategy(path, 2, scene, tt.sampler)

			// Verify sampled light point (only if we expect a non-zero contribution)
			if !tt.expectZeroContribution && sampledVertex != nil {
				if !sampledVertex.Point.Equals(tt.expectedLightPoint) {
					t.Errorf("Expected sampled light point %v, got %v", tt.expectedLightPoint, sampledVertex.Point)
				}
			}

			// Verify contribution
			if tt.expectZeroContribution {
				if contribution.Luminance() > tt.tolerance {
					t.Errorf("Expected zero contribution for %s, got %v (luminance: %v)",
						tt.testDescription, contribution, contribution.Luminance())
				}
			} else {
				// For non-zero cases, use the helper to calculate expected contribution
				emission := core.NewVec3(5.0, 5.0, 5.0) // All our test lights use this emission
				expectedContrib := calculateExpectedDirectLighting(tt.cameraVertex, tt.expectedLightPoint, emission)

				if math.Abs(contribution.X-expectedContrib.X) > tt.tolerance ||
					math.Abs(contribution.Y-expectedContrib.Y) > tt.tolerance ||
					math.Abs(contribution.Z-expectedContrib.Z) > tt.tolerance {
					t.Errorf("Expected contribution %v, got %v (tolerance: %e)",
						expectedContrib, contribution, tt.tolerance)
				}

				t.Logf("%s: Expected %v, Got %v", tt.name, expectedContrib, contribution)
			}

			// Verify sampled vertex properties
			if sampledVertex != nil && !tt.expectZeroContribution {
				if !sampledVertex.IsLight {
					t.Error("Sampled vertex should be marked as light")
				}
			}
		})
	}
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
		shapes:      []core.Shape{sphere, light.Quad},
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
