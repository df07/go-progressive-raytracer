# Bidirectional Path Tracing (BDPT) Implementation Specification

## Overview

This document outlines the implementation requirements for adding Bidirectional Path Tracing (BDPT) to our progressive raytracer through a clean integrator abstraction. BDPT is a sophisticated light transport algorithm that constructs paths from both camera and light sources, then connects them to create complete light transport paths. This approach is particularly effective for complex lighting scenarios involving caustics, focused indirect lighting, and small bright light sources.

**Key Architectural Goal**: Refactor the current monolithic `Raytracer` into a pluggable integrator system that supports multiple rendering algorithms (Path Tracing, BDPT, future techniques like MLT/VCM) while maintaining our progressive tile-based architecture.

## Integrator Architecture Refactoring

### Current Architecture Limitations

The existing system tightly couples the path tracing algorithm with the progressive renderer:

1. **Monolithic Raytracer**: `pkg/renderer/raytracer.go` contains path tracing logic mixed with pixel sampling
2. **Hard-coded Algorithm**: `rayColorRecursive()` is embedded in the `Raytracer` struct
3. **No Algorithm Abstraction**: Cannot switch between different light transport methods
4. **Mixed Responsibilities**: Scene intersection, material sampling, and integration logic are intertwined

### Proposed Integrator Interface

```go
// Integrator defines the interface for light transport algorithms
type Integrator interface {
    // RayColor computes the color for a single ray (matches current rayColorRecursive signature)  
    RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, depth int, throughput core.Vec3, sampleIndex int) core.Vec3
}

// IntegratorConfig contains common configuration for all integrators
type IntegratorConfig struct {
    MaxDepth                  int
    RussianRouletteMinBounces int
    RussianRouletteMinSamples int
}
```

### Refactored Architecture

#### 1. Core Integrator Implementations
```
pkg/integrator/
├── interface.go        # Integrator interface and config types
├── path_tracing.go    # Current algorithm extracted and cleaned
├── bdpt.go           # New BDPT integrator
└── integrator_test.go # Cross-algorithm validation tests
```

#### 2. Updated Progressive Renderer
```go
// ProgressiveRaytracer becomes integrator-agnostic
type ProgressiveRaytracer struct {
    scene         core.Scene
    width, height int
    config        ProgressiveConfig
    integrator    Integrator  // Pluggable rendering algorithm
    tiles         []*Tile
    // ... rest unchanged
}

// NewProgressiveRaytracer now accepts an integrator
func NewProgressiveRaytracer(scene core.Scene, width, height int, 
                           config ProgressiveConfig, integrator Integrator, 
                           logger core.Logger) *ProgressiveRaytracer
```

#### 3. Simple Constructors (No Factory Needed)
```go
// Simple constructors - no factory pattern complexity
func NewPathTracingIntegrator(scene core.Scene, config IntegratorConfig) *PathTracingIntegrator
func NewBDPTIntegrator(scene core.Scene, config BDPTConfig) *BDPTIntegrator

// Usage is direct and clear:
integrator := NewPathTracingIntegrator(scene, config)
// or
integrator := NewBDPTIntegrator(scene, bdptConfig)
```

### Migration Strategy

#### Phase 1: Extract Current Path Tracer
1. **Create Integrator Interface**: Define minimal `Integrator` interface in `pkg/integrator/interface.go`
2. **Extract Path Tracing Logic**: Move `rayColorRecursive()` to `PathTracingIntegrator.RayColor()`
3. **Update Progressive Renderer**: Modify to use `Integrator` interface instead of embedded logic

#### Phase 2: BDPT Implementation
1. **Implement BDPT Integrator**: Following the consolidated approach in `bdpt.go`
2. **Add Integrator Selection**: Update CLI and web interface to choose integrators  
3. **Integration Testing**: Compare BDPT vs Path Tracing convergence

## Current Strengths (Preserved in Refactoring)

Our current raytracer has several architectural advantages that will be preserved:

1. **Clean Interface Design**: Well-separated interfaces (`Shape`, `Material`, `Light`) with consistent APIs
2. **Multiple Importance Sampling**: Already implemented via `PowerHeuristic()` in `pkg/core/sampling.go`
3. **Path Tracing Foundation**: Robust unidirectional path tracing with proper PDF tracking
4. **Material System**: Unified `ScatterResult` with PDF support for both specular and diffuse materials
5. **Progressive Architecture**: Tile-based parallel processing that can accommodate multiple integrators

## Extracted Path Tracing Integrator

### PathTracingIntegrator Implementation
```go
type PathTracingIntegrator struct {
    config IntegratorConfig
    bvh    *core.BVH  // Scene acceleration structure
}

func (pt *PathTracingIntegrator) RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, depth int, throughput core.Vec3, sampleIndex int) core.Vec3 {
    // Direct implementation of current rayColorRecursive logic:
    // - Russian Roulette termination
    // - BVH intersection via scene.GetBVH().Hit()
    // - Background gradient
    // - Material scattering with MIS
    // - Recursive ray tracing
    
    // No unnecessary wrapper - this IS the core algorithm
}
```

## BDPT Algorithm Requirements (Consolidated in bdpt.go)

All BDPT-specific types and logic will be contained in `pkg/integrator/bdpt.go`:

```go
// BDPT-specific types (internal to bdpt.go)
type Vertex struct {
    Point     Vec3     // 3D position
    Normal    Vec3     // Surface normal  
    Material  Material // Material at this vertex
    
    // Path tracing information
    IncomingDirection Vec3   // Direction ray arrived from
    OutgoingDirection Vec3   // Direction ray continues to
    
    // MIS probability densities
    ForwardPDF  float64     // PDF for generating this vertex forward
    ReversePDF  float64     // PDF for generating this vertex reverse
    
    // Vertex classification
    IsLight     bool        // On light source
    IsCamera    bool        // On camera
    IsSpecular  bool        // Specular interaction
    
    // Transport quantities
    Throughput  Vec3        // Accumulated throughput to vertex
    Beta        Vec3        // BSDF * cos(theta) / pdf
}

type Path struct {
    Vertices []Vertex
    Length   int
}

// BDPTIntegrator implements full BDPT algorithm
type BDPTIntegrator struct {
    config       BDPTConfig
    bvh          *core.BVH
    lightPaths   []Path        // Cached light subpaths per tile
}

func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, depth int, throughput core.Vec3, sampleIndex int) core.Vec3 {
    // Complete BDPT implementation:
    // 1. Generate camera subpath starting from ray
    // 2. Connect to cached light subpaths  
    // 3. Calculate MIS weights for all strategies
    // 4. Return weighted contribution sum
}

// All supporting functions (path generation, connection, MIS) in same file
func (bdpt *BDPTIntegrator) generateCameraSubpath(...) Path { }
func (bdpt *BDPTIntegrator) generateLightSubpath(...) Path { }
func (bdpt *BDPTIntegrator) connectVertices(...) float64 { }
func (bdpt *BDPTIntegrator) calculateMISWeight(...) float64 { }
```

**Benefits of consolidation**:
- Single file contains complete BDPT algorithm
- Easy to understand data flow between components
- Can split into multiple files later if needed
- Reduces import complexity and circular dependencies

## Implementation Strategy

### Phase 1: Core Infrastructure

#### 1.1 Vertex and Path Data Structures
- Create `Vertex` struct with all required geometric and transport information
- Implement `Path` container with helper methods for vertex manipulation
- Add path validation and debugging utilities

#### 1.2 Path Generation Framework
- Extend current `rayColorRecursive()` to generate and store vertex sequences
- Implement light path generation starting from light sources
- Add proper PDF tracking for both forward and reverse directions

#### 1.3 Basic Connection Logic
- Implement visibility testing between arbitrary vertices
- Calculate geometric terms G(x,y) for vertex connections
- Handle edge cases (specular surfaces, light sources, camera)

### Phase 2: Complete BDPT Implementation

#### 2.1 MIS Weight Calculation
- Implement full BDPT MIS weight calculation following PBRT formulation
- Handle all path generation strategies (s,t) where s+t = path length
- Add proper handling of delta distributions (specular materials)

#### 2.2 Integration with Progressive Renderer
- Modify tile rendering to use BDPT instead of unidirectional path tracing
- Ensure deterministic results with tile-specific random seeds
- Maintain compatibility with existing progressive architecture

#### 2.3 Performance Optimization
- Implement efficient light subpath reuse across pixels
- Add Russian Roulette termination for both subpaths
- Optimize memory usage for path storage

### Phase 3: Advanced Features

#### 3.1 Specialized Sampling Strategies
- Add dedicated strategies for direct illumination (s=1, t=1)
- Implement light tracing strategy (s=0, t>1) 
- Handle camera connections (s>0, t=1)

#### 3.2 Material-Specific Optimizations
- Optimize connections involving specular materials
- Add proper handling of emissive materials in path vertices
- Implement efficient sampling for area lights

## Required Code Changes

### Phase 1: Integrator Refactoring

#### New Files to Create
```
pkg/integrator/
├── interface.go       # Integrator interface and common types
├── sampler.go        # Random sampling abstraction  
├── path_tracing.go   # Extracted current algorithm
└── integrator_test.go # Cross-algorithm validation
```

#### Files to Modify
```
pkg/renderer/progressive.go  # Accept Integrator interface
pkg/renderer/raytracer.go   # Deprecate or remove monolithic implementation
main.go                     # Add integrator selection CLI flags
web/server/server.go        # Add integrator selection to web interface
```

### Phase 2: BDPT Implementation  

#### New Files to Create
```
pkg/integrator/
├── interface.go       # Integrator interface and common types
├── sampler.go        # Random sampling abstraction  
├── path_tracing.go   # Extracted current algorithm (includes hitWorld, backgroundGradient)
├── bdpt.go           # Complete BDPT integrator with vertex/path logic
└── integrator_test.go # Cross-algorithm validation
```

**Rationale**: Consolidate related functionality instead of micro-files:
- `hitWorld()` and `backgroundGradient()` are simple utilities that belong with the integrator that uses them
- BDPT vertex/path logic can be in the main `bdpt.go` file unless it becomes unwieldy
- BVH intersection logic already exists in `pkg/core/bvh.go`

#### Configuration Updates
```
pkg/integrator/interface.go # Add BDPT-specific config
main.go                     # Add --integrator=bdpt CLI flag
web/static/index.html       # Add integrator selection dropdown
```

### Integrator Interface Migration

#### Before (Current):
```go
// Tightly coupled in progressive.go
raytracer := NewRaytracer(scene, width, height)
// Direct method calls on raytracer
```

#### After (Refactored):
```go
// Flexible integrator selection
integrator := CreateIntegrator(PathTracing, config)
progressive := NewProgressiveRaytracer(scene, width, height, config, integrator, logger)

// Or for BDPT:
integrator := CreateIntegrator(BDPT, bdptConfig)
```

### CLI Interface Updates

#### Updated CLI Flags
```bash
# Current (implicit path tracing)
./raytracer --scene=caustic-glass --mode=progressive

# New (explicit integrator selection)  
./raytracer --scene=caustic-glass --mode=progressive --integrator=path-tracing
./raytracer --scene=caustic-glass --mode=progressive --integrator=bdpt
./raytracer --scene=caustic-glass --mode=progressive --integrator=bdpt --bdpt-light-paths=16
```

### Web Interface Updates

#### Integrator Selection UI
```html
<select id="integrator-select">
  <option value="path-tracing">Path Tracing (Current)</option>
  <option value="bdpt">Bidirectional Path Tracing</option>
</select>

<div id="bdpt-config" style="display:none;">
  <label>Light Paths per Tile: <input type="number" value="16"></label>
  <label>Max Light Depth: <input type="number" value="10"></label>
</div>
```

### Backward Compatibility Strategy

#### Option 1: Deprecation Path
```go
// Keep existing Raytracer for compatibility
func NewRaytracer(scene core.Scene, width, height int) *Raytracer {
    // Internally use PathTracingIntegrator
    integrator := NewPathTracingIntegrator(scene.GetSamplingConfig())
    return &Raytracer{integrator: integrator, ...}
}
```

#### Option 2: Clean Break
```go
// Remove Raytracer entirely, force migration to integrator system
// Better long-term architecture, requires updating all call sites
```

### Simplified File Structure
```
pkg/core/                  # Core types, mostly unchanged
├── interfaces.go         # Extend Scene interface with GetBVH() method
├── sampling.go          # Extend for BDPT MIS weight calculations
└── (existing files)     # vec3.go, scene.go, bvh.go unchanged

pkg/integrator/           # New integrator system (4 files total)
├── interface.go         # Integrator interface and config types
├── path_tracing.go     # Extracted current algorithm + utilities
├── bdpt.go            # Complete BDPT implementation  
└── integrator_test.go  # Cross-algorithm validation

pkg/renderer/            # Progressive system, integrator-agnostic
├── progressive.go      # Updated to use Integrator interface  
├── camera.go          # Unchanged
├── worker_pool.go     # Unchanged
└── raytracer.go       # Deprecated/removed after migration
```

**Key Simplifications**:
1. **No Sampler abstraction**: Keep using `*rand.Rand` directly - it works fine
2. **No factory pattern**: Simple constructors are clearer than `CreateIntegrator()` 
3. **No GetName()/GetConfig()/Preprocess()**: Remove unused interface methods
4. **No scene_utils.go/intersection.go**: Simple utilities stay with integrators
5. **No bdpt/ subdirectory**: All BDPT logic in single `bdpt.go` file
6. **4 new files total**: Minimal, focused implementation

## Implementation Challenges

### 1. Path PDF Tracking
**Challenge**: Tracking both forward and reverse PDFs for each vertex in both subpaths.

**Solution**: 
- Store PDFs during path generation in `Vertex` struct
- Implement `CalculateReversePDF()` for each material type
- Use area measure consistently throughout implementation

### 2. Specular Material Handling
**Challenge**: Connecting paths through specular materials requires special handling.

**Solution**:
- Mark specular vertices with delta flags
- Skip invalid connections involving specular surfaces
- Use proper Dirac delta handling in MIS weight calculation

### 3. Camera and Light Endpoint Handling
**Challenge**: Camera and light endpoints have different sampling characteristics.

**Solution**:
- Implement separate endpoint vertex types
- Handle camera ray generation PDF correctly
- Add proper light emission PDF calculation

### 4. Memory and Performance
**Challenge**: BDPT requires storing multiple path vertices and testing many connections.

**Solution**:
- Reuse light subpaths across pixels in the same tile
- Implement efficient early termination strategies
- Use spatial data structures for visibility testing optimization

## Integration with Progressive Architecture

### Integrator-Agnostic Tile Processing
```go
// Progressive renderer becomes integrator-agnostic
func (pr *ProgressiveRaytracer) RenderTile(tile *Tile, pass int, sampler Sampler) {
    // Integrator-specific preprocessing (e.g., BDPT light path generation)
    pr.integrator.Preprocess(pr.scene, tile.Bounds)
    
    // Process each pixel in tile using the selected integrator
    for y := tile.Bounds.Min.Y; y < tile.Bounds.Max.Y; y++ {
        for x := tile.Bounds.Min.X; x < tile.Bounds.Max.X; x++ {
            sampler.StartPixel(x, y)
            
            for sample := 0; sample < targetSamples; sample++ {
                sampler.StartSample(sample)
                
                // Get ray for this pixel/sample
                ray := camera.GetRay(x, y, sampler)
                
                // Use integrator to compute color  
                color := pr.integrator.RayColor(ray, pr.scene, random, maxDepth, core.Vec3{1,1,1}, sample)
                
                // Accumulate in pixel statistics
                pr.pixelStats[y][x].AddSample(color)
            }
        }
    }
}
```

### Integrator-Specific Configuration
```go
// Base configuration shared by all integrators
type IntegratorConfig struct {
    MaxDepth                  int
    RussianRouletteMinBounces int
    RussianRouletteMinSamples int
}

// BDPT-specific configuration extends base
type BDPTConfig struct {
    IntegratorConfig                    // Embedded base config
    MaxCameraDepth       int           // Maximum camera subpath length  
    MaxLightDepth        int           // Maximum light subpath length
    LightPathsPerTile    int           // Number of light subpaths per tile
    UseDirectConnections bool          // Include direct lighting strategies
    Strategy             BDPTStrategy  // Which BDPT strategies to use
}

type BDPTStrategy int
const (
    AllStrategies BDPTStrategy = iota
    LightTracingOnly  // s=0, t>0 (good for complex indirect lighting)
    CameraTracingOnly // s>0, t=0 (equivalent to path tracing)
    BalancedStrategy  // Mix of strategies
)
```

### Integrator Factory with Configuration
```go
func CreateIntegrator(integratorType IntegratorType, config interface{}) Integrator {
    switch integratorType {
    case PathTracing:
        if cfg, ok := config.(IntegratorConfig); ok {
            return NewPathTracingIntegrator(cfg)
        }
        return NewPathTracingIntegrator(DefaultIntegratorConfig())
        
    case BDPT:
        if cfg, ok := config.(BDPTConfig); ok {
            return NewBDPTIntegrator(cfg)
        }
        return NewBDPTIntegrator(DefaultBDPTConfig())
        
    default:
        return NewPathTracingIntegrator(DefaultIntegratorConfig())
    }
}
```

## Testing and Validation Strategy

### 1. Unit Tests
- Test path generation produces valid vertex sequences
- Verify MIS weights sum to 1.0 across all strategies
- Test connection logic handles edge cases correctly

### 2. Integration Tests
- Compare BDPT results with unidirectional path tracing for simple scenes
- Verify caustic rendering improvement in glass scenes
- Test progressive convergence maintains deterministic results

### 3. Performance Benchmarks
- Measure BDPT vs standard path tracing performance
- Profile memory usage for path storage
- Benchmark tile-level light subpath reuse efficiency

## Benefits for Caustic Glass Scene

BDPT implementation will significantly improve our caustic glass scene rendering:

1. **Light Path Starting Points**: Light subpaths starting from the spot light will efficiently find caustic paths through glass
2. **Reduced Variance**: Multiple path generation strategies will reduce noise in focused light areas
3. **Better Convergence**: Complex glass-to-diffuse surface transport will converge faster
4. **Spot Light Efficiency**: Direct connections from light source will handle sharp light distributions better

## Implementation Checklist

### Critical Design Decisions Made

1. **Ultra-minimal Integrator interface** - just `RayColor()` with exact same signature as current `rayColorRecursive()`
2. **Keep `*rand.Rand`** - no Sampler abstraction needed
3. **Simple constructors** - no factory pattern 
4. **4 files only**: `interface.go`, `path_tracing.go`, `bdpt.go`, `integrator_test.go`
5. **All BDPT logic consolidated** in single `bdpt.go` file

### Implementation Strategy

**Phase 1: Extract Current Path Tracer**
- Move `rayColorRecursive()` → `PathTracingIntegrator.RayColor()`
- Update `ProgressiveRaytracer` to use `Integrator` interface
- **Critical**: Verify identical results with existing system
- No other changes - pure refactoring step

**Phase 2: Add BDPT**
- Implement complete BDPT algorithm in single `bdpt.go` file
- Add CLI flag: `--integrator=bdpt`
- Add web interface dropdown for integrator selection
- Test caustic glass scene improvements

### Key Benefits for Caustic Glass Scene

BDPT will significantly improve caustics because:
- **Light subpaths starting from spot light** find caustic paths efficiently
- **Multiple connection strategies** reduce variance in focused lighting areas
- **Better convergence** for complex glass→diffuse surface transport
- **Handles small bright lights** much better than standard path tracing

### Implementation Don't Forget

- **BDPT light path caching per tile** for performance (reuse across pixels)
- **Maintain deterministic tile-based parallelism** with proper random seeding
- **Add `--integrator=path-tracing|bdpt` CLI flag** 
- **Add web interface dropdown** for integrator selection
- **Test identical results** - path tracer extraction must give same output
- **BVH access via `scene.GetBVH()`** instead of internal `rt.bvh`
- **Background gradient via `scene.GetBackgroundColors()`** 

### Validation Requirements

1. **Regression test**: Extracted path tracer produces identical images
2. **Performance test**: No slowdown for path tracing mode
3. **BDPT test**: Caustic glass scene shows visible improvement
4. **Determinism test**: Same seeds produce same results across integrators

## Conclusion

Implementing BDPT represents a significant enhancement to our raytracer's capabilities, particularly for complex lighting scenarios like our caustic glass scene. The ultra-minimal architecture keeps complexity low while enabling pluggable rendering algorithms.

The proposed phased approach allows for incremental development and testing, ensuring each component works correctly before integration. The resulting BDPT implementation will provide superior rendering quality for challenging lighting scenarios while maintaining compatibility with our progressive tile-based architecture.