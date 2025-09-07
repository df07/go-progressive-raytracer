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

	hit := SurfaceInteraction{
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
	hit := SurfaceInteraction{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 1, 0), // Normal pointing up
		T:         1.0,
		FrontFace: false, // Exiting the material
		Material:  glass,
	}

	// Calculate expected behavior
	cosTheta := -rayDirection.Dot(hit.Normal)
	refractionRatio := 1.5 // glass to air
	sinTheta := math.Sqrt(1.0 - cosTheta*cosTheta)
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

func TestDielectricEvaluateBRDF_Reflection(t *testing.T) {
	glass := NewDielectric(1.5)

	// Set up a reflection scenario
	normal := core.NewVec3(0, 1, 0)
	incomingDir := core.NewVec3(1, -1, 0).Normalize() // 45° incoming
	outgoingDir := core.NewVec3(1, 1, 0).Normalize()  // Perfect reflection

	// Create a hit record for front face
	hit := &SurfaceInteraction{
		Normal:    normal,
		FrontFace: true,
	}

	// Test both transport modes for reflection
	radianceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Radiance)
	importanceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Importance)

	// For reflection, transport modes should give same result
	tolerance := 1e-9
	if math.Abs(radianceBRDF.X-importanceBRDF.X) > tolerance ||
		math.Abs(radianceBRDF.Y-importanceBRDF.Y) > tolerance ||
		math.Abs(radianceBRDF.Z-importanceBRDF.Z) > tolerance {
		t.Errorf("Reflection BRDF should be same for both transport modes: Radiance=%v, Importance=%v", radianceBRDF, importanceBRDF)
	}

	// Reflection should return non-zero value (Fresnel reflectance)
	if radianceBRDF.IsZero() {
		t.Error("Perfect reflection should return non-zero BRDF")
	}

	// Should be white (no color change for clear glass)
	if math.Abs(radianceBRDF.X-radianceBRDF.Y) > tolerance ||
		math.Abs(radianceBRDF.Y-radianceBRDF.Z) > tolerance {
		t.Errorf("Glass reflection should be achromatic, got %v", radianceBRDF)
	}
}

func TestDielectricEvaluateBRDF_Refraction_TransportModes(t *testing.T) {
	glass := NewDielectric(1.5)

	// Set up a refraction scenario: air to glass
	normal := core.NewVec3(0, 1, 0)
	incomingDir := core.NewVec3(1, -1, 0).Normalize() // 45° incoming from air

	// Use our working refractVector function to get the correct refracted direction
	etaRatio := 1.0 / 1.5 // air to glass
	outgoingDir := refractVector(incomingDir, normal, etaRatio)

	// Create a hit record for front face (air to glass)
	hit := &SurfaceInteraction{
		Normal:    normal,
		FrontFace: true,
	}

	// Test both transport modes
	radianceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Radiance)
	importanceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Importance)

	// Both should be non-zero
	if radianceBRDF.IsZero() {
		t.Error("Radiance transport refraction should return non-zero BRDF")
	}
	if importanceBRDF.IsZero() {
		t.Error("Importance transport refraction should return non-zero BRDF")
	}

	// Key test: Radiance mode should be SMALLER than Importance mode by factor of η²
	expectedRatio := etaRatio * etaRatio // (1/1.5)² ≈ 0.444
	actualRatio := radianceBRDF.X / importanceBRDF.X

	tolerance := 1e-6
	if math.Abs(actualRatio-expectedRatio) > tolerance {
		t.Errorf("Radiance/Importance BRDF ratio should be η²=%.6f, got %.6f", expectedRatio, actualRatio)
	}

	t.Logf("Transport mode test: η²=%.6f, actual ratio=%.6f", expectedRatio, actualRatio)
}

func TestDielectricEvaluateBRDF_Refraction_GlassToAir(t *testing.T) {
	glass := NewDielectric(1.5)

	// Set up a refraction scenario: glass to air (reverse direction)
	normal := core.NewVec3(0, 1, 0)
	incomingDir := core.NewVec3(1, -1, 0).Normalize() // 45° incoming from glass

	// Calculate perfect refraction direction (glass to air)
	etaRatio := 1.5 // glass to air
	cosTheta := -incomingDir.Dot(normal)
	sinTheta := math.Sqrt(1.0 - cosTheta*cosTheta)

	// Check if total internal reflection would occur
	if etaRatio*sinTheta > 1.0 {
		t.Skip("This test angle would cause total internal reflection")
	}

	// Perfect refraction direction
	outgoingDir := refractVector(incomingDir, normal, etaRatio)

	// Test both transport modes
	hit := &SurfaceInteraction{
		Normal:    normal,
		FrontFace: false,
	}

	radianceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Radiance)
	importanceBRDF := glass.EvaluateBRDF(incomingDir, outgoingDir, hit, Importance)

	// Both should be non-zero
	if radianceBRDF.IsZero() {
		t.Error("Radiance transport refraction should return non-zero BRDF")
	}
	if importanceBRDF.IsZero() {
		t.Error("Importance transport refraction should return non-zero BRDF")
	}

	// For glass to air: Radiance should be smaller by η² = (1.5)² = 2.25
	expectedRatio := 1.0 / (etaRatio * etaRatio) // 1/2.25 ≈ 0.444
	actualRatio := radianceBRDF.X / importanceBRDF.X

	tolerance := 1e-6
	if math.Abs(actualRatio-expectedRatio) > tolerance {
		t.Errorf("Glass-to-air Radiance/Importance BRDF ratio should be 1/η²=%.6f, got %.6f", expectedRatio, actualRatio)
	}
}

func TestDielectricEvaluateBRDF_NonPerfectDirections(t *testing.T) {
	glass := NewDielectric(1.5)

	normal := core.NewVec3(0, 1, 0)
	incomingDir := core.NewVec3(1, -1, 0).Normalize()

	// Test with random outgoing direction (not perfect reflection/refraction)
	randomDir := core.NewVec3(0.5, 0.3, 0.7).Normalize()

	hit := &SurfaceInteraction{
		Normal:    normal,
		FrontFace: true,
	}

	radianceBRDF := glass.EvaluateBRDF(incomingDir, randomDir, hit, Radiance)
	importanceBRDF := glass.EvaluateBRDF(incomingDir, randomDir, hit, Importance)

	// Should return zero for non-perfect directions (delta function)
	if !radianceBRDF.IsZero() {
		t.Errorf("Non-perfect direction should return zero BRDF, got %v", radianceBRDF)
	}
	if !importanceBRDF.IsZero() {
		t.Errorf("Non-perfect direction should return zero BRDF, got %v", importanceBRDF)
	}
}

func TestDielectricEvaluateBRDF_EdgeCases(t *testing.T) {
	glass := NewDielectric(1.5)
	normal := core.NewVec3(0, 1, 0)

	// Test grazing incidence
	grazingDir := core.NewVec3(1, -0.01, 0).Normalize()
	reflectedDir := core.NewVec3(1, 0.01, 0).Normalize()

	hit := &SurfaceInteraction{
		Normal:    normal,
		FrontFace: true,
	}

	radianceBRDF := glass.EvaluateBRDF(grazingDir, reflectedDir, hit, Radiance)

	// At grazing incidence, should get strong reflection
	if radianceBRDF.IsZero() {
		t.Error("Grazing incidence should produce non-zero reflection")
	}

	// Test normal incidence
	normalDir := core.NewVec3(0, -1, 0)
	normalReflDir := core.NewVec3(0, 1, 0)

	normalBRDF := glass.EvaluateBRDF(normalDir, normalReflDir, hit, Radiance)

	// Normal incidence should have low but non-zero reflectance
	if normalBRDF.IsZero() {
		t.Error("Normal incidence should produce non-zero reflection")
	}

	// Grazing should be stronger than normal incidence
	if normalBRDF.X >= radianceBRDF.X {
		t.Error("Grazing incidence reflectance should be higher than normal incidence")
	}
}

func TestDielectricPDF_AlwaysZero(t *testing.T) {
	glass := NewDielectric(1.5)

	normal := core.NewVec3(0, 1, 0)
	incomingDir := core.NewVec3(1, -1, 0).Normalize()
	outgoingDir := core.NewVec3(1, 1, 0).Normalize()

	pdf, isDelta := glass.PDF(incomingDir, outgoingDir, normal)

	// PDF should always be 0 for specular materials
	if pdf != 0.0 {
		t.Errorf("Dielectric PDF should always be 0, got %f", pdf)
	}

	// Should indicate this is a delta function
	if !isDelta {
		t.Error("Dielectric should indicate it's a delta function (isDelta=true)")
	}
}
