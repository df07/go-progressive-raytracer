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

// TestBDPTvsPathTracingCameraPath compares BDPT vs path tracing on a simple Cornell setup
// This test isolates the camera path, testing camera rays that directly hit the light
func TestBDPTvsPathTracingCameraPath(t *testing.T) {
	scene := createMinimalCornellScene(false)

	bdpt := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 3})
	pt := NewPathTracingIntegrator(core.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100}) // disable RR

	// Camera is at 278, 400, -200
	// Ceiling is at 0, 556, 0 to 556,556,556
	rayToLight := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(278, 556, 278).Subtract(core.NewVec3(278, 400, -200)).Normalize(),
	)

	random := rand.New(rand.NewSource(42))
	cameraPath := bdpt.generateCameraSubpath(rayToLight, scene, random, bdpt.config.MaxDepth)
	lightPath := bdpt.generateLightSubpath(scene, random, bdpt.config.MaxDepth)

	// Camera path should have 2 vertices: the camera and the light
	if len(cameraPath.Vertices) != 2 {
		t.Errorf("Camera path should have 2 vertices, got %d", len(cameraPath.Vertices))
	}

	// The first vertex should be the camera
	if !cameraPath.Vertices[0].IsCamera {
		t.Errorf("First vertex should be the camera, got %v", cameraPath.Vertices[0])
	}

	// second vertex should be the light
	if !cameraPath.Vertices[1].IsLight {
		t.Errorf("Second vertex should be the light, got %v", cameraPath.Vertices[1])
	}

	// The light should have a non-zero emission
	if cameraPath.Vertices[1].EmittedLight.Luminance() == 0 {
		t.Errorf("Light should have non-zero emission, got %v", cameraPath.Vertices[1].EmittedLight)
	}

	// bdpt contribution from camera path should be the same as path tracing
	bdptContribution, _ := bdpt.RayColor(rayToLight, scene, random)
	ptContribution, _ := pt.RayColor(rayToLight, scene, random)

	bdptLuminance := bdptContribution.Luminance()
	pathLuminance := ptContribution.Luminance()

	ratio := bdptLuminance / pathLuminance
	if ratio < 0.999 { // BDPT is dimmer
		t.Errorf("BDPT result too dim compared to path tracing: ratio %.4f (BDPT: %.6f, PT: %.6f)",
			ratio, bdptContribution, ptContribution)
	}
	if ratio > 1.001 { // BDPT is brighter
		t.Errorf("BDPT result too bright compared to path tracing: ratio %.4f (BDPT: %.6f, PT: %.6f)",
			ratio, bdptLuminance, pathLuminance)
	}

	// BDPT should generate the s=0,t=2 strategy with the correct contribution
	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, scene, random)
	if len(strategies) != 1 {
		t.Errorf("BDPT should generate 1 strategy, got %d", len(strategies))
		t.FailNow()
	}

	strategy := strategies[0]
	if strategy.s != 0 || strategy.t != 2 || strategy.contribution.Luminance() != bdptLuminance {
		t.Errorf("BDPT should generate the s=0,t=2 strategy with the correct contribution, got %d,%d,%v", strategy.s, strategy.t, strategy.contribution)
	}
}

// TestCornellSpecularReflections compares BDPT vs path tracing on specular reflections
// This test checks if BDPT properly handles camera ray → mirror → light paths
func TestCornellSpecularReflections(t *testing.T) {
	// Create Cornell scene with mirror box and light
	scene := createMinimalCornellScene(true)

	// Setup a ray that should hit the ceiling above the mirror box and reflect to the light
	// Mirror box is at (185, 165, 351) with size (82.5, 165, 82.5)
	// Light is at ceiling around (278, 556, 279)
	rayToMirror := core.NewRayTo(
		core.NewVec3(278, 300, 100), // Camera position
		core.NewVec3(165, 556, 390), // Ray toward ceiling above mirror box
	)

	config := core.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100}
	pt := NewPathTracingIntegrator(config)
	bdpt := NewBDPTIntegrator(config)

	ptLight := core.NewVec3(0, 0, 0)
	bdptLight := core.NewVec3(0, 0, 0)
	ptMaxLuminance, bdptMaxLuminance := 0.0, 0.0
	ptMaxIndex, bdptMaxIndex := 0, 0

	seed := int64(42)
	count := 50

	for i := 0; i < count; i++ {
		//fmt.Printf("i=%d\n", i)
		ptRandom := rand.New(rand.NewSource(seed + int64(i)*492))
		ptResult, _ := pt.RayColor(rayToMirror, scene, ptRandom)

		bdptRandom := rand.New(rand.NewSource(seed + int64(i)*492))
		bdptResult, _ := bdpt.RayColor(rayToMirror, scene, bdptRandom)

		ptLight = ptLight.Add(ptResult)
		bdptLight = bdptLight.Add(bdptResult)

		if ptResult.Luminance() > ptMaxLuminance {
			ptMaxLuminance = ptResult.Luminance()
			ptMaxIndex = i
		}
		// i == 43
		if bdptResult.Luminance() > bdptMaxLuminance {
			bdptMaxLuminance = bdptResult.Luminance()
			bdptMaxIndex = i
		}
	}

	ptLight = ptLight.Multiply(1.0 / float64(count))
	bdptLight = bdptLight.Multiply(1.0 / float64(count))

	ptLuminance := ptLight.Luminance()
	bdptLuminance := bdptLight.Luminance()

	t.Logf("Path tracing average: %v", ptLight)
	t.Logf("BDPT average: %v", bdptLight)
	t.Logf("Path tracing average luminance: %.6f", ptLuminance)
	t.Logf("BDPT average luminance: %.6f", bdptLuminance)
	t.Logf("Path tracing max ray luminance: %.6f", ptMaxLuminance)
	t.Logf("BDPT max ray luminance: %.6f", bdptMaxLuminance)

	// Check if results are similar
	ratio := bdptLuminance / ptLuminance
	if math.Abs(ratio-1.0) > 0.1 {
		t.Errorf("FAIL: BDPT should have similar light contribution, got ratio %.3f (bdpt=%.3g, pt=%.3g)", ratio, bdptLuminance, ptLuminance)
	}

	// check max luminance
	ratio = bdptMaxLuminance / ptMaxLuminance
	if math.Abs(ratio-1.0) > 0.025 {
		t.Errorf("FAIL: BDPT should have similar max luminance, got ratio %.3f (bdpt=%.3g, pt=%.3g)", ratio, bdptMaxLuminance, ptMaxLuminance)

		if testing.Verbose() {
			// re-run the ray with verbose on for the max index
			pt.Verbose = testing.Verbose()
			bdpt.Verbose = testing.Verbose()

			ptRandom := rand.New(rand.NewSource(seed + int64(ptMaxIndex)*492))
			bdptRandom := rand.New(rand.NewSource(seed + int64(bdptMaxIndex)*492))

			t.Logf("\n=== PATH TRACING ===\n")
			ptResult, _ := pt.RayColor(rayToMirror, scene, ptRandom)
			t.Logf("\n=== BDPT ===\n")
			bdptResult, _ := bdpt.RayColor(rayToMirror, scene, bdptRandom)
			if ptResult.Luminance() != ptMaxLuminance {
				t.Errorf("FAIL: Path tracing max ray should have luminance %.6f, got %.6f", ptMaxLuminance, ptResult.Luminance())
			}
			if bdptResult.Luminance() != bdptMaxLuminance {
				t.Errorf("FAIL: BDPT max ray should have luminance %.6f, got %.6f", bdptMaxLuminance, bdptResult.Luminance())
			}
		}
	}
}

// TestBDPTvsPathTracingDirectLighting compares BDPT vs path tracing on a simple Cornell setup
// This test isolates the direct lighting issue - BDPT should perform similarly to path tracing
func TestBDPTvsPathTracingDirectLighting(t *testing.T) {
	// Create a minimal Cornell scene: just floor + quad light
	scene := createMinimalCornellScene(false)

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
	pathResult, _ := pathIntegrator.RayColor(rayToFloor, scene, pathRandom)

	// BDPT result with debug output
	bdptRandom := rand.New(rand.NewSource(seed))
	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)
	bdptIntegrator.Verbose = testing.Verbose() // Enable verbose logging to see MIS weights

	// Get the final result through RayColor for comparison
	bdptRandom = rand.New(rand.NewSource(seed)) // Reset seed
	bdptResult, _ := bdptIntegrator.RayColor(rayToFloor, scene, bdptRandom)

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
	return renderer.NewCamera(config)
}

// TestCamera is a minimal camera implementation for testing
type TestCamera struct{}

func (c *TestCamera) GetRay(i, j int, random *rand.Rand) core.Ray {
	// Return a simple ray for testing
	return core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1))
}

func (c *TestCamera) CalculateRayPDFs(ray core.Ray) (areaPDF, directionPDF float64) {
	// Simple test values
	return 1.0, 1.0
}

func (c *TestCamera) GetCameraForward() core.Vec3 {
	return core.NewVec3(0, 0, 1)
}

// TestLightPathDirectionAndIntersection verifies that light paths are generated correctly
func TestLightPathDirectionAndIntersection(t *testing.T) {
	scene := createMinimalCornellScene(false)

	// Generate multiple light paths to test consistency
	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	successfulPaths := 0
	totalPaths := 10

	for i := 0; i < totalPaths; i++ {
		lightPath := bdptIntegrator.generateLightSubpath(scene, random, bdptConfig.MaxDepth)

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
	scene := createMinimalCornellScene(false)

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
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToLight, scene, random, bdptConfig.MaxDepth)

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
	scene := createMinimalCornellScene(false)

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, bdptConfig.MaxDepth)

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
		contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 1, 2, scene)
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
	scene := createMinimalCornellScene(false)

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, bdptConfig.MaxDepth)

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

// TestBDPTvsDirectLightSampling compares BDPT s=1,t=2 with direct light sampling
func TestBDPTvsPTDirectLightSampling(t *testing.T) {
	scene := createMinimalCornellScene(false)

	random := rand.New(rand.NewSource(64))

	bdpt := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 1})
	pt := NewPathTracingIntegrator(core.SamplingConfig{MaxDepth: 1, RussianRouletteMinBounces: 100}) // disable RR

	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdpt.generateCameraSubpath(rayToFloor, scene, random, bdpt.config.MaxDepth)
	lightPath := bdpt.generateLightSubpath(scene, random, bdpt.config.MaxDepth)

	LogPath(t, "Camera", cameraPath)
	LogPath(t, "Light", lightPath)

	// BDPT s=1,t=2 connection
	//bdptContribution := bdpt.evaluateConnectionStrategy(cameraPath, lightPath, 1, 2, scene)
	//t.Logf("BDPT s=1,t=2 contribution: %v (luminance: %.6f)", bdptContribution, bdptContribution.Luminance())

	bdptContribution, _ := bdpt.RayColor(rayToFloor, scene, random)
	ptContribution, _ := pt.RayColor(rayToFloor, scene, random)

	t.Logf("BDPT contribution: %v (luminance: %.3f)", bdptContribution, bdptContribution.Luminance())
	t.Logf("PT contribution: %v (luminance: %.3f)", ptContribution, ptContribution.Luminance())

	ratio := bdptContribution.Luminance() / ptContribution.Luminance()
	if math.Abs(ratio-1.0) > 0.025 {
		t.Errorf("BDPT s=1,t=2 should match direct lighting exactly, got ratio %.3f", ratio)
	}

	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, scene, random)
	for i, strategy := range strategies {
		t.Logf("Strategy %d: s=%d,t=%d: contribution=%v (lum: %.3f), weight=%f",
			i, strategy.s, strategy.t, strategy.contribution, strategy.contribution.Luminance(), strategy.misWeight)
	}
}

func LogPath(t *testing.T, name string, path Path) {
	t.Logf("=== %s path (length: %d) ===", name, path.Length)
	for i, vertex := range path.Vertices {
		if vertex.IsLight {
			t.Logf("  %s[%d]: pos=%v, fwdPdf=%f, beta=%f, IsLight=true, Emission=%v (luminance=%v)",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.Beta, vertex.EmittedLight, vertex.EmittedLight.Luminance())

		} else if vertex.IsCamera {
			t.Logf("  %s[%d]: pos=%v, fwdPdf=%f, beta=%f, IsCamera=true",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.Beta)
		} else {
			t.Logf("  %s[%d]: pos=%v, fwdPdf=%f, beta=%f, Material=%v",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.Beta, vertex.Material != nil)
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
		camera: &MockCamera{},
	}

	config := core.SamplingConfig{MaxDepth: 3}
	integrator := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	lightPath := integrator.generateLightSubpath(testScene, random, config.MaxDepth)

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
	scene := createMinimalCornellScene(false)

	seed := int64(42)
	random := rand.New(rand.NewSource(seed))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor and then bounces to wall
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, 200),          // Position inside Cornell box
		core.NewVec3(0, -1, 0.1).Normalize(), // Ray pointing mostly down with slight Z component
	)
	cameraPath := bdptIntegrator.generateCameraSubpath(rayToFloor, scene, random, bdptConfig.MaxDepth)
	lightPath := bdptIntegrator.generateLightSubpath(scene, random, bdptConfig.MaxDepth)

	// Test the s=0,t=1 strategy in isolation
	s0t1Contribution := bdptIntegrator.evaluateConnectionStrategy(cameraPath, lightPath, 0, 1, scene)
	t.Logf("s=0,t=1 individual contribution: %v (luminance: %.6f)", s0t1Contribution, s0t1Contribution.Luminance())

	// Now test all strategies through evaluateBDPTStrategies
	strategies := bdptIntegrator.generateBDPTStrategies(cameraPath, lightPath, scene, random)
	allStrategies, _ := bdptIntegrator.evaluateBDPTStrategies(strategies)
	t.Logf("All strategies result: %v (luminance: %.6f)", allStrategies, allStrategies.Luminance())

	// The all-strategies result should be positive since s=0,t=1 works
	if allStrategies.Luminance() <= 0 {
		t.Errorf("All strategies returned zero, but s=0,t=1 works individually")
	}

	// Debug path structures first
	LogPath(t, "Camera", cameraPath)
	LogPath(t, "Light", lightPath)

	// Generate strategies and debug them in detail
	t.Logf("=== Strategy generation and debugging ===")
	t.Logf("Generated %d valid strategies", len(strategies))

	for i, strategy := range strategies {
		t.Logf("Strategy %d: s=%d,t=%d: contribution=%v (lum: %.6f)",
			i, strategy.s, strategy.t, strategy.contribution, strategy.contribution.Luminance())

		// Debug betas for key strategies
		if (strategy.s == 0 && strategy.t == 1) || (strategy.s == 1 && strategy.t == 1) {
			cameraThru := cameraPath.Vertices[strategy.t-1].Beta
			lightThru := lightPath.Vertices[strategy.s-1].Beta
			t.Logf("  -> Camera beta (len %d): %v (lum: %.6f)", strategy.t, cameraThru, cameraThru.Luminance())
			t.Logf("  -> Light beta (len %d): %v (lum: %.6f)", strategy.s, lightThru, lightThru.Luminance())

			// Debug individual vertex betas
			if strategy.t-1 < len(cameraPath.Vertices) {
				t.Logf("  -> Camera vertex[%d] beta: %v", strategy.t-1, cameraPath.Vertices[strategy.t-1].Beta)
			}
			if strategy.s < len(lightPath.Vertices) {
				t.Logf("  -> Light vertex[%d] beta: %v", strategy.s, lightPath.Vertices[strategy.s].Beta)
			}
		}
	}

	// Also show what strategies were skipped
	totalPossible := 0
	for s := 0; s < lightPath.Length; s++ {
		for tVert := 1; tVert < cameraPath.Length; tVert++ { // t starts at 1 like in generateBDPTStrategies
			totalPossible++
			// Check if this strategy was generated
			found := false
			for _, strategy := range strategies {
				if strategy.s == s && strategy.t == tVert {
					found = true
					break
				}
			}
			if !found {
				t.Logf("Strategy s=%d,t=%d: SKIPPED or ZERO contribution", s, tVert)
			}
		}
	}
	t.Logf("Found %d working strategies out of %d possible", len(strategies), totalPossible)
}

// TestBDPTIndirectLighting tests BDPT with a ray that hits a corner (indirect lighting only)
func TestBDPTIndirectLighting(t *testing.T) {
	scene := createMinimalCornellScene(false)

	// Ray aimed at center of top back wall, a little below the top
	// Should see minimal direct lighting, but lots of indirect lighting
	cameraPos := core.NewVec3(278, 400, 278)
	rayToCorner := core.NewRay(cameraPos,
		core.NewVec3(556/2, 556-1, 556).Subtract(cameraPos).Normalize(), // Ray pointing toward back top corner
	)

	seed := int64(11)

	// Average multiple samples for both integrators to get stable results
	numSamples := 10
	var pathTotal, bdptTotal core.Vec3

	pt := NewPathTracingIntegrator(core.SamplingConfig{MaxDepth: 5, RussianRouletteMinBounces: 100})
	bdpt := NewBDPTIntegrator(core.SamplingConfig{MaxDepth: 5})
	//bdpt.Verbose = true

	for i := 0; i < numSamples; i++ {
		// Path tracing sample
		pathRandom := rand.New(rand.NewSource(seed + int64(i)))
		pathSample, _ := pt.RayColor(rayToCorner, scene, pathRandom)
		pathTotal = pathTotal.Add(pathSample)

		// BDPT sample
		bdptRandom := rand.New(rand.NewSource(seed + int64(i)))
		bdptSample, _ := bdpt.RayColor(rayToCorner, scene, bdptRandom)
		bdptTotal = bdptTotal.Add(bdptSample)
	}

	pathResult := pathTotal.Multiply(1.0 / float64(numSamples))
	bdptResult := bdptTotal.Multiply(1.0 / float64(numSamples))

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
		camera:      &MockCamera{},
	}

	config := core.SamplingConfig{MaxDepth: 5}

	// Create both integrators
	pathTracer := NewPathTracingIntegrator(config)
	bdptTracer := NewBDPTIntegrator(config)

	// Test ray that should hit the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	// Sample multiple times to get average (reduces noise)
	numSamples := 10
	var pathTracingTotal, bdptTotal core.Vec3

	for i := 0; i < numSamples; i++ {
		random := rand.New(rand.NewSource(int64(42 + i)))

		// Path tracing result
		ptResult, _ := pathTracer.RayColor(ray, testScene, random)
		pathTracingTotal = pathTracingTotal.Add(ptResult)

		// BDPT result
		random = rand.New(rand.NewSource(int64(42 + i))) // Reset seed for fair comparison
		bdptResult, _ := bdptTracer.RayColor(ray, testScene, random)
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
		shapes:      []core.Shape{sphere},
		config:      core.SamplingConfig{MaxDepth: 3},
		bvh:         bvh,
		camera:      &MockCamera{},
		topColor:    core.NewVec3(0.5, 0.7, 1.0), // Blue sky background
		bottomColor: core.NewVec3(1.0, 1.0, 1.0), // White ground
	}

	config := core.SamplingConfig{MaxDepth: 3}
	bdpt := NewBDPTIntegrator(config)

	random := rand.New(rand.NewSource(42))
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))
	// camera path should contain camera, specular hit, and background light
	cameraPath := bdpt.generateCameraSubpath(ray, testScene, random, config.MaxDepth)
	LogPath(t, "Camera", cameraPath)

	if cameraPath.Length != 3 {
		t.Errorf("BDPT produced camera path with length %d, expected 3", cameraPath.Length)
		t.FailNow()
	}

	specularVertex := cameraPath.Vertices[1]
	if !specularVertex.IsSpecular || specularVertex.Material != metal {
		t.Error("BDPT did not produce correct specular vertex in camera path")
		LogPath(t, "Camera", cameraPath)
	}

	// all vertices should have reasonable pdfs and betas
	for i, vertex := range cameraPath.Vertices {
		if vertex.AreaPdfForward < 0 {
			t.Errorf("FAIL: Vertex[%d]: negative area pdf forward: %v", i, vertex.AreaPdfForward)
		}
		if vertex.Beta.Luminance() < 0.01 || vertex.Beta.Luminance() > 100 {
			t.Errorf("FAIL: Vertex[%d]: invalid beta: %v", i, vertex.Beta)
		}
	}

	// Debug: Generate light path and strategies to see what's happening
	lightPath := bdpt.generateLightSubpath(testScene, random, config.MaxDepth)
	LogPath(t, "Light", lightPath)

	// Generate strategies to debug why result is zero
	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, testScene, random)
	t.Logf("Generated %d strategies", len(strategies))

	for i, strategy := range strategies {
		t.Logf("Strategy %d: s=%d,t=%d: contribution=%v (lum: %.6f), weight=%f",
			i, strategy.s, strategy.t, strategy.contribution, strategy.contribution.Luminance(), strategy.misWeight)
	}

	// Should produce a ray color with valid luminance
	result, _ := bdpt.RayColor(ray, testScene, random)
	t.Logf("Final RayColor result: %v (luminance: %.6f)", result, result.Luminance())

	// Result should be valid (not NaN/Inf, not black, not too bright)
	if result.Luminance() < 0.01 || result.Luminance() > 10 {
		t.Error("FAIL: RayColor produced invalid result with specular material: ", result)
	}
}

func SceneWithGroundPlane(includeBackground bool, includeLight bool) (core.Scene, core.SamplingConfig) {
	// simple scene with a green ground plane mirroring default scene (without spheres)
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0).Multiply(0.6))
	groundPlane := geometry.NewPlane(core.NewVec3(0, 0, 0), core.NewVec3(0, 1, 0), lambertianGreen)

	shapes := []core.Shape{groundPlane}
	lights := []core.Light{}
	if includeLight {
		emissiveMaterial := material.NewEmissive(core.NewVec3(15.0, 14.0, 13.0))
		light := geometry.NewSphereLight(core.NewVec3(30, 30.5, 15), 10, emissiveMaterial)
		shapes = append(shapes, light.Sphere)
		lights = append(lights, light)
	}

	config := core.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100}

	testScene := &MockScene{
		lights:      lights,
		shapes:      shapes,
		config:      config,
		camera:      &MockCamera{},
		topColor:    core.NewVec3(0.5, 0.7, 1.0), // Blue sky background
		bottomColor: core.NewVec3(1.0, 1.0, 1.0), // White ground
	}

	if !includeBackground {
		testScene.topColor = core.NewVec3(0, 0, 0)
		testScene.bottomColor = core.NewVec3(0, 0, 0)
	}

	return testScene, config
}

func GroundPlaneTestRays() []struct {
	name string
	ray  core.Ray
} {
	cameraCenter := core.NewVec3(0, 0.75, 2) // From default scene
	return []struct {
		name string
		ray  core.Ray
	}{
		{"Sky", core.NewRay(cameraCenter, core.NewVec3(0, 1, 0))},
		{"Ground", core.NewRay(cameraCenter, core.NewVec3(0, 0.5, -1).Subtract(cameraCenter).Normalize())},
		{"Far", core.NewRay(cameraCenter, core.NewVec3(0, 0.5, -100).Subtract(cameraCenter).Normalize())},
	}
}
func TestBackgroundHandling(t *testing.T) {
	testScene, config := SceneWithGroundPlane(true, false)
	testRays := GroundPlaneTestRays()

	bdpt := NewBDPTIntegrator(config)
	pt := NewPathTracingIntegrator(config)

	for _, testRay := range testRays {
		// Enable verbose mode for debugging
		bdpt.Verbose = testing.Verbose()
		pt.Verbose = testing.Verbose()

		// compare bdpt and pt results
		t.Logf("=== Testing %s ray ===", testRay.name)
		bdptResult, _ := bdpt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(42)))
		ptResult, _ := pt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(42)))

		t.Logf("%s: BDPT=%v, PT=%v", testRay.name, bdptResult, ptResult)

		// check if the results are similar
		ratio := bdptResult.Luminance() / ptResult.Luminance()
		if ratio < 0.9 || ratio > 1.1 {
			t.Errorf("FAIL: %s ray luminance ratio of %.3f: BDPT=%v, PT=%v", testRay.name, ratio, bdptResult, ptResult)
		}
	}
}

func TestBackgroundWithLight(t *testing.T) {
	testScene, config := SceneWithGroundPlane(true, true)
	testRays := GroundPlaneTestRays()

	bdpt := NewBDPTIntegrator(config)
	pt := NewPathTracingIntegrator(config)

	bdpt.Verbose = testing.Verbose()
	pt.Verbose = testing.Verbose()

	for _, testRay := range testRays {
		// compare bdpt and pt results
		bdptResult, _ := bdpt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(42)))
		ptResult, _ := pt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(42)))

		t.Logf("%s: BDPT=%v, PT=%v", testRay.name, bdptResult, ptResult)

		// check if the results are similar
		ratio := bdptResult.Luminance() / ptResult.Luminance()
		if ratio < 0.9 || ratio > 1.1 {
			t.Errorf("FAIL: %s ray luminance ratio of %.3f: BDPT=%v, PT=%v", testRay.name, ratio, bdptResult, ptResult)
		}

	}
}

func TestNoBackgroundWithLight(t *testing.T) {
	testScene, config := SceneWithGroundPlane(false, true)
	testRays := GroundPlaneTestRays()

	bdpt := NewBDPTIntegrator(config)
	pt := NewPathTracingIntegrator(config)

	bdpt.Verbose = false // Disable verbose for multi-sample test
	pt.Verbose = false

	for _, testRay := range testRays {
		// Test with multiple samples to average out randomness differences
		numSamples := 100
		bdptSum := core.Vec3{X: 0, Y: 0, Z: 0}
		ptSum := core.Vec3{X: 0, Y: 0, Z: 0}

		for i := 0; i < numSamples; i++ {
			seed := int64(42 + i)

			// Enable verbose for first sample to debug
			if i == 0 {
				bdpt.Verbose = testing.Verbose()
				pt.Verbose = testing.Verbose()
			} else {
				bdpt.Verbose = false
				pt.Verbose = false
			}

			bdptResult, _ := bdpt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(seed)))
			ptResult, _ := pt.RayColor(testRay.ray, testScene, rand.New(rand.NewSource(seed)))

			bdptSum = bdptSum.Add(bdptResult)
			ptSum = ptSum.Add(ptResult)
		}

		bdptAvg := bdptSum.Multiply(1.0 / float64(numSamples))
		ptAvg := ptSum.Multiply(1.0 / float64(numSamples))

		t.Logf("%s (averaged over %d samples): BDPT=%v, PT=%v", testRay.name, numSamples, bdptAvg, ptAvg)

		// check if the results are similar
		ratio := bdptAvg.Luminance() / ptAvg.Luminance()
		if ratio < 0.95 || ratio > 1.05 {
			t.Errorf("FAIL: %s ray luminance ratio of %.3f: BDPT=%v, PT=%v", testRay.name, ratio, bdptAvg, ptAvg)
		}

	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
