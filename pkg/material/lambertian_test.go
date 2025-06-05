package material

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestLambertian_PDFCalculation(t *testing.T) {
	albedo := core.NewVec3(0.8, 0.8, 0.8)
	lambertian := NewLambertian(albedo)
	random := rand.New(rand.NewSource(42))

	// Normal pointing up (z-axis)
	normal := core.NewVec3(0, 0, 1)
	hit := core.HitRecord{
		Point:  core.NewVec3(0, 0, 0),
		Normal: normal,
	}
	ray := core.NewRay(core.NewVec3(0, 0, 1), core.NewVec3(0, 0, -1))

	// Test that PDF calculation matches expected formula
	for i := 0; i < 100; i++ {
		scatter, didScatter := lambertian.Scatter(ray, hit, random)
		if !didScatter {
			t.Fatal("Lambertian should always scatter")
		}

		// Verify PDF calculation matches expected formula
		scatterDirection := scatter.Scattered.Direction.Normalize()
		cosTheta := scatterDirection.Dot(normal)
		expectedPDF := cosTheta / math.Pi
		tolerance := 1e-10
		if math.Abs(scatter.PDF-expectedPDF) > tolerance {
			t.Errorf("PDF mismatch: got %f, expected %f", scatter.PDF, expectedPDF)
		}
	}
}

func TestLambertian_EnergyConservation(t *testing.T) {
	albedo := core.NewVec3(0.5, 0.7, 0.9)
	lambertian := NewLambertian(albedo)
	random := rand.New(rand.NewSource(42))

	hit := core.HitRecord{
		Point:  core.NewVec3(0, 0, 0),
		Normal: core.NewVec3(0, 0, 1),
	}
	ray := core.NewRay(core.NewVec3(0, 0, 1), core.NewVec3(0, 0, -1))

	scatter, didScatter := lambertian.Scatter(ray, hit, random)
	if !didScatter {
		t.Fatal("Lambertian should always scatter")
	}

	// BRDF should be albedo/π
	expectedBRDF := albedo.Multiply(1.0 / math.Pi)
	tolerance := 1e-10
	if math.Abs(scatter.Attenuation.X-expectedBRDF.X) > tolerance ||
		math.Abs(scatter.Attenuation.Y-expectedBRDF.Y) > tolerance ||
		math.Abs(scatter.Attenuation.Z-expectedBRDF.Z) > tolerance {
		t.Errorf("BRDF mismatch: got %v, expected %v", scatter.Attenuation, expectedBRDF)
	}

	// Attenuation should never exceed original albedo values
	if scatter.Attenuation.X > albedo.X ||
		scatter.Attenuation.Y > albedo.Y ||
		scatter.Attenuation.Z > albedo.Z {
		t.Errorf("BRDF %v exceeds albedo %v (energy violation)", scatter.Attenuation, albedo)
	}
}
