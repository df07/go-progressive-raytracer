package scene

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewGroundQuad creates a large quad to replace infinite ground planes
// Creates a horizontal quad centered at the given point with normal pointing up (0,1,0)
func NewGroundQuad(center core.Vec3, size float64, material core.Material) *geometry.Quad {
	// Create corner at bottom-left of the quad
	corner := core.NewVec3(center.X-size/2, center.Y, center.Z-size/2)
	// Edge vectors: u along X axis, v along Z axis
	// u × v = (size,0,0) × (0,0,size) = (0,size²,0) which normalizes to (0,1,0)
	u := core.NewVec3(size, 0, 0)
	v := core.NewVec3(0, 0, size)
	return geometry.NewQuad(corner, u, v, material)
}

// Scene contains all the elements needed for rendering
type Scene struct {
	Camera         core.Camera
	Shapes         []core.Shape      // Objects in the scene
	Lights         []core.Light      // Lights in the scene
	LightSampler   core.LightSampler // Light sampler
	SamplingConfig core.SamplingConfig
	CameraConfig   core.CameraConfig
	BVH            *core.BVH // Acceleration structure for ray-object intersection
}

// NewDefaultScene creates a default scene with spheres, ground, and camera
func NewDefaultScene(cameraOverrides ...core.CameraConfig) *Scene {
	// Default camera configuration
	defaultCameraConfig := core.CameraConfig{
		Center:        core.NewVec3(0, 0.75, 2), // Position camera higher and farther back
		LookAt:        core.NewVec3(0, 0.5, -1), // Look at the sphere center
		Up:            core.NewVec3(0, 1, 0),    // Standard up direction
		Width:         400,
		AspectRatio:   16.0 / 9.0,
		VFov:          40.0, // Narrower field of view for focus effect
		Aperture:      0.05, // Strong depth of field blur
		FocusDistance: 0.0,  // Auto-calculate focus distance
	}

	// Apply any overrides using the reusable merge function
	cameraConfig := defaultCameraConfig
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(defaultCameraConfig, cameraOverrides[0])
	}

	camera := geometry.NewCamera(cameraConfig)

	samplingConfig := core.SamplingConfig{
		SamplesPerPixel:           200,
		MaxDepth:                  50,
		RussianRouletteMinBounces: 20,   // Need a lot of bounces for complex glass
		AdaptiveMinSamples:        0.15, // 15% of max samples minimum for adaptive sampling
		AdaptiveThreshold:         0.01, // 1% relative error threshold
	}

	s := &Scene{
		Camera:         camera,
		Shapes:         make([]core.Shape, 0),
		Lights:         make([]core.Light, 0),
		SamplingConfig: samplingConfig,
		CameraConfig:   cameraConfig,
	}

	// Create materials
	lambertianGreen := material.NewLambertian(core.NewVec3(0.8, 0.8, 0.0).Multiply(0.6))
	lambertianBlue := material.NewLambertian(core.NewVec3(0.1, 0.2, 0.5))
	lambertianRed := material.NewLambertian(core.NewVec3(0.65, 0.25, 0.2))
	metalSilver := material.NewMetal(core.NewVec3(0.8, 0.8, 0.8), 0.0)
	metalGold := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.3)
	materialGlass := material.NewDielectric(1.5)

	// Create layered material: glass coating over gold base
	coatedRed := material.NewLayered(materialGlass, lambertianRed)

	// Create spheres with different materials
	sphereCenter := geometry.NewSphere(core.NewVec3(0, 0.5, -1), 0.5, coatedRed)
	sphereLeft := geometry.NewSphere(core.NewVec3(-1, 0.5, -1), 0.5, metalSilver)
	sphereRight := geometry.NewSphere(core.NewVec3(1, 0.5, -1), 0.5, metalGold)
	solidGlassSphere := geometry.NewSphere(core.NewVec3(0.5, 0.25, -0.5), 0.25, materialGlass)

	// Create ground quad instead of infinite plane (large but finite for proper bounds)
	groundQuad := NewGroundQuad(core.NewVec3(0, 0, 0), 10000.0, lambertianGreen)

	// Create hollow glass sphere with blue sphere inside
	hollowGlassOuter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.25, materialGlass)
	hollowGlassInner := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), -0.24, materialGlass)
	hollowGlassCenter := geometry.NewSphere(core.NewVec3(-0.5, 0.25, -0.5), 0.20, lambertianBlue)

	// Add objects to the scene
	groundOnly := false
	if groundOnly {
		s.Shapes = append(s.Shapes, groundQuad)
	} else {
		// Add the specified light: pos [30, 30.5, 15], r: 10, emit: [15.0, 14.0, 13.0]
		s.AddSphereLight(
			core.NewVec3(30, 30.5, 15),     // position
			10,                             // radius
			core.NewVec3(15.0, 14.0, 13.0), // emission
		)
		s.Shapes = append(s.Shapes, sphereCenter, sphereLeft, sphereRight, groundQuad,
			solidGlassSphere, hollowGlassOuter, hollowGlassInner, hollowGlassCenter)
	}

	// Add gradient infinite light (replaces background gradient)
	s.AddGradientInfiniteLight(
		core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
		core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white ground)
	)

	return s
}

// Interface methods removed - access fields directly: s.Camera, s.Lights, s.BVH, etc.

// Preprocess prepares the scene for rendering by preprocessing all objects that need it
func (s *Scene) Preprocess() error {
	// Create the BVH
	s.BVH = core.NewBVH(s.Shapes)

	// Preprocess all lights that implement the Preprocessor interface
	for _, light := range s.Lights {
		if preprocessor, ok := light.(core.Preprocessor); ok {
			if err := preprocessor.Preprocess(s.BVH); err != nil {
				return err
			}
		}
	}

	// Create the light sampler after lights are preprocessed
	sceneRadius := s.BVH.Radius

	// Use uniform light sampling
	if s.LightSampler == nil {
		s.LightSampler = core.NewUniformLightSampler(s.Lights, sceneRadius)
	}
	// Alternative: weighted sampling
	//s.LightSampler = core.NewWeightedLightSampler(s.Lights, []float64{0.9, 0.1}, sceneRadius)

	// Could also preprocess shapes here in the future if needed
	for _, shape := range s.Shapes {
		if preprocessor, ok := shape.(core.Preprocessor); ok {
			if err := preprocessor.Preprocess(s.BVH); err != nil {
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
func (s *Scene) countPrimitivesInShape(shape core.Shape) int {
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
	sphereLight := geometry.NewSphereLight(center, radius, emissiveMat)
	s.Lights = append(s.Lights, sphereLight)
	s.Shapes = append(s.Shapes, sphereLight.Sphere)
}

// AddQuadLight adds a rectangular area light to the scene
func (s *Scene) AddQuadLight(corner, u, v core.Vec3, emission core.Vec3) {
	emissiveMat := material.NewEmissive(emission)
	quadLight := geometry.NewQuadLight(corner, u, v, emissiveMat)
	s.Lights = append(s.Lights, quadLight)
	s.Shapes = append(s.Shapes, quadLight.Quad)
}

// AddSpotLight adds a disc spot light with custom cone angle and falloff
func (s *Scene) AddSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees, radius float64) {
	spotLight := geometry.NewDiscSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees, radius)
	s.Lights = append(s.Lights, spotLight)
	// Add the underlying disc to shapes for caustic ray intersection
	s.Shapes = append(s.Shapes, spotLight.GetDisc())
}

// AddPointSpotLight adds a point spot light to the scene
func (s *Scene) AddPointSpotLight(from, to, emission core.Vec3, coneAngleDegrees, coneDeltaAngleDegrees, radius float64) {
	spotLight := geometry.NewPointSpotLight(from, to, emission, coneAngleDegrees, coneDeltaAngleDegrees)
	s.Lights = append(s.Lights, spotLight)
}

// AddUniformInfiniteLight adds a uniform infinite light to the scene
func (s *Scene) AddUniformInfiniteLight(emission core.Vec3) {
	infiniteLight := geometry.NewUniformInfiniteLight(emission)
	s.Lights = append(s.Lights, infiniteLight)
}

// AddGradientInfiniteLight adds a gradient infinite light to the scene
func (s *Scene) AddGradientInfiniteLight(topColor, bottomColor core.Vec3) {
	infiniteLight := geometry.NewGradientInfiniteLight(topColor, bottomColor)
	s.Lights = append(s.Lights, infiniteLight)
}
