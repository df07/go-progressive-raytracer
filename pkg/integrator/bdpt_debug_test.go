package integrator

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
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

	camera := geometry.NewCamera(geometry.CameraConfig{
		Center:      core.NewVec3(0, 0, 0),
		LookAt:      core.NewVec3(0, 3, 0),
		Up:          core.NewVec3(0, 1, 0),
		VFov:        45,
		AspectRatio: 1,
	})

	testScene := &scene.Scene{
		Lights:         []geometry.Light{light},
		Shapes:         []geometry.Shape{light.Sphere, sphere},
		SamplingConfig: core.SamplingConfig{MaxDepth: 5},
		Camera:         camera,
	}

	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.1, 0.1, 0.1),    // topColor
		core.NewVec3(0.05, 0.05, 0.05), // bottomColor
	)
	testScene.Lights = append(testScene.Lights, infiniteLight)
	testScene.Preprocess()

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

func SceneWithGroundPlane(includeBackground bool, includeLight bool) (*scene.Scene, core.SamplingConfig) {
	// simple scene with a green ground quad mirroring default scene (without spheres)
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0).Multiply(0.6))
	groundQuad := scene.NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGreen)

	shapes := []geometry.Shape{groundQuad}
	lights := []geometry.Light{}
	if includeLight {
		emissiveMaterial := material.NewEmissive(core.NewVec3(15.0, 14.0, 13.0))
		light := geometry.NewSphereLight(core.NewVec3(30, 30.5, 15), 10, emissiveMaterial)
		shapes = append(shapes, light.Sphere)
		lights = append(lights, light)
	}

	defaultCameraConfig := geometry.CameraConfig{
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

	testScene := &scene.Scene{
		Lights:         lights,
		Shapes:         shapes,
		SamplingConfig: config,
		Camera:         geometry.NewCamera(defaultCameraConfig),
	}

	if includeBackground {
		// Add infinite light to match background colors for BDPT compatibility
		infiniteLight := geometry.NewGradientInfiniteLight(
			core.NewVec3(0.5, 0.7, 1.0), // topColor
			core.NewVec3(1.0, 1.0, 1.0), // bottomColor
		)
		// Preprocess the infinite light with scene bounds
		lights = append(lights, infiniteLight)
		testScene.Lights = lights
	}

	testScene.Preprocess()

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
	testScene := &scene.Scene{
		Lights:         []geometry.Light{infiniteLight},
		Shapes:         []geometry.Shape{groundQuad},
		SamplingConfig: core.SamplingConfig{MaxDepth: 3},
	}

	testScene.Preprocess()

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
		hit, isHit := testScene.BVH.Hit(emissionRay, 0.001, math.Inf(1))
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

func SceneWithReflectiveGroundPlane() (*scene.Scene, core.SamplingConfig) {
	// Create a reflective (metal) ground plane
	metalMaterial := material.NewMetal(core.NewVec3(0.8, 0.8, 0.9), 0.0) // Mirror-like
	groundQuad := scene.NewGroundQuad(core.NewVec3(0, 0, 0), 1000.0, metalMaterial)

	shapes := []geometry.Shape{groundQuad}

	// Add infinite light for reflections
	infiniteLight := geometry.NewGradientInfiniteLight(
		core.NewVec3(0.8, 0.9, 1.0), // Blue sky
		core.NewVec3(0.9, 0.9, 1.0), // Light horizon
	)

	camera := geometry.NewCamera(geometry.CameraConfig{
		Center:      core.NewVec3(0, 2, 2),
		LookAt:      core.NewVec3(0, 0, -1),
		Up:          core.NewVec3(0, 1, 0),
		Width:       400,
		AspectRatio: 1.0,
		VFov:        45.0,
	})

	config := core.SamplingConfig{MaxDepth: 6, RussianRouletteMinBounces: 100}

	testScene := &scene.Scene{
		Lights:         []geometry.Light{infiniteLight},
		Shapes:         shapes,
		SamplingConfig: config,
		Camera:         camera,
	}

	testScene.Preprocess()

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
