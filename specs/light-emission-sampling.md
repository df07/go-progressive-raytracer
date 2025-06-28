# Light Emission Sampling for BDPT

## Problem
The current Light interface only supports `Sample(point, random)` which samples light toward a specific shading point. BDPT light path generation requires **emission sampling** - sampling points on the light surface and emission directions from those points.

## Current vs Required Behavior

### Current Light.Sample(point, random)
- Takes a shading point as input
- Samples a point on light surface
- Returns direction FROM light TO shading point
- For sphere lights: only samples hemisphere facing the shading point

### Required for BDPT: EmissionSample(random)
- No input point needed
- Samples uniformly over ENTIRE light surface
- Samples emission direction from that surface point  
- Returns emission direction FROM the light surface

## Required Interface Changes

### Update Light Interface (`pkg/core/interfaces.go`)

```go
type Light interface {
    // Existing method - samples light toward a specific point
    Sample(point Vec3, random *rand.Rand) LightSample
    PDF(point Vec3, direction Vec3) float64
    
    // NEW: Sample emission from light surface
    SampleEmission(random *rand.Rand) EmissionSample
    
    // NEW: Calculate PDF for emission sampling
    EmissionPDF(point Vec3, direction Vec3) float64
}

// NEW: EmissionSample contains information about a sampled emission
type EmissionSample struct {
    Point     Vec3    // Point on the light surface
    Normal    Vec3    // Surface normal at the emission point
    Direction Vec3    // Emission direction FROM the surface
    Emission  Vec3    // Emitted radiance
    PDF       float64 // PDF for this emission sample (area × direction)
}
```

## Implementation for Each Light Type

### SphereLight (`pkg/geometry/sphere_light.go`)

```go
func (sl *SphereLight) SampleEmission(random *rand.Rand) EmissionSample {
    // Step 1: Sample point uniformly on ENTIRE sphere surface
    z := 1.0 - 2.0*random.Float64() // z ∈ [-1, 1] 
    r := math.Sqrt(math.Max(0, 1.0-z*z))
    phi := 2.0 * math.Pi * random.Float64()
    x := r * math.Cos(phi)
    y := r * math.Sin(phi)
    
    localDir := core.NewVec3(x, y, z)
    samplePoint := sl.Center.Add(localDir.Multiply(sl.Radius))
    normal := localDir // Surface normal points outward
    
    // Step 2: Sample emission direction (cosine-weighted hemisphere)
    emissionDir := core.RandomCosineDirection(normal, random)
    
    // Step 3: Calculate combined PDF (area sampling × direction sampling)
    areaPDF := 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)
    directionPDF := emissionDir.Dot(normal) / math.Pi // cosine-weighted
    combinedPDF := areaPDF * directionPDF
    
    // Get emission from material
    var emission core.Vec3
    if emitter, ok := sl.Material.(core.Emitter); ok {
        dummyRay := core.NewRay(samplePoint, emissionDir)
        dummyHit := core.HitRecord{Point: samplePoint, Normal: normal, Material: sl.Material}
        emission = emitter.Emit(dummyRay, dummyHit)
    }
    
    return EmissionSample{
        Point:     samplePoint,
        Normal:    normal, 
        Direction: emissionDir,
        Emission:  emission,
        PDF:       combinedPDF,
    }
}

func (sl *SphereLight) EmissionPDF(point Vec3, direction Vec3) float64 {
    // Check if point is on sphere surface
    distFromCenter := point.Subtract(sl.Center).Length()
    if math.Abs(distFromCenter - sl.Radius) > 0.001 {
        return 0.0 // Point not on sphere
    }
    
    // Calculate surface normal
    normal := point.Subtract(sl.Center).Normalize()
    
    // Check if direction is in correct hemisphere
    cosTheta := direction.Dot(normal)
    if cosTheta <= 0 {
        return 0.0 // Direction below surface
    }
    
    // Combined PDF: area sampling × cosine-weighted direction
    areaPDF := 1.0 / (4.0 * math.Pi * sl.Radius * sl.Radius)
    directionPDF := cosTheta / math.Pi
    return areaPDF * directionPDF
}
```

### DiscLight, QuadLight, etc.
Similar implementation but with appropriate surface sampling for each geometry.

## Key Differences from Current Sampling

1. **Surface Coverage**: Uniform sampling over ENTIRE light surface, not just hemisphere
2. **Direction Sampling**: FROM light surface (emission) vs TO light surface (illumination)
3. **PDF Calculation**: Combined area × direction PDF vs just area PDF
4. **Use Case**: Light path generation vs direct lighting evaluation

## BDPT Integration

Once implemented, update BDPT light path generation:

```go
// Replace current problematic sampling:
lightSample := light.Sample(core.Vec3{X: 0, Y: 0, Z: 0}, random)

// With proper emission sampling:
emissionSample := light.SampleEmission(random)
```

## Testing Requirements

- Verify uniform surface coverage (e.g., sphere light samples all parts equally)
- Energy conservation (integrated emission matches light power)  
- PDF consistency (SampleEmission and EmissionPDF should be consistent)
- Hemisphere correctness (emission directions point away from surface)

This will fix the dark BDPT images by ensuring proper light path generation.