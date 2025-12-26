# Testing Strategy for Integrators

## Overview

Integrator testing uses a three-level strategy: unit tests verify individual components (ray bouncing, PDFs), integration tests compare luminance between integrators (PT vs BDPT should match), and visual tests catch rendering artifacts. The key insight is that path tracing and BDPT must produce statistically similar results for simple scenes, making luminance comparison the primary correctness indicator.

## Testing Levels

### Unit Tests

**Purpose**: Verify individual integrator components in isolation

**What to test**:
- Ray bouncing mechanics (`extendPath` logic)
- PDF calculations (forward/reverse probabilities)
- Russian Roulette termination probabilities
- MIS weight calculations
- Material evaluation consistency

**Example unit tests**:
- `/pkg/integrator/path_tracing_test.go`: `TestPathTracingDepthTermination`, `TestPathTracingRussianRoulette`
- `/pkg/integrator/bdpt_test.go`: `TestExtendPath`, `TestBDPTPathGeneration`
- `/pkg/integrator/bdpt_mis_test.go`: MIS weight calculation tests

**When bugs appear here**: Usually indicates algorithmic errors, incorrect math, or broken interfaces

### Integration Tests (Luminance Comparison)

**Purpose**: Verify that different integrators produce statistically equivalent results

**Key principle**: Path tracing and BDPT trace light through different strategies but must converge to the same answer for unbiased rendering. Luminance comparison detects systematic bias.

**Test pattern** (`/pkg/renderer/progressive_integration_test.go`):
```go
func TestIntegratorLuminanceComparison(t *testing.T) {
    scene := createTestScene()

    // Render with path tracing
    ptIntegrator := integrator.NewPathTracingIntegrator(samplingConfig)
    ptImage := renderFullPass(scene, ptIntegrator)
    ptLuminance := calculateAverageLuminance(ptImage)

    // Render with BDPT
    bdptIntegrator := integrator.NewBDPTIntegrator(samplingConfig)
    bdptImage := renderFullPass(scene, bdptIntegrator)
    bdptLuminance := calculateAverageLuminance(bdptImage)

    // Compare luminance (should match within tolerance)
    percentDiff := abs(bdptLuminance - ptLuminance) / ptLuminance * 100
    if percentDiff > tolerance {
        t.Errorf("Integrators differ by %.2f%%", percentDiff)
    }
}
```

**Luminance calculation** (from `progressive_integration_test.go:595-618`):
```go
func calculateAverageLuminance(img *image.RGBA) float64 {
    totalLuminance := 0.0
    pixelCount := bounds.Dx() * bounds.Dy()

    for each pixel {
        r, g, b := normalizedRGB(pixel)
        // Standard ITU-R BT.709 luminance formula
        luminance := 0.299*r + 0.587*g + 0.114*b
        totalLuminance += luminance
    }

    return totalLuminance / float64(pixelCount)
}
```

**Why luminance comparison works**:
- Single number captures overall image brightness
- Sensitive to systematic bias (incorrect PDFs, missing contributions)
- Robust to Monte Carlo variance (averaging across all pixels)
- Independent of specific scene details

**Typical tolerances**:
- Simple scenes (uniform lighting, single sphere): 5-10% tolerance
- Complex scenes (area lights, indirect illumination): 10-15% tolerance
- Very complex scenes (multiple bounces, caustics): 15-20% tolerance

**When bugs appear here**: Indicates integrator-specific implementation errors (BDPT path construction bugs, missing MIS terms, splat processing errors)

### Visual Tests

**Purpose**: Catch rendering artifacts and qualitative issues not visible in luminance metrics

**Process**:
1. Run test with `-v` flag to save debug images: `go test -v ./pkg/renderer`
2. Images saved to `/output/debug_renders/<testname>_pt.png` and `<testname>_bdpt.png`
3. Manually inspect for artifacts: fireflies, color bleeding, incorrect shadows, texture sampling errors

**What to look for**:
- **Texture consistency**: Do textured surfaces look identical between PT and BDPT?
- **Brightness consistency**: Are equivalent regions equally bright?
- **Fireflies**: Excessive bright pixels indicate variance issues or unhandled edge cases
- **Color accuracy**: Does material color match expectations?

**When bugs appear here**: Visual artifacts often indicate edge cases, numerical precision issues, or data flow bugs that don't affect average luminance

## Standard Test Scenes

Integration tests use specific scenes designed to exercise different code paths:

### Infinite Light (Uniform)
- **Purpose**: Test background lighting with no geometry
- **Exercises**: Infinite light sampling, miss handling
- **File**: `progressive_integration_test.go:44-70`

### Single Sphere with Area Light
- **Purpose**: Test basic direct lighting and material evaluation
- **Exercises**: Sphere intersection, area light sampling, Lambertian BRDF
- **File**: `progressive_integration_test.go:72-103`

### Cornell Box (Various Configurations)
- **Purpose**: Test enclosed environment with indirect lighting
- **Exercises**: Multiple bounces, color bleeding, MIS weighting
- **Variants**: Sphere light, quad light, light at different positions
- **File**: `progressive_integration_test.go:143-358`

### Textured Checkerboard Quad
- **Purpose**: Test texture sampling consistency between integrators
- **Exercises**: UV coordinate preservation, texture evaluation in BRDF
- **Regression test for**: BDPT texture sampling bug (missing UV in SurfaceInteraction)
- **File**: `progressive_integration_test.go:360-402`

## Writing New Integrator Tests

### Adding a Luminance Comparison Test

1. **Define test scene**:
```go
{
    name: "My Test Scene",
    createScene: func() *scene.Scene {
        // Build scene with geometry, materials, lights
        // Keep simple: fewer variables = easier debugging
        return scene
    },
    tolerance: 10.0, // Adjust based on scene complexity
}
```

2. **Choose appropriate tolerance**:
   - Start with 10% for moderate complexity
   - Increase if Monte Carlo variance is high (many bounces, small lights)
   - Decrease if scene is very simple (direct lighting only)

3. **Run test**: `go test ./pkg/renderer -run TestIntegratorLuminanceComparison`

4. **Interpret results**:
   - **Pass**: Integrators are consistent for this scene
   - **Fail**: Systematic difference indicates bug in one integrator
   - **Look at saved images** to see which integrator is wrong

### Debugging Test Failures

**Step 1: Verify test scene is correct**
- Check light intensities (too bright/dim affects variance)
- Ensure geometry is valid (no degenerate triangles, inside-out normals)
- Confirm materials make sense (emissive materials should be in lights list)

**Step 2: Identify which integrator is wrong**
- Compare rendered images to expectation
- If PT is correct and BDPT is wrong, bug is in BDPT-specific code
- If both look wrong, bug is in shared material/geometry code

**Step 3: Isolate the bug**
- Simplify scene: remove geometry until bug disappears
- Test with solid colors: replace textures with `SolidColor` to isolate texture bugs
- Disable features: turn off indirect lighting, reduce bounces, use point lights

**Step 4: Add logging**
```go
// In material evaluation
func (l *Lambertian) EvaluateBRDF(...) core.Vec3 {
    albedo := l.Albedo.Evaluate(hit.UV, hit.Point)
    fmt.Printf("BRDF eval: UV=(%.3f,%.3f) albedo=(%.3f,%.3f,%.3f)\n",
               hit.UV.X, hit.UV.Y, albedo.X, albedo.Y, albedo.Z)
    return albedo
}
```

## When Bugs Appear at Different Levels

### Bug appears in unit tests but not integration tests
- Likely a test bug or overly strict assertion
- Check test assumptions and expected values

### Bug appears in integration tests but not unit tests
- Component works in isolation but fails in full pipeline
- Often indicates data flow issues (UV not preserved, PDFs miscalculated)
- Example: BDPT texture bug - `Scatter()` worked, but connection evaluation failed

### Bug appears in visual tests but not luminance tests
- Localized artifacts that don't affect overall brightness
- Check for: incorrect UV mapping, specular highlight bugs, normal interpolation errors

### Bug appears in full renders but not any tests
- Test coverage gap - need new test scene
- Environment-specific issue (race condition, floating point precision)
- Create minimal reproduction and add as regression test

## Testing Best Practices

**Sensitive assertions**:
- Use percentage-based tolerances, not absolute values
- Test should fail with small errors (5% difference should be detectable)
- Verify sensitivity: temporarily introduce small error, test should fail

**Deterministic rendering**:
- Use fixed random seeds in tests
- Single-tile rendering for determinism (`TileSize = Width` in test config)
- Disable Russian Roulette for predictable path counts

**Fast iteration**:
- Use small resolutions (32x32) for quick tests
- Use moderate sample counts (256 samples sufficient for luminance comparison)
- Reserve high-quality renders (1000+ samples) for final validation

**Coverage**:
- Test each light type (sphere, quad, point, infinite)
- Test each material type (Lambertian, Metal, Dielectric, Emissive)
- Test textured and untextured materials
- Test simple and complex geometry (sphere, mesh, boxes)

## Example Debugging Workflow

**Scenario**: BDPT produces 20% brighter image than PT for textured quad

1. **Confirm bug**: Run `go test -v ./pkg/renderer -run Textured`
   - Test fails, BDPT luminance 20% higher
   - Saved images show BDPT is uniformly brighter

2. **Simplify**: Replace checkerboard with solid color
   ```go
   solidColor := material.NewSolidColor(core.NewVec3(0.5, 0.5, 0.5))
   mat := material.NewLambertian(solidColor)
   ```
   - If bug disappears: texture-related bug
   - If bug persists: BRDF evaluation bug

3. **Add logging**: Log texture evaluations
   ```go
   func (c *Checkerboard) Evaluate(uv Vec2, p Vec3) Vec3 {
       color := // ... calculate color ...
       fmt.Printf("Checkerboard: UV=(%.3f,%.3f) -> (%.3f,%.3f,%.3f)\n",
                  uv.X, uv.Y, color.X, color.Y, color.Z)
       return color
   }
   ```

4. **Identify pattern**:
   - PT logs show varying UV coordinates
   - BDPT logs show UV=(0,0) repeatedly
   - **Hypothesis**: BDPT not preserving UV coordinates

5. **Find root cause**: Search BDPT code for SurfaceInteraction creation
   - Found: Connection evaluation creates new SurfaceInteraction without copying UV
   - **Fix**: Preserve full SurfaceInteraction in vertex, use it during connection

6. **Verify fix**:
   - Run test again: now passes within tolerance
   - Check visual output: texture appears correctly in both integrators
   - Run full test suite: all tests still pass

## Access Log
