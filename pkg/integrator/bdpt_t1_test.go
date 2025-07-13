package integrator

import (
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// TestBDPTT1StrategySpecularReflections tests that t=1 strategies improve specular reflection handling
// This is based on TestCornellSpecularReflections but specifically validates t=1 strategy generation
func TestBDPTT1StrategySpecularReflections(t *testing.T) {
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
	bdpt := NewBDPTIntegrator(config)
	bdpt.Verbose = true // Enable logging to see t=1 strategy details

	// Test a single ray to examine the strategies generated
	random := rand.New(rand.NewSource(42))

	// Generate camera and light paths
	cameraPath := bdpt.generateCameraSubpath(rayToMirror, scene, random, 3)
	lightPath := bdpt.generateLightSubpath(scene, random, 3)

	t.Logf("=== PATH ANALYSIS ===")
	t.Logf("Camera path length: %d", cameraPath.Length)
	for i, vertex := range cameraPath.Vertices {
		t.Logf("  Camera[%d]: pos=%v, IsSpecular=%t, Material=%t", i, vertex.Point, vertex.IsSpecular, vertex.Material != nil)
	}

	t.Logf("Light path length: %d", lightPath.Length)
	for i, vertex := range lightPath.Vertices {
		t.Logf("  Light[%d]: pos=%v, IsLight=%t, IsSpecular=%t, Material=%t", i, vertex.Point, vertex.IsLight, vertex.IsSpecular, vertex.Material != nil)
	}

	// Generate all BDPT strategies
	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, scene, random)

	t.Logf("=== STRATEGY ANALYSIS ===")
	t.Logf("Generated %d total strategies", len(strategies))

	// Analyze strategies by type
	var t1Strategies []bdptStrategy
	var connectionStrategies []bdptStrategy
	var pathTracingStrategies []bdptStrategy

	for _, strategy := range strategies {
		switch {
		case strategy.t == 1:
			t1Strategies = append(t1Strategies, strategy)
		case strategy.s == 0:
			pathTracingStrategies = append(pathTracingStrategies, strategy)
		default:
			connectionStrategies = append(connectionStrategies, strategy)
		}
	}

	t.Logf("T=1 strategies: %d", len(t1Strategies))
	t.Logf("Connection strategies: %d", len(connectionStrategies))
	t.Logf("Path tracing strategies: %d", len(pathTracingStrategies))

	// We should have t=1 strategies for this scenario
	if len(t1Strategies) == 0 {
		t.Error("Expected t=1 strategies to be generated for specular reflection scenario")
	}

	// Check that t=1 strategies generate splat rays
	totalSplats := 0
	foundSpecularSplat := false
	for _, strategy := range t1Strategies {
		t.Logf("T=1 strategy (s=%d,t=%d): contribution=%v, splats=%d, weight=%f",
			strategy.s, strategy.t, strategy.contribution, len(strategy.splatRays), strategy.misWeight)
		totalSplats += len(strategy.splatRays)

		for i, splat := range strategy.splatRays {
			t.Logf("  (s=%d,t=%d) Splats[%d]: origin=%v, direction=%v, color=%v", strategy.t, strategy.s, i, splat.Ray.Origin, splat.Ray.Direction, splat.Color)

			// For specular reflection scenario, we expect s=2,t=1 to have bright splats
			if strategy.s == 2 && strategy.t == 1 {
				if splat.Color.Luminance() < 0.001 {
					t.Errorf("S=2,T=1 splat color is too low for specular reflection: %v", splat.Color)
				} else {
					foundSpecularSplat = true
				}
			}
		}
	}

	if !foundSpecularSplat {
		t.Error("Expected to find bright s=2,t=1 splat for specular reflection scenario")
	}

	if totalSplats == 0 {
		t.Error("Expected t=1 strategies to generate splat rays")
	}

	t.Logf("Total splat rays from t=1 strategies: %d", totalSplats)
}

// TestBDPTT1StrategyImprovement tests that t=1 strategies actually improve the result
func TestBDPTT1StrategyImprovement(t *testing.T) {
	// Create Cornell scene with mirror box and light
	scene := createMinimalCornellScene(true)

	// Setup a ray that should hit the ceiling above the mirror box and reflect to the light
	rayToMirror := core.NewRayTo(
		core.NewVec3(278, 300, 100), // Camera position
		core.NewVec3(165, 556, 390), // Ray toward ceiling above mirror box
	)

	config := core.SamplingConfig{MaxDepth: 3, RussianRouletteMinBounces: 100}
	bdpt := NewBDPTIntegrator(config)
	bdpt.Verbose = false // Disable logging for this test

	// Test multiple samples to get a good average
	bdptTotal := core.Vec3{X: 0, Y: 0, Z: 0}
	totalSplats := 0

	count := 20
	for i := 0; i < count; i++ {
		random := rand.New(rand.NewSource(42 + int64(i)*492))
		result, splats := bdpt.RayColor(rayToMirror, scene, random)

		bdptTotal = bdptTotal.Add(result)
		totalSplats += len(splats)

		if len(splats) > 0 {
			t.Logf("Sample %d: direct=%v, splats=%d", i, result, len(splats))
		}
	}

	bdptAverage := bdptTotal.Multiply(1.0 / float64(count))
	avgSplats := float64(totalSplats) / float64(count)

	t.Logf("=== RESULTS ===")
	t.Logf("BDPT average result: %v (luminance: %.6f)", bdptAverage, bdptAverage.Luminance())
	t.Logf("Average splat rays per sample: %.2f", avgSplats)

	// We should get some meaningful contribution for this difficult lighting scenario
	if bdptAverage.Luminance() <= 0.01 {
		t.Errorf("Expected significant luminance from BDPT with t=1 strategies, got %.6f", bdptAverage.Luminance())
	}

	// We should generate some splat rays
	if avgSplats < 0.5 {
		t.Errorf("Expected t=1 strategies to generate splat rays, got average %.2f", avgSplats)
	}
}
