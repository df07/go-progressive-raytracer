package material

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestLayeredBasicBehavior(t *testing.T) {
	// Create simple materials for testing
	redLambertian := NewLambertian(core.NewVec3(0.8, 0.1, 0.1))  // Red diffuse
	blueLambertian := NewLambertian(core.NewVec3(0.1, 0.1, 0.8)) // Blue diffuse

	// Create layered material: red outer, blue inner
	layered := NewLayered(redLambertian, blueLambertian)

	// Create test ray and hit record
	ray := core.Ray{
		Origin:    core.NewVec3(0, 1, 0),
		Direction: core.NewVec3(0, -1, 0), // Ray going straight down
	}

	hit := core.HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0), // Normal pointing up
		T:         1.0,
		FrontFace: true,
		Material:  layered,
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	result, scattered := layered.Scatter(ray, hit, sampler)

	// Basic checks
	if !scattered {
		t.Error("Layered material should scatter")
	}

	// Check that attenuation combines both materials
	// Should be multiplicative combination of red and blue
	if result.Attenuation.X <= 0 || result.Attenuation.Z <= 0 {
		t.Error("Expected combined attenuation from both layers")
	}

	// The combined red*blue should reduce both components
	maxComponent := math.Max(math.Max(result.Attenuation.X, result.Attenuation.Y), result.Attenuation.Z)
	if maxComponent > 0.5 { // Should be significantly attenuated
		t.Errorf("Expected significant attenuation, got max component: %.3f", maxComponent)
	}
}

func TestLayeredOutwardScattering(t *testing.T) {
	// Create materials where outer reflects upward
	metalMaterial := NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0) // Perfect mirror
	redLambertian := NewLambertian(core.NewVec3(0.8, 0.1, 0.1))

	layered := NewLayered(metalMaterial, redLambertian)

	// Ray coming from above at an angle that should reflect outward
	ray := core.Ray{
		Origin:    core.NewVec3(-1, 1, 0),
		Direction: core.NewVec3(1, -1, 0).Normalize(), // 45 degree angle
	}

	hit := core.HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0),
		T:         1.0,
		FrontFace: true,
		Material:  layered,
	}

	sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))
	result, scattered := layered.Scatter(ray, hit, sampler)

	if !scattered {
		t.Error("Should scatter")
	}

	// For a perfect mirror reflecting a 45-degree ray, the result should go upward
	if result.Scattered.Direction.Y <= 0 {
		t.Error("Expected outward reflection (upward), but got inward direction")
	}

	// Should only have outer material attenuation (metallic)
	expectedAttenuation := core.NewVec3(0.9, 0.9, 0.9)
	tolerance := 0.1
	if math.Abs(result.Attenuation.X-expectedAttenuation.X) > tolerance ||
		math.Abs(result.Attenuation.Y-expectedAttenuation.Y) > tolerance ||
		math.Abs(result.Attenuation.Z-expectedAttenuation.Z) > tolerance {
		t.Errorf("Expected outer-only attenuation %v, got %v", expectedAttenuation, result.Attenuation)
	}
}

func TestLayeredInwardScattering(t *testing.T) {
	// Create a glass outer layer that refracts inward, with lambertian inner
	glass := NewDielectric(1.5)
	redLambertian := NewLambertian(core.NewVec3(0.8, 0.1, 0.1))

	layered := NewLayered(glass, redLambertian)

	// Ray that should refract inward through glass
	ray := core.Ray{
		Origin:    core.NewVec3(0, 1, 0),
		Direction: core.NewVec3(0, -1, 0), // Straight down - should refract inward
	}

	hit := core.HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0),
		T:         1.0,
		FrontFace: true,
		Material:  layered,
	}

	// Test multiple times to catch cases where glass refracts inward
	foundInwardScattering := false

	for seed := int64(0); seed < 100; seed++ {
		sampler := core.NewRandomSampler(rand.New(rand.NewSource(seed)))
		result, scattered := layered.Scatter(ray, hit, sampler)

		if !scattered {
			continue
		}

		// Check if we got a combination (both materials interacted)
		// This would show up as attenuation that's not pure white (from glass only)
		if result.Attenuation.X < 0.9 || result.Attenuation.Y < 0.9 || result.Attenuation.Z < 0.9 {
			foundInwardScattering = true

			// Verify it's a combination of glass (white) and red lambertian
			if result.Attenuation.X <= result.Attenuation.Y || result.Attenuation.X <= result.Attenuation.Z {
				t.Errorf("Expected red-tinted result from layered interaction, got %v", result.Attenuation)
			}
			break
		}
	}

	if !foundInwardScattering {
		t.Error("Expected to find cases where light penetrates to inner layer")
	}
}

func TestLayeredConstructor(t *testing.T) {
	// Test that the constructor works correctly
	redLambertian := NewLambertian(core.NewVec3(0.8, 0.1, 0.1))
	blueLambertian := NewLambertian(core.NewVec3(0.1, 0.1, 0.8))

	layered := NewLayered(redLambertian, blueLambertian)

	if layered.Outer != redLambertian {
		t.Error("Outer material not set correctly")
	}

	if layered.Inner != blueLambertian {
		t.Error("Inner material not set correctly")
	}
}
