# CLI Usage and Testing Workflow

## Overview

The raytracer CLI provides flags for controlling render quality, choosing integrators, and enabling profiling. Output is automatically saved to `output/<scene>/render_<timestamp>.png` with no custom output path support. Quick test renders use low sample counts (--max-samples=20) and few passes (--max-passes=1) for fast iteration during debugging.

## Complete CLI Flag Reference

### Required Build
```bash
# Build the CLI raytracer
go build -o raytracer main.go

# Or use the binary directly after building
./raytracer [flags]
```

### Available Flags

**Scene Selection** (`--scene`):
```bash
--scene=<name>         # Built-in scene or PBRT file path (default: "default")
```

Built-in scenes:
- `default` - Mixed materials showcase with spheres and plane
- `cornell` - Cornell box with spheres and mirror surfaces
- `cornell-boxes` - Cornell box with rotated boxes
- `cornell-pbrt` - Cornell box loaded from PBRT file
- `spheregrid` - 10x10 grid of metallic spheres (BVH testing)
- `trianglemesh` - Triangle mesh geometry showcase
- `dragon` - Dragon PLY mesh (requires separate download)
- `caustic-glass` - Glass caustic geometry (excellent for BDPT testing)

PBRT scenes:
- `cornell-empty` - Cornell box without objects
- `simple-sphere` - Basic sphere scene
- `test` - Test scene
- Or direct path: `scenes/my-scene.pbrt`

**Quality Control**:
```bash
--max-passes=N         # Maximum progressive passes (default: 5)
--max-samples=N        # Maximum samples per pixel (default: 50)
```

**Integrator Selection**:
```bash
--integrator=<type>    # 'path-tracing' (default) or 'bdpt'
```

**Parallelism**:
```bash
--workers=N            # Number of parallel workers (default: 0 = auto-detect CPU count)
```

**Profiling**:
```bash
--cpuprofile=<file>    # Write CPU profile to file (e.g., cpu.prof)
```

**Help**:
```bash
--help                 # Show help information
```

### Flags That Do NOT Exist

**No custom output path**: Output is always `output/<scene>/render_<timestamp>.png`
- Cannot specify custom filename or directory
- Cannot disable automatic output

**No resolution control**: Resolution is defined in scene code
- Cannot override width/height from CLI
- Must edit scene definition in `/pkg/scene/<scene>.go`

**No camera control**: Camera position/orientation defined in scene
- Cannot override from CLI
- Must edit scene code to change viewpoint

## Output Behavior

### Automatic Output Path

**Format**: `output/<scene_name>/render_<timestamp>.png`

**Examples**:
```bash
./raytracer --scene=cornell
# Saves to: output/cornell/render_20250126_143052.png

./raytracer --scene=default --integrator=bdpt
# Saves to: output/default/render_20250126_143105.png

./raytracer --scene=scenes/custom.pbrt
# Saves to: output/scenes-custom-pbrt/render_20250126_143120.png
```

**Directory creation**: Output directories are created automatically if they don't exist

**Timestamp format**: `YYYYMMDD_HHMMSS` (24-hour format, local time)

### Console Output

**Progress information**:
```
Starting Progressive Raytracer...
Scene: cornell (512x512)
Integrator: path-tracing
Workers: 16
Pass 1: Target 1 samples per pixel (using 16 workers)...
  Rendering 64 tiles...
  [========================================] 100% (64/64 tiles)
Pass 2: Target 12 samples per pixel (using 16 workers)...
  Rendering 64 tiles...
  [========================================] 100% (64/64 tiles)
...
Render completed in 12.5s
Samples per pixel: 50.0 (range 50 - 50)
Average Luminosity: 0.1245
Render saved as output/cornell/render_20250126_143052.png
```

**Luminosity metric**: Average luminance across all pixels (0.0 = black, 1.0 = white)
- Useful for comparing integrator output
- Detects systematic brightness differences

## Quick Test Renders

### Fast Preview (1-2 seconds)
```bash
./raytracer --scene=cornell --max-samples=10 --max-passes=1
```
- Single pass with 10 samples per pixel
- Noisy but fast feedback
- Good for: verifying scene loads, catching crashes, basic visual check

### Medium Quality Test (5-15 seconds)
```bash
./raytracer --scene=cornell --max-samples=50 --max-passes=3
```
- Three passes: 1 sample, then 24, then 50 samples
- Moderate noise, acceptable for debugging
- Good for: comparing integrators, testing material changes, UV mapping verification

### High Quality Test (30-120 seconds)
```bash
./raytracer --scene=cornell --max-samples=200 --max-passes=5
```
- Five progressive passes up to 200 samples
- Low noise, suitable for visual validation
- Good for: final verification, generating reference images, presentation

### Production Quality (minutes to hours)
```bash
./raytracer --scene=cornell --max-samples=2000 --max-passes=10 --workers=20
```
- High sample count, many workers
- Very low noise, publication quality
- Good for: final renders, stress testing, performance benchmarking

## Comparing Integrators (PT vs BDPT)

### Side-by-Side Render Comparison

**Workflow**:
```bash
# Render with path tracing
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100 --max-passes=1
# Note luminosity from console output (e.g., 0.1245)

# Render with BDPT
./raytracer --scene=cornell --integrator=bdpt --max-samples=100 --max-passes=1
# Note luminosity from console output (e.g., 0.1238)

# Compare images visually
# Files in: output/cornell/render_*.png
```

**What to compare**:
- **Luminosity values**: Should match within 5-15% for same scene
- **Visual appearance**: Textures, shadows, color should be identical
- **Noise pattern**: Different but overall quality should be similar
- **Artifacts**: BDPT may show different fireflies than PT

### Automated Luminosity Comparison

**Script** (`benchmark.sh` or custom script):
```bash
#!/bin/bash
# Compare PT vs BDPT luminosity

echo "Rendering with Path Tracing..."
PT_OUTPUT=$(./raytracer --scene=cornell --integrator=path-tracing --max-samples=100 --max-passes=1 | grep "Average Luminosity")
PT_LUM=$(echo $PT_OUTPUT | awk '{print $3}')

echo "Rendering with BDPT..."
BDPT_OUTPUT=$(./raytracer --scene=cornell --integrator=bdpt --max-samples=100 --max-passes=1 | grep "Average Luminosity")
BDPT_LUM=$(echo $BDPT_OUTPUT | awk '{print $3}')

echo "Path Tracing Luminosity: $PT_LUM"
echo "BDPT Luminosity: $BDPT_LUM"
```

## Typical Debugging Workflow

### Scenario: Testing Material Change

**Goal**: Verify new material looks correct

1. **Make code change**: Edit `/pkg/material/lambertian.go`

2. **Rebuild**: `go build -o raytracer main.go`

3. **Quick test**: `./raytracer --scene=default --max-samples=20 --max-passes=1`
   - Visual check: does material look right?
   - Console check: does render complete without errors?

4. **Compare integrators**:
   ```bash
   ./raytracer --scene=default --integrator=path-tracing --max-samples=100 --max-passes=1
   ./raytracer --scene=default --integrator=bdpt --max-samples=100 --max-passes=1
   ```
   - Check luminosity values are similar
   - Visually compare images

5. **Run unit tests**: `go test ./pkg/material`
   - Verify material contract is satisfied

6. **Run integration tests**: `go test ./pkg/renderer -run Luminance`
   - Verify PT vs BDPT equivalence

### Scenario: Debugging Texture Sampling

**Goal**: Fix texture appearing wrong in BDPT

1. **Reproduce visually**:
   ```bash
   ./raytracer --scene=default --integrator=bdpt --max-samples=50 --max-passes=1
   ```
   - Observe: texture looks solid color instead of checkerboard

2. **Compare to PT**:
   ```bash
   ./raytracer --scene=default --integrator=path-tracing --max-samples=50 --max-passes=1
   ```
   - Observe: PT shows checkerboard correctly
   - **Hypothesis**: BDPT-specific texture sampling bug

3. **Run targeted test**:
   ```bash
   go test -v ./pkg/renderer -run "Textured.*Checkerboard"
   ```
   - Test fails, confirms BDPT texture issue
   - Saves debug images to `output/debug_renders/`

4. **Add debug logging**: Edit `/pkg/material/color_source.go`
   ```go
   func (c *Checkerboard) Evaluate(uv core.Vec2, p core.Vec3) core.Vec3 {
       fmt.Printf("UV: (%.3f, %.3f)\n", uv.X, uv.Y)
       // ... rest of function
   }
   ```

5. **Re-run with logging**:
   ```bash
   ./raytracer --scene=default --integrator=bdpt --max-samples=1 --max-passes=1 | grep "UV:"
   ```
   - Observe: all UV values are (0.000, 0.000)
   - **Root cause identified**: UV coordinates not preserved

6. **Fix and verify**:
   - Make fix in BDPT code
   - Rebuild: `go build -o raytracer main.go`
   - Test: `go test ./pkg/renderer -run Textured`
   - Visual check: `./raytracer --scene=default --integrator=bdpt --max-samples=50`

### Scenario: Performance Optimization

**Goal**: Measure performance improvement from BVH optimization

1. **Establish baseline**:
   ```bash
   ./raytracer --scene=dragon --max-samples=50 --max-passes=1
   # Note render time: e.g., "Render completed in 45.2s"
   ```

2. **Make optimization**: Edit BVH code

3. **Profile**:
   ```bash
   ./raytracer --scene=dragon --max-samples=50 --max-passes=1 --cpuprofile=cpu.prof
   ```

4. **Analyze profile**:
   ```bash
   go tool pprof cpu.prof
   # Commands: top, list <function>, web
   ```

5. **Compare performance**:
   ```bash
   ./benchmark.sh  # Compares current vs baseline
   # Or manually compare render times
   ```

## Profiling

### CPU Profiling

**Enable profiling**:
```bash
./raytracer --scene=dragon --max-samples=100 --cpuprofile=cpu.prof
```

**Analyze interactively**:
```bash
go tool pprof cpu.prof

# Interactive commands:
(pprof) top          # Show top functions by CPU time
(pprof) top -cum     # Show top functions by cumulative time
(pprof) list BVH.Hit # Show line-by-line profile for function
(pprof) web          # Generate call graph (requires graphviz)
```

**Generate reports**:
```bash
# Text report
go tool pprof -text cpu.prof > profile.txt

# PDF call graph
go tool pprof -pdf cpu.prof > profile.pdf
```

**Common bottlenecks to look for**:
- `BVH.Hit` - Intersection testing (should be fast with good BVH)
- `Material.Scatter` - Material evaluation (texture sampling can be slow)
- `Integrator.RayColor` - Integration overhead
- `rand.Float64` - Random number generation (significant in high-sample renders)

## Testing Workflow Summary

**Daily development**:
```bash
# Edit code
go build -o raytracer main.go
./raytracer --scene=cornell --max-samples=20 --max-passes=1
# Quick visual check
```

**Before committing**:
```bash
# Full test suite
go test ./...

# Integration test with visual output
go test -v ./pkg/renderer -run Luminance

# Manual integrator comparison
./raytracer --scene=cornell --integrator=path-tracing --max-samples=100
./raytracer --scene=cornell --integrator=bdpt --max-samples=100
```

**For pull requests**:
```bash
# High-quality validation renders
./raytracer --scene=cornell --integrator=bdpt --max-samples=500 --max-passes=5
./raytracer --scene=caustic-glass --integrator=bdpt --max-samples=500 --max-passes=5

# Benchmark performance
./benchmark.sh

# Full test coverage
go test -v ./...
```

## Common Gotchas

**Output files accumulate**: Old renders are not deleted automatically
- Manually clean: `rm -rf output/*/render_*.png`
- Or keep for comparison

**No progress bar for single pass**: Only shows progress when MaxPasses > 1
- Use `--max-passes=2` to see progress

**Timestamp collisions**: Running multiple renders rapidly creates files with same timestamp
- Add delay between renders or differentiate by scene name

**Large output files**: High-resolution renders create large PNG files
- 1920x1080 @ 2000 samples can be 5-10 MB
- Consider compressing or archiving old renders

**Memory usage**: High sample counts with complex scenes use significant RAM
- Monitor with `htop` or Activity Monitor
- Reduce samples or workers if memory constrained

## Access Log
