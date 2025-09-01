package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Scene contains all the elements needed for rendering
type Scene struct {
	Camera         *geometry.Camera
	Shapes         []geometry.Shape    // Objects in the scene
	Lights         []lights.Light      // Lights in the scene
	LightSampler   lights.LightSampler // Light sampler
	SamplingConfig SamplingConfig
	CameraConfig   geometry.CameraConfig
	BVH            *geometry.BVH // Acceleration structure for ray-object intersection
}

// SamplingConfig contains rendering configuration
type SamplingConfig struct {
	Width                     int     // Image width
	Height                    int     // Image height
	SamplesPerPixel           int     // Number of rays per pixel
	MaxDepth                  int     // Maximum ray bounce depth
	RussianRouletteMinBounces int     // Minimum bounces before Russian Roulette can activate
	AdaptiveMinSamples        float64 // Minimum samples as percentage of max samples (0.0-1.0)
	AdaptiveThreshold         float64 // Relative error threshold for adaptive convergence (0.01 = 1%)
}

// NewGroundQuad creates a large quad to replace infinite ground planes
// Creates a horizontal quad centered at the given point with normal pointing up (0,1,0)
func NewGroundQuad(center core.Vec3, size float64, material material.Material) *geometry.Quad {
	// Create corner at bottom-left of the quad
	corner := core.NewVec3(center.X-size/2, center.Y, center.Z-size/2)
	// Edge vectors: u along X axis, v along Z axis
	// u × v = (size,0,0) × (0,0,size) = (0,size²,0) which normalizes to (0,1,0)
	u := core.NewVec3(size, 0, 0)
	v := core.NewVec3(0, 0, size)
	return geometry.NewQuad(corner, u, v, material)
}

// Preprocess prepares the scene for rendering by preprocessing all objects that need it
func (s *Scene) Preprocess() error {
	// Create the BVH
	s.BVH = geometry.NewBVH(s.Shapes)

	// Preprocess all lights that implement the Preprocessor interface
	for _, light := range s.Lights {
		if preprocessor, ok := light.(geometry.Preprocessor); ok {
			if err := preprocessor.Preprocess(s.BVH.Center, s.BVH.Radius); err != nil {
				return err
			}
		}
	}

	// Create the light sampler after lights are preprocessed
	sceneRadius := s.BVH.Radius

	// Use uniform light sampling
	if s.LightSampler == nil {
		s.LightSampler = lights.NewUniformLightSampler(s.Lights, sceneRadius)
	}
	// Alternative: weighted sampling
	//s.LightSampler = core.NewWeightedLightSampler(s.Lights, []float64{0.9, 0.1}, sceneRadius)

	// Could also preprocess shapes here in the future if needed
	for _, shape := range s.Shapes {
		if preprocessor, ok := shape.(geometry.Preprocessor); ok {
			if err := preprocessor.Preprocess(s.BVH.Center, s.BVH.Radius); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetPrimitiveCount returns the total number of primitive objects in the scene
func (s *Scene) GetPrimitiveCount() int {
	count := 0
	for _, shape := range s.Shapes {
		count += s.countPrimitivesInShape(shape)
	}
	return count
}

// countPrimitivesInShape counts primitives in a single shape, handling complex objects
func (s *Scene) countPrimitivesInShape(shape geometry.Shape) int {
	switch obj := shape.(type) {
	case *geometry.TriangleMesh:
		// Triangle meshes contain multiple triangles
		return obj.GetTriangleCount()
	default:
		// Regular shapes count as 1 primitive each
		return 1
	}
}

// AddSphereLight adds a spherical light to the scene
func (s *Scene) AddSphereLight(center core.Vec3, radius float64, emission core.Vec3) {
	emissiveMat := material.NewEmissive(emission)
	sphereLight := lights.NewSphereLight(center, radius, emissiveMat)
	s.Lights = append(s.Lights, sphereLight)
	s.Shapes = append(s.Shapes, sphereLight.Sphere)
}

// AddQuadLight adds a rectangular area light to the scene
func (s *Scene) AddQuadLight(corner, u, v core.Vec3, emission core.Vec3) {
	emissiveMat := material.NewEmissive(emission)
	quadLight := lights.NewQuadLight(corner, u, v, emissiveMat)
	s.Lights = append(s.Lights, quadLight)
	s.Shapes = append(s.Shapes, quadLight.Quad)
}

// AddSpotLight adds a disc spot light with custom cone angle and falloff
func (s *Scene) AddSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees, radius float64) {
	spotLight := lights.NewDiscSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees, radius)
	s.Lights = append(s.Lights, spotLight)
	// Add the underlying disc to shapes for caustic ray intersection
	s.Shapes = append(s.Shapes, spotLight.GetDisc())
}

// AddPointSpotLight adds a point spot light to the scene
func (s *Scene) AddPointSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees, radius float64) {
	spotLight := lights.NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)
	s.Lights = append(s.Lights, spotLight)
}

// AddUniformInfiniteLight adds a uniform infinite light to the scene
func (s *Scene) AddUniformInfiniteLight(emission core.Vec3) {
	infiniteLight := lights.NewUniformInfiniteLight(emission)
	s.Lights = append(s.Lights, infiniteLight)
}

// AddGradientInfiniteLight adds a gradient infinite light to the scene
func (s *Scene) AddGradientInfiniteLight(topColor, bottomColor core.Vec3) {
	infiniteLight := lights.NewGradientInfiniteLight(topColor, bottomColor)
	s.Lights = append(s.Lights, infiniteLight)
}
