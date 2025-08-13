# Light Importance Sampling Specification

## Problem Statement

Current uniform light sampling causes noise when scenes contain both bright finite lights (sun) and dim infinite lights (sky). The integrator samples both lights equally, leading to inefficient use of samples and increased noise.

**Example**: Scene with bright sun (emission=100) and dim sky (emission=0.5) currently samples each 50% of the time, even though the sun contributes much more to the final image.

## Solution Overview

Implement light importance sampling to weight lights by their expected contribution to each shading point. Sample bright, nearby, well-oriented lights more frequently than dim, distant, or poorly-oriented lights.

## Phase 1: Importance-Based Sampling (IMPLEMENTED)

### Core Implementation

#### 1. Light Power Calculation
Implement power calculation following PBRT's `Phi()` method for each light type:

```go
type Light interface {
    CalculatePower(sceneRadius float64) float64
}

// Infinite lights (corrected to avoid overwhelming finite lights)
func (il *UniformInfiniteLight) CalculatePower(sceneRadius float64) float64 {
    // Modified formula: π * r² * emission (flux through a disk)
    // This prevents infinite lights from completely dominating finite lights
    avgEmission := (il.emission.X + il.emission.Y + il.emission.Z) / 3.0
    return math.Pi * sceneRadius * sceneRadius * avgEmission
}

func (gil *GradientInfiniteLight) CalculatePower(sceneRadius float64) float64 {
    avgTop := (gil.topColor.X + gil.topColor.Y + gil.topColor.Z) / 3.0
    avgBottom := (gil.bottomColor.X + gil.bottomColor.Y + gil.bottomColor.Z) / 3.0
    avgEmission := (avgTop + avgBottom) / 2.0
    // Same corrected formula as uniform infinite lights
    return math.Pi * sceneRadius * sceneRadius * avgEmission
}

// Finite lights  
func (sl *SphereLight) CalculatePower(sceneRadius float64) float64 {
    // PBRT: 4π * emission (total flux from point/sphere light)
    avgEmission := (sl.emission.X + sl.emission.Y + sl.emission.Z) / 3.0
    return 4.0 * math.Pi * avgEmission
}

func (ql *QuadLight) CalculatePower(sceneRadius float64) float64 {
    // PBRT: π * area * emission (for Lambertian area lights)
    avgEmission := getMaterialEmission(ql.Material)
    return math.Pi * ql.Area * avgEmission
}
```

#### 2. Importance-Based Light Sampling  
Build importance distribution considering both power and geometric factors:

```go
type PowerBasedLightSampler struct {
    lights      []Light
    powers      []float64
    cdf         []float64  // Static power-based CDF
    totalPower  float64
    sceneRadius float64    // Store for geometric calculations
}

func NewPowerBasedLightSampler(lights []Light, sceneRadius float64) *PowerBasedLightSampler {
    powers := make([]float64, len(lights))
    totalPower := 0.0
    
    for i, light := range lights {
        powers[i] = light.CalculatePower(sceneRadius)
        totalPower += powers[i]
    }
    
    // Build static power-based CDF (fallback)
    cdf := make([]float64, len(lights))
    if totalPower > 0 {
        cdf[0] = powers[0] / totalPower
        for i := 1; i < len(lights); i++ {
            cdf[i] = cdf[i-1] + powers[i]/totalPower
        }
    }
    
    return &PowerBasedLightSampler{lights, powers, cdf, totalPower, sceneRadius}
}

// Point-specific importance sampling (accounts for geometry)
func (pls *PowerBasedLightSampler) SampleLightImportance(point Vec3, normal Vec3, u float64) (Light, float64) {
    importances := make([]float64, len(pls.lights))
    totalImportance := 0.0
    
    for i, light := range pls.lights {
        power := pls.powers[i]
        
        // Geometric factors
        distance := 1.0
        geometricFactor := 1.0
        
        if light.Type() == LightTypeInfinite {
            // For infinite lights, use sceneRadius² to offset the radius² in power
            distance = pls.sceneRadius * pls.sceneRadius
        } else {
            // For finite lights, calculate actual distance and orientation
            lightSample := light.Sample(point, normal, NewVec2(0.5, 0.5))
            if lightSample.Distance > 0 {
                distance = lightSample.Distance * lightSample.Distance
                distance = math.Max(distance, 1.0) // Prevent division by zero
                geometricFactor = math.Max(0, normal.Dot(lightSample.Direction))
            }
        }
        
        importances[i] = power * geometricFactor / distance
        totalImportance += importances[i]
    }
    
    // Sample based on importance
    if totalImportance > 0 {
        var cumulative float64
        for i := 0; i < len(pls.lights); i++ {
            cumulative += importances[i]
            if u <= cumulative/totalImportance {
                return pls.lights[i], importances[i]/totalImportance
            }
        }
    }
    
    // Fallback to power-based sampling
    return pls.SampleLight(u)
}

// Legacy power-only sampling (for emission sampling where no point context)
func (pls *PowerBasedLightSampler) SampleLight(u float64) (Light, float64) {
    // Binary search on power-based CDF
    for i, cdfValue := range pls.cdf {
        if u <= cdfValue {
            samplingPdf := pls.powers[i] / pls.totalPower
            return pls.lights[i], samplingPdf
        }
    }
    lastIdx := len(pls.lights) - 1
    samplingPdf := pls.powers[lastIdx] / pls.totalPower
    return pls.lights[lastIdx], samplingPdf
}
```

#### 3. Integration Points in Current Codebase (IMPLEMENTED)

**A. Core Sampling Functions (`pkg/core/sampling.go`)**

Updated to use scene-based importance sampling:

```go
// IMPLEMENTED: Importance-based light sampling
func SampleLight(scene Scene, point Vec3, normal Vec3, sampler Sampler) (LightSample, Light, bool) {
    lights := scene.GetLights()
    if len(lights) == 0 {
        return LightSample{}, nil, false
    }

    // Use importance-based light sampler that considers surface point and normal
    lightSampler := scene.GetLightSampler()
    selectedLight, lightSelectionPdf := lightSampler.SampleLightImportance(point, normal, sampler.Get1D())

    sample := selectedLight.Sample(point, normal, sampler.Get2D())
    sample.PDF *= lightSelectionPdf // Combined PDF for MIS calculations

    return sample, selectedLight, true
}

// IMPLEMENTED: Power-based emission sampling (for BDPT)
func SampleLightEmission(scene Scene, sampler Sampler) (EmissionSample, bool) {
    lights := scene.GetLights()
    if len(lights) == 0 {
        return EmissionSample{}, false
    }

    // Use power-based light sampler for importance sampling (no point context)
    lightSampler := scene.GetLightSampler()
    selectedLight, lightSelectionPdf := lightSampler.SampleLight(sampler.Get1D())

    sample := selectedLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
    sample.AreaPDF *= lightSelectionPdf // Apply light selection probability

    return sample, true
}
```

**B. Path Tracing Integration (`pkg/integrator/path_tracing.go`)** ✓ IMPLEMENTED

Direct lighting calculation automatically uses `core.SampleLight()` with importance sampling.

Indirect lighting MIS updated to account for importance-based light selection:

```go
// IMPLEMENTED: Updated CalculateLightPDF for importance sampling
func CalculateLightPDF(scene Scene, point, normal, direction Vec3) float64 {
    lights := scene.GetLights()
    if len(lights) == 0 {
        return 0.0
    }

    lightSampler := scene.GetLightSampler()
    totalPDF := 0.0

    // For each light, calculate the PDF weighted by its selection probability
    for i, light := range lights {
        lightPDF := light.PDF(point, normal, direction)
        lightSelectionPdf := lightSampler.GetLightProbability(i)
        totalPDF += lightPDF * lightSelectionPdf
    }

    return totalPDF
}
```

**C. BDPT Light Path Generation (`pkg/integrator/bdpt.go`)**

Light path generation (lines 141-148) currently uses uniform sampling:

```go
// Current uniform sampling (line 146)
sampledLight := lights[int(sampler.Get1D()*float64(len(lights)))]
lightSelectionPdf := 1.0 / float64(len(lights))

// NEW: Replace with power-based sampling
lightSampler := GetOrCreateLightSampler(lights, scene.GetBVH().FiniteWorldRadius)
sampledLight, lightSelectionPdf := lightSampler.SampleLight(sampler.Get1D())
```

**D. BDPT Direct Lighting Strategy (`pkg/integrator/bdpt.go`)**

Direct lighting in s=1,t>1 strategies (around line 336) uses `core.SampleLight()`, so **no changes needed**.

#### 4. BDPT Light Path Generation Strategy

BDPT light path generation shall use the same power-based sampling strategy as direct lighting sampling.

**Rationale:**
- Uniform light selection results in equal probability for all lights regardless of contribution
- Weak infinite lights generate as many light paths as strong finite lights
- Power-based sampling concentrates light paths on high-contribution sources

**Implementation Requirements:**
- Light path generation must use `PowerBasedLightSampler` instead of uniform selection
- MIS calculations must account for non-uniform light selection probabilities
- All bidirectional strategies (s=1,2,3+) benefit from improved light vertex distribution

**Technical Details:**
```go
// Light path sampling distribution:
// s=1 paths: Higher probability of starting from bright lights
// s=2+ paths: Better continuation from high-contribution light vertices
// MIS weighting: Maintains correctness via lightSelectionPdf factors
```

**Expected Benefits:**
- Reduced noise in bidirectional light transport
- Better convergence for mixed-intensity lighting scenarios
- Improved efficiency while maintaining unbiasedness
- Enhanced performance in sun+sky environments

**E. Scene-Level Caching**

Add light sampler to Scene struct for efficient reuse:

```go
// In pkg/scene/scene.go
type Scene struct {
    // ... existing fields ...
    lightSampler *PowerBasedLightSampler // Cache for efficient reuse
}

func (s *Scene) GetLightSampler() *PowerBasedLightSampler {
    if s.lightSampler == nil {
        s.lightSampler = NewPowerBasedLightSampler(s.Lights, s.GetBVH().FiniteWorldRadius)
    }
    return s.lightSampler
}
```

### Expected Results
- **Dramatic noise reduction** in sun+sky scenarios
- **Simple implementation** with minimal performance overhead  
- **Theoretically correct** power ratios matching PBRT

### Limitations
- No geometric consideration (distance, orientation)
- Uniform importance across all shading points
- May oversample dim lights in shadow regions

## Phase 2: Geometric Importance Sampling (Optional)

### Enhanced Importance Function
Extend Phase 1 with point-specific geometric factors following PBRT's approach:

```go
func (l Light) CalculateImportance(point Vec3, normal Vec3, sceneRadius float64) float64 {
    basePower := l.CalculatePower(sceneRadius)
    
    // Distance factor (for finite lights)
    distanceFactor := 1.0
    if !l.IsInfinite() {
        lightPos := l.GetPosition()
        distanceSquared := point.Subtract(lightPos).LengthSquared()
        distanceSquared = math.Max(distanceSquared, l.GetMinDistance())
        distanceFactor = 1.0 / distanceSquared
    }
    
    // Geometric factor (surface orientation)
    geometricFactor := 1.0
    if normal.LengthSquared() > 0 {
        lightDir := l.GetDirectionTo(point)
        cosTheta := math.Max(0, normal.Dot(lightDir))
        geometricFactor = cosTheta
    }
    
    return basePower * distanceFactor * geometricFactor
}
```

### Adaptive Sampling
Build importance distribution per shading point:

```go
type AdaptiveLightSampler struct {
    baseSampler *PowerBasedLightSampler
}

func (als *AdaptiveLightSampler) SampleLight(point Vec3, normal Vec3, u float64) (Light, float64) {
    // Calculate importance for each light at this point
    importances := make([]float64, len(als.baseSampler.lights))
    totalImportance := 0.0
    
    for i, light := range als.baseSampler.lights {
        importances[i] = light.CalculateImportance(point, normal, als.sceneRadius)
        totalImportance += importances[i]
    }
    
    // Sample based on local importance
    // ... build local CDF and sample
}
```

### Advanced Features
- **Light bounds**: Implement PBRT-style bounding boxes for lights
- **Visibility estimation**: Consider occlusion in importance calculation
- **Two-sided materials**: Handle bidirectional surface properties
- **Complex emission patterns**: Support spot lights, IES profiles

## Implementation Strategy

### Phase 1 Implementation
1. **Add power calculation methods** to all light types
2. **Create PowerBasedLightSampler** class
3. **Integrate with PathTracingIntegrator** direct lighting
4. **Integrate with BDPTIntegrator** light path generation
5. **Test with sun+sky scenes** to verify noise reduction

### Phase 2 Implementation (Optional)
1. **Extend Light interface** with importance calculation
2. **Implement geometric factors** for distance and orientation
3. **Create AdaptiveLightSampler** for point-specific sampling
4. **Performance optimization** with caching and approximation
5. **Advanced geometric calculations** following PBRT methodology

## Testing Strategy

### Phase 1 Tests
- **Power calculation correctness**: Verify PBRT formula implementation
- **Sampling distribution**: Test CDF construction and sampling
- **Noise reduction**: Compare uniform vs power-based sampling in renders
- **Energy conservation**: Ensure MIS weights remain correct

### Phase 2 Tests  
- **Geometric accuracy**: Test distance and orientation factors
- **Edge case handling**: Points inside lights, grazing angles
- **Performance benchmarks**: Measure overhead of importance calculation
- **Visual quality**: Compare against PBRT reference renders

## Performance Considerations

### Phase 1
- **Minimal overhead**: Power calculation once per scene preprocessing
- **Fast sampling**: O(log N) light selection via binary search
- **Memory efficient**: Only stores power values and CDF

### Phase 2
- **Per-point calculation**: Importance computed for each shading point
- **Caching opportunities**: Spatial coherence in importance values
- **Approximation strategies**: Simplified geometric calculations for speed

## Migration Path

1. **Implement Phase 1** as default light sampling strategy
2. **Keep uniform sampling** as fallback option via flag
3. **Gradual rollout** with A/B testing on different scene types
4. **Phase 2 development** based on Phase 1 performance and quality results
5. **Future extensions** for advanced lighting scenarios

This specification provides immediate noise reduction benefits with a clear path for future enhancement to match PBRT's sophisticated light importance sampling.