package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/renderer"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

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
	pathSampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))
	pathConfig := core.SamplingConfig{MaxDepth: 5}
	pathIntegrator := NewPathTracingIntegrator(pathConfig)
	pathResult, _ := pathIntegrator.RayColor(rayToFloor, scene, pathSampler)

	// BDPT result with debug output
	bdptSampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))
	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)
	bdptIntegrator.Verbose = testing.Verbose() // Enable verbose logging to see MIS weights

	// Get the final result through RayColor for comparison
	bdptSampler = core.NewRandomSampler(rand.New(rand.NewSource(seed)))
	bdptResult, _ := bdptIntegrator.RayColor(rayToFloor, scene, bdptSampler)

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

// TestLightPathDirectionAndIntersection verifies that light paths are generated correctly
func TestLightPathDirectionAndIntersection(t *testing.T) {
	scene := createMinimalCornellScene(false)

	// Generate multiple light paths to test consistency
	seed := int64(42)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	successfulPaths := 0
	totalPaths := 10

	for i := 0; i < totalPaths; i++ {
		lightPath := bdptIntegrator.generateLightPath(scene, sampler, bdptConfig.MaxDepth)

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that should hit the light
	cameraPath := bdptIntegrator.generateCameraPath(rayToLight, scene, sampler, bdptConfig.MaxDepth)

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraPath(rayToFloor, scene, sampler, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightPath(scene, sampler, bdptConfig.MaxDepth)

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
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))

	bdptConfig := core.SamplingConfig{MaxDepth: 5}
	bdptIntegrator := NewBDPTIntegrator(bdptConfig)

	// Generate camera path that hits floor
	rayToFloor := core.NewRay(
		core.NewVec3(278, 400, -200),
		core.NewVec3(0, -1, 0.5).Normalize(),
	)
	cameraPath := bdptIntegrator.generateCameraPath(rayToFloor, scene, sampler, bdptConfig.MaxDepth)

	// Generate light path
	lightPath := bdptIntegrator.generateLightPath(scene, sampler, bdptConfig.MaxDepth)

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

func LogPath(t *testing.T, name string, path Path) {
	t.Logf("=== %s path (length: %d) ===", name, path.Length)
	for i, vertex := range path.Vertices {
		if vertex.IsLight {
			t.Logf("  %s[%d]: LIGHT    pos=%v, fwdPdf=%0.3g, revPdf=%0.3g, beta=%v, Emission=%v",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.AreaPdfReverse, vertex.Beta, vertex.EmittedLight)

		} else if vertex.IsCamera {
			t.Logf("  %s[%d]: CAMERA   pos=%v, fwdPdf=%0.3g, revPdf=%0.3g, beta=%v",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.AreaPdfReverse, vertex.Beta)
		} else if vertex.IsSpecular {
			t.Logf("  %s[%d]: SPECULAR pos=%v, fwdPdf=%0.3g, revPdf=%0.3g, beta=%v, Material=%v",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.AreaPdfReverse, vertex.Beta, vertex.Material != nil)
		} else {
			t.Logf("  %s[%d]: MATERIAL pos=%v, fwdPdf=%0.3g, revPdf=%0.3g, beta=%v, Material=%v",
				name, i, vertex.Point, vertex.AreaPdfForward, vertex.AreaPdfReverse, vertex.Beta, vertex.Material != nil)
		}

	}
}

// TestBDPTvsPathTracingBackgroundHandling compares BDPT vs PT with a background plane
func TestBDPTvsPathTracingBackgroundHandling(t *testing.T) {
	testScene, config := SceneWithGroundPlane(true, false)
	testRays := GroundPlaneTestRays()

	bdpt := NewBDPTIntegrator(config)
	pt := NewPathTracingIntegrator(config)

	for _, testRay := range testRays {
		// compare bdpt and pt results
		bdptResult, _ := bdpt.RayColor(testRay.ray, testScene, core.NewRandomSampler(rand.New(rand.NewSource(42))))
		ptResult, _ := pt.RayColor(testRay.ray, testScene, core.NewRandomSampler(rand.New(rand.NewSource(42))))

		t.Logf("%s: BDPT=%v, PT=%v", testRay.name, bdptResult, ptResult)

		// check if the results are similar
		// Note: BDPT splits contribution between direct rays and splats, while PT puts everything in direct ray
		// So BDPT's RayColor will be dimmer for scenes with significant light path strategies (splats)
		ratio := bdptResult.Luminance() / ptResult.Luminance()
		if ratio < 0.8 || ratio > 1.2 {
			t.Errorf("FAIL: %s ray luminance ratio of %.3f: BDPT=%v, PT=%v", testRay.name, ratio, bdptResult, ptResult)
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

	camera := renderer.NewCamera(renderer.CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 3, 0),
		Up:          core.NewVec3(0, 1, 0),
		VFov:        45,
		AspectRatio: 1,
	})

	testScene := &MockScene{
		lights: []core.Light{light},
		shapes: []core.Shape{light.Sphere, sphere},
		config: core.SamplingConfig{MaxDepth: 5},
		camera: camera,
	}

	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.1, 0.1, 0.1),    // topColor
		core.NewVec3(0.05, 0.05, 0.05), // bottomColor
	)
	// Preprocess the infinite light with scene bounds
	infiniteLight.Preprocess(testScene)
	testScene.lights = append(testScene.lights, infiniteLight)

	config := core.SamplingConfig{MaxDepth: 5}

	// Create both integrators
	pathTracer := NewPathTracingIntegrator(config)
	bdptTracer := NewBDPTIntegrator(config)

	// Test ray that should hit the sphere
	ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, -1))

	// Sample multiple times to get average (reduces noise)
	numSamples := 100
	var pathTracingTotal, bdptTotal core.Vec3

	for i := 0; i < numSamples; i++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(42 + i))))

		// Path tracing result
		ptResult, _ := pathTracer.RayColor(ray, testScene, sampler)
		pathTracingTotal = pathTracingTotal.Add(ptResult)

		// BDPT result
		bdptSampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(42 + i)))) // Reset seed for fair comparison
		bdptResult, _ := bdptTracer.RayColor(ray, testScene, bdptSampler)
		bdptTotal = bdptTotal.Add(bdptResult)
	}

	// Average the results
	pathTracingAvg := pathTracingTotal.Multiply(1.0 / float64(numSamples))
	bdptAvg := bdptTotal.Multiply(1.0 / float64(numSamples))

	// Results should be similar (within reasonable tolerance due to different sampling strategies)
	tolerance := 0.01 // BDPT and PT can have different variance characteristics

	if math.Abs(pathTracingAvg.X-bdptAvg.X) > tolerance ||
		math.Abs(pathTracingAvg.Y-bdptAvg.Y) > tolerance ||
		math.Abs(pathTracingAvg.Z-bdptAvg.Z) > tolerance {
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

func SceneWithGroundPlane(includeBackground bool, includeLight bool) (core.Scene, core.SamplingConfig) {
	// simple scene with a green ground quad mirroring default scene (without spheres)
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0).Multiply(0.6))
	groundQuad := scene.NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGreen)

	shapes := []core.Shape{groundQuad}
	lights := []core.Light{}
	if includeLight {
		emissiveMaterial := material.NewEmissive(core.NewVec3(15.0, 14.0, 13.0))
		light := geometry.NewSphereLight(core.NewVec3(30, 30.5, 15), 10, emissiveMaterial)
		shapes = append(shapes, light.Sphere)
		lights = append(lights, light)
	}

	defaultCameraConfig := renderer.CameraConfig{
		Center:        core.NewVec3(0, 0.75, 2), // Position camera higher and farther back
		LookAt:        core.NewVec3(0, 0.5, -1), // Look at the sphere center
		Up:            core.NewVec3(0, 1, 0),    // Standard up direction
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          40.0, // Narrower field of view for focus effect
		Aperture:      0.05, // Strong depth of field blur
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	config := core.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100}

	testScene := &MockScene{
		lights: lights,
		shapes: shapes,
		config: config,
		camera: renderer.NewCamera(defaultCameraConfig),
	}

	if includeBackground {
		// Add infinite light to match background colors for BDPT compatibility
		infiniteLight := geometry.NewGradientInfiniteLight(
			core.NewVec3(0.5, 0.7, 1.0), // topColor
			core.NewVec3(1.0, 1.0, 1.0), // bottomColor
		)
		// Preprocess the infinite light with scene bounds
		infiniteLight.Preprocess(testScene)
		lights = append(lights, infiniteLight)
		testScene.lights = lights
	}

	return testScene, config
}

// TestInfiniteLightEmissionSampling tests that infinite lights emit rays toward the scene properly
func TestInfiniteLightEmissionSampling(t *testing.T) {
	// Create simple scene with ground quad (finite but large)
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0))
	groundQuad := scene.NewGroundQuad(core.NewVec3(0, 0, 0), 1000.0, lambertianGreen)

	// Create gradient infinite light
	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor
	)

	// Create mock scene
	testScene := &MockScene{
		lights: []core.Light{infiniteLight},
		shapes: []core.Shape{groundQuad},
		config: core.SamplingConfig{MaxDepth: 3},
	}

	// Preprocess to set world bounds
	err := infiniteLight.Preprocess(testScene)
	if err != nil {
		t.Fatalf("Failed to preprocess infinite light: %v", err)
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	t.Logf("=== Testing Infinite Light Emission Sampling ===")

	intersectionCount := 0
	totalSamples := 10

	for i := 0; i < totalSamples; i++ {
		sample := infiniteLight.SampleEmission(sampler.Get2D(), sampler.Get2D())

		t.Logf("Sample %d:", i)
		t.Logf("  Point: %v", sample.Point)
		t.Logf("  Direction: %v", sample.Direction)
		t.Logf("  Normal: %v", sample.Normal)
		t.Logf("  AreaPDF: %f, DirectionPDF: %f", sample.AreaPDF, sample.DirectionPDF)

		// Create ray from emission sample
		emissionRay := core.NewRay(sample.Point, sample.Direction)

		// Check if ray is pointing roughly toward scene center
		sceneCenter := core.NewVec3(0, 0, 0) // Ground plane center
		toScene := sceneCenter.Subtract(sample.Point).Normalize()
		dotProduct := sample.Direction.Dot(toScene)
		t.Logf("  Direction toward scene center: %f (should be > 0.5)", dotProduct)

		// Test ray intersection with scene
		hit, isHit := testScene.GetBVH().Hit(emissionRay, 0.001, math.Inf(1))
		if isHit {
			intersectionCount++
			t.Logf("  HIT: %v (material: %v)", hit.Point, hit.Material != nil)
		} else {
			t.Logf("  MISS: Ray did not intersect scene")
		}
	}

	t.Logf("Intersection rate: %d/%d (%.1f%%)", intersectionCount, totalSamples, float64(intersectionCount)*100.0/float64(totalSamples))

	// Most rays should intersect the ground plane
	if intersectionCount == 0 {
		t.Errorf("No emission rays intersected the scene - this suggests rays are pointing away from scene")
	}

	if float64(intersectionCount)/float64(totalSamples) < 0.16 {
		t.Errorf("Too few emission rays intersected scene: %d/%d (%.1f%%). Expected >16%%",
			intersectionCount, totalSamples, float64(intersectionCount)*100.0/float64(totalSamples))
	}
}

// TestBDPTvsPathTracingReflectiveGround tests BDPT vs PT on a reflective surface
// This isolates potential issues with specular paths in BDPT
func TestBDPTvsPathTracingReflectiveGround(t *testing.T) {
	// Create scene with reflective ground plane and infinite light
	testScene, config := SceneWithReflectiveGroundPlane()

	bdpt := NewBDPTIntegrator(config)
	bdpt.Verbose = false // Disable debug logging for cleaner test output
	pt := NewPathTracingIntegrator(config)

	// Ray hitting reflective ground at an angle (should reflect to sky)
	cameraCenter := core.NewVec3(0, 2, 2)
	rayToGround := core.NewRay(cameraCenter, core.NewVec3(0, -0.8, -0.6).Normalize())

	t.Logf("=== Testing Reflective Ground ===")

	// Sample multiple times for statistical accuracy
	numSamples := 50
	var ptTotal, bdptTotal core.Vec3

	for i := 0; i < numSamples; i++ {
		ptSampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(100 + i))))
		bdptSampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(100 + i))))

		ptResult, _ := pt.RayColor(rayToGround, testScene, ptSampler)
		bdptResult, _ := bdpt.RayColor(rayToGround, testScene, bdptSampler)

		ptTotal = ptTotal.Add(ptResult)
		bdptTotal = bdptTotal.Add(bdptResult)
	}

	ptAvg := ptTotal.Multiply(1.0 / float64(numSamples))
	bdptAvg := bdptTotal.Multiply(1.0 / float64(numSamples))

	t.Logf("Path Tracing average: %v (luminance: %.6f)", ptAvg, ptAvg.Luminance())
	t.Logf("BDPT average: %v (luminance: %.6f)", bdptAvg, bdptAvg.Luminance())

	ratio := bdptAvg.Luminance() / ptAvg.Luminance()
	t.Logf("BDPT/PT ratio: %.3f", ratio)

	// Check if BDPT is significantly dimmer than PT for reflective surfaces
	if ratio < 0.95 || ratio > 1.05 {
		t.Errorf("FAIL: Reflective ground BDPT/PT ratio %.3f outside expected range [0.95, 1.05]: PT=%v, BDPT=%v",
			ratio, ptAvg, bdptAvg)
	}
}

func SceneWithReflectiveGroundPlane() (core.Scene, core.SamplingConfig) {
	// Create a reflective (metal) ground plane
	metalMaterial := material.NewMetal(core.NewVec3(0.8, 0.8, 0.9), 0.0) // Mirror-like
	groundQuad := scene.NewGroundQuad(core.NewVec3(0, 0, 0), 1000.0, metalMaterial)

	shapes := []core.Shape{groundQuad}

	// Add infinite light for reflections
	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.8, 0.9, 1.0), // Blue sky
		core.NewVec3(0.9, 0.9, 1.0), // Light horizon
	)

	camera := renderer.NewCamera(renderer.CameraConfig{
		Center:      core.NewVec3(0, 2, 2),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	})

	config := core.SamplingConfig{MaxDepth: 6, RussianRouletteMinBounces: 100}

	testScene := &MockScene{
		lights: []core.Light{infiniteLight},
		shapes: shapes,
		config: config,
		camera: camera,
	}

	// Preprocess the infinite light
	infiniteLight.Preprocess(testScene)

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
