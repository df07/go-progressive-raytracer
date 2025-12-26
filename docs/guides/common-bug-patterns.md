# Common Bug Patterns in Raytracers

## Overview

Raytracer bugs cluster into recognizable patterns with standard fixes. Data loss bugs (not preserving UV, normals through pipeline) manifest as wrong textures or colors. Coordinate system bugs (world vs local, UV wrapping) cause geometry artifacts. Integrator inconsistencies (PT vs BDPT divergence) indicate PDF or MIS errors. Understanding these patterns accelerates debugging from hours to minutes.

## Data Loss Bugs

### Pattern: Data Not Preserved Through Pipeline

**Symptoms**:
- Textures appear as solid color instead of pattern
- BDPT wrong but PT correct for same scene
- Information present at intersection lost by material evaluation

**Root cause**: Creating new data structures instead of preserving complete originals

**Example 1: UV Coordinates Lost**

**Buggy code**:
```go
// In BDPT connection evaluation
newHit := material.SurfaceInteraction{
    Point:  vertex.Point,
    Normal: vertex.Normal,
    // UV field not set → defaults to (0, 0)
}
brdf := material.EvaluateBRDF(..., &newHit, ...)
// Always samples texture at (0, 0) → solid color
```

**Fix**:
```go
// Use existing SurfaceInteraction from vertex
brdf := material.EvaluateBRDF(..., vertex.SurfaceInteraction, ...)
// UV preserved from original intersection
```

**Real occurrence**: BDPT texture sampling bug (fixed Dec 2025). Checkerboard appeared solid blue in BDPT because connection evaluation created new SurfaceInteraction without UV.

**Example 2: Normal Lost in Vertex**

**Buggy code**:
```go
vertex := Vertex{
    Point: hit.Point,
    // Normal not stored
}
// Later, need normal for cosine term
cosTheta := ray.Direction.Dot(???)  // Normal is gone!
```

**Fix**:
```go
vertex := Vertex{
    SurfaceInteraction: hit,  // Includes Normal, UV, etc.
}
cosTheta := ray.Direction.Dot(vertex.Normal)
```

**Prevention**:
- Always store complete SurfaceInteraction, never partial copy
- Use embedded structs to inherit all fields automatically
- Add assertions to check critical fields are non-zero

**Detection**:
- Compare PT vs BDPT: data loss usually affects BDPT more (more pipeline stages)
- Log field values: if always zero or constant, data is lost
- Visual inspection: solid colors instead of textures = UV lost

### Pattern: Overwriting Valid Data

**Symptoms**:
- Material assignment seems correct but renders use wrong material
- Geometry correct but appears with default/fallback properties

**Root cause**: Data overwritten after initial correct assignment

**Example: Material Overwritten**

**Buggy code**:
```go
sphere := geometry.NewSphere(center, radius, redMaterial)
scene.AddShape(sphere)

// Later, accidental overwrite
sphere.Material = defaultMaterial  // Overwrites redMaterial
```

**Fix**:
```go
// Don't modify after construction
sphere := geometry.NewSphere(center, radius, redMaterial)
scene.AddShape(sphere)
// Material remains redMaterial
```

**Prevention**:
- Immutable objects where possible
- Copy instead of modify if must change properties
- Defensive copies for shared data

## Coordinate System Bugs

### Pattern: World Space vs Local Space Confusion

**Symptoms**:
- Normals point wrong direction
- Transformations don't compose correctly
- Lighting appears from wrong side

**Root cause**: Mixing coordinate systems without proper transformation

**Example: Normal in Wrong Space**

**Buggy code**:
```go
// Mesh has local-space normals
meshNormal := mesh.Normals[vertexIndex]

// Use directly for lighting (WRONG if mesh is transformed)
cosTheta := lightDir.Dot(meshNormal)
```

**Fix**:
```go
// Transform normal to world space
worldNormal := mesh.Transform.TransformNormal(meshNormal)
worldNormal = worldNormal.Normalize()

cosTheta := lightDir.Dot(worldNormal)
```

**Prevention**:
- Name variables by space: `worldPos`, `localNormal`, `objectSpace`
- Document transformation requirements in comments
- Assert vector magnitudes after transformations

**Detection**:
- Lighting from wrong direction
- Shadows in wrong places
- Reflections at wrong angles

### Pattern: UV Wrapping Issues

**Symptoms**:
- Texture seams visible
- Texture appears stretched or compressed near poles/edges
- Discontinuities in otherwise smooth texture

**Root cause**: UV coordinates not wrapped to [0,1] range or wrapped incorrectly

**Example: Forgetting UV Wrapping**

**Buggy code**:
```go
// UV may be outside [0,1]
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    x := int(uv.X * float64(t.Width))   // Negative or > Width!
    y := int(uv.Y * float64(t.Height))
    return t.Pixels[y*t.Width + x]  // Array out of bounds
}
```

**Fix**:
```go
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    // Wrap to [0,1]
    u := uv.X - math.Floor(uv.X)
    v := 1 - (uv.Y - math.Floor(uv.Y))  // Also flip V

    x := int(u * float64(t.Width))
    y := int(v * float64(t.Height))

    // Clamp for safety
    x = clamp(x, 0, t.Width-1)
    y = clamp(y, 0, t.Height-1)

    return t.Pixels[y*t.Width + x]
}
```

**Prevention**:
- Always wrap UV coordinates in texture sampling
- Clamp to valid range as safety check
- Test with UVs outside [0,1] range

**Detection**:
- Array bounds panics during rendering
- Visual seams or discontinuities
- Stretched/compressed texture at edges

## Integrator Inconsistency Bugs

### Pattern: PT and BDPT Produce Different Results

**Symptoms**:
- Luminance differs by >15% between PT and BDPT
- Visual appearance differs (brightness, color, texture)
- One integrator correct, other produces artifacts

**Root cause**: Algorithm-specific bugs in BDPT (PT simpler, less prone to errors)

**Common causes**:
1. Incorrect PDF calculations (missing terms, wrong measure)
2. Missing MIS weights (double-counting)
3. Geometric term errors (missing 1/distance²)
4. Data not preserved in vertices (see Data Loss pattern)
5. Transport mode confusion (Radiance vs Importance)

**Example 1: Missing Geometric Term**

**Buggy code**:
```go
// BDPT connection without geometric term
contribution := lightBeta.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraBeta)
// Missing geometric term causes scale-dependent brightness
```

**Fix**:
```go
direction := lightVertex.Point.Sub(cameraVertex.Point)
distance := direction.Length()
direction = direction.Normalize()

cosCam := direction.AbsDot(cameraVertex.Normal)
cosLight := direction.AbsDot(lightVertex.Normal)
G := cosCam * cosLight / (distance * distance)

contribution := lightBeta.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraBeta).Multiply(G)
```

**Example 2: Incorrect PDF Measure**

**Buggy code**:
```go
// Storing solid angle PDF in area PDF field
vertex.AreaPdfForward = pdfSolidAngle  // WRONG MEASURE
```

**Fix**:
```go
// Convert solid angle to area measure
vertex.AreaPdfForward = vertexPrev.convertSolidAngleToAreaPdf(&vertex, pdfSolidAngle)

// Conversion formula:
// areaPDF = solidAnglePDF × cosθ / distance²
```

**Detection**:
- Run luminance comparison test: `go test ./pkg/renderer -run Luminance`
- Compare console luminosity values from CLI renders
- Visual diff: load PT and BDPT images side-by-side
- Check for scale dependence: test at unit scale and 278× scale

**Prevention**:
- Every BDPT change must pass PT comparison test
- Add logging for PDF values, check magnitude is reasonable
- Document which measure (solid angle vs area) each PDF uses
- Use type system to distinguish measures if possible

### Pattern: MIS Weights Incorrect

**Symptoms**:
- Image too bright overall (double-counting)
- Image too dark overall (under-counting)
- Fireflies (extreme brightness outliers)

**Root cause**: MIS weights don't sum to 1 or miss some strategies

**Example: Forgetting to Apply MIS**

**Buggy code**:
```go
for each strategy (s,t):
    contribution := evaluateStrategy(s, t)
    totalLight = totalLight.Add(contribution)  // No MIS!
// Multiple strategies contribute same path → double-counting
```

**Fix**:
```go
for each strategy (s,t):
    contribution := evaluateStrategy(s, t)
    misWeight := calculateMISWeight(s, t, ...)
    totalLight = totalLight.Add(contribution.Multiply(misWeight))
// MIS prevents double-counting
```

**Detection**:
- Add assertion: sum of MIS weights for each pixel should equal 1
- Check for fireflies (MIS weights >1 indicate bug)
- Compare to path tracing: if BDPT much brighter, missing MIS

**Prevention**:
- Always apply MIS weights to BDPT contributions
- Test MIS calculation in isolation (unit test)
- Verify sum of weights equals 1 for all pixels

## Floating-Point Precision Issues

### Pattern: Accumulation Drift

**Symptoms**:
- Results vary slightly across runs with same seed
- Brightness increases or decreases with sample count
- Non-deterministic behavior

**Root cause**: Order of floating-point operations affects result

**Example: Sample Accumulation**

**Buggy code**:
```go
// Accumulating in different order based on tile completion
color := Vec3{}
for each sample (in random completion order):
    color = color.Add(sampleColor)
// Order depends on parallel scheduling → non-deterministic
```

**Fix**:
```go
// Accumulate in deterministic order
color := Vec3{}
for i := 0; i < numSamples; i++:  // Fixed order
    color = color.Add(samples[i])
// Always same order → deterministic result
```

**Prevention**:
- Use Kahan summation for high-precision accumulation
- Process samples in fixed order
- Use tile-specific seeds for determinism despite parallelism

**Detection**:
- Run same render twice, compare pixel-by-pixel
- Check if results differ (should be bit-identical)

### Pattern: Self-Intersection

**Symptoms**:
- Shadow acne (incorrect self-shadowing)
- Light leaks through surfaces
- Reflection artifacts

**Root cause**: Ray starts exactly on surface, immediately intersects same surface

**Example: Ray Origin on Surface**

**Buggy code**:
```go
// Cast ray from exact hit point
shadowRay := core.NewRay(hit.Point, directionToLight)
occluded := scene.Hit(shadowRay, 0.0, lightDistance)
// Immediately hits same surface (t ≈ 0)
```

**Fix**:
```go
// Offset ray origin slightly
shadowRay := core.NewRay(hit.Point, directionToLight)
occluded := scene.Hit(shadowRay, 0.001, lightDistance - 0.001)
//                                ↑              ↑
//                              tMin          tMax with epsilon
```

**Prevention**:
- Always use epsilon (0.001) for tMin
- Subtract epsilon from tMax for shadow rays
- Alternatively: offset ray origin along normal

**Detection**:
- Visual: shadow acne (speckled shadows)
- Log: t values near zero in hit tests
- Count: excessive intersection tests per ray

## Parallel Rendering Bugs

### Pattern: Race Conditions

**Symptoms**:
- Non-deterministic results with same seed
- Occasional crashes or corrupted pixels
- Results differ with different worker counts

**Root cause**: Concurrent writes to shared memory without synchronization

**Example: Shared Pixel Buffer**

**Buggy code**:
```go
// Multiple workers write to same pixel
// Worker 1:
pixelColors[y][x] = color1

// Worker 2 (simultaneous):
pixelColors[y][x] = color2

// Result: undefined (could be color1, color2, or corrupted)
```

**Fix** (Option 1 - Locks):
```go
mutex.Lock()
pixelColors[y][x] = pixelColors[y][x].Add(color)
mutex.Unlock()
```

**Fix** (Option 2 - Partition):
```go
// Each worker writes to non-overlapping regions
// Worker 1: tiles 0-15
// Worker 2: tiles 16-31
// No overlap → no race
```

**Fix** (Option 3 - Queue):
```go
// Workers append to lock-free queue
splatQueue.Add(splat)

// Single thread processes queue later
for each splat in queue:
    pixels[splat.y][splat.x] = pixels[splat.y][splat.x].Add(splat.color)
```

**Prevention**:
- Use `go test -race` to detect races
- Design lock-free where possible (partitioning, queuing)
- Document shared state and access patterns

**Detection**:
- Run with `-race` flag
- Compare results across runs
- Vary worker count: if results change, race likely

### Pattern: Non-Deterministic Random Sequences

**Symptoms**:
- Results vary despite same seed
- Tile order affects output
- Parallel scheduling changes image

**Root cause**: Shared random number generator or seed conflicts

**Example: Shared RNG**

**Buggy code**:
```go
// Global RNG shared by all workers
var globalRNG = rand.New(rand.NewSource(42))

// Worker 1:
sample := globalRNG.Float64()

// Worker 2 (simultaneous):
sample := globalRNG.Float64()

// Order depends on scheduling → non-deterministic
```

**Fix**:
```go
// Each tile has unique seed
tileSeed := baseSeed + tileX*1000 + tileY

// Each worker creates independent RNG
rng := rand.New(rand.NewSource(tileSeed))
sample := rng.Float64()

// Order doesn't matter, each tile deterministic
```

**Prevention**:
- Never share RNG across goroutines
- Derive unique seeds from tile/pixel coordinates
- Document seeding strategy

**Detection**:
- Render same scene multiple times
- Compare pixel values (should be identical)
- Run with different worker counts (should match)

## Scale-Dependent Bugs

### Pattern: Results Change with Scene Scale

**Symptoms**:
- Unit-scale scene renders correctly
- 278× scale scene too bright or too dark
- Brightness changes linearly with scale

**Root cause**: Missing area or distance terms in PDFs

**Example: PDF Missing Area Term**

**Buggy code**:
```go
// Light PDF without area correction
pdf := 1.0 / numLights  // Just selection probability
// Same PDF regardless of light size → brightness changes with scale
```

**Fix**:
```go
// Include spatial density
pdf := (1.0 / numLights) * light.AreaPDF()
// Accounts for light size → scale-invariant
```

**Real occurrence**: BDPT scale-dependent brightness bug (fixed Dec 2025). Missing area term in `PDF_Le` caused BDPT to be brighter at large scales.

**Detection**:
- Test at multiple scales: 1×, 10×, 278×
- Luminance should be identical (within tolerance)
- Integration test: "Large-Scale Cornell Box" in `progressive_integration_test.go`

**Prevention**:
- Always include geometric terms (area, distance²)
- Test at multiple scales before committing
- Understand physical units of PDFs (per area, per solid angle)

## Real-World Bug Examples from This Codebase

### 1. BDPT Texture Sampling Bug (Dec 2025)

**Symptom**: Checkerboard texture appeared as solid blue in BDPT, correct in PT

**Root cause**: Connection evaluation created new SurfaceInteraction without copying UV
```go
// BUGGY CODE (simplified)
newHit := SurfaceInteraction{
    Point: vertex.Point,
    Normal: vertex.Normal,
    // UV not copied → defaults to (0,0)
}
```

**Fix**: Use complete SurfaceInteraction from vertex
```go
brdf := vertex.Material.EvaluateBRDF(..., vertex.SurfaceInteraction, ...)
```

**Pattern**: Data loss bug (UV coordinates)

**Test**: "Textured Checkerboard Quad" in `progressive_integration_test.go:360-402`

### 2. BDPT Scale-Dependent Brightness (Dec 2025)

**Symptom**: BDPT 20% brighter than PT at 278× scale, matched at unit scale

**Root cause**: `PDF_Le` missing area term in light path PDF

**Fix**: Include complete PDF calculation with spatial and directional components

**Pattern**: Scale-dependent bug (missing area term)

**Test**: "Large-Scale Cornell Box" in `progressive_integration_test.go:404-475`

### 3. EmissionPDF Function Removed (Dec 2025)

**Symptom**: Light interface cleanup after fixing PDF_Le

**Root cause**: `EmissionPDF()` didn't account for spatial and directional density separately

**Fix**: Use `PDF_Le()` which properly separates components

**Pattern**: API design bug (incomplete abstraction)

**Commit**: "Remove deprecated EmissionPDF function from Light interface"

## Debugging Checklist

When encountering a rendering bug:

**1. Classify the bug**:
- [ ] Data loss? (textures wrong, fields zero)
- [ ] Coordinate system? (geometry artifacts, wrong lighting)
- [ ] Integrator inconsistency? (PT vs BDPT differ)
- [ ] Floating-point? (non-deterministic, accumulation drift)
- [ ] Parallelism? (race detector triggered, worker-count dependent)
- [ ] Scale-dependent? (works at one scale, fails at another)

**2. Isolate the bug**:
- [ ] Simplify scene (fewer objects, simpler materials)
- [ ] Test with solid colors (eliminate texture bugs)
- [ ] Single worker (eliminate parallelism bugs)
- [ ] Unit scale (eliminate scale-dependent bugs)

**3. Compare integrators**:
- [ ] Render with PT: correct or also wrong?
- [ ] Render with BDPT: correct or also wrong?
- [ ] If PT correct, BDPT wrong: BDPT-specific bug
- [ ] If both wrong: shared code bug (material, geometry)

**4. Add logging**:
- [ ] Log critical values (UVs, PDFs, colors)
- [ ] Check for patterns (always zero, always same value)
- [ ] Verify data flow (value at generation, value at use)

**5. Write regression test**:
- [ ] Create minimal reproduction
- [ ] Add to integration test suite
- [ ] Verify test fails before fix, passes after

**6. Apply fix pattern**:
- [ ] Data loss → preserve complete structures
- [ ] Coordinate system → transform to correct space
- [ ] Integrator inconsistency → fix PDFs, MIS, geometric terms
- [ ] Floating-point → use epsilon, deterministic order
- [ ] Parallelism → partition, queue, or lock
- [ ] Scale-dependent → add missing area/distance terms

## Access Log
