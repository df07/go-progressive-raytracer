package material

import (
	"math/rand"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

func TestEmissive_Scatter(t *testing.T) {
	tests := []struct {
		name     string
		emission core.Vec3
	}{
		{
			name:     "Red emission",
			emission: core.NewVec3(1.0, 0.0, 0.0),
		},
		{
			name:     "White emission",
			emission: core.NewVec3(1.0, 1.0, 1.0),
		},
		{
			name:     "Zero emission",
			emission: core.NewVec3(0.0, 0.0, 0.0),
		},
		{
			name:     "High intensity emission",
			emission: core.NewVec3(10.0, 5.0, 2.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emissive := NewEmissive(tt.emission)

			// Test that emissive materials don't scatter
			ray := core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(1, 0, 0))
			hit := HitRecord{
				Point:  core.NewVec3(1, 0, 0),
				Normal: core.NewVec3(-1, 0, 0),
				T:      1.0,
			}
			sampler := core.NewRandomSampler(rand.New(rand.NewSource(42)))

			_, scattered := emissive.Scatter(ray, hit, sampler)
			if scattered {
				t.Error("Emissive material should not scatter rays")
			}
		})
	}
}

func TestEmissive_Emit(t *testing.T) {
	const tolerance = 1e-9

	tests := []struct {
		name     string
		emission core.Vec3
	}{
		{
			name:     "Red emission",
			emission: core.NewVec3(1.0, 0.0, 0.0),
		},
		{
			name:     "White emission",
			emission: core.NewVec3(1.0, 1.0, 1.0),
		},
		{
			name:     "Zero emission",
			emission: core.NewVec3(0.0, 0.0, 0.0),
		},
		{
			name:     "High intensity emission",
			emission: core.NewVec3(10.0, 5.0, 2.0),
		},
		{
			name:     "Negative values (edge case)",
			emission: core.NewVec3(-1.0, 0.0, 0.0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			emissive := NewEmissive(tt.emission)
			emitted := emissive.Emit(core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(1, 0, 0)))

			// Verify emission matches what was set
			if abs(emitted.X-tt.emission.X) > tolerance {
				t.Errorf("Expected emission X = %f, got %f", tt.emission.X, emitted.X)
			}
			if abs(emitted.Y-tt.emission.Y) > tolerance {
				t.Errorf("Expected emission Y = %f, got %f", tt.emission.Y, emitted.Y)
			}
			if abs(emitted.Z-tt.emission.Z) > tolerance {
				t.Errorf("Expected emission Z = %f, got %f", tt.emission.Z, emitted.Z)
			}
		})
	}
}

func TestEmissive_InterfaceCompliance(t *testing.T) {
	emissive := NewEmissive(core.NewVec3(1.0, 1.0, 1.0))

	// Test that it implements Material interface
	var _ Material = emissive

	// Test that it implements Emitter interface
	var _ Emitter = emissive
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
