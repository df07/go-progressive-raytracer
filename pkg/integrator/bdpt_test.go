package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// TestBDPTvsPathTracingDirectLighting compares BDPT vs path tracing on a simple Cornell setup
// This test isolates the direct lighting issue - BDPT should perform similarly to path tracing
func TestBDPTvsPathTracingDirectLighting(t *testing.T) {
	// Create a minimal Cornell scene: just floor + quad light
	scene := createMinimalCornellScene()

	// Setup a ray hitting the floor center
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),         // Camera position (above and in front)
		core.NewVec3(0, -1, 0.5).Normalize(), // Ray pointing down toward floor center
	)

	// Test both integrators with same random seed for reproducibility
	seed := int64(42)

	// Path tracing result
	pathRandom := rand.New(rand.NewSource(seed))
	pathConfig := core.SamplingConfig{MaxDepth: 5}
	pathIntegrator := NewPathTracingIntegrator(pathConfig)
	pathResult := pathIntegrator.RayColor(rayToFloor, scene, pathRandom, 5, core.NewVec3(1, 1, 1), 0)

	// BDPT result with debug output
	bdptRandom := rand.New(rand.NewSource(seed))
	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate paths for debugging
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, bdptRandom, 5, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, bdptRandom, 5)

	t.Logf("=== DEBUG: Path Generation ===")
	t.Logf("Camera path length: %d", cameraPath.Length)
	for i, vertex := range cameraPath.Vertices {
		t.Logf("  Camera[%d]: pos=%v, IsCamera=%v, IsLight=%v, Material=%v, Throughput=%v",
			i, vertex.Point, vertex.IsCamera, vertex.IsLight, vertex.Material != nil, vertex.Throughput)
	}

	t.Logf("Light path length: %d", lightPath.Length)
	for i, vertex := range lightPath.Vertices {
		t.Logf("  Light[%d]: pos=%v, IsLight=%v, Material=%v, Emission=%v, Throughput=%v",
			i, vertex.Point, vertex.IsLight, vertex.Material != nil, vertex.EmittedLight.Luminance(), vertex.Throughput)
	}

	// Debug individual strategies
	t.Logf("=== DEBUG: Individual Strategy Contributions ===")
	workingStrategies := 0

	for s := 0; s < lightPath.Length; s++ {
		for tVert := 0; tVert < cameraPath.Length; tVert++ {
			contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, s, tVert, scene)

			if contribution.Luminance() > 0 {
				workingStrategies++
				t.Logf("Strategy s=%d,t=%d: contribution=%v (lum: %.6f) - WORKING",
					s, tVert, contribution, contribution.Luminance())

				// Calculate MIS weight for this strategy
				pathPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, s, tVert)
				t.Logf("  -> Path PDF: %.9f", pathPDF)

				// Debug throughputs for key strategies
				if (s == 0 && tVert == 1) || (s == 1 && tVert == 0) || (s == 1 && tVert == 1) {
					cameraThru := bdptIntegrator.calculateCameraPathThroughput(cameraPath, tVert+1)
					lightThru := bdptIntegrator.calculateLightPathThroughput(lightPath, s+1)
					t.Logf("  -> Camera throughput (len %d): %v (lum: %.6f)", tVert+1, cameraThru, cameraThru.Luminance())
					t.Logf("  -> Light throughput (len %d): %v (lum: %.6f)", s+1, lightThru, lightThru.Luminance())
				}
			} else {
				t.Logf("Strategy s=%d,t=%d: ZERO contribution", s, tVert)
			}
		}
	}

	t.Logf("Found %d working strategies out of %d total", workingStrategies, lightPath.Length*cameraPath.Length)

	// Now get the actual BDPT result through evaluateBDPTStrategies (with MIS)
	bdptStrategyResult := bdptIntegrator.evaluateBDPTStrategies(cameraPath, lightPath, scene)
	t.Logf("BDPT strategies combined result: %v (luminance: %.6f)", bdptStrategyResult, bdptStrategyResult.Luminance())

	// Get the final result through RayColor for comparison
	bdptRandom = rand.New(rand.NewSource(seed)) // Reset seed
	bdptResult := bdptIntegrator.RayColor(rayToFloor, scene, bdptRandom, 5, core.NewVec3(1, 1, 1), 0)

	t.Logf("=== FINAL COMPARISON ===")
	t.Logf("Path tracing result: %v (luminance: %.6f)", pathResult, pathResult.Luminance())
	t.Logf("BDPT result: %v (luminance: %.6f)", bdptResult, bdptResult.Luminance())

	// BDPT should not be dramatically darker than path tracing
	// Allow for some variance, but BDPT shouldn't be orders of magnitude dimmer
	pathLuminance := pathResult.Luminance()
	bdptLuminance := bdptResult.Luminance()

	if pathLuminance > 0.001 { // Only test if path tracing found significant light
		ratio := bdptLuminance / pathLuminance
		if ratio < 0.1 { // BDPT is more than 10x dimmer
			t.Errorf("BDPT result too dim compared to path tracing: ratio %.4f (BDPT: %.6f, PT: %.6f)",
				ratio, bdptLuminance, pathLuminance)
		}
		if ratio > 10.0 { // BDPT is more than 10x brighter
			t.Errorf("BDPT result too bright compared to path tracing: ratio %.4f (BDPT: %.6f, PT: %.6f)",
				ratio, bdptLuminance, pathLuminance)
		}
	}
}

// createMinimalCornellScene creates a Cornell scene with walls, floor and quad light
func createMinimalCornellScene() core.Scene {
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

	// Right wall (red)
	rightWall := geometry.NewQuad(
		core.NewVec3(0.0, 0.0, 0.0),
		core.NewVec3(0.0, 0.0, 556),
		core.NewVec3(0.0, 556, 0.0),
		red,
	)

	// Left wall (green)
	leftWall := geometry.NewQuad(
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

	return scene
}

// TestScene is a minimal scene implementation for testing
type TestScene struct {
	shapes []core.Shape
	lights []core.Light
	bvh    *core.BVH
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
	return &TestCamera{} // Minimal camera for testing
}

// TestCamera is a minimal camera implementation for testing
type TestCamera struct{}

func (c *TestCamera) GetRay(i, j int, random *rand.Rand) core.Ray {
	// Return a simple ray for testing
	return core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1))
}

// TestLightPathDirectionAndIntersection verifies that light paths are generated correctly
func TestLightPathDirectionAndIntersection(t *testing.T) {
	scene := createMinimalCornellScene()

	// Generate multiple light paths to test consistency
	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	successfulPaths := 0
	totalPaths := 10

	for i := 0; i < totalPaths; i++ {
		lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

		t.Logf("Light path %d: length=%d", i, lightPath.Length)

		if lightPath.Length == 0 {
			t.Logf("  No light path generated (no lights or invalid sample)")
			continue
		}

		// Check initial light vertex
		lightVertex := lightPath.Vertices[0]
		t.Logf("  Light vertex: pos=%v, normal=%v, outgoing=%v",
			lightVertex.Point, lightVertex.Normal, lightVertex.OutgoingDirection)
		t.Logf("  Light vertex: IsLight=%v, EmittedLight=%v",
			lightVertex.IsLight, lightVertex.EmittedLight)

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
			t.Logf("  Vertex %d: pos=%v, material=%v, IsLight=%v",
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
	scene := createMinimalCornellScene()

	// Create a ray that should hit the light directly
	rayToLight := core.NewRay(
		core.NewVec3(278, 400, 278), // Camera position below light
		core.NewVec3(0, 1, 0),       // Ray pointing straight up toward light
	)

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that should hit the light
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToLight, scene, random, 3, core.NewVec3(1, 1, 1), 0)

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
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)

	// Generate light path
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

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
		// s=0: light source (first vertex in light path)
		// t=1: floor hit (second vertex in camera path, first bounce from camera)
		contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, scene)
		t.Logf("Connection strategy (s=0, t=1) contribution: %v (luminance: %.6f)",
			contribution, contribution.Luminance())

		// Assert: Connection should produce some contribution (this is the key test!)
		if contribution.Luminance() <= 0 {
			t.Errorf("Connection strategy should produce positive contribution when connecting light source to floor hit")
		}
	}
}

// TestBDPTPathPDFPositive tests that path PDFs are positive for valid paths
func TestBDPTPathPDFPositive(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate paths
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

	// Assert: Camera path PDF should be positive
	if cameraPath.Length > 0 {
		pathPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, 0, cameraPath.Length)
		if pathPDF <= 0 {
			t.Errorf("Camera path PDF should be positive, got %.6f", pathPDF)
		}
	}

	// Assert: Light path PDF should be positive
	if lightPath.Length > 0 {
		lightPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, lightPath.Length, 0)
		if lightPDF <= 0 {
			t.Errorf("Light path PDF should be positive, got %.6f", lightPDF)
		}
	}

	// Assert: Connection PDFs should not be negative
	if cameraPath.Length >= 1 && lightPath.Length >= 1 {
		connectionPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, 1, 1)
		if connectionPDF < 0 {
			t.Errorf("Connection PDF should not be negative, got %.6f", connectionPDF)
		}
	}
}

// TestBDPTPathIndexing verifies how paths are indexed in our implementation
func TestBDPTPathIndexing(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)

	// Generate light path
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

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
		contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, scene)
		t.Logf("Connection contribution: %v (luminance: %.6f)", contribution, contribution.Luminance())
	}
}

// TestBDPTvsDirectLightSampling compares BDPT s=0,t=1 with direct light sampling
func TestBDPTvsDirectLightSampling(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

	// Get the floor hit vertex (t=1)
	floorVertex := cameraPath.Vertices[1]
	lightVertex := lightPath.Vertices[0]

	t.Logf("Floor vertex: pos=%v, normal=%v", floorVertex.Point, floorVertex.Normal)
	t.Logf("Light vertex: pos=%v, normal=%v, emission=%v", lightVertex.Point, lightVertex.Normal, lightVertex.EmittedLight)

	// BDPT s=0,t=1 connection
	bdptContribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, scene)
	t.Logf("BDPT s=0,t=1 contribution: %v (luminance: %.6f)", bdptContribution, bdptContribution.Luminance())

	// Verify that BDPT uses correct area measure PDF
	expectedAreaPDF := 1.0 / 13650.0 // Light area for Cornell box quad light
	if math.Abs(lightVertex.ForwardPDF-expectedAreaPDF) > 1e-8 {
		t.Errorf("Light vertex PDF should be area PDF %.9f, got %.9f", expectedAreaPDF, lightVertex.ForwardPDF)
	}

	// Manual direct light sampling calculation (what path tracing would do)
	direction := lightVertex.Point.Subtract(floorVertex.Point)
	distance := direction.Length()
	direction = direction.Multiply(1.0 / distance)

	// Calculate direct lighting contribution
	cosAtFloor := direction.Dot(floorVertex.Normal)
	cosAtLight := direction.Multiply(-1).Dot(lightVertex.Normal)

	// Calculate geometric term for direct lighting
	geometricTerm := (cosAtFloor * cosAtLight) / (distance * distance)

	// Check visibility
	shadowRay := core.NewRay(floorVertex.Point, direction)
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, distance-0.001)

	if !blocked && cosAtFloor > 0 && cosAtLight > 0 {
		// Evaluate BRDF at floor (for direct lighting)
		brdf := bdptIntegrator.evaluateBRDF(floorVertex, direction)

		// Light area (approximate) - our quad light is 130 x 105
		lightArea := 130.0 * 105.0 // From Cornell scene setup
		lightSamplingPDF := 1.0 / lightArea

		// Direct lighting formula: BRDF * emission * geometric_term / light_sampling_PDF
		directContribution := brdf.MultiplyVec(lightVertex.EmittedLight).Multiply(geometricTerm / lightSamplingPDF)

		// Compare BDPT with direct lighting - they should match exactly
		ratio := bdptContribution.Luminance() / directContribution.Luminance()

		if math.Abs(ratio-1.0) > 0.01 {
			t.Errorf("BDPT s=0,t=1 should match direct lighting exactly, got ratio %.6f", ratio)
		}
	}
}

// Test basic light path generation
func TestLightPathGeneration(t *testing.T) {
	// Create a simple scene with a light
	emissiveMaterial := material.NewEmissive(core.NewVec3(1, 1, 1))
	light := geometry.NewSphereLight(core.NewVec3(0, 5, 0), 1.0, emissiveMaterial)

	testScene := &MockScene{
		lights: []core.Light{light},
		shapes: []core.Shape{light},
		config: core.SamplingConfig{MaxDepth: 3},
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	lightPath := integrator.generateLightSubpath(testScene, random, 3)

	if lightPath.Length == 0 {
		t.Error("Expected light path to have at least one vertex")
	}

	if lightPath.Length > 0 {
		firstVertex := lightPath.Vertices[0]
		if !firstVertex.IsLight {
			t.Error("First vertex in light path should be marked as light")
		}

		if firstVertex.EmittedLight.Luminance() <= 0 {
			t.Error("Light vertex should have positive emission")
		}
	}
}

// TestBDPTMISWeighting tests the MIS weighting calculation specifically
func TestBDPTMISWeighting(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor and then bounces to wall
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, 200),          // Position inside Cornell box
		core.NewVec3(0, -1, 0.1).Normalize(), // Ray pointing mostly down with slight Z component
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

	// Test the s=0,t=1 strategy in isolation
	s0t1Contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, scene)
	t.Logf("s=0,t=1 individual contribution: %v (luminance: %.6f)", s0t1Contribution, s0t1Contribution.Luminance())

	// Now test all strategies through evaluateBDPTStrategies
	allStrategies := bdptIntegrator.evaluateBDPTStrategies(cameraPath, lightPath, scene)
	t.Logf("All strategies result: %v (luminance: %.6f)", allStrategies, allStrategies.Luminance())

	// The all-strategies result should be positive since s=0,t=1 works
	if allStrategies.Luminance() <= 0 {
		t.Errorf("All strategies returned zero, but s=0,t=1 works individually")
	}

	// Debug path structures first
	t.Logf("=== Path structures ===")
	t.Logf("Camera path length: %d", cameraPath.Length)
	for i, vertex := range cameraPath.Vertices {
		t.Logf("  Camera[%d]: pos=%v, IsCamera=%v, IsLight=%v, IsSpecular=%v, Material=%v",
			i, vertex.Point, vertex.IsCamera, vertex.IsLight, vertex.IsSpecular, vertex.Material != nil)
	}

	t.Logf("Light path length: %d", lightPath.Length)
	for i, vertex := range lightPath.Vertices {
		t.Logf("  Light[%d]: pos=%v, IsLight=%v, IsSpecular=%v, Material=%v, Emission=%v",
			i, vertex.Point, vertex.IsLight, vertex.IsSpecular, vertex.Material != nil, vertex.EmittedLight.Luminance())
	}

	// Debug all strategies in detail
	t.Logf("=== Debugging all strategies ===")
	workingStrategies := 0
	for s := 0; s < lightPath.Length; s++ {
		for tVert := 0; tVert < cameraPath.Length; tVert++ {
			contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, s, tVert, scene)
			if contribution.Luminance() > 0 {
				workingStrategies++
				t.Logf("Strategy s=%d,t=%d: contribution=%v (lum: %.6f)",
					s, tVert, contribution, contribution.Luminance())
			} else {
				t.Logf("Strategy s=%d,t=%d: ZERO contribution", s, tVert)
			}

			// Debug throughputs for key strategies
			if (s == 0 && tVert == 1) || (s == 1 && tVert == 1) {
				cameraThru := bdptIntegrator.calculateCameraPathThroughput(cameraPath, tVert+1)
				lightThru := bdptIntegrator.calculateLightPathThroughput(lightPath, s+1)
				t.Logf("  -> Camera throughput (len %d): %v (lum: %.6f)", tVert+1, cameraThru, cameraThru.Luminance())
				t.Logf("  -> Light throughput (len %d): %v (lum: %.6f)", s+1, lightThru, lightThru.Luminance())

				// Debug individual vertex throughputs
				if tVert < len(cameraPath.Vertices) {
					t.Logf("  -> Camera vertex[%d] throughput: %v", tVert, cameraPath.Vertices[tVert].Throughput)
				}
				if s < len(lightPath.Vertices) {
					t.Logf("  -> Light vertex[%d] throughput: %v", s, lightPath.Vertices[s].Throughput)
				}
			}
		}
	}
	t.Logf("Found %d working strategies out of %d total", workingStrategies, lightPath.Length*cameraPath.Length)
}

// TestBDPTIndirectLighting tests BDPT with a ray that hits a corner (indirect lighting only)
func TestBDPTIndirectLighting(t *testing.T) {
	scene := createMinimalCornellScene()

	// Ray aimed at back top corner that should only get very indirect lighting
	// The corner (0, 556, 556) is far from the light and requires multiple bounces
	rayToCorner := core.NewRay(
		core.NewVec3(278, 400, 278),             // Camera position in center of room
		core.NewVec3(556, 556, 556).Normalize(), // Ray pointing toward back top corner
	)

	seed := int64(42)

	// Average multiple samples for both integrators to get stable results
	numSamples := 10
	var pathTotal, bdptTotal core.Vec3

	pathConfig := core.SamplingConfig{MaxDepth: 5}
	pathIntegrator := NewPathTracingIntegrator(pathConfig)
	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	for i := 0; i < numSamples; i++ {
		// Path tracing sample
		pathRandom := rand.New(rand.NewSource(seed + int64(i)))
		pathSample := pathIntegrator.RayColor(rayToCorner, scene, pathRandom, 5, core.NewVec3(1, 1, 1), i)
		pathTotal = pathTotal.Add(pathSample)

		// BDPT sample
		bdptRandom := rand.New(rand.NewSource(seed + int64(i)))
		bdptSample := bdptIntegrator.RayColor(rayToCorner, scene, bdptRandom, 5, core.NewVec3(1, 1, 1), i)
		bdptTotal = bdptTotal.Add(bdptSample)
	}

	pathResult := pathTotal.Multiply(1.0 / float64(numSamples))
	bdptResult := bdptTotal.Multiply(1.0 / float64(numSamples))

	// Also do single sample debug with specific seed
	bdptRandom := rand.New(rand.NewSource(seed))
	bdptIntegratorDebug := NewBDPTIntegrator(bdptConfig)

	// Generate paths for debugging
	cameraPath := bdptIntegratorDebug.generateCameraSubpath(rayToCorner, scene, bdptRandom, 5, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegratorDebug.generateLightSubpath(scene, bdptRandom, 5)

	t.Logf("=== DEBUG: Corner Lighting Path Generation ===")
	t.Logf("Camera path length: %d", cameraPath.Length)
	for i, vertex := range cameraPath.Vertices {
		t.Logf("  Camera[%d]: pos=%v, IsCamera=%v, IsLight=%v, Material=%v, Throughput=%v",
			i, vertex.Point, vertex.IsCamera, vertex.IsLight, vertex.Material != nil, vertex.Throughput)
	}

	t.Logf("Light path length: %d", lightPath.Length)
	for i, vertex := range lightPath.Vertices {
		t.Logf("  Light[%d]: pos=%v, IsLight=%v, Material=%v, Emission=%v, Throughput=%v",
			i, vertex.Point, vertex.IsLight, vertex.Material != nil, vertex.EmittedLight.Luminance(), vertex.Throughput)
	}

	// Debug individual strategies - focus on indirect lighting strategies
	t.Logf("=== DEBUG: Indirect Lighting Strategy Contributions ===")
	workingStrategies := 0

	for s := 0; s < lightPath.Length && s < 3; s++ {
		for tVert := 0; tVert < cameraPath.Length && tVert < 3; tVert++ {
			strategyType := "DIRECT"
			if s > 0 {
				strategyType = "INDIRECT"
			} else if tVert > 1 {
				strategyType = "MULTI-BOUNCE" // s=0,t>1 can be indirect via camera bounces
			}

			contribution := bdptIntegratorDebug.evaluateConnectionStrategy(cameraPath, lightPath, s, tVert, scene)

			if contribution.Luminance() > 0 {
				workingStrategies++
				t.Logf("Strategy s=%d,t=%d (%s): contribution=%v (lum: %.9f) - WORKING",
					s, tVert, strategyType, contribution, contribution.Luminance())

				// Calculate MIS weight for this strategy
				pathPDF := bdptIntegratorDebug.calculatePathPDF(cameraPath, lightPath, s, tVert)
				t.Logf("  -> Path PDF: %.12f", pathPDF)

				// Debug throughputs for important strategies
				if (s == 0 && tVert == 1) || (s == 0 && tVert == 2) || (s == 1 && tVert == 1) {
					cameraThru := bdptIntegrator.calculateCameraPathThroughput(cameraPath, tVert+1)
					lightThru := bdptIntegrator.calculateLightPathThroughput(lightPath, s+1)
					t.Logf("  -> Camera throughput (len %d): %v (lum: %.6f)", tVert+1, cameraThru, cameraThru.Luminance())
					t.Logf("  -> Light throughput (len %d): %v (lum: %.6f)", s+1, lightThru, lightThru.Luminance())
				}
			} else {
				t.Logf("Strategy s=%d,t=%d (%s): ZERO contribution", s, tVert, strategyType)
			}
		}
	}

	t.Logf("Found %d working strategies total", workingStrategies)

	// Analyze strategy contributions to catch s>=1 undercontribution issues
	var s0Contribution, sGeq1Contribution float64
	for s := 0; s < lightPath.Length; s++ {
		for tVert := 0; tVert < cameraPath.Length; tVert++ {
			contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, s, tVert, scene)
			if contribution.Luminance() > 0 {
				if s == 0 {
					s0Contribution += contribution.Luminance()
				} else {
					sGeq1Contribution += contribution.Luminance()
				}
			}
		}
	}

	t.Logf("s=0 strategies total contribution: %.9f", s0Contribution)
	t.Logf("s>=1 strategies total contribution: %.9f", sGeq1Contribution)

	// For indirect lighting, s>=1 strategies should contribute significantly
	// If s>=1 contributes less than 1% of s=0, there's likely a bug
	if s0Contribution > 0.001 && sGeq1Contribution > 0 {
		sRatio := sGeq1Contribution / s0Contribution
		t.Logf("s>=1 to s=0 contribution ratio: %.6f", sRatio)
		if sRatio < 0.01 {
			t.Errorf("s>=1 strategies severely undercontributing: %.9f vs s=0: %.9f (ratio %.6f)",
				sGeq1Contribution, s0Contribution, sRatio)
		}
	}

	// Results are already calculated above from averaging

	t.Logf("=== CORNER LIGHTING COMPARISON ===")
	t.Logf("Path tracing result: %v (luminance: %.6f)", pathResult, pathResult.Luminance())
	t.Logf("BDPT result: %v (luminance: %.6f)", bdptResult, bdptResult.Luminance())

	// For indirect lighting, both should produce similar illumination (corners shouldn't be black)
	if pathResult.Luminance() > 0.001 {
		ratio := bdptResult.Luminance() / pathResult.Luminance()
		if ratio < 0.8 { // BDPT is more than 20% dimmer - indicates s>=1 strategies not working
			t.Errorf("BDPT indirect lighting too dim: ratio %.4f (BDPT: %.6f, PT: %.6f)",
				ratio, bdptResult.Luminance(), pathResult.Luminance())
		}
		if ratio > 1.2 { // BDPT is more than 20% brighter
			t.Errorf("BDPT indirect lighting too bright: ratio %.4f (BDPT: %.6f, PT: %.6f)",
				ratio, bdptResult.Luminance(), pathResult.Luminance())
		}
	} else {
		// If path tracing gives very little light, BDPT should too
		if bdptResult.Luminance() > 0.01 {
			t.Errorf("BDPT shows light where path tracing shows none: BDPT=%.6f, PT=%.6f",
				bdptResult.Luminance(), pathResult.Luminance())
		}
	}
}

// TestBDPTConnectionPDFMissing tests that PDF calculations are missing connection PDF
func TestBDPTConnectionPDFMissing(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate a ray that will create paths suitable for connection
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, 278),
		core.NewVec3(0, -1, 0).Normalize(),
	)

	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

	if cameraPath.Length < 2 || lightPath.Length < 2 {
		t.Skip("Need paths with at least 2 vertices each for connection test")
	}

	// Test s=1, tVert=1 strategy (connect light path vertex 1 to camera path vertex 1)
	s, tVert := 1, 1

	// Get the vertices that would be connected
	cameraVertex := cameraPath.Vertices[tVert]
	lightVertex := lightPath.Vertices[s]

	t.Logf("=== Connection PDF Analysis ===")
	t.Logf("Camera vertex[%d]: pos=%v", tVert, cameraVertex.Point)
	t.Logf("Light vertex[%d]: pos=%v", s, lightVertex.Point)

	// Calculate connection geometry
	direction := lightVertex.Point.Subtract(cameraVertex.Point)
	distance := direction.Length()
	direction = direction.Multiply(1.0 / distance)

	t.Logf("Connection distance: %.6f", distance)
	t.Logf("Connection direction: %v", direction)

	// Calculate what the connection PDF should be using material PDFs
	cosAtLight := direction.Multiply(-1).Dot(lightVertex.Normal)
	cosAtCamera := direction.Dot(cameraVertex.Normal)

	t.Logf("cos_theta at light: %.6f", cosAtLight)
	t.Logf("cos_theta at camera: %.6f", cosAtCamera)

	// Calculate expected material-based connection PDFs
	var expectedConnectionPDF float64 = 1.0
	if cameraVertex.Material != nil {
		cameraPDF := cameraVertex.Material.PDF(cameraVertex.IncomingDirection, direction, cameraVertex.Normal)
		expectedConnectionPDF *= cameraPDF
		t.Logf("Camera material PDF: %.9f", cameraPDF)
	}
	if lightVertex.Material != nil {
		lightPDF := lightVertex.Material.PDF(lightVertex.IncomingDirection, direction.Multiply(-1), lightVertex.Normal)
		expectedConnectionPDF *= lightPDF
		t.Logf("Light material PDF: %.9f", lightPDF)
	}

	t.Logf("Expected material-based connection PDF: %.9f", expectedConnectionPDF)

	// Now get the actual PDF calculated by calculatePathPDF
	actualPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, s, tVert)
	t.Logf("Actual path PDF (from calculatePathPDF): %.9f", actualPDF)

	// Calculate what the PDF should be with material-based connection PDF
	pathOnlyPDF := 1.0

	// Camera path PDFs (up to but not including connection vertex, following our i<t approach)
	for i := 0; i < tVert && i < len(cameraPath.Vertices); i++ {
		vertex := cameraPath.Vertices[i]
		pathOnlyPDF *= vertex.ForwardPDF
	}

	// Light path PDFs (up to but not including connection vertex, following our i<s approach)
	for i := 0; i < s && i < len(lightPath.Vertices); i++ {
		vertex := lightPath.Vertices[i]
		pathOnlyPDF *= vertex.ForwardPDF
	}

	expectedTotalPDF := pathOnlyPDF * expectedConnectionPDF

	t.Logf("=== PDF Breakdown ===")
	t.Logf("Path-only PDF (vertices): %.9f", pathOnlyPDF)
	t.Logf("Expected material connection PDF: %.9f", expectedConnectionPDF)
	t.Logf("Expected total PDF: %.9f", expectedTotalPDF)
	t.Logf("Actual PDF from function: %.9f", actualPDF)

	// Test that connection PDFs are properly included (within reasonable tolerance)
	if expectedTotalPDF > 0 {
		ratio := actualPDF / expectedTotalPDF
		t.Logf("Actual/Expected PDF ratio: %.6f", ratio)

		// Allow some tolerance for numerical differences, but check it's roughly correct
		if ratio < 0.5 || ratio > 2.0 {
			t.Errorf("PDF calculation incorrect: actual %.9f vs expected %.9f (ratio %.3f)",
				actualPDF, expectedTotalPDF, ratio)
		} else {
			t.Logf("PDF calculation correct - material-based connection PDFs properly included")
		}
	}
}

// TestBDPTStrategyPDFCalculation tests that PDF calculations for strategies are correct
func TestBDPTStrategyPDFCalculation(t *testing.T) {
	scene := createMinimalCornellScene()

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, 3, core.NewVec3(1, 1, 1), 0)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, 3)

	// Focus on s=0,t=1 strategy PDF calculation
	s, tVert := 0, 1

	// Calculate PDFs manually using the actual method
	pathPDF := bdptIntegrator.calculatePathPDF(cameraPath, lightPath, s, tVert)

	// Verify PDFs are reasonable
	if lightPath.Length > 0 && lightPath.Vertices[0].ForwardPDF <= 0 {
		t.Errorf("Light vertex PDF should be positive, got %.9f", lightPath.Vertices[0].ForwardPDF)
	}
	if cameraPath.Length > 1 && cameraPath.Vertices[1].ForwardPDF <= 0 {
		t.Errorf("Camera path vertex PDF should be positive, got %.9f", cameraPath.Vertices[1].ForwardPDF)
	}

	// The combined PDF should be positive for a valid strategy
	if pathPDF <= 0 {
		t.Errorf("Combined path PDF should be positive for valid strategy, got: %.9f", pathPDF)
	}
}

// Test basic camera path generation
func TestCameraPathGeneration(t *testing.T) {
	// Create a simple scene with a sphere
	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	testScene := &MockScene{
		shapes: []core.Shape{sphere},
		config: core.SamplingConfig{MaxDepth: 3},
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	cameraPath := integrator.generateCameraSubpath(ray, testScene, random, 3, throughput, 0)

	if cameraPath.Length == 0 {
		t.Error("Expected camera path to have at least one vertex")
	}

	if cameraPath.Length > 0 {
		firstVertex := cameraPath.Vertices[0]
		if firstVertex.IsLight {
			t.Error("First vertex in camera path should not be marked as light")
		}
	}
}

// Test MIS weight calculation
func TestMISWeightCalculation(t *testing.T) {
	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	strategies := []bdptStrategy{
		{s: 0, t: 2, contribution: core.NewVec3(0.5, 0.5, 0.5), pdf: 0.1},
		{s: 1, t: 1, contribution: core.NewVec3(0.3, 0.3, 0.3), pdf: 0.2},
		{s: 2, t: 0, contribution: core.NewVec3(0.2, 0.2, 0.2), pdf: 0.15},
	}

	weight := integrator.calculateMISWeight(strategies[0], strategies)

	// Power heuristic: weight = pdf^2 / sum(all_pdf^2)
	expectedWeight := (0.1 * 0.1) / (0.1*0.1 + 0.2*0.2 + 0.15*0.15)

	if abs(weight-expectedWeight) > 1e-6 {
		t.Errorf("Expected MIS weight %f, got %f", expectedWeight, weight)
	}
}

// Test BDPT vs Path Tracing consistency
func TestBDPTvsPathTracingConsistency(t *testing.T) {
	// Create a simple scene with a light and diffuse surface
	emissiveMaterial := material.NewEmissive(core.NewVec3(2, 2, 2))
	light := geometry.NewSphereLight(core.NewVec3(0, 3, 0), 0.5, emissiveMaterial)

	lambertian := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, lambertian)

	bvh := core.NewBVH([]core.Shape{light, sphere})

	testScene := &MockScene{
		lights:      []core.Light{light},
		shapes:      []core.Shape{light, sphere},
		config:      core.SamplingConfig{MaxDepth: 5},
		bvh:         bvh,
		topColor:    core.NewVec3(0.1, 0.1, 0.1),
		bottomColor: core.NewVec3(0.05, 0.05, 0.05),
	}

	config := core.SamplingConfig{MaxDepth: 5}

	// Create both integrators
	pathTracer := NewPathTracingIntegrator(config)
	bdptTracer := NewBDPTIntegrator(config)

	// Test ray that should hit the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	// Sample multiple times to get average (reduces noise)
	numSamples := 10
	var pathTracingTotal, bdptTotal core.Vec3

	for i := 0; i < numSamples; i++ {
		random := rand.New(rand.NewSource(int64(42 + i)))

		// Path tracing result
		ptResult := pathTracer.RayColor(ray, testScene, random, config.MaxDepth, throughput, i)
		pathTracingTotal = pathTracingTotal.Add(ptResult)

		// BDPT result
		random = rand.New(rand.NewSource(int64(42 + i))) // Reset seed for fair comparison
		bdptResult := bdptTracer.RayColor(ray, testScene, random, config.MaxDepth, throughput, i)
		bdptTotal = bdptTotal.Add(bdptResult)
	}

	// Average the results
	pathTracingAvg := pathTracingTotal.Multiply(1.0 / float64(numSamples))
	bdptAvg := bdptTotal.Multiply(1.0 / float64(numSamples))

	// Results should be similar (within reasonable tolerance due to different sampling strategies)
	tolerance := 0.5 // BDPT and PT can have different variance characteristics

	if abs(pathTracingAvg.X-bdptAvg.X) > tolerance ||
		abs(pathTracingAvg.Y-bdptAvg.Y) > tolerance ||
		abs(pathTracingAvg.Z-bdptAvg.Z) > tolerance {
		t.Errorf("BDPT and Path Tracing results differ too much:\nPath Tracing: %v\nBDPT: %v",
			pathTracingAvg, bdptAvg)
	}

	// Both should produce some illumination (not black)
	if pathTracingAvg.Luminance() < 0.01 {
		t.Error("Path tracing produced unexpectedly dark result")
	}

	if bdptAvg.Luminance() < 0.01 {
		t.Error("BDPT produced unexpectedly dark result")
	}
}

// Test that BDPT handles specular materials correctly
func TestBDPTSpecularHandling(t *testing.T) {
	// Create scene with metal sphere
	metal := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0)
	sphere := geometry.NewSphere(core.NewVec3(0, 0, -1), 0.5, metal)

	bvh := core.NewBVH([]core.Shape{sphere})

	testScene := &MockScene{
		shapes: []core.Shape{sphere},
		config: core.SamplingConfig{MaxDepth: 3},
		bvh:    bvh,
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	throughput := core.NewVec3(1, 1, 1)

	// Should not crash on specular materials
	result := integrator.RayColor(ray, testScene, random, config.MaxDepth, throughput, 0)

	// Result should be valid (not NaN/Inf)
	if result.X != result.X || result.Y != result.Y || result.Z != result.Z {
		t.Error("BDPT produced NaN result with specular material")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
