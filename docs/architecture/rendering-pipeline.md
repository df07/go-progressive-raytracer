# Rendering Pipeline Architecture

## Overview

The rendering pipeline flows from Camera → Tiles → Workers → Integrator → Pixels, with progressive passes refining the image over time. The ProgressiveRaytracer orchestrates parallel tile rendering via WorkerPool, while the Integrator handles ray-scene interaction and light transport. BDPT adds a splat system for cross-tile light contributions that are processed after each pass.

## High-Level Pipeline

```
User CLI Command
    ↓
main.go (createScene, renderProgressive)
    ↓
ProgressiveRaytracer.RenderPass()
    ↓
WorkerPool (parallel tile processing)
    ↓
TileRenderer.RenderTile() [per tile, per worker]
    ↓
Integrator.RayColor() [per pixel, per sample]
    ↓
Scene.Hit() → Material.Scatter() → Material.EvaluateBRDF()
    ↓
Pixel Color + SplatRays (BDPT only)
    ↓
ProgressiveRaytracer.ProcessSplats() [post-pass]
    ↓
Final Image (image.RGBA)
```

## Component Responsibilities

### ProgressiveRaytracer (`/pkg/renderer/progressive.go`)

**Purpose**: Orchestrates multi-pass progressive rendering with tile-based parallelism

**Responsibilities**:
- Divide image into 64x64 tiles
- Manage progressive passes (1, 2, 4, 8, ... samples per pixel)
- Coordinate worker pool for parallel processing
- Accumulate pixel statistics across passes
- Process splat rays after each pass (BDPT only)
- Generate final image

**Key data structures**:
```go
type ProgressiveRaytracer struct {
    scene       *scene.Scene
    config      ProgressiveConfig      // Quality settings
    tiles       []*Tile                // Tile grid
    currentPass int                    // Progressive state
    pixelStats  [][]PixelStats         // Accumulated pixel data
    splatQueue  *SplatQueue            // BDPT cross-tile contributions
    integrator  integrator.Integrator  // Light transport algorithm
    workerPool  *WorkerPool            // Parallel workers
    logger      core.Logger
}
```

**Progressive sampling strategy** (`getSamplesForPass()`):
- Pass 1: 1 sample (quick preview)
- Pass 2: ~12 samples (gradual refinement)
- Pass 3: ~25 samples
- ...
- Final pass: MaxSamplesPerPixel (e.g., 50)

**Why progressive rendering**:
- Immediate visual feedback (see image after 1 sample)
- User can stop early if quality is sufficient
- Tile-based animation shows progress

### WorkerPool (`/pkg/renderer/worker_pool.go`)

**Purpose**: Manage parallel tile rendering across CPU cores

**Responsibilities**:
- Create worker goroutines (default: number of CPU cores)
- Distribute tiles to workers via task queue
- Collect completed tiles
- Prevent race conditions with per-tile random seeds

**Worker lifecycle**:
```
Start() → spawn N goroutines → each goroutine:
    loop:
        task := taskQueue.pop()
        if task == nil: exit
        result := renderTile(task)
        resultQueue.push(result)
```

**Determinism guarantee**:
- Each tile gets unique random seed based on: `baseSeed + tileX*1000 + tileY`
- Same seed → identical random sequence → reproducible results
- Parallel execution order doesn't affect output

### TileRenderer (`/pkg/renderer/tile_renderer.go`)

**Purpose**: Render a single tile to target sample count

**Responsibilities**:
- Generate camera rays for each pixel in tile
- Call integrator for each ray
- Accumulate samples into pixel statistics
- Collect splat rays from BDPT
- Return rendered tile

**Per-pixel sampling loop**:
```go
for each pixel in tile:
    pixelSampler := NewRandomSampler(tileSeed + pixelX*100 + pixelY)
    for sample := currentSamples; sample < targetSamples; sample++:
        ray := camera.GetRay(pixelX, pixelY, pixelSampler)
        color, splats := integrator.RayColor(ray, scene, pixelSampler)

        pixelStats[pixelY][pixelX].accumulate(color)
        if splats != nil:
            splatQueue.add(splats)
```

**Coordinate systems**:
- Tile coordinates: (0,0) to (tileSize-1, tileSize-1) relative to tile origin
- Image coordinates: (0,0) to (width-1, height-1) absolute pixel positions
- Conversion: `imageX = tile.startX + tileX`

### Integrator (`/pkg/integrator/`)

**Purpose**: Compute pixel color by tracing light paths through the scene

**Responsibilities**:
- Construct light transport paths (camera to light, or bidirectional)
- Evaluate materials at ray-surface intersections
- Sample lights for direct illumination
- Apply MIS weighting to reduce variance
- Return pixel color and optional splat rays

**Interface**:
```go
type Integrator interface {
    // RayColor computes the color for a single ray
    // Returns: (pixel color, splat rays for cross-tile contributions)
    RayColor(ray core.Ray, scene *scene.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay)
}
```

**Two implementations**:
- **PathTracingIntegrator**: Unidirectional path tracing (no splats)
- **BDPTIntegrator**: Bidirectional path tracing (produces splats for t=1 strategies)

**When integrator is called**:
- Once per pixel per sample
- Example: 512x512 image, 50 samples = 13,107,200 integrator calls
- Performance critical - must be fast

### Scene (`/pkg/scene/`)

**Purpose**: Provide geometry intersection testing and light access

**Responsibilities**:
- BVH acceleration structure for fast ray-geometry intersection
- Light list for direct lighting sampling
- Camera for ray generation
- Material assignment to geometry

**Primary operation**:
```go
func (s *Scene) Hit(ray core.Ray, tMin, tMax float64) (*core.SurfaceInteraction, bool) {
    // Use BVH to find closest intersection
    // Returns surface point, normal, UV, material, etc.
}
```

**Preprocessing** (`Preprocess()`):
- Build BVH from scene geometry
- Initialize light sampling structures
- Validate scene consistency

### Material (`/pkg/material/`)

**Purpose**: Evaluate BRDF and sample scattering directions at surface points

**Responsibilities**:
- `Scatter()`: Generate random scattered direction for path continuation
- `EvaluateBRDF()`: Evaluate BRDF for specific incoming/outgoing directions
- `PDF()`: Compute probability density for sampled directions
- Texture evaluation via `ColorSource.Evaluate(uv, point)`

**Material evaluation in pipeline**:
```
Integrator calls Material.Scatter() at intersection
    → Material.Scatter() calls ColorSource.Evaluate(hit.UV, hit.Point)
        → Returns albedo, scattered direction, PDF

Integrator calls Material.EvaluateBRDF() for direct lighting
    → Material.EvaluateBRDF() calls ColorSource.Evaluate(hit.UV, hit.Point)
        → Returns BRDF value

CRITICAL: Both must use same UV coordinates from SurfaceInteraction
```

## Progressive Rendering Flow

### Pass 1: Initial Preview (1 sample per pixel)

```
RenderPass(1) called
    ↓
Calculate target samples: 1
    ↓
WorkerPool.Start() → spawn worker goroutines
    ↓
Submit all tiles as tasks to queue
    ↓
Workers render tiles in parallel:
    For each pixel in tile:
        1 sample → integrator → color
        Accumulate into pixelStats[y][x]
    ↓
All tiles complete
    ↓
ProcessSplats() if BDPT
    ↓
GenerateImage() from pixelStats
    ↓
Return noisy preview image
```

**Result**: Low-quality preview available in ~1 second (512x512 image)

### Pass 2: Progressive Refinement (target 12 samples per pixel)

```
RenderPass(2) called
    ↓
Calculate target samples: 12
    ↓
Reuse existing pixelStats (already have 1 sample)
    ↓
Workers render tiles:
    For each pixel:
        11 additional samples → integrator
        Accumulate into pixelStats[y][x] (now 12 total)
    ↓
ProcessSplats()
    ↓
GenerateImage() with averaged samples
    ↓
Return improved image
```

**Result**: Better quality, less noise, still fast (~3 seconds total)

### Final Pass: High Quality (target 50 samples per pixel)

```
RenderPass(N) called
    ↓
Target samples: 50
    ↓
Workers add remaining samples to reach 50
    ↓
ProcessSplats()
    ↓
GenerateImage()
    ↓
Return final high-quality image
```

**Result**: Smooth, low-noise image (~10 seconds total for 512x512)

## BDPT Splat System

### Why Splats Are Needed

**Problem**: Light tracing connects light paths to camera, but camera ray may originate from different tile than the one being rendered.

**Example**:
```
Worker 1 rendering tile (0, 0):
    Pixel (5, 5) → generates light path
    Light path vertex connects to camera
    Connection ray goes through pixel (450, 380) [different tile!]
    Cannot write to (450, 380) from tile (0, 0) [race condition]
```

**Solution**: Return splat rays for cross-tile contributions, process deterministically after all tiles complete.

### Splat Data Structure

```go
type SplatRay struct {
    Ray   core.Ray  // Ray from camera defining pixel location
    Color core.Vec3 // Light contribution (already MIS-weighted)
}
```

**How splats work**:
1. BDPT integrator creates splat ray during light tracing (s≥2, t=1 strategy)
2. TileRenderer adds splat to shared `splatQueue` (lock-free append)
3. After all tiles complete, `ProcessSplats()` traces each splat ray to find pixel coordinates
4. Splat color added to corresponding pixel's accumulated color

### Splat Processing

```go
func (pr *ProgressiveRaytracer) ProcessSplats() {
    for each splat in splatQueue:
        // Trace camera ray to determine pixel coordinates
        u, v := camera.WorldToRaster(splat.Ray)
        pixelX, pixelY := int(u), int(v)

        // Add splat contribution to pixel
        pixelStats[pixelY][pixelX].AddSplat(splat.Color)
}
```

**Why deterministic**:
- Splats processed in queue order (not tile completion order)
- Same splats → same pixel updates → reproducible results
- No race conditions (single-threaded post-pass)

**Performance impact**:
- Splat processing is fast (typically <1% of render time)
- Lock-free queue avoids contention during tile rendering
- Progressive passes show splats immediately (not deferred to end)

## Data Flow Example: Single Pixel Render

**Scenario**: Render pixel (256, 256) with BDPT, 1 sample

```
1. ProgressiveRaytracer.RenderPass(1)
   ↓
2. WorkerPool assigns tile containing (256, 256) to Worker 3
   ↓
3. TileRenderer.RenderTile() for Worker 3:
   - Tile seed: baseSeed + tileX*1000 + tileY*100
   - Pixel (256, 256) local tile coords: (0, 0) [if tile starts at (256, 256)]
   ↓
4. Generate camera ray:
   - pixelSampler := RandomSampler(tileSeed + 0*100 + 0)
   - ray := camera.GetRay(256.5, 256.5, pixelSampler)
   ↓
5. BDPTIntegrator.RayColor(ray, scene, pixelSampler):
   - Generate camera path: trace ray through scene
   - Hit geometry at (0, 0.5, -2) → create vertex with SurfaceInteraction
   - Material.Scatter() → samples texture at UV=(0.3, 0.7) → albedo=(0.8, 0.2, 0.2)
   - Generate light path from random light
   - Connect paths with MIS weighting
   - Create splat ray for t=1 strategy
   - Return (color=(0.15, 0.05, 0.05), splats=[splatRay1, splatRay2])
   ↓
6. TileRenderer accumulates:
   - pixelStats[256][256].accumulate(color)
   - splatQueue.add(splats)
   ↓
7. Tile completes, returns result
   ↓
8. All tiles complete
   ↓
9. ProcessSplats():
   - Trace splatRay1 → pixel (100, 300) → add contribution
   - Trace splatRay2 → pixel (450, 150) → add contribution
   ↓
10. GenerateImage():
    - pixel[256][256] = pixelStats[256][256].average()
    - Apply tone mapping, gamma correction
    - Convert to 8-bit RGB
    ↓
11. Return final image
```

## Why Bugs Appear Only in Full Renders

### Unit Tests vs Integration Tests vs Full Renders

**Unit test** (`Material.Scatter()` test):
- Tests single function in isolation
- Controlled inputs, no pipeline complexity
- May miss: data flow bugs, coordinate transformations, integrator-specific issues

**Integration test** (luminance comparison):
- Tests full pipeline with small image (32x32)
- Single tile (tile size = image size for determinism)
- May miss: tile boundary issues, splat race conditions, large scene issues

**Full render** (CLI 512x512 with 64x64 tiles):
- Multiple tiles rendered in parallel
- Splat processing across tile boundaries
- Real-world scene complexity
- Exposes: race conditions, tile stitching bugs, accumulation errors

### Common Issues Appearing Only in Full Renders

**Tile boundary artifacts**:
- Splats landing on tile edges
- Coordinate transformation errors between tile-local and image-global coords
- BVH traversal issues at tile boundaries (incorrect hit testing)

**Race conditions**:
- Concurrent writes to shared data (pixelStats, splatQueue)
- Non-deterministic results across runs
- Fix: Use locks or ensure lock-free append

**Accumulation errors**:
- Incorrect sample averaging across multiple passes
- Splat contributions not properly weighted
- Tone mapping applied at wrong stage

**Memory issues**:
- Large scenes exceed memory limits
- BVH build fails for high triangle counts
- Out-of-bounds array access with unusual image dimensions

## Performance Characteristics

### Render Time Breakdown (typical 512x512 scene, 50 samples)

- BVH traversal: 40-60% (geometry intersection testing)
- Material evaluation: 20-30% (BRDF, texture sampling)
- Integrator logic: 10-15% (path construction, MIS weighting)
- Random sampling: 5-10% (RNG calls)
- Image output: <1% (PNG encoding)
- Splat processing: <1% (BDPT only)

### Scaling Behavior

**Resolution scaling** (with constant samples):
- Time ∝ pixels (linear with width × height)
- Memory ∝ pixels (pixelStats array)
- Example: 512x512 → 1024x1024 = 4× time, 4× memory

**Sample scaling** (with constant resolution):
- Time ∝ samples (linear with samples per pixel)
- Memory constant (only final image grows slightly with quality)
- Example: 50 samples → 500 samples = 10× time, same memory

**Worker scaling**:
- Near-linear speedup up to CPU core count
- Diminishing returns beyond core count (overhead)
- Example: 1 worker = 100s, 8 workers = 15s (6.7× speedup)

**Scene complexity scaling** (BVH acceleration):
- Time ∝ log(triangles) with BVH (logarithmic)
- Time ∝ triangles without BVH (linear)
- Example: 1M triangle mesh with BVH = ~2× slower than 1K mesh, without BVH = ~1000× slower

## Debugging the Pipeline

### Isolating Pipeline Stages

**Test scene loading**:
```go
scene, err := createScene("cornell")
if err != nil {
    // Scene loading bug
}
scene.Preprocess() // BVH build happens here
```

**Test camera ray generation**:
```go
ray := camera.GetRay(256, 256, sampler)
// Verify ray origin and direction
```

**Test single integrator call**:
```go
color, splats := integrator.RayColor(ray, scene, sampler)
// Check color is reasonable, splats have correct format
```

**Test tile rendering**:
```go
tile := tiles[0]
result := tileRenderer.RenderTile(tile, 1, 50, sampler)
// Verify tile pixels are non-zero
```

**Test splat processing**:
```go
splatQueue.Add(testSplat)
ProcessSplats()
// Verify splat landed in correct pixel
```

### Common Debug Techniques

**Add logging at pipeline boundaries**:
```go
// In TileRenderer
fmt.Printf("Tile (%d,%d) rendered %d pixels\n", tile.startX, tile.startY, pixelCount)

// In Integrator
fmt.Printf("Ray color: (%f,%f,%f), splats: %d\n", color.X, color.Y, color.Z, len(splats))
```

**Render single tile**:
- Set `TileSize = Width` in test config
- Forces single-tile render (eliminates parallelism)
- Easier to debug without concurrency

**Disable splats**:
- Use path tracing instead of BDPT
- Isolates splat-related bugs

**Reduce sample count**:
- Use 1 sample per pixel for fast iteration
- Easier to trace individual ray paths

## Access Log
