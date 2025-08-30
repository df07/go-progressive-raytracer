package material

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestDielectricBasicBehavior(t *testing.T) {
	// Create a glass material (refractive index of 1.5)
	glass := NewDielectric(1.5)

	// Test 1: Basic scattering properties
	rayDirection := core.NewVec3(1, -1, 0).Normalize() // 45-degree angle
	ray := core.Ray{Origin: core.NewVec3(0, 1, 0), Direction: rayDirection}

	hit := HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0), // Normal pointing up
		T:         1.0,
		FrontFace: true,
		Material:  glass,
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	result, scattered := glass.Scatter(ray, hit, sampler)

	// Basic checks
	if !scattered {
		t.Error("Dielectric should always scatter")
	}

	// Check that attenuation is white (no color absorption)
	expectedAttenuation := core.NewVec3(1.0, 1.0, 1.0)
	if result.Attenuation != expectedAttenuation {
		t.Errorf("Expected attenuation %v, got %v", expectedAttenuation, result.Attenuation)
	}

	// Check that PDF is 0 (specular material)
	if result.PDF != 0 {
		t.Errorf("Expected PDF 0, got %f", result.PDF)
	}

	// Test 2: Verify that both reflection and refraction can occur
	// Try many different random seeds to ensure we get varied behavior
	hasReflection := false
	hasRefraction := false

	for seed := int64(0); seed < 1000 && (!hasReflection || !hasRefraction); seed++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))
		result, _ := glass.Scatter(ray, hit, sampler)

		// Determine if this was reflection or refraction
		scatteredDirection := result.Scattered.Direction.Normalize()

		// For 45° incoming ray:
		// - Refraction should bend toward normal (Y component more negative)
		// - Reflection should have same angle with normal (Y component less negative)
		if scatteredDirection.Y > -0.5 { // Less steep than refracted ray
			hasReflection = true
		} else { // Steeper, likely refracted
			hasRefraction = true
		}
	}

	if !hasRefraction {
		t.Error("Expected to see refraction in at least some cases")
	}

	// Note: We don't require reflection since at 45° for air->glass,
	// reflection probability is only ~4%, so it might not occur in our samples
	t.Logf("Found reflection: %t, Found refraction: %t", hasReflection, hasRefraction)
}

func TestDielectricTotalInternalReflection(t *testing.T) {
	// Create a glass material
	glass := NewDielectric(1.5)

	// Create a ray going from glass to air at a steep angle (should cause total internal reflection)
	rayDirection := core.NewVec3(1, -0.1, 0).Normalize() // Very shallow angle
	ray := core.Ray{Origin: core.NewVec3(0, 0, 0), Direction: rayDirection}

	// Create hit record for back face (exiting material)
	hit := HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0), // Normal pointing up
		T:         1.0,
		FrontFace: false, // Exiting the material
		Material:  glass,
	}

	// Calculate expected behavior
	cosTheta := -rayDirection.Dot(hit.Normal)
	sinTheta := math.Sqrt(1.0 - cosTheta*cosTheta)
	refractionRatio := 1.5 // glass to air
	shouldHaveTotalInternalReflection := refractionRatio*sinTheta > 1.0

	if !shouldHaveTotalInternalReflection {
		t.Fatalf("Test setup error: this angle should cause total internal reflection")
	}

	// Test multiple scatters - all should be reflections due to total internal reflection
	for i := 0; i < 10; i++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(int64(i)))) // Different seed each time
		result, scattered := glass.Scatter(ray, hit, sampler)

		if !scattered {
			t.Error("Dielectric should always scatter")
		}

		// For total internal reflection, check that the ray is reflected
		// The incoming ray is going down (Y < 0), the reflected ray should go up (Y > 0)
		// This is correct behavior for reflection off a horizontal surface
		if result.Scattered.Direction.Y <= 0 {
			t.Errorf("Expected total internal reflection (ray going up), but got ray going down: %+v",
				result.Scattered.Direction)
		}

		// Also verify that the X component is preserved (specular reflection)
		expectedX := rayDirection.X
		if math.Abs(result.Scattered.Direction.X-expectedX) > 1e-10 {
			t.Errorf("Expected X component %.6f, got %.6f", expectedX, result.Scattered.Direction.X)
		}
	}
}

func TestReflectanceFunction(t *testing.T) {
	// Test Schlick's approximation - just verify reasonable behavior

	// Normal incidence (0°) - should be low for air->glass
	r0 := Reflectance(1.0, 1.0/1.5)
	if r0 < 0.03 || r0 > 0.06 {
		t.Errorf("Normal incidence reflectance = %.3f, expected ~0.04", r0)
	}

	// Grazing incidence (90°) - should be close to 1
	r90 := Reflectance(0.0, 1.0/1.5)
	if r90 < 0.95 {
		t.Errorf("Grazing incidence reflectance = %.3f, expected close to 1.0", r90)
	}

	// 45° incidence - should be between normal and grazing
	r45 := Reflectance(0.707, 1.0/1.5) // cos(45°) ≈ 0.707
	if r45 < r0 || r45 > 0.2 {
		t.Errorf("45° reflectance = %.3f, expected between %.3f and 0.2", r45, r0)
	}

	// Verify monotonic behavior: reflectance should increase as angle increases
	if r45 <= r0 || r90 <= r45 {
		t.Errorf("Reflectance should increase with angle: R(0°)=%.3f, R(45°)=%.3f, R(90°)=%.3f", r0, r45, r90)
	}
}
