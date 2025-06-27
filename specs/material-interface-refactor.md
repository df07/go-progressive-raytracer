# Material Interface Refactor for BDPT Support

## Problem
The current Material interface only supports random sampling via `Scatter()`, but BDPT requires evaluating BRDF and PDF for specific given directions during connection evaluation.

## Required Changes

### 1. Update Material Interface (`pkg/core/interfaces.go`)

Add two new methods to the Material interface:

```go
type Material interface {
    // Existing method - generates random scattered direction
    Scatter(rayIn Ray, hit HitRecord, random *rand.Rand) (ScatterResult, bool)
    
    // NEW: Evaluate BRDF for specific incoming/outgoing directions
    EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3
    
    // NEW: Calculate PDF for specific incoming/outgoing directions  
    PDF(incomingDir, outgoingDir, normal Vec3) float64
}
```

### 2. Implement Methods for Each Material

#### Lambertian (`pkg/material/lambertian.go`)
```go
func (l *Lambertian) EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3 {
    // Lambertian BRDF is constant: albedo / π
    cosTheta := outgoingDir.Dot(normal)
    if cosTheta <= 0 {
        return Vec3{X: 0, Y: 0, Z: 0} // Below surface
    }
    return l.Albedo.Multiply(1.0 / math.Pi)
}

func (l *Lambertian) PDF(incomingDir, outgoingDir, normal Vec3) float64 {
    // Cosine-weighted hemisphere sampling: cos(θ) / π
    cosTheta := outgoingDir.Dot(normal)
    if cosTheta <= 0 {
        return 0.0
    }
    return cosTheta / math.Pi
}
```

#### Metal (`pkg/material/metal.go`)
```go
func (m *Metal) EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3 {
    // Perfect reflection only - delta function
    reflected := incomingDir.Reflect(normal)
    
    // Check if outgoing direction matches perfect reflection (within tolerance)
    if outgoingDir.Subtract(reflected).Length() < 0.001 {
        return m.Albedo // Delta function contribution
    }
    
    return Vec3{X: 0, Y: 0, Z: 0} // No contribution for non-reflection directions
}

func (m *Metal) PDF(incomingDir, outgoingDir, normal Vec3) float64 {
    // Delta function - PDF is 0 for evaluation (handled specially in integrator)
    return 0.0
}
```

#### Dielectric (`pkg/material/dielectric.go`)
```go
func (d *Dielectric) EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3 {
    // Check for perfect reflection or refraction
    // Implementation similar to metal but also handles refraction case
    // Return appropriate Fresnel-weighted contribution or zero
    
    // Simplified - full implementation needs Fresnel calculations
    return Vec3{X: 0, Y: 0, Z: 0} // Delta function materials
}

func (d *Dielectric) PDF(incomingDir, outgoingDir, normal Vec3) float64 {
    return 0.0 // Delta function
}
```

#### Emissive (`pkg/material/emissive.go`)
```go
func (e *Emissive) EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3 {
    // Lights don't reflect - they only emit
    return Vec3{X: 0, Y: 0, Z: 0}
}

func (e *Emissive) PDF(incomingDir, outgoingDir, normal Vec3) float64 {
    return 0.0
}
```

### 3. Update Any Composite Materials

For materials like `Layered` and `Mix`, delegate to component materials:

```go
func (layered *Layered) EvaluateBRDF(incomingDir, outgoingDir, normal Vec3) Vec3 {
    // Combine BRDFs from both layers with appropriate weights
    brdf1 := layered.Material1.EvaluateBRDF(incomingDir, outgoingDir, normal)
    brdf2 := layered.Material2.EvaluateBRDF(incomingDir, outgoingDir, normal)
    return brdf1.Multiply(layered.Weight).Add(brdf2.Multiply(1.0 - layered.Weight))
}
```

### 4. Key Implementation Notes

- **Direction conventions**: All directions should point AWAY from surface
- **Coordinate system**: Ensure consistent hemisphere orientation with normal
- **Delta functions**: Metal/Dielectric return 0 PDF but non-zero BRDF for exact matches
- **Energy conservation**: Verify BRDF values maintain proper energy relationships
- **Existing Scatter() compatibility**: Don't break existing path tracing implementation

### 5. Testing Requirements

After implementation, verify:
- Path tracing still works identically (Scatter method unchanged)
- BRDF evaluation gives expected values for known cases
- PDF evaluation matches what Scatter method would generate
- Energy conservation maintained for all materials

## Why This is Needed

BDPT requires these capabilities:
1. **Path generation**: Uses `Scatter()` for random sampling
2. **Connection evaluation**: Uses `EvaluateBRDF()` and `PDF()` for specific directions
3. **MIS weighting**: Uses `PDF()` to calculate proper importance sampling weights

The current approximation in BDPT (reusing ScatterResult.Attenuation) is incorrect because it's the BRDF for the originally sampled direction, not the connection direction.

## Current Status Note

I noticed the BDPT code was reverted back to the old MIS implementation. After this refactor is complete, we'll need to:
1. Re-implement the proper MIS weighting with power heuristic
2. Fix the PDF calculations that got reverted 
3. Update the connection evaluation to use the new EvaluateBRDF method