# T=1 Strategies Implementation Specification

## Executive Summary

This specification outlines the implementation of t=1 strategies (light tracing to camera) for BDPT, which will complete our bidirectional path tracing implementation and allow removal of the current specular MIS hack.

## Problem Statement

Currently, our BDPT implementation only supports s=0 (path tracing) and s≥1,t≥2 (bidirectional connections). The missing t=1 strategies are crucial for:

1. **Proper MIS Weighting**: Without t=1 strategies, MIS incorrectly assumes these strategies exist and severely downweights path tracing strategies for certain light paths
2. **Caustic Rendering**: t=1 strategies are essential for rendering caustics and specular-diffuse-emissive paths
3. **Completeness**: A complete BDPT implementation should support all valid (s,t) combinations

## Current Architecture Analysis

### 1. RayColor Interface Limitations
**Current**: `RayColor(ray, scene, random, depth, throughput, sampleIndex) Vec3`
- Returns single color for single pixel
- No mechanism to contribute to arbitrary pixels
- Assumes 1:1 ray-to-pixel mapping

### 2. Parallel Rendering Constraints
**Current**: Tile-based parallelism with non-overlapping regions
- Each worker modifies only its assigned tile
- Thread-safe because of spatial partitioning
- Shared `pixelStats [][]PixelStats` array

### 3. Progressive Rendering Pipeline
**Current**: Linear pipeline from ray generation to pixel accumulation
- `camera.GetRay()` → `integrator.RayColor()` → `pixelStats.AddSample()`
- No support for samples affecting multiple pixels

## Architecture Changes Required

### 1. Enhanced Integrator Interface

```go
// SplatRay represents a ray-based color contribution that needs to be mapped to pixels
type SplatRay struct {
    Ray   Ray  // Ray that should contribute to some pixel
    Color Vec3 // Color contribution
}

// Enhanced integrator interface
type Integrator interface {
    // Legacy interface (keep existing name for backward compatibility)
    RayColor(ray Ray, scene Scene, random *rand.Rand, depth int, throughput Vec3, sampleIndex int) Vec3
    
    // Enhanced interface supporting ray-based splatting
    // Returns (pixel color, splat rays)
    RayColorWithSplats(ray Ray, scene Scene, random *rand.Rand, depth int, throughput Vec3, sampleIndex int) (Vec3, []SplatRay)
}
```

### 2. Simple Splat Queue System

```go
// SplatXY represents a splat with pre-computed pixel coordinates
type SplatXY struct {
    X, Y  int      // Pixel coordinates (computed when enqueuing)
    Color core.Vec3 // Color contribution
}

// Simple SplatQueue with mutex protection
type SplatQueue struct {
    splats []SplatXY
    mutex  sync.Mutex
}

func NewSplatQueue() *SplatQueue {
    return &SplatQueue{
        splats: make([]SplatXY, 0, 1000), // Pre-allocate buffer
    }
}

func (sq *SplatQueue) AddSplat(x, y int, color core.Vec3) {
    sq.mutex.Lock()
    defer sq.mutex.Unlock()
    sq.splats = append(sq.splats, SplatXY{X: x, Y: y, Color: color})
}

// ExtractSplatsForTile removes and returns splats affecting this tile
func (sq *SplatQueue) ExtractSplatsForTile(bounds image.Rectangle) []SplatXY {
    sq.mutex.Lock()
    defer sq.mutex.Unlock()
    
    var tileSplats []SplatXY
    var remaining []SplatXY
    
    for _, splat := range sq.splats {
        if splat.X >= bounds.Min.X && splat.X < bounds.Max.X &&
           splat.Y >= bounds.Min.Y && splat.Y < bounds.Max.Y {
            tileSplats = append(tileSplats, splat)
        } else {
            remaining = append(remaining, splat)
        }
    }
    
    sq.splats = remaining
    return tileSplats
}
```

### 3. Enhanced Camera Interface for T=1 Strategies

```go
// CameraSample represents camera sampling result for t=1 strategies
type CameraSample struct {
    Ray         Ray     // Ray from camera toward reference point
    Weight      Vec3    // Camera importance weight (We function result)
    PDF         float64 // Probability density for this sample
    PixelX      int     // Raster X coordinate
    PixelY      int     // Raster Y coordinate
}

// Enhanced camera interface for t=1 strategies
type Camera interface {
    // Existing methods
    GetRay(i, j int, random *rand.Rand) Ray
    CalculateRayPDFs(ray Ray) (areaPDF, directionPDF float64)
    GetCameraForward() Vec3
    
    // New method for t=1 strategies - sample camera from a reference point
    // Camera handles lens sampling internally, returns complete sample
    SampleCameraFromPoint(refPoint Vec3, random *rand.Rand) *CameraSample
    
    // Map ray back to pixel coordinates (for splat placement)
    MapRayToPixel(ray Ray) (x, y int, ok bool)
}
```

## Implementation Plan (Test-First Approach)

### Phase 1: T=1 Strategy Implementation (Test-Only)
1. Add `SplatRay` type and `RayColorWithSplats` method to BDPT integrator
2. Implement `SampleCameraFromPoint` in perspective camera
3. Add t=1 strategy to `ConnectBDPT` function  
4. **Create test in `bdpt_test.go` based on `TestCornellSpecularReflections`**
5. Validate that t=1 strategies generate expected splat rays
6. Test MIS weighting and contribution calculations

### Phase 2: Splat System Integration
1. Add `SplatXY` type and `SplatQueue` 
2. Modify `TileRenderer` to use `RayColorWithSplats` and handle splat queue
3. Update `ProgressiveRaytracer` to pass shared `SplatQueue`
4. Implement tile-local splat extraction and application

### Phase 3: Pipeline Integration and Cleanup
1. Add splat contribution to web streaming
2. Remove specular MIS hack from `calculateMISWeight`
3. Performance testing and optimization

### Phase 4: Testing and Validation
1. Create comprehensive t=1 strategy tests
2. Validate against PBRT reference implementation
3. Test caustic rendering scenarios
4. Performance benchmarking

### Phase 5: Cleanup
1. Remove specular MIS hack from `calculateMISWeight`
2. Remove legacy `RayColorLegacy` interface
3. Update documentation and examples

## Key Implementation Details

### 1. T=1 Strategy in ConnectBDPT

```go
// T=1 case: Sample camera and connect to light subpath
if t == 1 {
    qs := lightPath.Vertices[s-1]
    if qs.IsConnectible() {
        // Sample camera from light vertex position
        // Camera handles lens sampling internally
        cameraSample := camera.SampleCameraFromPoint(qs.Point, random)
        if cameraSample != nil && cameraSample.PDF > 0 {
            // Calculate contribution  
            wi := cameraSample.Ray.Direction.Negate() // Direction from camera to light
            contribution := qs.Beta.Multiply(
                qs.BRDF(wi, qs.Normal),
            ).Multiply(cameraSample.Weight)
            
            // Apply geometry term (cosine at surface)
            if qs.IsOnSurface() && !contribution.IsZero() {
                cosTheta := math.Max(0, qs.Normal.Dot(wi))
                contribution = contribution.MultiplyScalar(cosTheta)
            }
            
            // Apply MIS weight with splat scaling
            misWeight := calculateMISWeight(lightPath, cameraPath, s, t) 
            splatScale := float64(width * height) / float64(pixelArea)
            contribution = contribution.MultiplyScalar(misWeight * splatScale)
            
            // Add as splat ray (ray from light toward camera for mapping)
            if !contribution.IsZero() {
                splats = append(splats, SplatRay{
                    Ray:   cameraSample.Ray.Reverse(), // Ray from light toward camera
                    Color: contribution,
                })
            }
        }
    }
}
```

### 2. Camera Sampling Implementation

```go
func (c *PerspectiveCamera) SampleCameraFromPoint(refPoint Vec3, random *rand.Rand) *CameraSample {
    // Sample lens point (camera handles lens sampling internally)
    lensRadius := c.lensRadius
    if lensRadius == 0 {
        lensRadius = 1e-6 // Treat pinhole as tiny lens
    }
    
    // Sample lens coordinates using concentric disk sampling
    lensU := random.Float64()
    lensV := random.Float64()
    pLens := concentricSampleDisk(lensU, lensV).MultiplyScalar(lensRadius)
    
    // Transform lens point to world space
    lensPoint := c.cameraToWorld.TransformPoint(Vec3{X: pLens.X, Y: pLens.Y, Z: 0})
    
    // Create ray from lens toward reference point
    direction := refPoint.Subtract(lensPoint).Normalize()
    ray := Ray{Origin: lensPoint, Direction: direction}
    
    // Calculate PDF (inverse square law + lens area + cosine)
    distance := refPoint.Subtract(lensPoint).Length()
    lensArea := math.Pi * lensRadius * lensRadius
    cosTheta := math.Max(0, c.forward.Dot(direction))
    if cosTheta == 0 {
        return nil // Ray doesn't hit camera front
    }
    
    pdf := (distance * distance) / (cosTheta * lensArea)
    
    // Calculate camera importance weight using We function
    we := c.We(ray) // Camera response function
    if we.IsZero() {
        return nil // Ray doesn't contribute
    }
    
    // Map ray to pixel coordinates
    pixelX, pixelY, ok := c.MapRayToPixel(ray)
    if !ok {
        return nil // Ray doesn't hit image plane
    }
    
    return &CameraSample{
        Ray:    ray,
        Weight: we,
        PDF:    pdf,
        PixelX: pixelX,
        PixelY: pixelY,
    }
}

func (c *PerspectiveCamera) MapRayToPixel(ray Ray) (int, int, bool) {
    // Project ray back onto image plane and convert to pixel coordinates
    // This is the inverse of GetRay - map world ray to pixel coordinates
    
    // Transform ray to camera space
    cameraRay := c.worldToCamera.TransformRay(ray)
    
    // Project onto image plane (z=focal_distance or z=1 for pinhole)
    t := c.focalDistance / (-cameraRay.Direction.Z)
    if t <= 0 {
        return 0, 0, false // Ray going wrong direction
    }
    
    hitPoint := cameraRay.Origin.Add(cameraRay.Direction.MultiplyScalar(t))
    
    // Convert to raster coordinates
    x := int(hitPoint.X + 0.5)
    y := int(hitPoint.Y + 0.5)
    
    if x >= 0 && x < c.width && y >= 0 && y < c.height {
        return x, y, true
    }
    
    return 0, 0, false
}
```

### 3. Tile Rendering Integration (Phase 2)

```go
func (tr *TileRenderer) RenderTileBounds(bounds image.Rectangle, pixelStats [][]PixelStats, 
    splatQueue *SplatQueue, random *rand.Rand, targetSamples int) RenderStats {
    
    camera := tr.scene.GetCamera()
    samplingConfig := tr.scene.GetSamplingConfig()
    stats := tr.initRenderStatsForBounds(bounds, targetSamples)
    
    // Step 1: Regular tile processing with splat generation
    for j := bounds.Min.Y; j < bounds.Max.Y; j++ {
        for i := bounds.Min.X; i < bounds.Max.X; i++ {
            samplesUsed := tr.adaptiveSamplePixelWithSplats(
                camera, i, j, &pixelStats[j][i], splatQueue, random, targetSamples, samplingConfig)
            tr.updateStats(&stats, samplesUsed)
        }
    }
    
    // Step 2: Extract and apply splats affecting this tile
    tileSplats := splatQueue.ExtractSplatsForTile(bounds)
    for _, splat := range tileSplats {
        // Apply splat to pixel (coordinates already computed)
        pixelStats[splat.Y][splat.X].AddSample(splat.Color)
    }
    
    tr.finalizeStats(&stats)
    return stats
}

func (tr *TileRenderer) adaptiveSamplePixelWithSplats(camera core.Camera, i, j int, 
    ps *PixelStats, splatQueue *SplatQueue, random *rand.Rand, maxSamples int, 
    samplingConfig core.SamplingConfig) int {
    
    initialSampleCount := ps.SampleCount
    
    for ps.SampleCount < maxSamples && !tr.shouldStopSampling(ps, maxSamples, samplingConfig) {
        ray := camera.GetRay(i, j, random)
        
        // Use enhanced integrator with splat support
        pixelColor, splatRays := tr.integrator.RayColorWithSplats(ray, tr.scene, random, 
            samplingConfig.MaxDepth, core.Vec3{X: 1.0, Y: 1.0, Z: 1.0}, ps.SampleCount)
        
        // Add regular contribution
        ps.AddSample(pixelColor)
        
        // Process splat contributions
        for _, splatRay := range splatRays {
            // Map ray to pixel coordinates and add to queue
            if x, y, ok := camera.MapRayToPixel(splatRay.Ray); ok {
                splatQueue.AddSplat(x, y, splatRay.Color)
            }
        }
    }
    
    return ps.SampleCount - initialSampleCount
}
```

## Concurrency Strategy

### 1. Splat Accumulation
- Use fine-grained locking per pixel or pixel region
- Consider lock-free atomic operations for simple accumulation
- Batch splat contributions to reduce lock contention

### 2. Memory Layout
- Maintain existing `pixelStats` array for regular contributions
- Add separate `SplatAccumulator` for cross-tile contributions
- Merge splat contributions during final image assembly

### 3. Performance Optimization
- Pre-allocate splat buffers to avoid allocations
- Use buffered channels for splat communication
- Consider spatial partitioning for splat contributions

## Testing Strategy

### 1. Unit Tests
- Camera sampling correctness
- PDF calculations
- MIS weight calculations
- Splat accumulation thread safety

### 2. Integration Tests
- T=1 strategy contribution validation
- Caustic rendering tests
- Performance comparison with/without t=1

### 3. Visual Validation
- Cornell box with caustic lighting
- Specular-diffuse-emissive test scenes
- Comparison with PBRT reference renders

## Migration Strategy

### 1. Backward Compatibility
- Maintain existing `RayColorLegacy` method
- Default implementation returns empty splats
- Gradual migration of integrators

### 2. Feature Flags
- Add `enableT1Strategies` configuration flag
- Allow testing with/without t=1 strategies
- Performance comparison tooling

### 3. Documentation
- Update architecture documentation
- Add t=1 strategy examples
- Performance tuning guidelines

## Success Criteria

1. **Correctness**: T=1 strategies produce visually correct caustics and specular reflections
2. **Performance**: <20% performance overhead for scenes without caustics
3. **Completeness**: All BDPT strategies (s,t) combinations implemented
4. **Stability**: No race conditions or memory leaks in parallel rendering
5. **Compatibility**: Backward compatible with existing integrators

## Implementation Priority

1. **High Priority**: Core interface changes and camera sampling
2. **High Priority**: T=1 strategy implementation in BDPT
3. **Medium Priority**: Thread-safe splat accumulation
4. **Medium Priority**: Web streaming integration
5. **Low Priority**: Performance optimizations and advanced features

## Future Enhancements

1. **Adaptive Sampling**: Adjust t=1 strategy sampling based on scene characteristics
2. **Metropolis Integration**: Support for Metropolis Light Transport with t=1
3. **GPU Implementation**: CUDA/OpenCL support for parallel t=1 calculations
4. **Advanced Caustics**: Photon mapping integration for complex caustic scenarios