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

# Automated performance benchmarking
./benchmark.sh  # Compares performance before/after uncommitted changes
```

## Architecture Overview

This is a sophisticated progressive raytracer with a clean modular architecture:

### Core Design Philosophy
- **Progressive Rendering**: Multi-pass rendering with immediate visual feedback via tile-based parallel processing
- **Direct Struct Access**: Clean separation with concrete types for better performance (no interface overhead)
- **Deterministic Parallelism**: Tile-specific random seeds ensure identical results regardless of worker count
- **Zero External Dependencies**: Uses only Go standard library

### Dependency Hierarchy

The codebase follows a strict hierarchical dependency structure to avoid circular imports and maintain clean separation of concerns:

```
renderer → integrator → scene → {geometry, material, core}
                              ↘ 
web → renderer                  core (foundation types only)
```

**Dependency Rules:**
- `core/`: Foundation types (Vec3, Ray, interfaces) - no dependencies on other packages
- `geometry/`, `material/`: Primitive types and implementations - depend only on `core/`  
- `scene/`: Scene management and presets - depends on `core/`, `geometry/`, `material/`
- `integrator/`: Rendering algorithms - depends on `core/`, `geometry/`, `material/`, `scene/`
- `renderer/`: Progressive rendering engine - depends on all lower levels
- `web/`: Web interface - depends on `renderer/` and lower levels

This hierarchy was established during a major refactor to eliminate interface overhead and enable direct field access (e.g., `scene.Camera`, `scene.Lights`) for better performance.

### Package Structure
```
pkg/core/          # Foundation types and minimal interfaces (Vec3, Ray, Shape, Material)
pkg/geometry/      # Shape primitives with BVH acceleration
pkg/material/      # Physically-based materials (lambertian, metal, glass, emissive)
pkg/renderer/      # Progressive raytracing engine with worker pools
pkg/scene/         # Scene management and presets
pkg/loaders/       # File format loaders (PLY mesh support)
pkg/integrators/   # BDPT and path tracing integrators
web/               # Real-time web interface with Server-Sent Events
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

**BDPT Splat System**: 
- Lock-free splat queue for cross-tile light contributions
- Post-pass deterministic splat processing eliminates race conditions
- Progressive tile animation + final splat update for real-time feedback

## Integrators and Scenes

**Integrators**:
- `path-tracing` - Standard unidirectional path tracing (default, no splats)
- `bdpt` - Bidirectional Path Tracing (produces splats for cross-tile light contributions)

**Available Scenes**:
- `default` - Mixed materials showcase
- `cornell` - Classic Cornell box with area lighting  
- `cornell-boxes` - Cornell box with rotated boxes
- `caustic-glass` - Glass caustic geometry (excellent for BDPT testing)
- `spheregrid` - BVH performance testing
- `trianglemesh` - Complex procedural triangle geometry
- `dragon` - High-poly mesh (1.8M triangles, requires separate PLY download)

## CLI Usage Examples

```bash
# Progressive rendering with path tracing
./raytracer --scene=cornell --max-passes=3 --max-samples=25

# BDPT with caustic-glass scene (excellent for complex lighting)
./raytracer --scene=caustic-glass --integrator=bdpt --max-samples=100

# High quality render
./raytracer --scene=default --max-passes=10 --max-samples=200 --workers=4

# Web interface (usually running at localhost:8080 via air in background)
```

## Real-Time Web Interface

- **Comprehensive options**: Web interface exposes a variety of options to the user to customize the render
- **Render Endpoint**: `/api/render` uses SSE to stream tiles as they complete, as well as debug log output
- **Inspect endpoint**: `/api/inspect` allows clicking the image and getting back information about the objects hit

## Testing

- **Sensitive**: All tests should be sensitive to small errors, particularly in rendering.
- **Verify**: Verify tests by temporarily introducing small errors and confirming the test fails, then revert the error.

## Critical Development Notes

⚠️ **TWO main.go files**: `/main.go` (CLI) vs `/web/main.go` (web server) - check directory before building
**New scenes**: Update `pkg/scene/`, `main.go`, `web/server/server.go`, `web/static/index.html`

## Git Commit Message Format

Use this format for commit messages:
- **Summary line**: Single line describing what was done (imperative mood)
- **Description**: A few sentences with more detail about the changes and why
- Always include the Claude Code attribution footer