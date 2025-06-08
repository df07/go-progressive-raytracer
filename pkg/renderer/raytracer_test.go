package renderer

import (
	"math"
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// MockMaterial implements core.Material for testing
type MockMaterial struct {
	scatterFn func(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool)
}

func (m MockMaterial) Scatter(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
	return m.scatterFn(rayIn, hit, random)
}

// MockShape implements core.Shape for testing
type MockShape struct {
	hitFn func(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool)
}

func (m MockShape) Hit(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
	return m.hitFn(ray, tMin, tMax)
}

// MockScene implements core.Scene for testing
type MockScene struct {
	camera          *Camera
	shapes          []core.Shape
	backgroundColor core.Vec3
	lights          []core.Light
}

func (m MockScene) GetCamera() core.Camera  { return m.camera }
func (m MockScene) GetShapes() []core.Shape { return m.shapes }
func (m MockScene) GetBackgroundColors() (core.Vec3, core.Vec3) {
	return m.backgroundColor, m.backgroundColor
}
func (m MockScene) GetLights() []core.Light { return m.lights }

func TestRaytracer_DiffuseColorCalculation(t *testing.T) {
	// Create a scene with a single diffuse surface
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          45.0,
		Aperture:      0.0,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)
	random := rand.New(rand.NewSource(42))

	// Create a mock material that always scatters with known values
	material := &MockMaterial{
		scatterFn: func(rayIn core.Ray, hit core.HitRecord, random *rand.Rand) (core.ScatterResult, bool) {
			// Return predictable values for testing
			return core.ScatterResult{
				Scattered: core.NewRay(
					core.NewVec3(0, 0, 0),
					core.NewVec3(0, 0, 1), // Straight up
				),
				Attenuation: core.NewVec3(0.5/math.Pi, 0.5/math.Pi, 0.5/math.Pi), // BRDF = albedo/π
				PDF:         1.0 / math.Pi,                                       // Cosine-weighted PDF
			}, true
		},
	}

	// Create a mock shape that returns a hit for the initial ray but not for the scattered ray
	shape := &MockShape{
		hitFn: func(ray core.Ray, tMin, tMax float64) (*core.HitRecord, bool) {
			// Only hit on the initial ray, not the scattered ray
			if ray.Direction.Y < 0 { // Initial ray coming from above
				return &core.HitRecord{
					Point:     core.NewVec3(0, 0, 0),
					Normal:    core.NewVec3(0, 0, 1),
					Material:  material,
					T:         1.0,
					FrontFace: true,
				}, true
			}
			return nil, false // Let scattered ray hit background
		},
	}

	scene := &MockScene{
		camera:          camera,
		shapes:          []core.Shape{shape},
		backgroundColor: core.NewVec3(1, 1, 1), // White background
	}

	raytracer := NewRaytracer(scene, 800, 600)

	// Test diffuse color calculation
	scatter := core.ScatterResult{
		Scattered:   core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1)),
		Attenuation: core.NewVec3(0.5/math.Pi, 0.5/math.Pi, 0.5/math.Pi), // BRDF = albedo/π
		PDF:         1.0 / math.Pi,
	}
	hit := &core.HitRecord{
		Point:     core.NewVec3(0, 0, 0),
		Normal:    core.NewVec3(0, 0, 1),
		Material:  material,
		T:         1.0,
		FrontFace: true,
	}

	// Test that PDF is properly used in Monte Carlo integration
	color := raytracer.calculateDiffuseColor(scatter, hit, 2, core.NewVec3(1, 1, 1), 0, random)

	// For cosine-weighted sampling with albedo=0.5:
	// - cosine = 1.0 (straight up)
	// - PDF = cosθ/π = 1/π
	// - BRDF = albedo/π = 0.5/π
	// - Background = 1.0
	// Monte Carlo: (BRDF * cosθ * incomingLight) / PDF
	//            = (0.5/π * 1.0 * 1.0) / (1/π)
	//            = 0.5
	expectedColor := core.NewVec3(0.5, 0.5, 0.5)
	tolerance := 1e-3

	if math.Abs(color.X-expectedColor.X) > tolerance ||
		math.Abs(color.Y-expectedColor.Y) > tolerance ||
		math.Abs(color.Z-expectedColor.Z) > tolerance {
		t.Errorf("Incorrect diffuse color calculation: got %v, expected %v", color, expectedColor)
	}
}

func TestRaytracer_SpecularColorCalculation(t *testing.T) {
	// Create a scene with a single specular surface
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          45.0,
		Aperture:      0.0,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)

	scene := &MockScene{
		camera:          camera,
		shapes:          []core.Shape{},
		backgroundColor: core.NewVec3(1, 1, 1),
	}

	raytracer := NewRaytracer(scene, 800, 600)
	random := rand.New(rand.NewSource(42))

	// Test specular color calculation
	scatter := core.ScatterResult{
		Scattered:   core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(0, 0, 1)),
		Attenuation: core.NewVec3(0.8, 0.8, 0.8),
		PDF:         0,
	}

	// For specular reflection:
	// Result = attenuation * incoming = 0.8 * 1.0 = 0.8
	color := raytracer.calculateSpecularColor(scatter, 5, core.NewVec3(1, 1, 1), 0, random)
	expectedColor := core.NewVec3(0.8, 0.8, 0.8)
	tolerance := 1e-3

	if math.Abs(color.X-expectedColor.X) > tolerance ||
		math.Abs(color.Y-expectedColor.Y) > tolerance ||
		math.Abs(color.Z-expectedColor.Z) > tolerance {
		t.Errorf("Incorrect specular color calculation: got %v, expected %v", color, expectedColor)
	}
}

func TestRaytracer_RecursiveRayColor(t *testing.T) {
	config := CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          45.0,
		Aperture:      0.0,
		FocusDistance: 1.0,
	}
	camera := NewCamera(config)

	// Test depth limiting
	scene := &MockScene{
		camera:          camera,
		shapes:          []core.Shape{},
		backgroundColor: core.NewVec3(1, 1, 1),
	}

	raytracer := NewRaytracer(scene, 800, 600)
	ray := core.NewRay(core.NewVec3(0, 0, -1), core.NewVec3(0, 0, 1))
	random := rand.New(rand.NewSource(42))

	// At depth 0, should return black
	color := raytracer.rayColorRecursive(ray, 0, core.NewVec3(1, 1, 1), 0, random)
	if color.X != 0 || color.Y != 0 || color.Z != 0 {
		t.Errorf("Expected black at depth 0, got %v", color)
	}

	// Test background color when no intersection
	color = raytracer.rayColorRecursive(ray, 5, core.NewVec3(1, 1, 1), 0, random)
	expectedColor := core.NewVec3(1, 1, 1) // White background
	tolerance := 1e-3

	if math.Abs(color.X-expectedColor.X) > tolerance ||
		math.Abs(color.Y-expectedColor.Y) > tolerance ||
		math.Abs(color.Z-expectedColor.Z) > tolerance {
		t.Errorf("Incorrect background color: got %v, expected %v", color, expectedColor)
	}
}
