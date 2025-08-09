package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// MockScene for tile renderer testing
type MockScene struct {
	width       int
	height      int
	shapes      []core.Shape
	lights      []core.Light
	topColor    core.Vec3
	bottomColor core.Vec3
	camera      core.Camera
	config      core.SamplingConfig
	bvh         *core.BVH
}

func (m *MockScene) GetWidth() int                               { return m.width }
func (m *MockScene) GetHeight() int                              { return m.height }
func (m *MockScene) GetCamera() core.Camera                      { return m.camera }
func (m *MockScene) GetBackgroundColors() (core.Vec3, core.Vec3) { return m.topColor, m.bottomColor }
func (m *MockScene) GetShapes() []core.Shape                     { return m.shapes }
func (m *MockScene) GetLights() []core.Light                     { return m.lights }
func (m *MockScene) GetSamplingConfig() core.SamplingConfig      { return m.config }
func (m *MockScene) GetBVH() *core.BVH                           { return m.bvh }
func (m *MockScene) Preprocess() error                           { return nil }

func TestGradientInfiniteLight_Type(t *testing.T) {
	light := NewGradientInfiniteLight(
		core.NewVec3(0, 0, 1), // blue
		core.NewVec3(1, 1, 1), // white
	)

	if light.Type() != core.LightTypeInfinite {
		t.Errorf("Expected LightTypeInfinite, got %v", light.Type())
	}
}

func TestGradientInfiniteLight_EmissionForDirection(t *testing.T) {
	topColor := core.NewVec3(0, 0, 1)    // blue
	bottomColor := core.NewVec3(1, 1, 1) // white

	light := NewGradientInfiniteLight(topColor, bottomColor)

	tests := []struct {
		direction core.Vec3
		expected  core.Vec3
		name      string
	}{
		{core.NewVec3(0, 1, 0), topColor, "Top direction"},                     // Y = 1, should get top color
		{core.NewVec3(0, -1, 0), bottomColor, "Bottom direction"},              // Y = -1, should get bottom color
		{core.NewVec3(0, 0, 1), core.NewVec3(0.5, 0.5, 1), "Middle direction"}, // Y = 0, should get middle
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := light.emissionForDirection(tt.direction)
			if !result.Equals(tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGradientInfiniteLight_Sample(t *testing.T) {
	topColor := core.NewVec3(1, 0, 0)    // red
	bottomColor := core.NewVec3(0, 1, 0) // green

	light := NewGradientInfiniteLight(topColor, bottomColor)

	point := core.NewVec3(0, 0, 0)
	sample := core.NewVec2(0.5, 0.5)

	lightSample := light.Sample(point, sample)

	// Check that distance is infinite
	if !math.IsInf(lightSample.Distance, 1) {
		t.Errorf("Expected infinite distance, got %f", lightSample.Distance)
	}

	// Check that PDF is uniform over sphere
	expectedPDF := 1.0 / (4.0 * math.Pi)
	if math.Abs(lightSample.PDF-expectedPDF) > 1e-10 {
		t.Errorf("Expected PDF %f, got %f", expectedPDF, lightSample.PDF)
	}

	// Check that direction is normalized
	dirLength := lightSample.Direction.Length()
	if math.Abs(dirLength-1.0) > 1e-10 {
		t.Errorf("Expected normalized direction, got length %f", dirLength)
	}

	// Check that emission is valid gradient value
	expectedEmission := light.emissionForDirection(lightSample.Direction)
	if !lightSample.Emission.Equals(expectedEmission) {
		t.Errorf("Expected emission %v, got %v", expectedEmission, lightSample.Emission)
	}

	// Check that normal points toward scene (opposite to direction)
	expectedNormal := lightSample.Direction.Multiply(-1)
	if !lightSample.Normal.Equals(expectedNormal) {
		t.Errorf("Expected normal %v, got %v", expectedNormal, lightSample.Normal)
	}
}

func TestGradientInfiniteLight_PDF(t *testing.T) {
	light := NewGradientInfiniteLight(
		core.NewVec3(1, 0, 0),
		core.NewVec3(0, 1, 0),
	)

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(0, 1, 0)

	pdf := light.PDF(point, direction)
	expectedPDF := 1.0 / (4.0 * math.Pi)

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected PDF %f, got %f", expectedPDF, pdf)
	}
}

func TestGradientInfiniteLight_SampleEmission(t *testing.T) {
	topColor := core.NewVec3(1, 0, 1)    // magenta
	bottomColor := core.NewVec3(0, 1, 1) // cyan
	worldCenter := core.NewVec3(1, 2, 3)
	worldRadius := 15.0

	light := NewGradientInfiniteLight(topColor, bottomColor)
	light.Preprocess(&MockScene{
		bvh: &core.BVH{
			Center: worldCenter,
			Radius: worldRadius,
		},
	})

	samplePoint := core.NewVec2(0.3, 0.7)
	sampleDirection := core.NewVec2(0.1, 0.9)

	emissionSample := light.SampleEmission(samplePoint, sampleDirection)

	// Check emission matches gradient calculation
	expectedEmission := light.emissionForDirection(emissionSample.Direction)
	if !emissionSample.Emission.Equals(expectedEmission) {
		t.Errorf("Expected emission %v, got %v", expectedEmission, emissionSample.Emission)
	}

	// Check area PDF
	expectedAreaPDF := 1.0 / (math.Pi * worldRadius * worldRadius)
	if math.Abs(emissionSample.AreaPDF-expectedAreaPDF) > 1e-10 {
		t.Errorf("Expected area PDF %f, got %f", expectedAreaPDF, emissionSample.AreaPDF)
	}

	// Check direction PDF
	expectedDirectionPDF := 1.0 / (4.0 * math.Pi)
	if math.Abs(emissionSample.DirectionPDF-expectedDirectionPDF) > 1e-10 {
		t.Errorf("Expected direction PDF %f, got %f", expectedDirectionPDF, emissionSample.DirectionPDF)
	}

	// Check that normal points toward scene (opposite to ray direction)
	expectedNormal := emissionSample.Direction.Multiply(-1)
	if !emissionSample.Normal.Equals(expectedNormal) {
		t.Errorf("Expected normal %v (toward scene), got %v", expectedNormal, emissionSample.Normal)
	}
}

func TestGradientInfiniteLight_EmissionPDF(t *testing.T) {
	worldRadius := 25.0
	light := NewGradientInfiniteLight(
		core.NewVec3(1, 0, 0),
		core.NewVec3(0, 0, 1),
	)

	light.Preprocess(&MockScene{
		bvh: &core.BVH{Radius: worldRadius},
	})

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(1, 0, 0)

	pdf := light.EmissionPDF(point, direction)
	expectedPDF := 1.0 / (math.Pi * worldRadius * worldRadius)

	if math.Abs(pdf-expectedPDF) > 1e-10 {
		t.Errorf("Expected emission PDF %f, got %f", expectedPDF, pdf)
	}
}

func TestGradientInfiniteLight_EmissionPDF_ZeroRadius(t *testing.T) {
	worldRadius := 0.0
	light := NewGradientInfiniteLight(
		core.NewVec3(1, 0, 0),
		core.NewVec3(0, 0, 1),
	)

	light.Preprocess(&MockScene{
		bvh: &core.BVH{Radius: worldRadius},
	})

	point := core.NewVec3(0, 0, 0)
	direction := core.NewVec3(1, 0, 0)

	pdf := light.EmissionPDF(point, direction)

	if pdf != 0.0 {
		t.Errorf("Expected zero PDF for zero radius, got %f", pdf)
	}
}

func TestGradientInfiniteLight_GradientConsistency(t *testing.T) {
	// Test that the gradient interpolation works correctly across the Y range
	topColor := core.NewVec3(1, 0, 0)    // red at top (Y=1)
	bottomColor := core.NewVec3(0, 0, 1) // blue at bottom (Y=-1)

	light := NewGradientInfiniteLight(topColor, bottomColor)

	// Test specific Y values
	testCases := []struct {
		y        float64
		expected core.Vec3
		name     string
	}{
		{1.0, topColor, "Top Y=1"},
		{-1.0, bottomColor, "Bottom Y=-1"},
		{0.0, core.NewVec3(0.5, 0, 0.5), "Middle Y=0"},
		{0.5, core.NewVec3(0.75, 0, 0.25), "Upper middle Y=0.5"},
		{-0.5, core.NewVec3(0.25, 0, 0.75), "Lower middle Y=-0.5"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Don't normalize - we want to test specific Y values
			direction := core.NewVec3(0, tc.y, 0)
			result := light.emissionForDirection(direction)

			if !result.Equals(tc.expected) {
				t.Errorf("For Y=%f, expected %v, got %v", tc.y, tc.expected, result)
			}
		})
	}
}
