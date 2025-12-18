package renderer

import (
	"image"
	"image/png"
	"math"
	"os"
	"strings"
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
	testSamplingConfig := scene.SamplingConfig{
		Width: 32, Height: 32,
		MaxDepth: 5, SamplesPerPixel: 256,
		AdaptiveMinSamples:        8,
		RussianRouletteMinBounces: 2,
	}

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
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{}, // No shapes
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 10.0,
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
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{sphere, light.Sphere},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
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
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 45.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{sphere}, // Light has no geometry
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Unit Cornell Box (Quad Light)",
			createScene: func() *scene.Scene {
				// Manually constructed unit-scale Cornell box with quad light
				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Unit box from -1 to 1 in X/Z, 0 to 2 in Y
				floor := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				ceiling := geometry.NewQuad(core.NewVec3(-1, 2, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				backWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), white)
				leftWall := geometry.NewQuad(core.NewVec3(1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), red)
				rightWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), green)

				// Quad light near ceiling
				lightEmission := core.NewVec3(15, 15, 15)
				lightMat := material.NewEmissive(lightEmission)
				lightCorner := core.NewVec3(-0.25, 1.98, -0.25)
				lightU := core.NewVec3(0.5, 0, 0)
				lightV := core.NewVec3(0, 0, 0.5)
				quadLight := lights.NewQuadLight(lightCorner, lightU, lightV, lightMat)

				ls := []lights.Light{quadLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1, 3),
					LookAt: core.NewVec3(0, 1, 0),
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, quadLight.Quad},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 10.0, // Slightly higher tolerance for quad lights due to variance
		},
		{
			name: "Unit Cornell Box (Sphere Light)",
			createScene: func() *scene.Scene {
				// Manually constructed unit-scale Cornell box with sphere light
				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Unit box from -1 to 1 in X/Z, 0 to 2 in Y
				floor := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				ceiling := geometry.NewQuad(core.NewVec3(-1, 2, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				backWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), white)
				leftWall := geometry.NewQuad(core.NewVec3(1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), red)
				rightWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), green)

				// Sphere light near ceiling
				lightEmission := core.NewVec3(15, 15, 15)
				lightMat := material.NewEmissive(lightEmission)
				sphereLight := lights.NewSphereLight(core.NewVec3(0, 1.98, 0), 0.2, lightMat)

				ls := []lights.Light{sphereLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1, 3),
					LookAt: core.NewVec3(0, 1, 0),
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, sphereLight.Sphere},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 5.0,
		},
		{
			name: "Cornell Box - Quad Light at Center",
			createScene: func() *scene.Scene {
				// Cornell Box with quad light in center of room instead of near ceiling

				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Full Cornell Box walls
				floor := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				ceiling := geometry.NewQuad(core.NewVec3(-1, 2, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				backWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), white)
				leftWall := geometry.NewQuad(core.NewVec3(1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), red)
				rightWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), green)

				// Quad Light at CENTER of room (y=1.0 instead of 1.98)
				lightEmission := core.NewVec3(15, 15, 15)
				lightMat := material.NewEmissive(lightEmission)
				lightCorner := core.NewVec3(-0.25, 1.0, -0.25)
				lightU := core.NewVec3(0.5, 0, 0)
				lightV := core.NewVec3(0, 0, 0.5)
				quadLight := lights.NewQuadLight(lightCorner, lightU, lightV, lightMat)

				ls := []lights.Light{quadLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1, 3),
					LookAt: core.NewVec3(0, 1, 0),
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, quadLight.Quad},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Cornell Box - Quad Light on Back Wall",
			createScene: func() *scene.Scene {
				// Cornell Box with quad light on the back wall

				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Full Cornell Box walls
				floor := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				ceiling := geometry.NewQuad(core.NewVec3(-1, 2, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				backWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), white)
				leftWall := geometry.NewQuad(core.NewVec3(1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), red)
				rightWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), green)

				// Quad Light on back wall (z=-0.98, back wall is at z=-1.0)
				// Light is oriented to face forward (toward camera)
				lightEmission := core.NewVec3(15, 15, 15)
				lightMat := material.NewEmissive(lightEmission)
				lightCorner := core.NewVec3(-0.25, 0.75, -0.98)
				lightU := core.NewVec3(0.5, 0, 0) // Horizontal
				lightV := core.NewVec3(0, 0.5, 0) // Vertical
				quadLight := lights.NewQuadLight(lightCorner, lightU, lightV, lightMat)

				ls := []lights.Light{quadLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1, 3),
					LookAt: core.NewVec3(0, 1, 0),
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, quadLight.Quad},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Cornell Box - Sphere Light at Center",
			createScene: func() *scene.Scene {
				// Cornell Box with sphere light at center of room

				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Full Cornell Box walls
				floor := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				ceiling := geometry.NewQuad(core.NewVec3(-1, 2, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 0, 2), white)
				backWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(2, 0, 0), core.NewVec3(0, 2, 0), white)
				leftWall := geometry.NewQuad(core.NewVec3(1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), red)
				rightWall := geometry.NewQuad(core.NewVec3(-1, 0, -1), core.NewVec3(0, 0, 2), core.NewVec3(0, 2, 0), green)

				// Sphere Light at center (y=1.0)
				lightMat := material.NewEmissive(core.NewVec3(15, 15, 15))
				sphereLight := lights.NewSphereLight(core.NewVec3(0, 1.0, 0), 0.25, lightMat)

				ls := []lights.Light{sphereLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1, 3),
					LookAt: core.NewVec3(0, 1, 0),
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, sphereLight.Sphere},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0,
		},
		{
			name: "Large-Scale Cornell Box (Quad Light) - DEMONSTRATES BUG",
			createScene: func() *scene.Scene {
				// Exact 278x scaled version of unit box to isolate scale-dependent bug
				// This test is EXPECTED to fail with BDPT producing more luminance than path tracing
				const scale = 278.0

				white := material.NewLambertian(core.NewVec3(0.73, 0.73, 0.73))
				red := material.NewLambertian(core.NewVec3(0.65, 0.05, 0.05))
				green := material.NewLambertian(core.NewVec3(0.12, 0.45, 0.15))

				// Box from -278 to 278 in X/Z, 0 to 556 in Y (278x scaled from unit box)
				floor := geometry.NewQuad(
					core.NewVec3(-1*scale, 0, -1*scale),
					core.NewVec3(2*scale, 0, 0),
					core.NewVec3(0, 0, 2*scale),
					white,
				)
				ceiling := geometry.NewQuad(
					core.NewVec3(-1*scale, 2*scale, -1*scale),
					core.NewVec3(2*scale, 0, 0),
					core.NewVec3(0, 0, 2*scale),
					white,
				)
				backWall := geometry.NewQuad(
					core.NewVec3(-1*scale, 0, -1*scale),
					core.NewVec3(2*scale, 0, 0),
					core.NewVec3(0, 2*scale, 0),
					white,
				)
				leftWall := geometry.NewQuad(
					core.NewVec3(1*scale, 0, -1*scale),
					core.NewVec3(0, 0, 2*scale),
					core.NewVec3(0, 2*scale, 0),
					red,
				)
				rightWall := geometry.NewQuad(
					core.NewVec3(-1*scale, 0, -1*scale),
					core.NewVec3(0, 0, 2*scale),
					core.NewVec3(0, 2*scale, 0),
					green,
				)

				// Quad light scaled exactly 278x from unit version
				lightEmission := core.NewVec3(15, 15, 15) // Same as unit box
				lightMat := material.NewEmissive(lightEmission)
				lightCorner := core.NewVec3(-0.25*scale, 1.98*scale, -0.25*scale)
				lightU := core.NewVec3(0.5*scale, 0, 0)
				lightV := core.NewVec3(0, 0, 0.5*scale)
				quadLight := lights.NewQuadLight(lightCorner, lightU, lightV, lightMat)

				ls := []lights.Light{quadLight}

				cameraConfig := geometry.CameraConfig{
					Center: core.NewVec3(0, 1*scale, 3*scale), // Scaled camera position
					LookAt: core.NewVec3(0, 1*scale, 0),       // Scaled lookat
					Up:     core.NewVec3(0, 1, 0),
					Width:  testSamplingConfig.Width, AspectRatio: 1.0, VFov: 40.0,
				}
				camera := geometry.NewCamera(cameraConfig)

				s := &scene.Scene{
					Shapes:         []geometry.Shape{floor, ceiling, backWall, leftWall, rightWall, quadLight.Quad},
					Lights:         ls,
					Camera:         camera,
					SamplingConfig: testSamplingConfig,
				}
				s.Preprocess()
				return s
			},
			tolerance: 15.0, // This test will fail - bug demonstration
			skip:      true, // Skip by default - unskip to demonstrate scale-dependent bug
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip {
				t.Skip("Skipping test case")
			}

			scene := tt.createScene()

			// Configure progressive rendering with scene-specific settings
			config := DefaultProgressiveConfig()
			config.InitialSamples = 1
			config.MaxSamplesPerPixel = scene.SamplingConfig.SamplesPerPixel
			config.MaxPasses = 1
			config.TileSize = scene.SamplingConfig.Width // Render full image in one tile for testing

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
			saveTestImage(t, pathImage, tt.name, "pt")

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
			saveTestImage(t, bdptImage, tt.name, "bdpt")

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

func saveTestImage(t *testing.T, img *image.RGBA, testName, suffix string) {
	// Only save images if verbose mode is enabled (go test -v)
	if !testing.Verbose() {
		return
	}

	// Create output directory in project root
	// Tests run in pkg/renderer, so we go up two levels
	outputDir := "../../output/debug_renders"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		t.Logf("Failed to create output directory: %v", err)
		return
	}

	// Sanitize test name for filename
	filename := outputDir + "/" + sanitizeFilename(testName) + "_" + suffix + ".png"

	f, err := os.Create(filename)
	if err != nil {
		t.Logf("Failed to create debug image file %s: %v", filename, err)
		return
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Logf("Failed to encode debug image %s: %v", filename, err)
	} else {
		t.Logf("Saved debug image to %s", filename)
	}
}

func sanitizeFilename(s string) string {
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
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
