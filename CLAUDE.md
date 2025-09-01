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

# Run with CPU profiling
./raytracer --scene=default --mode=normal --profile=cpu.prof

# Analyze profile
go tool pprof cpu.prof

# Benchmark script
#   Without args: Compare current changes vs HEAD (stashes changes first)
#   With commit:  Compare current changes vs specified commit (stashes changes first)
./benchmark.sh [baseline_commit]
```

## Architecture Overview

This is a sophisticated progressive raytracer with a clean modular architecture:

### Core Design Philosophy
- **Progressive Rendering**: Multi-pass rendering with immediate visual feedback via tile-based parallel processing
- **Deterministic Parallelism**: Tile-specific random seeds and overrideable sampler ensure identical results
- **Zero External Dependencies**: Uses only Go standard library

### Dependency Hierarchy

The codebase follows a strict hierarchical dependency structure to avoid circular imports and maintain clean separation of concerns:

```
web → renderer → integrator → scene → lights → geometry → material → core
```

### Package Structure
```
pkg/core/          # Foundation types and minimal interfaces 
pkg/material/      # Physically-based materials (lambertian, metal, glass, emissive)
pkg/geometry/      # Shape primitives with BVH acceleration
pkg/lights/        # Physically-based lights (e.g. sphere, quad, infinite)
pkg/scene/         # Scene management and presets
pkg/integrator/    # BDPT and path tracing integrators
pkg/renderer/      # Progressive raytracing engine with worker pools
pkg/loaders/       # File format loaders (PLY mesh support)
web/               # Real-time web interface with Server-Sent Events
```

### Key Architectural Components

**Progressive Renderer (`pkg/renderer/`)**:
- `ProgressiveRaytracer`: Orchestrates multi-pass tile-based rendering
- `WorkerPool`: Manages parallel 64x64 tile processing

**BVH Acceleration**: 
- Optimized median-split algorithm with up to 39x speedup for complex meshes

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
- `cornell` - Classic Cornell box with area lighting and mirror surrfaces
- `spheregrid` - BVH performance testing
- `trianglemesh` - Complex procedural triangle geometry
- `dragon` - High-poly mesh (1.8M triangles, requires separate PLY download)
- `caustic-glass` - Glass with complex geometry for testing caustics and bdpt

## CLI Usage Examples

```bash
# Progressive rendering with path tracing
./raytracer --scene=cornell --max-passes=3 --max-samples=20

# BDPT with caustic-glass scene (excellent for complex lighting)
./raytracer --scene=caustic-glass --integrator=bdpt --max-samples=20

# High quality render
./raytracer --scene=default --max-passes=10 --max-samples=2000 --workers=20

# Web interface (usually running at localhost:8080 via air in background)
cd web && air
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
