package material

import (
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestNewMetal_FuzznessClamp(t *testing.T) {
	tests := []struct {
		name             string
		inputFuzzness    float64
		expectedFuzzness float64
	}{
		{"Valid fuzzness 0.0", 0.0, 0.0},
		{"Valid fuzzness 0.5", 0.5, 0.5},
		{"Valid fuzzness 1.0", 1.0, 1.0},
		{"Clamp above 1.0", 1.5, 1.0},
		{"Clamp below 0.0", -0.5, 0.0},
		{"Clamp large positive", 10.0, 1.0},
		{"Clamp large negative", -10.0, 0.0},
	}

	albedo := core.NewVec3(0.8, 0.8, 0.8)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metal := NewMetal(albedo, tt.inputFuzzness)
			if metal.Fuzzness != tt.expectedFuzzness {
				t.Errorf("Expected fuzzness %f, got %f", tt.expectedFuzzness, metal.Fuzzness)
			}
		})
	}
}

func TestMetal_PerfectReflection(t *testing.T) {
	// Test perfect reflection (fuzziness = 0)
	albedo := core.NewVec3(0.9, 0.9, 0.9)
	metal := NewMetal(albedo, 0.0)
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	// Ray hitting surface at 45 degrees
	rayIn := core.NewRay(core.NewVec3(0, 1, 1), core.NewVec3(0, -1, -1).Normalize())
	hit := SurfaceInteraction{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1), // Surface normal pointing up
	}

	scatter, didScatter := metal.Scatter(rayIn, hit, sampler)
	if !didScatter {
		t.Fatal("Metal should scatter")
	}

	// For perfect reflection: incident (0, -1, -1) normalized reflects to (0, -0.707, 0.707)
	expected := core.NewVec3(0, -1, 1).Normalize()
	actual := scatter.Scattered.Direction.Normalize()

	tolerance := 1e-10
	if actual.Subtract(expected).Length() > tolerance {
		t.Errorf("Perfect reflection failed: expected %v, got %v", expected, actual)
	}

	// Check that attenuation equals albedo
	if !scatter.Attenuation.Equals(albedo) {
		t.Errorf("Attenuation should equal albedo: expected %v, got %v", albedo, scatter.Attenuation)
	}

	// Check that PDF is zero (specular)
	if scatter.PDF != 0 {
		t.Errorf("Specular material PDF should be 0, got %f", scatter.PDF)
	}
}

func TestMetal_FuzzyReflection(t *testing.T) {
	// Test fuzzy reflection behavior
	albedo := core.NewVec3(0.8, 0.8, 0.8)
	metal := NewMetal(albedo, 0.5) // Moderate fuzziness
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

	rayIn := core.NewRay(core.NewVec3(0, 0, 1), core.NewVec3(0, 0, -1))
	hit := SurfaceInteraction{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1),
	}

	// Test multiple scatters to verify fuzziness introduces variation
	directions := make([]core.Vec3, 10)
	for i := 0; i < 10; i++ {
		scatter, didScatter := metal.Scatter(rayIn, hit, sampler)
		if !didScatter {
			t.Fatalf("Metal should scatter on iteration %d", i)
		}
		directions[i] = scatter.Scattered.Direction.Normalize()
	}

	// With fuzziness, directions should vary (not all identical)
	allSame := true
	for i := 1; i < len(directions); i++ {
		if directions[i].Subtract(directions[0]).Length() > 1e-10 {
			allSame = false
			break
		}
	}
	if allSame {
		t.Error("Fuzzy metal should produce varying reflection directions")
	}

	// All scattered rays should still be above the surface
	for i, dir := range directions {
		if dir.Dot(hit.Normal) <= 0 {
			t.Errorf("Scattered ray %d should be above surface, got dot product %f", i, dir.Dot(hit.Normal))
		}
	}
}

func TestMetal_ScatterAbsorption(t *testing.T) {
	// Test that rays scattered below the surface are absorbed
	metal := NewMetal(core.NewVec3(0.8, 0.8, 0.8), 1.0) // Maximum fuzziness
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(123)))

	// Grazing angle ray that might scatter below surface with high fuzziness
	rayIn := core.NewRay(core.NewVec3(-1, 0, 0.01), core.NewVec3(1, 0, -0.01).Normalize())
	hit := SurfaceInteraction{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1),
	}

	absorptionCount := 0
	scatterCount := 0

	for i := 0; i < 1000; i++ {
		_, didScatter := metal.Scatter(rayIn, hit, sampler)
		if didScatter {
			scatterCount++
		} else {
			absorptionCount++
		}
	}

	// With high fuzziness and grazing angle, some rays should be absorbed
	if absorptionCount == 0 {
		t.Error("Expected some rays to be absorbed with high fuzziness at grazing angle")
	}
	if scatterCount == 0 {
		t.Error("Expected some rays to be scattered")
	}
}

func TestMetal_EvaluateBRDF_PerfectReflection(t *testing.T) {
	albedo := core.NewVec3(0.9, 0.5, 0.3)
	metal := NewMetal(albedo, 0.0)

	hit := &SurfaceInteraction{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1),
	}

	// Test perfect reflection case
	// Incoming ray direction (towards surface)
	incomingDir := core.NewVec3(1, 0, -1).Normalize()
	// For BRDF: reflect(-incomingDir, normal) = reflect((-1,0,1), (0,0,1)) = (-1, 0, -1)
	outgoingDir := core.NewVec3(-1, 0, -1).Normalize() // Perfect reflection of negated incoming

	brdf := metal.EvaluateBRDF(incomingDir, outgoingDir, hit, Radiance)

	// Should return albedo for perfect reflection
	if !brdf.Equals(albedo) {
		t.Errorf("BRDF for perfect reflection should equal albedo: expected %v, got %v", albedo, brdf)
	}

	// Test non-reflection direction
	wrongDir := core.NewVec3(0, 1, 1).Normalize() // Not the reflection direction
	brdf = metal.EvaluateBRDF(incomingDir, wrongDir, hit, Radiance)

	expected := core.Vec3{X: 0, Y: 0, Z: 0}
	if !brdf.Equals(expected) {
		t.Errorf("BRDF for non-reflection direction should be zero: expected %v, got %v", expected, brdf)
	}
}

func TestMetal_EvaluateBRDF_TransportModeInvariance(t *testing.T) {
	// Test that transport mode doesn't affect metal BRDF (it's symmetric)
	albedo := core.NewVec3(0.8, 0.6, 0.4)
	metal := NewMetal(albedo, 0.0)

	hit := &SurfaceInteraction{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1),
	}

	incomingDir := core.NewVec3(1, 1, -1).Normalize()
	// For BRDF: reflect(-incomingDir, normal) = reflect((-1,-1,1), (0,0,1)) = (-1, -1, -1)
	outgoingDir := core.NewVec3(-1, -1, -1).Normalize() // Perfect reflection

	brdfRadiance := metal.EvaluateBRDF(incomingDir, outgoingDir, hit, Radiance)
	brdfImportance := metal.EvaluateBRDF(incomingDir, outgoingDir, hit, Importance)

	// Should be identical for symmetric BRDF
	if !brdfRadiance.Equals(brdfImportance) {
		t.Errorf("Metal BRDF should be invariant to transport mode: Radiance=%v, Importance=%v",
			brdfRadiance, brdfImportance)
	}
}

func TestMetal_PDF_AlwaysZero(t *testing.T) {
	metal := NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.5)

	normal := core.NewVec3(0, 0, 1)

	testCases := []struct {
		incoming core.Vec3
		outgoing core.Vec3
	}{
		{core.NewVec3(1, 0, -1).Normalize(), core.NewVec3(1, 0, 1).Normalize()}, // Perfect reflection
		{core.NewVec3(0, 1, -1).Normalize(), core.NewVec3(0, 1, 1).Normalize()}, // Different angle
		{core.NewVec3(1, 1, -1).Normalize(), core.NewVec3(0, 1, 1).Normalize()}, // Non-reflection
	}

	for i, tc := range testCases {
		pdf, valid := metal.PDF(tc.incoming, tc.outgoing, normal)

		if !valid {
			t.Errorf("Test case %d: PDF should always be valid for metal", i)
		}
		if pdf != 0.0 {
			t.Errorf("Test case %d: Metal PDF should always be 0 (delta function), got %f", i, pdf)
		}
	}
}

func TestReflectFunction(t *testing.T) {
	// Test the reflection utility function with various cases
	tests := []struct {
		name     string
		incident core.Vec3
		normal   core.Vec3
		expected core.Vec3
	}{
		{
			name:     "45 degree reflection",
			incident: core.NewVec3(1, 0, -1).Normalize(),
			normal:   core.NewVec3(0, 0, 1),
			expected: core.NewVec3(1, 0, 1).Normalize(),
		},
		{
			name:     "Normal incidence",
			incident: core.NewVec3(0, 0, -1),
			normal:   core.NewVec3(0, 0, 1),
			expected: core.NewVec3(0, 0, 1),
		},
		{
			name:     "Grazing incidence",
			incident: core.NewVec3(1, 0, -0.01).Normalize(),
			normal:   core.NewVec3(0, 0, 1),
			expected: core.NewVec3(1, 0, 0.01).Normalize(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := reflect(tt.incident, tt.normal)
			tolerance := 1e-10
			if result.Subtract(tt.expected).Length() > tolerance {
				t.Errorf("Reflection failed: expected %v, got %v", tt.expected, result)
			}
		})
	}
}
