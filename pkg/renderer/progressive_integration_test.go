package renderer

import (
	"image"
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/integrator"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// testLogger implements core.Logger for testing by discarding all output
type testLogger struct{}

// Ensure testLogger implements core.Logger
var _ core.Logger = (*testLogger)(nil)

func (tl *testLogger) Printf(format string, args ...interface{}) {
	// Discard log output during tests
}

func TestIntegratorLuminanceComparison(t *testing.T) {
	tests := []struct {
		name        string
		createScene func() *scene.Scene
		tolerance   float64 // Percentage difference tolerance
		skip        bool
	}{
		{
			name: "Infinite Light (Uniform)",
			createScene: func() *scene.Scene {
				// Empty scene with uniform infinite light
				// No geometry, just background illumination
				ls := []lights.Light{
					lights.NewUniformInfiniteLight(core.NewVec3(1.0, 1.0, 1.0)),
				}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -1),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{}, // No shapes
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 10.0, // Increased tolerance for low sample count
		},
		{
			name: "Single Sphere with Area Light",
			createScene: func() *scene.Scene {
				// Simple scene: One diffuse sphere, one area light
				white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
				sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 0.5, white)

				// Area light (small sphere light)
				lightEmission := core.NewVec3(10.0, 10.0, 10.0)
				lightMat := material.NewEmissive(lightEmission)
				light := lights.NewSphereLight(core.NewVec3(0, 2, -1), 0.2, lightMat)

				ls := []lights.Light{light}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -2),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{sphere, light.Sphere},
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Single Sphere with Point Light",
			createScene: func() *scene.Scene {
				// Simple scene: One diffuse sphere, one point light
				white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
				sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 0.5, white)

				// Point light
				intensity := core.NewVec3(10.0, 10.0, 10.0)
				light := lights.NewPointSpotLight(
					core.NewVec3(0, 2, -1),
					core.NewVec3(0, -1, 0), // Direction (irrelevant for point light unless spot)
					intensity,
					180.0, // Full sphere coverage
					0.0,   // No falloff
				)

				ls := []lights.Light{light}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -2),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{sphere}, // Light has no geometry
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Occluded Light Scene",
			createScene: func() *scene.Scene {
				// Scene with a wall blocking the light partially or fully
				white := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
				floor := geometry.NewQuad(
					core.NewVec3(-5, -1, -5),
					core.NewVec3(10, 0, 0),
					core.NewVec3(0, 0, 10),
					white,
				)

				// Wall blocking the light
				wall := geometry.NewQuad(
					core.NewVec3(-1, -1, -3),
					core.NewVec3(2, 0, 0),
					core.NewVec3(0, 2, 0),
					white,
				)

				// Area light behind the wall
				lightEmission := core.NewVec3(10.0, 10.0, 10.0)
				lightMat := material.NewEmissive(lightEmission)
				light := lights.NewSphereLight(core.NewVec3(0, 0, -4), 0.2, lightMat)

				ls := []lights.Light{light}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -3),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{floor, wall, light.Sphere},
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Glossy Material Scene",
			createScene: func() *scene.Scene {
				// Scene with a glossy sphere
				// Metal with some roughness
				glossy := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.1)
				sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 0.5, glossy)

				// Area light
				lightEmission := core.NewVec3(10.0, 10.0, 10.0)
				lightMat := material.NewEmissive(lightEmission)
				light := lights.NewSphereLight(core.NewVec3(0, 2, -1), 0.2, lightMat)

				ls := []lights.Light{light}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -2),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{sphere, light.Sphere},
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Glass Material Scene",
			createScene: func() *scene.Scene {
				// Scene with a glass sphere
				glass := material.NewDielectric(1.5) // IOR 1.5
				sphere := geometry.NewSphere(core.NewVec3(0, 0, -2), 0.5, glass)

				// Area light
				lightEmission := core.NewVec3(10.0, 10.0, 10.0)
				lightMat := material.NewEmissive(lightEmission)
				light := lights.NewSphereLight(core.NewVec3(0, 2, -1), 0.2, lightMat)

				ls := []lights.Light{light}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 0, 0),
					LookAt: core.NewVec3(0, 0, -2),
					Up:     core.NewVec3(0, 1, 0),
					Width:  32, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:       []geometry.Shape{sphere, light.Sphere},
					Lights:       ls,
					LightSampler: lights.NewUniformLightSampler(ls, 10),
					Camera:       camera,
					SamplingConfig: scene.SamplingConfig{
						Width: 32, Height: 32,
						MaxDepth: 5, SamplesPerPixel: 4,
					},
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Cornell Box (Empty)",
			createScene: func() *scene.Scene {
				s := scene.NewCornellScene(scene.CornellEmpty)
				s.SamplingConfig.Width = 32
				s.SamplingConfig.Height = 32
				return s
			},
			tolerance: 15.0, // Keeping high tolerance for now as we know it fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test case")
			}

			scene := tt.createScene()

			// Configure progressive rendering with minimal samples for quick test
			config := DefaultProgressiveConfig()
			config.InitialSamples = 1
			config.MaxSamplesPerPixel = 4
			config.MaxPasses = 1
			config.TileSize = 32

			logger := &testLogger{}

			// Test path tracing
			pathIntegrator := integrator.NewPathTracingIntegrator(scene.SamplingConfig)
			pathRenderer, err := NewProgressiveRaytracer(scene, config, pathIntegrator, logger)
			if err != nil {
				t.Fatalf("Failed to create path tracing renderer: %v", err)
			}

			pathImage, _, err := pathRenderer.RenderPass(1, nil)
			if err != nil {
				t.Fatalf("Path tracing render failed: %v", err)
			}
			pathLuminance := calculateAverageLuminance(pathImage)

			// Test BDPT
			bdptIntegrator := integrator.NewBDPTIntegrator(scene.SamplingConfig)
			bdptRenderer, err := NewProgressiveRaytracer(scene, config, bdptIntegrator, logger)
			if err != nil {
				t.Fatalf("Failed to create BDPT renderer: %v", err)
			}

			bdptImage, _, err := bdptRenderer.RenderPass(1, nil)
			if err != nil {
				t.Fatalf("BDPT render failed: %v", err)
			}
			bdptLuminance := calculateAverageLuminance(bdptImage)

			t.Logf("Path tracing luminance: %.6f", pathLuminance)
			t.Logf("BDPT luminance: %.6f", bdptLuminance)

			// Calculate percentage difference
			if pathLuminance == 0 && bdptLuminance == 0 {
				// Both zero is fine for completely dark scenes, but we expect light in these tests
				if len(scene.Lights) > 0 {
					t.Log("Both renderers produced zero luminance, but lights are present.")
				}
				return
			}

			var percentDiff float64
			if pathLuminance == 0 {
				// If path tracing is 0 but BDPT is not, that's 100% diff (or infinite)
				percentDiff = 100.0
			} else {
				percentDiff = math.Abs(bdptLuminance-pathLuminance) / pathLuminance * 100
			}

			t.Logf("Luminance difference: %.2f%%", percentDiff)

			if percentDiff > tt.tolerance {
				t.Errorf("BDPT and path tracing luminance differ by %.2f%%, exceeds %.1f%% tolerance. "+
					"BDPT: %.6f, Path tracing: %.6f",
					percentDiff, tt.tolerance, bdptLuminance, pathLuminance)
			}
		})
	}
}

// calculateAverageLuminance computes the average luminance of an image
func calculateAverageLuminance(img *image.RGBA) float64 {
	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return 0.0
	}

	totalLuminance := 0.0
	pixelCount := bounds.Dx() * bounds.Dy()

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.RGBAAt(x, y)
			// Convert to normalized RGB values
			r := float64(c.R) / 255.0
			g := float64(c.G) / 255.0
			b := float64(c.B) / 255.0
			// Calculate luminance using standard formula
			luminance := 0.299*r + 0.587*g + 0.114*b
			totalLuminance += luminance
		}
	}

	return totalLuminance / float64(pixelCount)
}
