# Infinite Lights in BDPT: Implementation Specification

## Current State Analysis

### Background Handling
Currently, infinite lights (background gradients) are handled as a special case:
- Background gradients computed via `BackgroundGradient()` method in integrators
- Not part of the scene's `GetLights()` array
- Background vertices created ad-hoc in BDPT with `IsInfiniteLight: true`
- MIS calculation uses early return workaround for infinite lights

### Architectural Limitation
The fundamental issue is that our infinite lights are **background gradients**, not proper **Light objects**. This means:
- Only accessible via path tracing (s=0 strategies)
- Cannot be sampled for light path generation (s>0 strategies)
- MIS calculation lacks competing strategies to balance

## Proposed Solution: True Infinite Light Objects

### 1. Core Light Interface Extensions

#### New Light Types
```go
const (
    LightTypeArea              LightType = "area"
    LightTypePoint             LightType = "point"
    LightTypeUniformInfinite   LightType = "uniform_infinite"   // NEW
    LightTypeGradientInfinite  LightType = "gradient_infinite"  // NEW
)
```

#### Separate Infinite Light Implementations

##### Uniform Infinite Light
```go
type UniformInfiniteLight struct {
    emission    Vec3    // Uniform emission color
    worldCenter Vec3    // Finite scene center from BVH
    worldRadius float64 // Finite scene radius from BVH
}

func (uil *UniformInfiniteLight) Type() LightType {
    return LightTypeUniformInfinite
}
```

##### Gradient Infinite Light  
```go
type GradientInfiniteLight struct {
    topColor    Vec3    // Top gradient color
    bottomColor Vec3    // Bottom gradient color  
    worldCenter Vec3    // Finite scene center from BVH (consistent with uniform)
    worldRadius float64 // Finite scene radius from BVH (consistent with uniform)
}

func (gil *GradientInfiniteLight) Type() LightType {
    return LightTypeGradientInfinite
}

// Emission function for gradient
func (gil *GradientInfiniteLight) emissionForDirection(direction Vec3) Vec3 {
    t := 0.5 * (direction.Y + 1.0) // Map Y from [-1,1] to [0,1]
    return gil.bottomColor.Multiply(1.0-t).Add(gil.topColor.Multiply(t))
}
```

### 2. Light Interface Method Implementations

#### Sample Method (Direct Lighting)
```go
func (il *InfiniteLight) Sample(point Vec3, sample Vec2) LightSample {
    // For infinite lights, we sample a direction uniformly on the sphere
    // and treat it as coming from infinite distance
    
    direction := uniformSampleSphere(sample)
    emission := il.emissionFunc(direction)
    
    return LightSample{
        Point:     point.Add(direction.Multiply(1e10)), // Far away point
        Normal:    direction.Multiply(-1),              // Points toward scene
        Direction: direction,
        Distance:  math.Inf(1),
        Emission:  emission,
        PDF:       1.0 / (4.0 * math.Pi), // Uniform over sphere
    }
}
```

#### PDF Method (Direct Lighting)
```go
func (il *InfiniteLight) PDF(point Vec3, direction Vec3) float64 {
    // Uniform sampling over sphere
    return 1.0 / (4.0 * math.Pi)
}
```

#### SampleEmission Method (Light Path Generation)  
```go
func (il *InfiniteLight) SampleEmission(samplePoint Vec2, sampleDirection Vec2) EmissionSample {
    // For BDPT light path generation, we need to:
    // 1. Sample a direction uniformly on the sphere
    // 2. Find where this direction intersects the scene bounding sphere
    // 3. Create emission ray from that point toward the scene
    
    direction := uniformSampleSphere(sampleDirection)
    emission := il.emissionFunc(direction)
    
    // Find scene center and create ray from scene boundary
    // Use consistent finite scene bounds from BVH
    emissionPoint := il.worldCenter.Add(direction.Multiply(-il.worldRadius))
    
    return EmissionSample{
        Point:        emissionPoint,
        Normal:       direction, // Points toward scene
        Direction:    direction,
        Emission:     emission,
        AreaPDF:      1.0 / (math.Pi * il.worldRadius * il.worldRadius), // PBRT: planar density
        DirectionPDF: 1.0 / (4.0 * math.Pi), // Uniform over sphere
    }
}
```

#### EmissionPDF Method (MIS Calculations)
```go
func (il *InfiniteLight) EmissionPDF(point Vec3, direction Vec3) float64 {
    // PBRT: For infinite lights, return planar sampling density
    return 1.0 / (math.Pi * il.worldRadius * il.worldRadius)
}
```

### 3. Scene Integration

#### Scene Interface Updates
```go
type Scene interface {
    GetCamera() Camera
    GetBackgroundColors() (topColor, bottomColor Vec3) // Keep for compatibility
    GetShapes() []Shape
    GetLights() []Light                                // Now includes infinite lights
    GetSamplingConfig() SamplingConfig
    GetBVH() *BVH
}
```

#### Scene Implementation
```go
func (s *Scene) AddUniformInfiniteLight(emission Vec3) {
    bvh := s.GetBVH()
    infiniteLight := &UniformInfiniteLight{
        emission:    emission,
        worldCenter: bvh.FiniteWorldCenter, // Use consistent scene bounds
        worldRadius: bvh.FiniteWorldRadius,
    }
    s.Lights = append(s.Lights, infiniteLight)
}

func (s *Scene) AddGradientInfiniteLight(topColor, bottomColor Vec3) {
    bvh := s.GetBVH()
    infiniteLight := &GradientInfiniteLight{
        topColor:    topColor,
        bottomColor: bottomColor,
        worldCenter: bvh.FiniteWorldCenter, // Use consistent scene bounds
        worldRadius: bvh.FiniteWorldRadius,
    }
    s.Lights = append(s.Lights, infiniteLight)
}

// Filter infinite lights from GetLights() instead of separate method
func GetInfiniteLights(lights []Light) []Light {
    infiniteLights := []Light{}
    for _, light := range lights {
        if light.Type() == LightTypeUniformInfinite || light.Type() == LightTypeGradientInfinite {
            infiniteLights = append(infiniteLights, light)
        }
    }
    return infiniteLights
}
```

### 4. BDPT Integration Changes

#### Light Path Generation Updates
```go
func (bdpt *BDPTIntegrator) generateLightSubpath(scene core.Scene, sampler core.Sampler, maxBounces int) Path {
    lights := scene.GetLights() // Now includes infinite lights
    if len(lights) == 0 {
        return Path{Length: 0}
    }
    
    // Uniform light selection (same as before)
    lightIndex := int(sampler.Get1D() * float64(len(lights)))
    sampledLight := lights[lightIndex]
    lightSelectionPdf := 1.0 / float64(len(lights))
    
    // Sample emission (now works for infinite lights too)
    emissionSample := sampledLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
    // ... rest unchanged
}
```

#### Vertex Creation for Infinite Lights
```go
func createInfiniteLightVertex(light Light, emissionSample EmissionSample, lightSelectionPdf float64) Vertex {
    return Vertex{
        Point:           emissionSample.Point,
        Normal:          emissionSample.Normal,
        Light:           light,
        AreaPdfForward:  emissionSample.AreaPDF * lightSelectionPdf,
        IsLight:         true,
        IsInfiniteLight: light.Type() == LightTypeInfinite,
        Beta:            emissionSample.Emission,
        EmittedLight:    emissionSample.Emission,
    }
}
```

### 5. MIS Calculation Updates

#### Remove Early Return Workaround
```go
func (bdpt *BDPTIntegrator) calculateMISWeight(...) float64 {
    if s+t == 2 {
        return 1.0
    }
    
    // REMOVE: Early return for infinite lights
    // Now all strategies are properly supported
    
    // ... rest of MIS calculation unchanged
}
```

#### PDF Calculation Updates
```go
func (bdpt *BDPTIntegrator) calculateLightPdf(curr *Vertex, to *Vertex, scene core.Scene) float64 {
    if curr.IsLight {
        if curr.IsInfiniteLight {
            // Use consistent finite scene bounds from BVH
            bvh := scene.GetBVH()
            direction := to.Point.Subtract(curr.Point).Normalize()
            return curr.Light.EmissionPDF(curr.Point, direction)
        } else {
            // ... existing finite light PDF calculation
        }
    }
    // ... rest unchanged
}
```

## Implementation Strategy

### Phase 1: Path Tracing Foundation
1. **Extend Light interface** with separate UniformInfiniteLight and GradientInfiniteLight types
2. **Implement both infinite light structs** with consistent scene bounds
3. **Update Scene to include infinite lights** in GetLights() 
4. **Test with path tracing integrator first** (simpler, no MIS complications)
5. **Validate infinite light sampling** works correctly for s=0 strategies

### Phase 2: BDPT Integration  
1. **Modify light path generation** to handle infinite lights
2. **Update vertex creation** for infinite light sources
3. **Test light path strategies** (s=1, s=2, etc.) with infinite lights
4. **Ensure consistent scene bounds** usage throughout

### Phase 3: MIS Fixes
1. **Remove early return workaround** from MIS calculation
2. **Update PDF calculations** for infinite lights using consistent bounds
3. **Test MIS weights** across all strategies

### Phase 4: Migration and Cleanup
1. **Convert one scene** (e.g., default) to use infinite lights instead of background gradient
2. **Test backward compatibility** with existing background gradient system
3. **Gradually migrate other scenes** as needed
4. **Eventually remove background gradient** system entirely

### Phase 5: Advanced Features
1. **Environment map support** instead of just gradients
2. **Light importance sampling** for complex environments
3. **Multiple infinite lights** support

## Benefits of This Approach

### Proper BDPT Support
- **All strategies work**: s=0 (path tracing), s=1 (direct lighting), s=2+ (bidirectional)
- **Correct MIS weights**: Proper balance between competing strategies
- **Better convergence**: Light tracing strategies can now reach infinite lights

### Architectural Consistency
- **Unified light handling**: Infinite lights are proper Light objects
- **Clean separation**: Background gradients become emission functions
- **Extensible design**: Easy to add environment maps, multiple infinite lights

### Performance Benefits
- **Better importance sampling**: Can sample infinite lights directly when beneficial
- **Reduced noise**: MIS properly weights different strategies
- **Faster convergence**: Especially for scenes dominated by environment lighting

## Migration Path

### Backward Compatibility
- Keep `GetBackgroundColors()` for existing scenes during transition
- Keep existing MIS early return workaround for background gradients
- Existing scenes continue to work unchanged during implementation

### Gradual Migration
1. **Add infinite light support** alongside existing background system
2. **Start with path tracing** to validate infinite light implementation
3. **Convert one scene** (e.g., default scene) to use GradientInfiniteLight
4. **Test BDPT integration** with the converted scene
5. **Migrate remaining scenes incrementally** to use InfiniteLight objects
6. **Remove MIS early return workaround** once all scenes use proper infinite lights
7. **Eventually deprecate** background gradient methods entirely

### Incremental Testing Strategy
- Test each phase independently with existing test suite
- Compare results between background gradient and infinite light implementations
- Ensure no regressions in existing functionality during transition

## Example Usage

```go
// Create scene with infinite light
scene := scene.NewScene(...)

// Add gradient infinite light (replaces background gradient)
scene.AddGradientInfiniteLight(
    core.NewVec3(0.5, 0.7, 1.0), // topColor (blue sky)
    core.NewVec3(1.0, 1.0, 1.0), // bottomColor (white ground)
)

// Or add uniform infinite light
scene.AddUniformInfiniteLight(
    core.NewVec3(0.8, 0.8, 0.8), // uniform white emission
)

// BDPT now automatically supports all strategies for infinite lights
bdpt := NewBDPTIntegrator(config)
result := bdpt.RayColor(ray, scene, sampler)
```

This specification provides a complete roadmap for implementing infinite lights as first-class citizens in the BDPT framework, enabling proper MIS calculations and all bidirectional strategies.