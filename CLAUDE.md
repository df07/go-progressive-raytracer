# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Development Commands

```bash
# Build the CLI raytracer
go build -o raytracer main.go

# Build the web server
cd web && go build -o web-server main.go

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for specific package
go test ./pkg/renderer/

# Run benchmarks
go test -bench=. ./pkg/core/
go test -bench=. ./pkg/geometry/

# Run with CPU profiling
./raytracer --scene=default --mode=normal --profile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

## Architecture Overview

This is a sophisticated progressive raytracer with a clean modular architecture:

### Core Design Philosophy
- **Progressive Rendering**: Multi-pass rendering with immediate visual feedback via tile-based parallel processing
- **Interface-Based Design**: Clean separation between core interfaces (in `pkg/core/`) and implementations
- **Deterministic Parallelism**: Tile-specific random seeds ensure identical results regardless of worker count
- **Zero External Dependencies**: Uses only Go standard library

### Package Structure
```
pkg/core/          # Foundation types and interfaces (Vec3, Ray, Shape, Material)
pkg/geometry/      # Shape primitives with BVH acceleration
pkg/material/      # Physically-based materials (lambertian, metal, glass, emissive)
pkg/renderer/      # Progressive raytracing engine with worker pools
pkg/scene/         # Scene management and presets
pkg/loaders/       # File format loaders (PLY mesh support)
web/              # Real-time web interface with Server-Sent Events
```

### Key Architectural Components

**Progressive Renderer (`pkg/renderer/`)**:
- `ProgressiveRaytracer`: Orchestrates multi-pass tile-based rendering
- `WorkerPool`: Manages parallel 64x64 tile processing
- Sample strategy: 1 sample → linear progression → adaptive final pass
- Thread-safe accumulation with deterministic seeds per tile

**Material System**: Unified `ScatterResult` eliminates type casting:
- All materials return consistent interface with PDF support
- Supports Multiple Importance Sampling (MIS) and Russian Roulette termination

**BVH Acceleration**: 
- Optimized median-split algorithm with up to 39x speedup for complex meshes
- Thread-safe: each worker creates its own BVH instance

## Available Scenes

- `default` - Mixed materials showcase
- `cornell` - Classic Cornell box with area lighting  
- `cornell-boxes` - Cornell box with rotated boxes
- `spheregrid` - BVH performance testing (perfect for benchmarks)
- `trianglemesh` - Complex procedural triangle geometry
- `dragon` - High-poly mesh (1.8M triangles, requires separate PLY download)

## CLI Usage Examples

```bash
# Quick progressive preview
./raytracer --scene=cornell --mode=progressive --max-passes=3 --max-samples=25

# High quality render
./raytracer --scene=default --mode=progressive --max-passes=10 --max-samples=200

# Specify worker count
./raytracer --scene=spheregrid --mode=progressive --workers=4

# Single-pass traditional rendering
./raytracer --scene=cornell --mode=normal --samples=100

# Web interface
cd web && go run main.go -port 8080
```

## Real-Time Tile Streaming

The web interface uses **real-time tile streaming** for immediate visual feedback:

### Architecture
- **SSE Endpoint**: `/api/render` streams individual tiles as they complete (~100+ updates per render)
- **Tile Completion Callbacks**: Workers call `onTileComplete` for each finished 64x64 tile
- **Canvas Compositor**: Client-side tile assembly with batched rendering
- **Thread-Safe Streaming**: Channel-based communication prevents SSE stream corruption

### Benefits
- **Immediate Feedback**: Tiles appear within milliseconds of completion
- **Efficient Bandwidth**: ~2-4KB per tile vs ~500KB for full images
- **Smooth Progress**: 100+ visual updates instead of 7-10 complete image updates

## Testing Strategy

Comprehensive test coverage includes:
- Unit tests for all materials, geometries, and core math functions
- Integration tests for progressive rendering and tile boundary handling
- Performance benchmarks for BVH construction and rendering speeds
- Visual verification tests comparing expected vs actual render outputs

## Output Structure

- CLI renders: `output/<scene_name>/render_<timestamp>.png`
- Progressive passes: `render_<timestamp>_pass_<N>.png` 
- Web renders: Real-time streaming via Server-Sent Events

## Performance Characteristics

- **Progressive Preview**: First pass in ~25-70ms
- **Multi-Core Scaling**: ~2x speedup with 4+ workers
- **BVH Acceleration**: Essential for complex meshes (dragon scene)
- **Memory Efficient**: Tile-based approach with adaptive sampling

## Key Implementation Details

**Deterministic Parallelism**: Each 64x64 tile uses `hash(tileX, tileY, pass)` as random seed, ensuring reproducible results regardless of worker count or timing.

**Progressive Quality**: Linear sample progression (1→2→4→8...) with adaptive final pass using perceptual luminance variance for quality assessment.

**Resource Management**: Proper lifecycle management with graceful shutdown, comprehensive error handling, and memory cleanup throughout the rendering pipeline.