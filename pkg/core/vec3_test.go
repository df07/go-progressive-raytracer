package core

import (
	"math"
	"testing"
)

func TestVec3_Rotate(t *testing.T) {
	tests := []struct {
		name     string
		vector   Vec3
		rotation Vec3
		expected Vec3
	}{
		{
			name:     "No rotation",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, 0, 0),
			expected: NewVec3(1, 0, 0),
		},
		{
			name:     "90 degree rotation around Z axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, 0, math.Pi/2),
			expected: NewVec3(0, 1, 0),
		},
		{
			name:     "90 degree rotation around Y axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi/2, 0),
			expected: NewVec3(0, 0, -1),
		},
		{
			name:     "90 degree rotation around X axis",
			vector:   NewVec3(0, 1, 0),
			rotation: NewVec3(math.Pi/2, 0, 0),
			expected: NewVec3(0, 0, 1),
		},
		{
			name:     "180 degree rotation around Y axis",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi, 0),
			expected: NewVec3(-1, 0, 0),
		},
		{
			name:     "Combined rotations",
			vector:   NewVec3(1, 0, 0),
			rotation: NewVec3(0, math.Pi/2, math.Pi/2), // 90° Y then 90° Z
			expected: NewVec3(0, 0, -1),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.vector.Rotate(tt.rotation)

			const tolerance = 1e-9
			if result.Subtract(tt.expected).Length() > tolerance {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
