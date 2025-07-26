package integrator

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

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
	integrator := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})

	// Use existing scene creation helper
	scene, _ := createGlancingTestSceneAndRay(material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)))

	// Helper to create camera path with specified vertices
	createCameraPath := func(vertices []Vertex) Path {
		path := Path{Vertices: vertices, Length: len(vertices)}
		return path
	}

	// Helper to create light path with specified vertices
	createLightPath := func(vertices []Vertex) Path {
		path := Path{Vertices: vertices, Length: len(vertices)}
		return path
	}

	tests := []struct {
		name                 string
		cameraPath           Path
		lightPath            Path
		s                    int // light path index (1-based)
		t                    int // camera path index (1-based)
		expectedContribution core.Vec3
		expectZero           bool
		tolerance            float64
		testDescription      string
	}{
		{
			name: "InvalidIndices_s_TooSmall",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true},
				{Point: core.NewVec3(0, 0, -1), Material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))},
			}),
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 1, -1), IsLight: true},
			}),
			s:               0, // s < 1 should return zero
			t:               1,
			expectZero:      true,
			testDescription: "s < 1 should return zero contribution",
		},
		{
			name: "InvalidIndices_t_TooSmall",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true},
			}),
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 1, -1), IsLight: true},
			}),
			s:               1,
			t:               0, // t < 1 should return zero
			expectZero:      true,
			testDescription: "t < 1 should return zero contribution",
		},
		{
			name: "InvalidIndices_s_TooLarge",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true},
				{Point: core.NewVec3(0, 0, -1), Material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))},
			}),
			lightPath: createLightPath([]Vertex{
				{Point: core.NewVec3(0, 1, -1), IsLight: true},
			}),
			s:               2, // s > lightPath.Length should return zero
			t:               1,
			expectZero:      true,
			testDescription: "s > lightPath.Length should return zero contribution",
		},
		{
			name: "SpecularVertex_Camera",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true},
				{
					Point:      core.NewVec3(0, 0, -1),
					Normal:     core.NewVec3(0, 0, 1),
					Material:   material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0),
					IsSpecular: true, // Can't connect through delta functions
					Beta:       core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:   core.NewVec3(0, 1, -1),
					Normal:  core.NewVec3(0, -1, 0),
					IsLight: true,
					Beta:    core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			s:               1,
			t:               2,
			expectZero:      true,
			testDescription: "Specular camera vertex should be skipped",
		},
		{
			name: "SpecularVertex_Light",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true},
				{
					Point:    core.NewVec3(0, 0, -1),
					Normal:   core.NewVec3(0, 0, 1),
					Material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
					Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:      core.NewVec3(0, 1, -1),
					Normal:     core.NewVec3(0, -1, 0),
					Material:   material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0),
					IsSpecular: true, // Can't connect through delta functions
					Beta:       core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			s:               1,
			t:               2,
			expectZero:      true,
			testDescription: "Specular light vertex should be skipped",
		},
		{
			name: "ValidConnection_SurfaceToSurface",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true, Beta: core.Vec3{X: 1, Y: 1, Z: 1}},
				{
					Point:    core.NewVec3(5, 5, 5),                               // far from scene geometry
					Normal:   core.NewVec3(0, 1, 0),                               // pointing up toward light (different direction)
					Material: material.NewLambertian(core.NewVec3(0.8, 0.6, 0.4)), // different reflectance
					Beta:     core.Vec3{X: 0.9, Y: 0.7, Z: 0.5},                   // different throughput
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:   core.NewVec3(5, 7, 5),  // directly above camera vertex, distance=2
					Normal:  core.NewVec3(0, -1, 0), // pointing down toward camera vertex
					IsLight: true,
					Beta:    core.Vec3{X: 2, Y: 2, Z: 2}, // different light intensity
				},
			}),
			s:                    1,
			t:                    2,
			expectedContribution: core.NewVec3(0.115, 0.0668, 0.0318), // measured with Y-axis geometry
			tolerance:            1e-3,
			testDescription:      "Valid surface-to-surface connection",
		},
		{
			name: "ValidConnection_LightToSurface",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true, Beta: core.Vec3{X: 1, Y: 1, Z: 1}},
				{
					Point:    core.NewVec3(7, 7, 7),                               // far from scene geometry
					Normal:   core.NewVec3(0, 0, 1),                               // pointing toward light in Z direction
					Material: material.NewLambertian(core.NewVec3(0.5, 0.9, 0.3)), // green-ish reflectance
					Beta:     core.Vec3{X: 0.6, Y: 1.2, Z: 0.4},                   // different throughput pattern
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:   core.NewVec3(7, 7, 9),  // 2 units away in Z direction
					Normal:  core.NewVec3(0, 0, -1), // pointing back toward camera vertex
					IsLight: true,
					Beta:    core.Vec3{X: 1.5, Y: 1.5, Z: 1.5}, // different light intensity
				},
			}),
			s:                    1,
			t:                    2,
			expectedContribution: core.NewVec3(0.0358, 0.129, 0.0143), // measured with Z-axis geometry
			tolerance:            1e-3,
			testDescription:      "Valid light-to-surface connection with Z-axis geometry",
		},
		{
			name: "ValidConnection_LargeDistance",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true, Beta: core.Vec3{X: 1, Y: 1, Z: 1}},
				{
					Point:    core.NewVec3(20, 20, 20),                            // far from scene geometry
					Normal:   core.NewVec3(-1, -1, 0).Normalize(),                 // pointing toward light
					Material: material.NewLambertian(core.NewVec3(0.9, 0.5, 0.1)), // orange reflectance
					Beta:     core.Vec3{X: 2, Y: 1, Z: 0.5},                       // asymmetric throughput
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:   core.NewVec3(15, 15, 20),          // distance=√50≈7.07 units away
					Normal:  core.NewVec3(1, 1, 0).Normalize(), // pointing toward camera vertex
					IsLight: true,
					Beta:    core.Vec3{X: 10, Y: 10, Z: 10}, // bright light
				},
			}),
			s:                    1,
			t:                    2,
			expectedContribution: core.NewVec3(0.115, 0.0318, 0.00318), // measured with large distance
			tolerance:            1e-3,
			testDescription:      "Valid connection with large distance and asymmetric materials",
		},
		{
			name: "DebugConnection_ClearPath",
			cameraPath: createCameraPath([]Vertex{
				{Point: core.NewVec3(0, 0, 0), IsCamera: true, Beta: core.Vec3{X: 1, Y: 1, Z: 1}},
				{
					Point:    core.NewVec3(10, 10, 10), // far from scene geometry
					Normal:   core.NewVec3(1, 0, 0),    // pointing toward light (positive cosine)
					Material: material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)),
					Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			lightPath: createLightPath([]Vertex{
				{
					Point:   core.NewVec3(11, 10, 10), // close to camera vertex, clear line of sight
					Normal:  core.NewVec3(-1, 0, 0),   // pointing toward camera vertex (positive cosine)
					IsLight: true,
					Beta:    core.Vec3{X: 1, Y: 1, Z: 1},
				},
			}),
			s:                    1,
			t:                    2,
			expectedContribution: core.NewVec3(0.223, 0.223, 0.223), // measured from actual output
			tolerance:            1e-3,
			testDescription:      "Debug connection with clear path away from scene geometry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			contribution := integrator.evaluateConnectionStrategy(tt.cameraPath, tt.lightPath, tt.s, tt.t, scene)

			if tt.expectZero {
				if contribution.Luminance() > 1e-10 {
					t.Errorf("%s: Expected zero contribution, got %v", tt.testDescription, contribution)
				}
			} else {
				if contribution.Luminance() <= 1e-10 {
					t.Errorf("%s: Expected non-zero contribution, got %v", tt.testDescription, contribution)
				} else if !isClose(contribution, tt.expectedContribution, tt.tolerance) {
					t.Errorf("%s:\nExpected: %v\nActual: %v\nDifference: %v",
						tt.testDescription,
						tt.expectedContribution,
						contribution,
						contribution.Subtract(tt.expectedContribution))
				}
			}
		})
	}
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
