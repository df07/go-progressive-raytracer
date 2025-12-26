# Debugging Guide for Rendering Issues

## Overview

Rendering bugs fall into distinct classes with specific diagnostic patterns. Brightness differences indicate PDF or MIS errors, color bleeding suggests incorrect material evaluation, and artifacts point to numerical precision issues. The primary debugging tool is PT vs BDPT comparison - integrators must produce identical results for simple scenes, so divergence immediately pinpoints integrator-specific bugs.

## Common Bug Classes

### 1. Brightness Differences (Luminosity Mismatch)

**Symptoms**:
- BDPT image consistently brighter or darker than PT
- Console luminosity values differ by >10%
- Uniform brightness difference across entire image

**Root causes**:
- Incorrect PDF calculations (forward/reverse PDFs swapped or missing)
- Missing MIS terms (connection strategies not weighted properly)
- Incorrect geometric term in BDPT connections (distance squared missing)
- Light power not properly normalized (area vs intensity confusion)

**Diagnostic workflow**:

1. **Verify with simple scene**:
```bash
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100 --max-passes=1
# Note luminosity: e.g., 0.1245

./raytracer --scene=cornell --integrator=bdpt --max-samples=100 --max-passes=1
# Note luminosity: e.g., 0.1580  (27% higher - BUG!)
```

2. **Run integration test**:
```bash
go test -v ./pkg/renderer -run "Cornell.*Quad"
```

3. **Check for scale dependence**:
- Bug may only appear at specific scene scales
- Test unit-scale scene vs 278× scaled scene
- Scale-dependent bugs indicate area/solid angle PDF confusion

4. **Add PDF logging**:
```go
// In BDPT connection evaluation
func evaluateConnection(...) {
    pdf := calculatePDF(...)
    fmt.Printf("Connection PDF: forward=%.6f reverse=%.6f\n", pdfFwd, pdfRev)
    // ...
}
```

5. **Common fixes**:
- Ensure `PDF_Le` used for light vertices (spatial × directional density)
- Check geometric term includes `1/distance²`
- Verify MIS weights sum to 1 for each pixel
- Confirm Russian Roulette compensation factor applied correctly

### 2. Color Bleeding (Incorrect Material Evaluation)

**Symptoms**:
- Colors appear in wrong locations (red wall tints nearby white wall)
- Texture appears on wrong surfaces
- Materials look correct individually but interactions are wrong

**Root causes**:
- Material evaluated at wrong surface point
- UV coordinates from wrong geometry
- Normal vectors not interpolated correctly
- Ray-material mismatch (ray hits sphere A, uses sphere B's material)

**Diagnostic workflow**:

1. **Isolate material issue**:
```bash
# Replace all materials with solid colors
# If bug persists: geometry/intersection bug
# If bug disappears: material evaluation bug
```

2. **Check material assignment**:
```go
// In scene setup
sphere1 := geometry.NewSphere(pos1, radius, redMaterial)
sphere2 := geometry.NewSphere(pos2, radius, greenMaterial)
// Verify each shape has correct material
```

3. **Log material evaluations**:
```go
func (l *Lambertian) EvaluateBRDF(...) {
    albedo := l.Albedo.Evaluate(hit.UV, hit.Point)
    fmt.Printf("BRDF: point=(%.2f,%.2f,%.2f) albedo=(%.2f,%.2f,%.2f)\n",
               hit.Point.X, hit.Point.Y, hit.Point.Z,
               albedo.X, albedo.Y, albedo.Z)
    return albedo.Scale(core.InvPi)
}
```

4. **Visual inspection**:
- Render single material at a time
- Compare expected vs actual color distribution
- Check if bleeding follows light paths (correct) or appears randomly (bug)

### 3. Texture Sampling Artifacts

**Symptoms**:
- Checkerboard appears as solid color
- Texture looks correct in PT but wrong in BDPT
- UV debug texture shows solid color instead of gradient
- Textures appear stretched or distorted

**Root causes**:
- UV coordinates not preserved through pipeline
- UV calculated at wrong point (intersection vs shading point)
- SurfaceInteraction created without UV field
- Texture coordinates not interpolated for triangle meshes

**Diagnostic workflow**:

1. **Test with UV debug texture**:
```go
uvDebug := material.NewUVDebugTexture()
mat := material.NewTexturedLambertian(uvDebug)
// Should show red-green gradient, not solid color
```

2. **Log UV values**:
```go
func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
    fmt.Printf("Texture eval: UV=(%.3f,%.3f)\n", uv.X, uv.Y)
    // If always (0,0): UV not being set
    // If varying: UV preservation working
    return t.sample(uv)
}
```

3. **Check SurfaceInteraction creation**:
```go
// CORRECT: Preserve complete SurfaceInteraction
vertex := Vertex{
    Hit: *hit,  // Full copy including UV
    // ...
}

// INCORRECT: Recreate without UV
vertex := Vertex{
    Hit: core.SurfaceInteraction{
        Point:  hit.Point,
        Normal: hit.Normal,
        // Missing: UV, Material, etc.
    },
}
```

4. **Test each geometry type**:
- Spheres: UV from spherical coordinates
- Quads: UV from barycentric coordinates
- Triangles: UV interpolated from vertex attributes
- Each has different UV generation code - test individually

**Real bug example** (BDPT texture sampling):
- **Symptom**: Checkerboard solid in BDPT, correct in PT
- **Cause**: BDPT connection created new SurfaceInteraction without copying UV
- **Fix**: Store full SurfaceInteraction in Vertex, reuse during connection
- **Test**: `progressive_integration_test.go:360-402` "Textured Checkerboard Quad"

### 4. Fireflies (Bright Pixel Outliers)

**Symptoms**:
- Random extremely bright pixels scattered across image
- Noise increases instead of decreases with more samples
- Some pixels orders of magnitude brighter than neighbors

**Root causes**:
- Division by very small PDF (near-zero probability path)
- Specular reflection hitting light source directly (valid but high variance)
- Incorrect PDF allowing impossible paths
- Missing importance sampling for difficult light paths

**Diagnostic workflow**:

1. **Check if physically correct**:
```bash
# Render with very high sample count
./raytracer --scene=cornell --max-samples=5000
# If fireflies average out: high variance but correct
# If fireflies persist: incorrect PDF or missing clamp
```

2. **Identify firefly source**:
```go
// Add assertion in integrator
if color.Luminance() > 1000.0 {
    fmt.Printf("Firefly detected: color=(%.2f,%.2f,%.2f) PDF=%.6f\n",
               color.X, color.Y, color.Z, pdf)
    // Inspect path that created firefly
}
```

3. **Check PDF calculations**:
- Ensure PDFs never exactly zero for reachable paths
- Use epsilon for numerical stability: `pdf = max(pdf, 1e-10)`
- Verify PDF matches sampling strategy (cosine-weighted, uniform, etc.)

4. **Test importance sampling**:
- Fireflies often indicate missing importance sampling
- Example: sampling diffuse BRDF when should sample light source
- Ensure MIS combines both BRDF and light sampling

### 5. Crashes and Numerical Errors

**Symptoms**:
- Segmentation fault during rendering
- NaN or Inf values in output
- Assertion failures in debug builds
- Inconsistent results across runs (non-deterministic)

**Root causes**:
- Division by zero (zero-length vectors, zero PDF)
- Array out of bounds (pixel coordinates outside image)
- Uninitialized variables
- Race conditions in parallel rendering
- Floating point overflow/underflow

**Diagnostic workflow**:

1. **Enable assertions and bounds checking**:
```go
// Add debug assertions
if vec.LengthSquared() < 1e-10 {
    panic("Near-zero vector in normalize")
}

if pdf <= 0 {
    panic(fmt.Sprintf("Invalid PDF: %.10f", pdf))
}
```

2. **Test with single worker**:
```bash
./raytracer --scene=cornell --workers=1
# If crash disappears: race condition
# If crash persists: deterministic bug
```

3. **Use race detector**:
```bash
go test -race ./pkg/renderer
# Detects concurrent access to shared memory
```

4. **Add NaN checking**:
```go
func checkColor(c core.Vec3, context string) {
    if math.IsNaN(c.X) || math.IsNaN(c.Y) || math.IsNaN(c.Z) {
        panic(fmt.Sprintf("NaN in color at %s: %v", context, c))
    }
}
```

5. **Inspect crash backtrace**:
```bash
# Build with debug symbols
go build -gcflags="-N -l" -o raytracer main.go

# Run in debugger
dlv exec ./raytracer -- --scene=cornell
(dlv) continue
# Crash occurs
(dlv) bt  # Backtrace
(dlv) locals  # Inspect variables
```

## Tools and Techniques

### Average Luminance Comparison

**Purpose**: Single metric for overall image brightness

**Usage**:
```bash
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100
# Output: Average Luminosity: 0.1245

./raytracer --scene=cornell --integrator=bdpt --max-samples=100
# Output: Average Luminosity: 0.1238
```

**Interpretation**:
- <5% difference: Excellent agreement (Monte Carlo variance)
- 5-15% difference: Acceptable for complex scenes, investigate if simple scene
- \>15% difference: Likely bug in one integrator
- Consistent bias (BDPT always brighter): Systematic error, not random noise

**Limitations**:
- Doesn't catch localized artifacts
- Doesn't detect color shifts (only luminance)
- Can miss compensating errors (one area too bright, another too dark)

### Visual Diff Comparison

**Process**:
1. Render same scene with PT and BDPT
2. Open images side-by-side in image viewer
3. Look for differences in:
   - Overall brightness
   - Texture appearance
   - Shadow sharpness
   - Color accuracy
   - Noise distribution

**What to look for**:
- **Identical appearance**: Integrators are equivalent (goal!)
- **Brightness difference**: Luminance bug (check console luminosity values)
- **Texture difference**: UV preservation bug
- **Different noise**: Normal (different random paths)
- **Different artifacts**: Integrator-specific bug

### Test Scene Selection

**Simple scenes** (for isolating bugs):
- **Empty scene with infinite light**: Tests background sampling
- **Single sphere, point light**: Tests basic direct lighting
- **Two spheres, area light**: Tests shadows and occlusion

**Complex scenes** (for finding integration bugs):
- **Cornell box**: Tests indirect lighting, color bleeding
- **Caustic glass**: Tests refractive paths, specular connections
- **Textured quad**: Tests UV coordinate flow

**Principle**: Start simple, add complexity until bug appears

### Debug Rendering Modes

**Render normals**:
```go
// In integrator
return core.Vec3{
    X: (hit.Normal.X + 1) / 2,
    Y: (hit.Normal.Y + 1) / 2,
    Z: (hit.Normal.Z + 1) / 2,
}
// Should show smooth color gradient, discontinuities indicate normal bugs
```

**Render UVs**:
```go
// In integrator
return core.Vec3{
    X: hit.UV.X,
    Y: hit.UV.Y,
    Z: 0,
}
// Should show red-green gradient, solid color indicates missing UV
```

**Render depth**:
```go
// In integrator
depth := hit.T / 10.0  // Scale to visible range
return core.Vec3{X: depth, Y: depth, Z: depth}
// Should show smooth grayscale gradient, discontinuities indicate geometry bugs
```

**Render material IDs**:
```go
// Assign unique color to each material type
materialColors := map[string]core.Vec3{
    "Lambertian": core.Vec3{X: 1, Y: 0, Z: 0},
    "Metal":      core.Vec3{X: 0, Y: 1, Z: 0},
    "Dielectric": core.Vec3{X: 0, Y: 0, Z: 1},
}
return materialColors[hit.Material.Type()]
// Verifies ray-material correspondence
```

## Step-by-Step Debugging Workflow

### Scenario: BDPT produces different colors than PT

**Step 1: Reproduce consistently**
```bash
# Render both integrators with identical settings
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100 --max-passes=1
./raytracer --scene=cornell --integrator=bdpt --max-samples=100 --max-passes=1

# Compare images visually
# Note: PT shows red-green checkerboard, BDPT shows solid blue
```

**Step 2: Verify with integration test**
```bash
go test -v ./pkg/renderer -run "Textured.*Checkerboard"
# Test FAILS: BDPT luminance differs by 25%
# Debug images saved to output/debug_renders/
```

**Step 3: Simplify scene**
```go
// Replace checkerboard with solid color
solidColor := material.NewSolidColor(core.NewVec3(0.5, 0.5, 0.5))
mat := material.NewTexturedLambertian(solidColor)

// Re-run test
# If bug persists: BRDF evaluation issue
# If bug disappears: Texture-specific issue
```
Result: Bug disappears → texture-related

**Step 4: Add diagnostic logging**
```go
// In Checkerboard.Evaluate()
func (c *Checkerboard) Evaluate(uv core.Vec2, p core.Vec3) core.Vec3 {
    fmt.Printf("[%s] UV=(%.3f,%.3f)\n",
               getCurrentIntegrator(), uv.X, uv.Y)
    // ... rest of function
}
```

**Step 5: Run with logging**
```bash
./raytracer --scene=cornell --integrator=path-tracing --max-samples=1 | grep UV
# Output: UV=(0.234,0.567) UV=(0.891,0.123) UV=(0.456,0.789) ...

./raytracer --scene=cornell --integrator=bdpt --max-samples=1 | grep UV
# Output: UV=(0.000,0.000) UV=(0.000,0.000) UV=(0.000,0.000) ...
```
Result: BDPT always samples at UV=(0,0) → UV not preserved

**Step 6: Locate UV loss in BDPT code**
```bash
# Search for SurfaceInteraction creation in BDPT
grep -n "SurfaceInteraction{" pkg/integrator/bdpt.go
# Found at line 245: connection evaluation creates new SurfaceInteraction
```

**Step 7: Inspect problematic code**
```go
// Line 245 in bdpt.go - INCORRECT
connectionHit := core.SurfaceInteraction{
    Point:  vertex.Hit.Point,
    Normal: vertex.Hit.Normal,
    // Missing: UV, Material
}
```

**Step 8: Apply fix**
```go
// Use stored SurfaceInteraction from vertex (already has UV)
connectionHit := vertex.Hit
```

**Step 9: Verify fix**
```bash
# Rebuild
go build -o raytracer main.go

# Re-run integration test
go test ./pkg/renderer -run "Textured.*Checkerboard"
# Test PASSES: luminance within tolerance

# Visual check
./raytracer --scene=cornell --integrator=bdpt --max-samples=100
# Now shows correct checkerboard pattern!
```

**Step 10: Add regression test**
```go
// Test already exists in progressive_integration_test.go:360-402
// Confirms bug won't reoccur
```

## PT vs BDPT Comparison as Debugging Tool

### Why This Works

**Principle**: Both integrators must converge to same result (unbiased rendering)
- Different algorithms, same physics
- PT tests one code path, BDPT tests another
- Divergence immediately identifies integrator-specific bug

### When to Use

**Use PT vs BDPT comparison when**:
- Testing new material implementation
- Modifying BDPT path construction
- Debugging texture sampling
- Verifying light importance sampling
- Testing MIS weight calculations

**Don't rely solely on comparison when**:
- Both integrators could have same bug (e.g., wrong material interface)
- Testing pure PT or pure BDPT features (Russian Roulette, light tracing)
- Performance optimization (correctness already verified)

### Interpreting Results

**Perfect match** (luminance within 5%):
- Both integrators correct for this scene
- High confidence in implementation

**Small difference** (5-15%):
- Likely Monte Carlo variance
- Increase sample count to verify
- Check if difference decreases with more samples

**Large difference** (\>15%):
- Bug in one or both integrators
- Identify which is wrong by comparing to ground truth
- Isolate with simpler scenes

**Completely different images**:
- Critical bug (wrong material evaluation, incorrect geometry)
- Start debugging from first principles
- Verify scene loads correctly, camera rays are correct

## Common Gotchas

**False positives**: Variance mistaken for bugs
- Solution: Use sufficient samples (100+) for comparison
- Check if difference decreases with more samples

**False negatives**: Bugs that affect both integrators equally
- Example: Wrong material albedo affects both PT and BDPT
- Solution: Also compare to analytical ground truth when possible

**Scale-dependent bugs**: Work at unit scale, fail at large scale
- Example: PDF_Le missing area term
- Solution: Test at multiple scene scales (1×, 10×, 100×, 278×)

**Scene-dependent bugs**: Work for simple scenes, fail for complex
- Solution: Test suite with diverse scenes (point lights, area lights, infinite lights)

**Optimization breaking correctness**: Fast approximation introduces bias
- Solution: Always verify optimization produces same result as original

## Access Log
