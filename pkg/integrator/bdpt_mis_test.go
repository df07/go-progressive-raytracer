package integrator

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

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
