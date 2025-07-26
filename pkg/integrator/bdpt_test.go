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
func calculateExpectedDirectLighting(vertex Vertex, lightPoint core.Vec3, emission core.Vec3) core.Vec3 {
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

// Helper function for vector tolerance checks
func isClose(a, b core.Vec3, tolerance float64) bool {
	return math.Abs(a.X-b.X) <= tolerance &&
		math.Abs(a.Y-b.Y) <= tolerance &&
		math.Abs(a.Z-b.Z) <= tolerance
}

// TestEvaluateLightTracingStrategy tests t=1 strategies
func TestEvaluateLightTracingStrategy(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})

	// Use existing scene creation helper
	scene, glancingRay := createGlancingTestSceneAndRay(material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)))
	_ = glancingRay // We'll use this when we add valid connection tests

	// Helper to create light path with specified vertices
	createLightPath := func(vertices []Vertex) Path {
		path := Path{Vertices: vertices, Length: len(vertices)}
		return path
	}

	tests := []struct {
		name               string
		lightPath          Path
		s                  int
		sampler            *TestSampler
		expectedSplats     int
		expectedSplatColor core.Vec3
		expectNilVertex    bool
		tolerance          float64
		testDescription    string
	}{
		{
			name: "InvalidPathLength_TooShort",
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 2, 0), IsLight: true},
			}),
			s:               1, // s <= 1 should fail
			sampler:         NewTestSampler([]float64{}, []core.Vec2{core.NewVec2(0.5, 0.5)}, []core.Vec3{}),
			expectedSplats:  0,
			expectNilVertex: true,
			testDescription: "Path length s=1 should return nil",
		},
		{
			name: "InvalidPathLength_TooLong",
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 2, 0), IsLight: true},
				{Point: core.NewVec3(0, 1, 0), Material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))},
			}),
			s:               3, // s > lightPath.Length should fail
			sampler:         NewTestSampler([]float64{}, []core.Vec2{core.NewVec2(0.5, 0.5)}, []core.Vec3{}),
			expectedSplats:  0,
			expectNilVertex: true,
			testDescription: "Path length s > lightPath.Length should return nil",
		},
		{
			name: "SpecularVertex_ShouldSkip",
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 2, 0), IsLight: true},
				{
					Point:      core.NewVec3(0, 1, 0),
					Normal:     core.NewVec3(0, 1, 0),
					Material:   material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0),
					IsSpecular: true, // Cannot connect through delta functions
					Beta:       core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			s:               2,
			sampler:         NewTestSampler([]float64{}, []core.Vec2{core.NewVec2(0.5, 0.5)}, []core.Vec3{}),
			expectedSplats:  0,
			expectNilVertex: true,
			testDescription: "Specular vertices should be skipped",
		},
		{
			name: "ValidDiffuseConnection_SlightOffset",
			lightPath: createLightPath([]Vertex{
				{
					Point:        core.NewVec3(0, 0, -4), // same as working test
					IsLight:      true,
					EmittedLight: core.NewVec3(2, 2, 2),
					Beta:         core.Vec3{X: 1, Y: 1, Z: 1},
				},
				{
					Point:             core.NewVec3(0.1, 0, -1.005),            // slight offset from center
					Normal:            core.NewVec3(0.1, 0, 0.995).Normalize(), // normal pointing toward camera
					Material:          material.NewLambertian(core.NewVec3(0.8, 0.6, 0.4)),
					Beta:              core.Vec3{X: 0.9, Y: 0.7, Z: 0.5},
					IncomingDirection: core.NewVec3(0, 0, 1), // same as working test
					IsSpecular:        false,
				},
			}),
			s: 2,
			sampler: NewTestSampler(
				[]float64{},
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // camera sampling
				[]core.Vec3{},
			),
			expectedSplats:     1,
			expectedSplatColor: core.NewVec3(0.326, 0.19, 0.0905), // measured from actual output
			tolerance:          1e-3,
			testDescription:    "Valid diffuse surface connection with slight offset",
		},
		{
			name: "ValidDiffuseConnection_CenterHit",
			lightPath: createLightPath([]Vertex{
				{
					Point:        core.NewVec3(0, 0, -4), // light behind sphere
					IsLight:      true,
					EmittedLight: core.NewVec3(2, 2, 2),
					Beta:         core.Vec3{X: 1, Y: 1, Z: 1},
				},
				{
					Point:             core.NewVec3(0, 0, -1), // front of sphere toward camera
					Normal:            core.NewVec3(0, 0, 1),  // pointing toward camera
					Material:          material.NewLambertian(core.NewVec3(0.8, 0.6, 0.4)),
					Beta:              core.Vec3{X: 0.9, Y: 0.7, Z: 0.5},
					IncomingDirection: core.NewVec3(0, 0, 1), // from light behind
					IsSpecular:        false,
				},
			}),
			s: 2,
			sampler: NewTestSampler(
				[]float64{},
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // camera sampling
				[]core.Vec3{},
			),
			expectedSplats:     1,
			expectedSplatColor: core.NewVec3(0.334, 0.195, 0.0928), // measured from actual output
			tolerance:          1e-3,
			testDescription:    "Valid diffuse surface connection at sphere center",
		},
		{
			name: "MetalSurface_NonSpecular",
			lightPath: createLightPath([]Vertex{
				{
					Point:        core.NewVec3(0, 0, -4), // same as working tests
					IsLight:      true,
					EmittedLight: core.NewVec3(2, 2, 2),
					Beta:         core.Vec3{X: 1, Y: 1, Z: 1},
				},
				{
					Point:             core.NewVec3(0, 0, -1), // front of sphere
					Normal:            core.NewVec3(0, 0, 1),
					Material:          material.NewMetal(core.NewVec3(0.9, 0.8, 0.7), 0.1), // rough metal
					Beta:              core.Vec3{X: 0.85, Y: 0.75, Z: 0.65},
					IncomingDirection: core.NewVec3(0, 0, 1), // same as working tests
					IsSpecular:        false,                 // roughened metal can be connected
				},
			}),
			s: 2,
			sampler: NewTestSampler(
				[]float64{},
				[]core.Vec2{core.NewVec2(0.5, 0.5)},
				[]core.Vec3{},
			),
			expectedSplats:     1,
			expectedSplatColor: core.NewVec3(1.11, 0.874, 0.663), // measured from actual output
			tolerance:          1e-2,
			testDescription:    "Roughened metal surface can be connected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splats, vertex := integrator.evaluateLightTracingStrategy(tt.lightPath, tt.s, scene, tt.sampler)

			// Check splat count
			if len(splats) != tt.expectedSplats {
				t.Errorf("%s: Expected %d splats, got %d", tt.testDescription, tt.expectedSplats, len(splats))
			}

			// Check vertex return
			if tt.expectNilVertex {
				if vertex != nil {
					t.Errorf("%s: Expected nil vertex, got %v", tt.testDescription, vertex)
				}
			} else {
				if vertex == nil {
					t.Errorf("%s: Expected non-nil vertex, got nil", tt.testDescription)
				}
			}

			// Check splat color for valid cases
			if tt.expectedSplats > 0 && len(splats) > 0 {
				splatColor := splats[0].Color
				if !isClose(splatColor, tt.expectedSplatColor, tt.tolerance) {
					t.Errorf("%s:\nExpected splat color: %v\nActual: %v\nDifference: %v",
						tt.testDescription,
						tt.expectedSplatColor,
						splatColor,
						splatColor.Subtract(tt.expectedSplatColor))
				}
			}
		})
	}
}

// TestCameraPathBetaPropagation tests beta calculation through actual BDPT methods
func TestCameraPathBetaPropagation(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})

	// Create a glancing ray that comes from the camera and hits the sphere at an angle
	// Camera is at origin, sphere is at (0,0,-2) with radius 1
	// To get a glancing hit, aim toward a point on the sphere's surface at an angle
	cameraOrigin := core.NewVec3(0, 0, 0)
	sphereGlancingPoint := core.NewVec3(0.5, 0, -1.5) // Point on sphere surface for glancing hit
	glancingRay := core.NewRayTo(cameraOrigin, sphereGlancingPoint)

	tests := []struct {
		name             string
		material         core.Material
		sampler          *TestSampler
		expectedVertices []ExpectedVertex
		testDescription  string
	}{
		{
			name:     "DiffuseMaterialWithGlancingAngle",
			material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
			sampler: NewTestSampler(
				[]float64{0.5, 0.5},                 // material sampling
				[]core.Vec2{core.NewVec2(0.0, 0.5)}, // hemisphere sampling
				[]core.Vec3{core.NewVec3(0, 1, 0)},  // scatter direction
			),
			expectedVertices: []ExpectedVertex{
				{index: 0, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: true, isSpecular: false, tolerance: 1e-9},        // camera vertex
				{index: 1, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: false, isSpecular: false, tolerance: 1e-9},       // surface hit (no attenuation yet)
				{index: 2, expectedBeta: core.Vec3{X: 0.7, Y: 0.7, Z: 0.7}, isCamera: false, isSpecular: false, tolerance: 1e-2}, // diffuse with cosTheta/PDF
			},
			testDescription: "Diffuse material should apply cosTheta/PDF correctly at glancing angle",
		},
		{
			name:     "SpecularMaterialWithGlancingAngle",
			material: material.NewMetal(core.NewVec3(0.9, 0.85, 0.8), 0.0),
			sampler: NewTestSampler(
				[]float64{0.5},                      // material sampling
				[]core.Vec2{core.NewVec2(0.5, 0.5)}, // not used for specular
				[]core.Vec3{core.NewVec3(0, 0, 1)},  // perfect reflection
			),
			expectedVertices: []ExpectedVertex{
				{index: 0, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: true, isSpecular: false, tolerance: 1e-9},         // camera vertex
				{index: 1, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: false, isSpecular: true, tolerance: 1e-9},         // surface hit is specular
				{index: 2, expectedBeta: core.Vec3{X: 0.9, Y: 0.85, Z: 0.8}, isCamera: false, isSpecular: false, tolerance: 1e-6}, // specular without cosTheta
			},
			testDescription: "Specular material should NOT multiply by cosTheta at glancing angle (most important test)",
		},
		{
			name:     "DielectricMaterialWithGlancingAngle",
			material: material.NewDielectric(1.5),
			sampler: NewTestSampler(
				[]float64{0.2, 0.5, 0.5, 0.5}, // more values for dielectric calculations
				[]core.Vec2{core.NewVec2(0.5, 0.5), core.NewVec2(0.3, 0.7)},
				[]core.Vec3{core.NewVec3(0, 0, 1), core.NewVec3(0, 1, 0)},
			),
			expectedVertices: []ExpectedVertex{
				{index: 0, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: true, isSpecular: false, tolerance: 1e-9}, // camera vertex
				{index: 1, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: false, isSpecular: true, tolerance: 1e-9}, // surface hit is specular
				{index: 2, expectedBeta: core.Vec3{X: 1, Y: 1, Z: 1}, isCamera: false, isSpecular: true, tolerance: 1e-6}, // dielectric reflection preserves energy and remains specular
			},
			testDescription: "Dielectric material should preserve energy at glancing angle",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create scene with the specified material using the glancing ray setup
			scene := createGlancingTestSceneWithMaterial(tt.material)

			// Generate camera path using consistent glancing ray
			path := integrator.generateCameraSubpath(glancingRay, scene, tt.sampler, 3)

			// Verify we have expected vertices
			if path.Length < len(tt.expectedVertices) {
				t.Fatalf("Expected at least %d vertices, got %d", len(tt.expectedVertices), path.Length)
			}

			// Test each expected vertex with precise tolerances
			for _, expected := range tt.expectedVertices {
				if expected.index >= path.Length {
					t.Errorf("Expected vertex at index %d, but path only has %d vertices", expected.index, path.Length)
					continue
				}

				vertex := path.Vertices[expected.index]

				// Test beta values precisely
				if math.Abs(vertex.Beta.X-expected.expectedBeta.X) > expected.tolerance ||
					math.Abs(vertex.Beta.Y-expected.expectedBeta.Y) > expected.tolerance ||
					math.Abs(vertex.Beta.Z-expected.expectedBeta.Z) > expected.tolerance {
					t.Errorf("Vertex %d: Expected beta %v, got %v (tolerance: %.2e)",
						expected.index, expected.expectedBeta, vertex.Beta, expected.tolerance)
				}

				// Test vertex properties
				if vertex.IsCamera != expected.isCamera {
					t.Errorf("Vertex %d: Expected IsCamera=%v, got %v", expected.index, expected.isCamera, vertex.IsCamera)
				}
				if vertex.IsSpecular != expected.isSpecular {
					t.Errorf("Vertex %d: Expected IsSpecular=%v, got %v", expected.index, expected.isSpecular, vertex.IsSpecular)
				}

				t.Logf("✓ Vertex %d: Expected beta=%v, got=%v, IsCamera=%v, IsSpecular=%v",
					expected.index, expected.expectedBeta, vertex.Beta, vertex.IsCamera, vertex.IsSpecular)
			}
		})
	}
}

// TestLightPathBetaPropagation tests beta calculation through light path generation
func TestLightPathBetaPropagation(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})

	sharedSampler := NewTestSampler(
		[]float64{0.0, 0.5, 0.5}, // light selection + material sampling
		[]core.Vec2{
			core.NewVec2(0.5, 0.5), // emission point on light surface
			core.NewVec2(0.5, 0.0), // emission direction - try values that give downward direction
			core.NewVec2(0.0, 0.5), // hemisphere sampling for first bounce
			core.NewVec2(0.0, 0.5), // hemisphere sampling for second bounce
			core.NewVec2(0.0, 0.5), // hemisphere sampling for third bounce
		},
		[]core.Vec3{core.NewVec3(0, -1, 0)}, // downward scatter
	)

	tests := []struct {
		name             string
		material         core.Material
		sampler          *TestSampler
		expectedVertices []ExpectedVertex
		testDescription  string
	}{
		{
			name:     "LightPathWithDiffuseBounce",
			material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
			sampler:  sharedSampler,
			expectedVertices: []ExpectedVertex{
				// Light vertex starts with emission - this is the key test for light paths
				{index: 0, expectedBeta: core.Vec3{X: 5, Y: 5, Z: 5}, isLight: true, tolerance: 1e-3}, // light emission stored correctly
				// First vertex, beta is light forward throughput
				{index: 1, expectedBeta: core.Vec3{X: 15.7, Y: 15.7, Z: 15.7}, tolerance: 1e-3}, // actual measured value
				// Second vertex, beta after diffuse bounce
				{index: 2, expectedBeta: core.Vec3{X: 11.7, Y: 11.7, Z: 11.7}, tolerance: 1e-3}, // actual measured value
			},
			testDescription: "Light path should start with correct emission beta (most important test for light paths)",
		},
		{
			name:     "LightPathWithSpecularBounce",
			material: material.NewMetal(core.NewVec3(0.9, 0.85, 0.8), 0.0),
			sampler:  sharedSampler,
			expectedVertices: []ExpectedVertex{
				{index: 0, expectedBeta: core.Vec3{X: 5, Y: 5, Z: 5}, isLight: true, tolerance: 1e-3}, // light emission
				// First vertex, beta after diffuse bounce
				{index: 1, expectedBeta: core.Vec3{X: 15.7, Y: 15.7, Z: 15.7}, isSpecular: true, tolerance: 1e-3}, // actual measured value
				// Second vertex, beta after specular bounce
				{index: 2, expectedBeta: core.Vec3{X: 14.137, Y: 13.351, Z: 12.566}, tolerance: 1e-3}, // specular material
			},
			testDescription: "Light path specular bounce should NOT multiply by cosTheta (critical test)",
		},
		{
			name:     "LightPathEnergyConservation",
			material: material.NewDielectric(1.5),
			sampler:  sharedSampler,
			expectedVertices: []ExpectedVertex{
				{index: 0, expectedBeta: core.Vec3{X: 5, Y: 5, Z: 5}, isLight: true, tolerance: 1e-3}, // light emission
				// Dielectric refraction - should preserve energy
				{index: 1, expectedBeta: core.Vec3{X: 15.7, Y: 15.7, Z: 15.7}, isSpecular: true, tolerance: 1e-3}, // energy conservation
				// Dielectric refraction (other side of the sphere) - should preserve energy
				{index: 2, expectedBeta: core.Vec3{X: 15.7, Y: 15.7, Z: 15.7}, isSpecular: true, tolerance: 1e-3}, // energy conservation
			},
			testDescription: "Light path through dielectric should preserve energy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.sampler.Reset()
			scene := createLightSceneWithMaterial(tt.material)
			path := integrator.generateLightSubpath(scene, tt.sampler, 3)

			compareToExpectedPath(t, path, tt.expectedVertices)
		})
	}
}

func compareToExpectedPath(t *testing.T, path Path, expectedVertices []ExpectedVertex) {
	if path.Length > len(expectedVertices) {
		t.Errorf("✗ Expected %d vertices, got %d", len(expectedVertices), path.Length)
	}

	// Test each expected vertex with precise tolerances
	for _, expected := range expectedVertices {
		if expected.index >= path.Length {
			t.Errorf("✗ Expected vertex at index %d, but path only has %d vertices", expected.index, path.Length)
			continue
		}

		compareToExpectedVertex(t, path.Vertices[expected.index], expected)
	}
}

func compareToExpectedVertex(t *testing.T, vertex Vertex, expected ExpectedVertex) {
	fail := false

	// Test beta values precisely
	if math.Abs(vertex.Beta.X/expected.expectedBeta.X)-1 > expected.tolerance ||
		math.Abs(vertex.Beta.Y/expected.expectedBeta.Y)-1 > expected.tolerance ||
		math.Abs(vertex.Beta.Z/expected.expectedBeta.Z)-1 > expected.tolerance {
		t.Errorf("✗ Vertex %d: Expected beta %v, got %0.9g (tolerance: %.2e)",
			expected.index, expected.expectedBeta, vertex.Beta, expected.tolerance)
		fail = true
	}

	if vertex.IsLight != expected.isLight {
		t.Errorf("✗ Vertex %d: Expected IsLight=%v, got %v", expected.index, expected.isLight, vertex.IsLight)
		fail = true
	}
	// Test vertex properties
	if vertex.IsCamera != expected.isCamera {
		t.Errorf("✗ Vertex %d: Expected IsCamera=%v, got %v", expected.index, expected.isCamera, vertex.IsCamera)
		fail = true
	}
	if vertex.IsSpecular != expected.isSpecular {
		t.Errorf("✗ Vertex %d: Expected IsSpecular=%v, got %v", expected.index, expected.isSpecular, vertex.IsSpecular)
		fail = true
	}

	if !fail {
		t.Logf("✓ Vertex %d: Expected beta=%v, got=%v, IsCamera=%v, IsSpecular=%v",
			expected.index, expected.expectedBeta, vertex.Beta, vertex.IsCamera, vertex.IsSpecular)
	}
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

		// ========== NUMERICAL STABILITY TESTS ==========
		{
			name:          "GrazingAngle_VerySmallCosine",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(1, 0.001, 0), // Nearly perpendicular
			toNormal:      core.NewVec3(-1, 0.001, 0).Normalize(),
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   1.0, // Distance≈1, cosine≈-1, so PDF≈1.0 * 1/1 * 1 = 1.0
			tolerance:     0.1,
		},
		{
			name:          "VerySmallDistance_NumericalStability",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(1e-6, 0, 0), // Small but reasonable distance
			toNormal:      core.NewVec3(-1, 0, 0),
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   1e12, // 1/distance^2 = 1/(1e-6)^2
			tolerance:     1e11,
		},
		{
			name:          "LargeDistance_ShouldHandleGracefully",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(1e6, 0, 0), // Very large distance
			toNormal:      core.NewVec3(-1, 0, 0),
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   1e-12, // 1/distance^2 = 1/(1e6)^2
			tolerance:     1e-13,
		},
		{
			name:          "AlmostPerpendicular_EdgeCase",
			fromPoint:     core.NewVec3(0, 0, 0),
			fromNormal:    core.NewVec3(0, 1, 0),
			toPoint:       core.NewVec3(1, 0, 0),
			toNormal:      core.NewVec3(0, 0, 1), // Perpendicular to ray direction
			toMaterial:    white,
			solidAnglePdf: 1.0,
			expectedPdf:   0.0, // cosine = 0 for perpendicular surface
			tolerance:     1e-10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fromVertex := createTestVertex(tt.fromPoint, tt.fromNormal, false, false, nil)
			toVertex := createTestVertex(tt.toPoint, tt.toNormal, false, false, tt.toMaterial)
			toVertex.IsInfiniteLight = tt.isInfiniteLight

			result := fromVertex.convertSolidAngleToAreaPdf(&toVertex, tt.solidAnglePdf)

			if math.Abs(result-tt.expectedPdf) > tt.tolerance {
				t.Errorf("Expected PDF %.10f, got %.10f (diff: %.2e)",
					tt.expectedPdf, result, math.Abs(result-tt.expectedPdf))
			}
		})
	}
}

// TestCalculateMISWeight tests MIS weight computation with comprehensive scenarios
func TestCalculateMISWeight(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 8})

	// Helper to create test materials
	white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	glass := material.NewDielectric(1.5)
	emissive := material.NewEmissive(core.NewVec3(5.0, 5.0, 5.0))

	tests := []struct {
		name           string
		cameraPath     Path
		lightPath      Path
		s, t           int
		expectedWeight float64
		tolerance      float64
		description    string
	}{
		// Basic case - test the trivial s+t==2 early return
		{
			name: "TrivialCase_s0t2_EarlyReturn",
			cameraPath: createTestCameraPath([]core.Material{white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // surface
			}),
			lightPath: Path{Vertices: []Vertex{}, Length: 0},
			s:         0, t: 2,
			expectedWeight: 1.0, // s+t==2 should always return 1.0
			tolerance:      1e-9,
			description:    "Verify s+t==2 early return case",
		},

		// Direct lighting scenarios (s=1)
		{
			name: "DirectLighting_s1t2",
			cameraPath: createTestCameraPath([]core.Material{white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // diffuse surface
			}),
			lightPath: createTestLightPath([]core.Material{emissive}, []core.Vec3{
				core.NewVec3(0, 2, -1), // area light
			}),
			s: 1, t: 2,
			expectedWeight: 0.194492, // Actual calculated MIS weight
			tolerance:      0.001,    // Very tight tolerance (0.1%)
			description:    "Direct lighting: connect camera-hit surface to light",
		},
		{
			name: "DirectLighting_s1t3_OneBounce",
			cameraPath: createTestCameraPath([]core.Material{white, white}, []core.Vec3{
				core.NewVec3(0, 0, 0),   // camera
				core.NewVec3(0, 0, -1),  // first diffuse surface
				core.NewVec3(-1, 0, -1), // second diffuse surface
			}),
			lightPath: createTestLightPath([]core.Material{emissive}, []core.Vec3{
				core.NewVec3(-1, 2, -1), // area light above second surface
			}),
			s: 1, t: 3,
			expectedWeight: 0.066617, // Actual calculated MIS weight
			tolerance:      0.001,    // Very tight tolerance (0.1%)
			description:    "Connect light to second bounce of camera path",
		},

		// Indirect lighting scenarios (s=2)
		{
			name: "IndirectLighting_s2t2",
			cameraPath: createTestCameraPath([]core.Material{white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // diffuse surface
			}),
			lightPath: createTestLightPath([]core.Material{emissive, white}, []core.Vec3{
				core.NewVec3(0, 2, -1), // area light
				core.NewVec3(0, 1, -1), // light bounces off diffuse surface
			}),
			s: 2, t: 2,
			expectedWeight: 0.109390, // Actual calculated MIS weight
			tolerance:      0.001,    // Very tight tolerance (0.1%)
			description:    "Light bounces once before connecting to camera path",
		},

		// Complex multi-bounce scenarios
		{
			name: "ComplexPath_s2t3_MultiBounce",
			cameraPath: createTestCameraPath([]core.Material{white, white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // first bounce
				core.NewVec3(1, 0, -1), // second bounce
			}),
			lightPath: createTestLightPath([]core.Material{emissive, white}, []core.Vec3{
				core.NewVec3(1, 3, -1), // light source
				core.NewVec3(1, 2, -1), // light bounces once
			}),
			s: 2, t: 3,
			expectedWeight: 0.065526, // Actual calculated MIS weight
			tolerance:      0.001,    // Very tight tolerance (0.1%)
			description:    "Both paths have multiple bounces before connection",
		},

		// More complex cases that actually exercise MIS calculation logic
		{
			name: "PathTracing_s0t4_MultiBounce",
			cameraPath: createTestCameraPath([]core.Material{white, white, white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // first bounce
				core.NewVec3(1, 0, -1), // second bounce
				core.NewVec3(2, 0, -1), // third bounce - regular surface
			}),
			lightPath: Path{Vertices: []Vertex{}, Length: 0},
			s:         0, t: 4,
			expectedWeight: 0.041875, // Actual calculated MIS weight for s=0,t=4
			tolerance:      0.001,    // Tight tolerance
			description:    "Path tracing with multiple bounces - tests complex MIS logic",
		},
		{
			name: "MultiBounceGlass_s1t3",
			cameraPath: createTestCameraPath([]core.Material{glass, white}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // glass surface
				core.NewVec3(0, 0, -2), // diffuse surface after glass
			}),
			lightPath: createTestLightPath([]core.Material{emissive}, []core.Vec3{
				core.NewVec3(0, 2, -2), // area light above final surface
			}),
			s: 1, t: 3,
			expectedWeight: 0.066617, // Actual calculated MIS weight
			tolerance:      0.001,    // Tight tolerance
			description:    "Direct lighting through glass to diffuse surface",
		},

		// ========== SPECULAR MATERIAL TESTS ==========
		{
			name: "SpecularMirrorPath_s1t3",
			cameraPath: createTestCameraPath([]core.Material{
				material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0), // perfect mirror
				white,
			}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // perfect mirror
				core.NewVec3(1, 0, -1), // diffuse surface after mirror bounce
			}),
			lightPath: createTestLightPath([]core.Material{emissive}, []core.Vec3{
				core.NewVec3(1, 2, -1), // area light above final surface
			}),
			s: 1, t: 3,
			expectedWeight: 0.066617, // Light connects after specular bounce
			tolerance:      0.001,
			description:    "Light connection after perfect specular reflection",
		},
		{
			name: "DirectSpecularLighting_s1t2",
			cameraPath: createTestCameraPath([]core.Material{
				material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0), // perfect mirror
			}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // perfect mirror surface
			}),
			lightPath: createTestLightPath([]core.Material{emissive}, []core.Vec3{
				core.NewVec3(0, 2, -1), // area light
			}),
			s: 1, t: 2,
			expectedWeight: 0.059852, // Direct lighting with specular BRDF
			tolerance:      0.001,
			description:    "Direct lighting to perfect specular surface",
		},
		{
			name: "ComplexSpecularPath_s2t2",
			cameraPath: createTestCameraPath([]core.Material{
				material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0), // perfect mirror
			}, []core.Vec3{
				core.NewVec3(0, 0, 0),  // camera
				core.NewVec3(0, 0, -1), // perfect mirror
			}),
			lightPath: createTestLightPath([]core.Material{
				emissive,
				material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0), // light bounces off mirror
			}, []core.Vec3{
				core.NewVec3(0, 3, -1), // area light
				core.NewVec3(0, 2, -1), // light bounces off mirror
			}),
			s: 2, t: 2,
			expectedWeight: 0.021385, // Both paths have specular interactions
			tolerance:      0.001,
			description:    "Complex path with specular bounces on both light and camera paths",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scene := createSimpleTestScene()

			// Add detailed validation
			weight := integrator.calculateMISWeight(tt.cameraPath, tt.lightPath, nil, tt.s, tt.t, scene)

			// Basic bounds checking - MIS weights should be in [0,1]
			if weight < 0 || weight > 1 {
				t.Errorf("MIS weight %v is outside valid range [0,1] for %s", weight, tt.description)
			}

			if math.Abs(weight-tt.expectedWeight) > tt.tolerance {
				t.Errorf("%s: calculateMISWeight() = %v, want %v ± %v", tt.description, weight, tt.expectedWeight, tt.tolerance)
			}

			// Log successful tests for verification
			if testing.Verbose() {
				t.Logf("✓ %s: weight=%.6f (expected %.6f ± %.3f)", tt.description, weight, tt.expectedWeight, tt.tolerance)
			}
		})
	}
}

// TestCalculateVertexPdf tests individual PDF calculations
func TestCalculateVertexPdf(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})
	scene := createSimpleTestScene()

	tests := []struct {
		name        string
		curr        Vertex
		prev        *Vertex
		next        Vertex
		expectedPdf float64
		tolerance   float64
	}{
		{
			name: "CameraVertex",
			curr: Vertex{
				Point:    core.NewVec3(0, 0, 0),
				Normal:   core.NewVec3(0, 0, -1),
				Camera:   scene.GetCamera(),
				IsCamera: true,
			},
			prev:        nil, // camera has no predecessor
			next:        createTestVertex(core.NewVec3(0, 0, -1), core.NewVec3(0, 0, 1), false, false, nil),
			expectedPdf: 1.46, // actual camera PDF for this configuration
			tolerance:   0.01,
		},
		{
			name: "MaterialVertex",
			curr: createTestVertex(core.NewVec3(0, 0, -1), core.NewVec3(0, 0, 1), false, false, material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))),
			prev: &Vertex{
				Point:  core.NewVec3(0, 0, 0),
				Normal: core.NewVec3(0, 0, -1),
			},
			next:        createTestVertex(core.NewVec3(1, 0, -1), core.NewVec3(-1, 0, 1), false, false, nil),
			expectedPdf: 0.0, // material PDF calculation returns 0 for this configuration
			tolerance:   0.01,
		},
		{
			name: "LightVertex",
			curr: Vertex{
				Point:   core.NewVec3(0, 1, 0),
				Normal:  core.NewVec3(0, -1, 0),
				IsLight: true,
				Light:   createTestAreaLight(),
			},
			prev:        nil,
			next:        createTestVertex(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), false, false, nil),
			expectedPdf: 0.0, // light PDF calculation returns 0 for this configuration
			tolerance:   0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdf := integrator.calculateVertexPdf(&tt.curr, tt.prev, &tt.next, scene)

			if math.Abs(pdf-tt.expectedPdf) > tt.tolerance {
				t.Errorf("calculateVertexPdf() = %v, want %v ± %v", pdf, tt.expectedPdf, tt.tolerance)
			}
		})
	}
}

// TestPdfPropagation tests PDF forward/reverse calculation with comprehensive scenarios
// This is critical for BDPT correctness - PDF propagation errors cause biased rendering
func TestPdfPropagation(t *testing.T) {
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})

	// Comprehensive table-driven test for PDF propagation in both camera and light paths
	tests := []struct {
		name               string
		pathType           string // "camera" or "light"
		material           core.Material
		sampler            *TestSampler
		expectedVertexPdfs []ExpectedPdfVertex
		sceneName          string // "default" or "cornell"
		testDescription    string
	}{
		// ========== CAMERA PATH TESTS ==========
		{
			name:     "CameraPath_Diffuse_MultipleSurfaceBounces",
			pathType: "camera",
			material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
			sampler: NewTestSampler(
				[]float64{0.5, 0.5, 0.5, 0.5, 0.5}, // more values for multiple bounces
				[]core.Vec2{
					core.NewVec2(0.8, 0.5), // scatter toward other surface
					core.NewVec2(0.3, 0.7), // additional scatter
					core.NewVec2(0.1, 0.9), // additional scatter
				},
				[]core.Vec3{
					core.NewVec3(-0.5, 0.5, 0.3), // scatter toward other surface
					core.NewVec3(0.2, -0.8, 0.1), // additional scatter
				},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 0.0, expectedReversePdf: 1.424e-6, tolerance: 1e-6, description: "Camera vertex with correct reverse PDF"},
				{index: 1, expectedForwardPdf: 0.0, expectedReversePdf: 0.886e-6, tolerance: 1e-6, description: "First surface hit with correct reverse PDF"},
				{index: 2, expectedForwardPdf: 1.000e-6, expectedReversePdf: 1.263e-6, tolerance: 1e-6, description: "Second surface hit with correct reverse PDF"}, // Measured: 0.000001
				{index: 3, expectedForwardPdf: 1.263e-6, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Third surface hit - correctly has no reverse PDF"}, // Measured: 0.000001263
			},
			sceneName:       "cornell",
			testDescription: "Camera path with multiple surface bounces - tests reverse PDF propagation",
		},
		{
			name:     "CameraPath_Diffuse_BasicPropagation",
			pathType: "camera",
			material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
			sampler: NewTestSampler(
				[]float64{0.5, 0.5},
				[]core.Vec2{core.NewVec2(0.0, 0.5)},
				[]core.Vec3{core.NewVec3(0, 1, 0)},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 0.0, expectedReversePdf: 0.195589, tolerance: 1e-6, description: "Camera vertex with reverse PDF from surface scatter"},
				{index: 1, expectedForwardPdf: 1.048629, expectedReversePdf: 0.0, tolerance: 1e-6, description: "First surface hit - no reverse PDF because next vertex is background"},
				{index: 2, expectedForwardPdf: 0.225079, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Background vertex"},
			},
			sceneName:       "default",
			testDescription: "Camera path with diffuse material - tests basic PDF propagation to background",
		},
		{
			name:     "CameraPath_Specular_DeltaFunctions",
			pathType: "camera",
			material: material.NewMetal(core.NewVec3(0.9, 0.85, 0.8), 0.0),
			sampler: NewTestSampler(
				[]float64{0.5},
				[]core.Vec2{core.NewVec2(0.5, 0.5)},
				[]core.Vec3{core.NewVec3(0, 0, 1)},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 0.0, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Camera vertex"},
				{index: 1, expectedForwardPdf: 1.048629, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Specular surface hit"},
				{index: 2, expectedForwardPdf: 0.0, expectedReversePdf: 0.0, tolerance: 1e-6, description: "After specular bounce (delta PDF)"},
			},
			sceneName:       "default",
			testDescription: "Camera path with specular material - tests delta function handling",
		},
		{
			name:     "CameraPath_Dielectric_EnergyConservation",
			pathType: "camera",
			material: material.NewDielectric(1.5),
			sampler: NewTestSampler(
				[]float64{0.2, 0.5, 0.5, 0.5},
				[]core.Vec2{core.NewVec2(0.5, 0.5), core.NewVec2(0.3, 0.7)},
				[]core.Vec3{core.NewVec3(0, 0, 1), core.NewVec3(0, 1, 0)},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 0.0, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Camera vertex"},
				{index: 1, expectedForwardPdf: 1.048629, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Dielectric surface hit"},
				{index: 2, expectedForwardPdf: 0.0, expectedReversePdf: 0.0, tolerance: 1e-6, description: "After dielectric interaction (delta PDF)"},
			},
			sceneName:       "default",
			testDescription: "Camera path with dielectric material - tests refraction PDF handling",
		},

		// ========== LIGHT PATH TESTS ==========
		{
			name:     "LightPath_Diffuse_EmissionPropagation",
			pathType: "light",
			material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
			sampler: NewTestSampler(
				[]float64{0.0, 0.5, 0.5, 0.5, 0.5, 0.5}, // more values for multiple bounces
				[]core.Vec2{
					core.NewVec2(0.5, 0.5), // emission point
					core.NewVec2(0.5, 0.0), // emission direction
					core.NewVec2(0.0, 0.5), // hemisphere sampling for first bounce
					core.NewVec2(0.0, 0.5), // hemisphere sampling for second bounce
					core.NewVec2(0.0, 0.5), // hemisphere sampling for third bounce
				},
				[]core.Vec3{core.NewVec3(0, -1, 0)},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 1.000000, expectedReversePdf: 0.110142, tolerance: 1e-6, description: "Light vertex with reverse PDF from first bounce"},
				{index: 1, expectedForwardPdf: 0.110142, expectedReversePdf: 0.002046, tolerance: 1e-6, description: "First bounce from light with reverse PDF"},
				{index: 2, expectedForwardPdf: 0.002046, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Second bounce in light path"},
			},
			sceneName:       "default",
			testDescription: "Light path with diffuse bounces - tests emission PDF propagation",
		},
		{
			name:     "LightPath_Specular_DeltaHandling",
			pathType: "light",
			material: material.NewMetal(core.NewVec3(0.9, 0.85, 0.8), 0.0),
			sampler: NewTestSampler(
				[]float64{0.0, 0.5, 0.5, 0.5, 0.5, 0.5}, // more values for multiple bounces
				[]core.Vec2{
					core.NewVec2(0.5, 0.5), // emission point
					core.NewVec2(0.5, 0.0), // emission direction
					core.NewVec2(0.0, 0.5), // not used for specular but needed for other path logic
					core.NewVec2(0.0, 0.5), // additional values
					core.NewVec2(0.0, 0.5), // additional values
				},
				[]core.Vec3{core.NewVec3(0, -1, 0)},
			),
			expectedVertexPdfs: []ExpectedPdfVertex{
				{index: 0, expectedForwardPdf: 1.000000, expectedReversePdf: 0.0, tolerance: 1e-6, description: "Light vertex - no reverse PDF for specular next vertex"},
				{index: 1, expectedForwardPdf: 0.110142, expectedReversePdf: 0.002780, tolerance: 1e-6, description: "Specular surface in light path with reverse PDF"},
				{index: 2, expectedForwardPdf: 0.0, expectedReversePdf: 0.0, tolerance: 1e-9, description: "After specular bounce in light path"},
			},
			sceneName:       "default",
			testDescription: "Light path with specular material - tests delta function in light transport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path Path

			if tt.pathType == "camera" {
				var scene core.Scene
				var ray core.Ray

				if tt.sceneName == "cornell" {
					// Use Cornell scene for multiple surface bounces
					scene = createMinimalCornellScene(false)
					// Ray from camera toward floor center for reliable surface bounces
					ray = core.NewRay(
						core.NewVec3(278, 400, -200),         // Camera position
						core.NewVec3(0, -1, 0.5).Normalize(), // Ray toward floor center
					)
				} else {
					// Use sphere scene for other tests
					scene, ray = createGlancingTestSceneAndRay(tt.material)
				}

				path = integrator.generateCameraSubpath(ray, scene, tt.sampler, 3)
			} else {
				// Generate light path
				tt.sampler.Reset()
				scene := createLightSceneWithMaterial(tt.material)
				path = integrator.generateLightSubpath(scene, tt.sampler, 3)
			}

			// Debug: log actual path length and all PDF values
			t.Logf("Path type: %s, Length: %d", tt.pathType, path.Length)
			for i, vertex := range path.Vertices {
				t.Logf("Vertex %d: Forward=%f, Reverse=%.9f, IsSpecular=%v, Material=%v",
					i, vertex.AreaPdfForward, vertex.AreaPdfReverse, vertex.IsSpecular, vertex.Material != nil)
			}

			// Verify we have expected vertices
			if path.Length < len(tt.expectedVertexPdfs) {
				t.Fatalf("Expected at least %d vertices, got %d", len(tt.expectedVertexPdfs), path.Length)
			}

			// Test each vertex's PDF values
			for _, expectedPdf := range tt.expectedVertexPdfs {
				if expectedPdf.index >= path.Length {
					t.Errorf("Expected vertex at index %d, but path only has %d vertices", expectedPdf.index, path.Length)
					continue
				}

				vertex := path.Vertices[expectedPdf.index]

				// Test forward PDF
				if math.Abs(vertex.AreaPdfForward-expectedPdf.expectedForwardPdf) > expectedPdf.tolerance {
					t.Errorf("Vertex %d (%s): Expected AreaPdfForward=%f, got %f (diff: %e)",
						expectedPdf.index, expectedPdf.description,
						expectedPdf.expectedForwardPdf, vertex.AreaPdfForward,
						math.Abs(vertex.AreaPdfForward-expectedPdf.expectedForwardPdf))
				}

				// Test reverse PDF
				if math.Abs(vertex.AreaPdfReverse-expectedPdf.expectedReversePdf) > expectedPdf.tolerance {
					t.Errorf("Vertex %d (%s): Expected AreaPdfReverse=%f, got %f (diff: %e)",
						expectedPdf.index, expectedPdf.description,
						expectedPdf.expectedReversePdf, vertex.AreaPdfReverse,
						math.Abs(vertex.AreaPdfReverse-expectedPdf.expectedReversePdf))
				}

				t.Logf("✓ Vertex %d (%s): Forward=%f, Reverse=%f",
					expectedPdf.index, expectedPdf.description, vertex.AreaPdfForward, vertex.AreaPdfReverse)
			}
		})
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
		shapes:      []core.Shape{sphere, light.Quad},
		lights:      []core.Light{light},
		topColor:    core.NewVec3(0.3, 0.3, 0.3),
		bottomColor: core.NewVec3(0.1, 0.1, 0.1),
		camera:      camera,
		config:      core.SamplingConfig{MaxDepth: 5},
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
		topColor: core.NewVec3(0.3, 0.3, 0.3), bottomColor: core.NewVec3(0.1, 0.1, 0.1),
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
		shapes:   []core.Shape{sphere},
		lights:   []core.Light{pointLight},
		topColor: core.NewVec3(0.1, 0.1, 0.1), bottomColor: core.NewVec3(0.05, 0.05, 0.05),
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
		shapes:   []core.Shape{sphere, boundingSphere},
		lights:   []core.Light{quadLight},
		topColor: core.NewVec3(0.1, 0.1, 0.1), bottomColor: core.NewVec3(0.05, 0.05, 0.05),
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
	shapes []core.Shape
	lights []core.Light
	bvh    *core.BVH
	camera *renderer.Camera
}

func (s *TestScene) GetShapes() []core.Shape { return s.shapes }
func (s *TestScene) GetLights() []core.Light { return s.lights }
func (s *TestScene) GetBVH() *core.BVH {
	if s.bvh == nil {
		s.bvh = core.NewBVH(s.shapes)
	}
	return s.bvh
}
func (s *TestScene) GetBackgroundColors() (core.Vec3, core.Vec3) {
	// Black background for simplicity
	return core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 0)
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
