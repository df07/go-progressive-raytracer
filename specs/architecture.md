# Progressive Raytracer Architecture Spec

## Overview
A progressive raytracer that renders scenes in multiple passes, improving quality with each iteration and outputting PNG images.

## Package Structure

```
github.com/df07/go-progressive-raytracer/
├── main.go                          # CLI entry point
├── pkg/
│   ├── core/                        # Foundation package - fundamental types
│   │   ├── vec3.go                  # Vec3 type and operations  
│   │   ├── ray.go                   # Ray type and methods
│   │   ├── interfaces.go            # Shape, Material, ScatterResult, HitRecord
│   │   └── sampling.go              # Monte Carlo sampling utilities
│   ├── geometry/                    # Shape implementations
│   │   ├── sphere.go                # Sphere primitive
│   │   └── sphere_test.go           # Sphere tests
│   ├── material/                    # Material implementations
│   │   ├── lambertian.go            # Lambertian (diffuse) material
│   │   ├── metal.go                 # Metallic reflection
│   │   ├── mix.go                   # Mix material (probabilistic blend)
│   │   └── *_test.go                # Material tests
│   ├── scene/                       # Scene management
│   │   ├── scene.go                 # Scene container and configuration
│   │   └── presets.go               # Pre-built scene configurations
│   └── renderer/                    # Core rendering logic
│       ├── raytracer.go             # Main raytracer with Monte Carlo integration
│       ├── camera.go                # Camera with ray generation
│       └── *_test.go                # Renderer tests
```

## Core Components

### 1. Core Package (`pkg/core/`)
- **Responsibility**: Foundation types and interfaces used across the entire raytracer
- **Key Types**: 
  - `Vec3` - 3D vector with math operations
  - `Ray` - Origin + direction 
  - `Shape` - Interface for hittable objects
  - `Material` - Interface for surface scattering
  - `ScatterResult` - Unified material output struct
  - `HitRecord` - Intersection data
- **No Dependencies**: Only imports Go standard library

### 2. Raytracer (`pkg/renderer/raytracer.go`)
- **Responsibility**: Orchestrate rendering process with proper Monte Carlo integration
- **Key Methods**: 
  - `NewRaytracer(scene Scene, width, height int) *Raytracer`
  - `RenderPass() *image.RGBA` - Single rendering pass with multi-sampling
  - `rayColorRecursive(ray Ray, depth int) Vec3` - Recursive ray tracing
  - `calculateDiffuseColor()` - Monte Carlo integration for diffuse materials
  - `calculateSpecularColor()` - Direct calculation for specular materials

### 3. Camera (`pkg/renderer/camera.go`)
- **Responsibility**: Generate rays through image plane, handle viewport
- **Key Methods**:
  - `NewCamera() *Camera` - Default camera setup
  - `GetRay(s, t float64) Ray` - Generate ray for normalized pixel coordinates

### 4. Shape Implementations (`pkg/geometry/`)
- **Responsibility**: Implement shape primitives for ray intersection
- **Interface**: `core.Shape` with `Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool)`
- **Current**: Sphere primitive with comprehensive tests
- **Future**: Plane, Quad, Triangle, BVH acceleration

### 5. Material Implementations (`pkg/material/`)
- **Responsibility**: Handle light scattering with proper PDF calculations
- **Interface**: `core.Material` with `Scatter(ray Ray, hit HitRecord, random *rand.Rand) (ScatterResult, bool)`
- **Types**: 
  - `Lambertian` - Cosine-weighted diffuse scattering
  - `Metal` - Specular reflection with optional fuzziness
  - `Mix` - Probabilistic blend of two materials
- **Key Feature**: Unified `ScatterResult` eliminates type casting

### 6. Scene Configuration (`pkg/scene/`)
- **Responsibility**: Container for all scene objects and configuration
- **Interface**: Implements renderer `Scene` interface
- **Methods**: `GetShapes()`, `GetCamera()`, `GetBackgroundColors()`

## Architecture Principles

### 1. Circular Import Prevention
- **Core package** contains all shared interfaces and types
- **Implementation packages** depend only on core, not each other
- **Clean dependency graph**: geometry → core, material → core, renderer → all

### 2. Mathematical Correctness
- **Proper Monte Carlo integration** with explicit PDF handling
- **Energy conservation** through correct BRDF normalization
- **Physically-based materials** following rendering equation principles

### 3. Unified Material System
- **Single ScatterResult struct** with `Scattered`, `Attenuation`, `PDF` fields
- **Specular detection** via `IsSpecular()` method (`PDF <= 0`)
- **No type casting** required in raytracer main loop

### 4. Testing Strategy
- **Focus on tricky code**: Monte Carlo integration, PDF calculations, sampling
- **Mathematical verification**: Cosine distributions, energy conservation
- **Edge case coverage**: Depth limits, invalid PDFs, boundary conditions

## Data Flow

```
main.go → Scene Setup → Camera → Raytracer → Monte Carlo Integration → PNG Output
```

1. Parse CLI arguments for scene selection
2. Create scene with shapes and materials  
3. Configure camera with desired viewport
4. Initialize raytracer with scene and camera
5. Execute multi-sampled rendering with proper PDF integration
6. Output PNG images with gamma correction

## Key Improvements Achieved

- **Eliminated circular imports** through core package architecture
- **Fixed Monte Carlo integration** with proper PDF usage in material calculations
- **Unified material handling** removing 50% of material-related code complexity
- **Comprehensive test coverage** for mathematical correctness
- **Clean separation of concerns** with single-responsibility packages 