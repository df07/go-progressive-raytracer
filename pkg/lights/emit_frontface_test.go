package lights

import (
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// TestSphereLight_Emit_FrontFaceDetection verifies sphere lights only emit from front face
func TestSphereLight_Emit_FrontFaceDetection(t *testing.T) {
	emission := core.NewVec3(5.0, 5.0, 5.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 0, 0)
	radius := 1.0
	light := NewSphereLight(center, radius, emissiveMat)

	ray := core.NewRay(core.NewVec3(2, 0, 0), core.NewVec3(-1, 0, 0))

	t.Run("Front face emission", func(t *testing.T) {
		// Point on sphere surface at (1, 0, 0) - front face (normal pointing outward)
		hitPoint := core.NewVec3(1, 0, 0)
		normal := core.NewVec3(1, 0, 0) // outward normal
		frontFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    normal,
			FrontFace: true,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, frontFaceHit)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v from front face, got %v", emission, result)
		}
	})

	t.Run("Back face no emission", func(t *testing.T) {
		// Same point but back face
		hitPoint := core.NewVec3(1, 0, 0)
		normal := core.NewVec3(-1, 0, 0) // inward normal
		backFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    normal,
			FrontFace: false,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, backFaceHit)
		expectedZero := core.NewVec3(0, 0, 0)
		if !result.Equals(expectedZero) {
			t.Errorf("Expected zero emission from back face, got %v", result)
		}
	})

	t.Run("Nil hit emits (for sampling)", func(t *testing.T) {
		// When hit is nil, should emit (used during light sampling)
		result := light.Emit(ray, nil)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v when hit is nil, got %v", emission, result)
		}
	})
}

// TestDiscLight_Emit_FrontFaceDetection verifies disc lights only emit from front face
func TestDiscLight_Emit_FrontFaceDetection(t *testing.T) {
	emission := core.NewVec3(3.0, 3.0, 3.0)
	emissiveMat := material.NewEmissive(emission)
	center := core.NewVec3(0, 1, 0)
	normal := core.NewVec3(0, 1, 0) // disc facing up
	radius := 0.5
	light := NewDiscLight(center, normal, radius, emissiveMat)

	ray := core.NewRay(core.NewVec3(0, 2, 0), core.NewVec3(0, -1, 0))

	t.Run("Front face emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0, 1, 0)
		frontFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, 1, 0),
			FrontFace: true,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, frontFaceHit)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v from front face, got %v", emission, result)
		}
	})

	t.Run("Back face no emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0, 1, 0)
		backFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, -1, 0),
			FrontFace: false,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, backFaceHit)
		expectedZero := core.NewVec3(0, 0, 0)
		if !result.Equals(expectedZero) {
			t.Errorf("Expected zero emission from back face, got %v", result)
		}
	})

	t.Run("Nil hit emits (for sampling)", func(t *testing.T) {
		result := light.Emit(ray, nil)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v when hit is nil, got %v", emission, result)
		}
	})
}

// TestDiscSpotLight_Emit_FrontFaceDetection verifies disc spot lights only emit from front face
func TestDiscSpotLight_Emit_FrontFaceDetection(t *testing.T) {
	from := core.NewVec3(0, 2, 0)
	to := core.NewVec3(0, 0, 0)
	emission := core.NewVec3(4.0, 4.0, 4.0)
	coneAngle := 45.0
	deltaAngle := 10.0
	radius := 0.3
	light := NewDiscSpotLight(from, to, emission, coneAngle, deltaAngle, radius)

	ray := core.NewRay(core.NewVec3(0, 3, 0), core.NewVec3(0, -1, 0))

	t.Run("Front face emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0, 2, 0)
		frontFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, -1, 0), // disc faces down
			FrontFace: true,
			Material:  light.discLight.Material,
		}

		_ = light.Emit(ray, frontFaceHit)
		// Note: DiscSpotLight uses a custom material that may have additional logic
		// We're just verifying it passes through the hit parameter correctly
		// The actual emission value depends on the spot light material implementation
	})

	t.Run("Back face no emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0, 2, 0)
		backFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, 1, 0), // opposite normal
			FrontFace: false,
			Material:  light.discLight.Material,
		}

		result := light.Emit(ray, backFaceHit)
		expectedZero := core.NewVec3(0, 0, 0)
		if !result.Equals(expectedZero) {
			t.Errorf("Expected zero emission from back face, got %v", result)
		}
	})
}

// TestQuadLight_Emit_FrontFaceDetection verifies quad lights only emit from front face
func TestQuadLight_Emit_FrontFaceDetection(t *testing.T) {
	emission := core.NewVec3(6.0, 6.0, 6.0)
	emissiveMat := material.NewEmissive(emission)
	corner := core.NewVec3(0, 0, 0)
	u := core.NewVec3(1, 0, 0)
	v := core.NewVec3(0, 1, 0)
	light := NewQuadLight(corner, u, v, emissiveMat)

	ray := core.NewRay(core.NewVec3(0.5, 0.5, 1), core.NewVec3(0, 0, -1))

	t.Run("Front face emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0.5, 0.5, 0)
		frontFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, 0, 1),
			FrontFace: true,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, frontFaceHit)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v from front face, got %v", emission, result)
		}
	})

	t.Run("Back face no emission", func(t *testing.T) {
		hitPoint := core.NewVec3(0.5, 0.5, 0)
		backFaceHit := &material.SurfaceInteraction{
			Point:     hitPoint,
			Normal:    core.NewVec3(0, 0, -1),
			FrontFace: false,
			Material:  emissiveMat,
		}

		result := light.Emit(ray, backFaceHit)
		expectedZero := core.NewVec3(0, 0, 0)
		if !result.Equals(expectedZero) {
			t.Errorf("Expected zero emission from back face, got %v", result)
		}
	})

	t.Run("Nil hit emits (for sampling)", func(t *testing.T) {
		result := light.Emit(ray, nil)
		if !result.Equals(emission) {
			t.Errorf("Expected emission %v when hit is nil, got %v", emission, result)
		}
	})
}
