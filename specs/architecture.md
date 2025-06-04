# Progressive Raytracer Architecture Spec

## Overview
A progressive raytracer that renders scenes in multiple passes, improving quality with each iteration and outputting PNG images.

## Package Structure

```
github.com/df07/go-progressive-raytracer/
├── main.go                          # CLI entry point
├── pkg/
│   ├── math/                        # Geometric utilities
│   │   ├── vec3.go                  # Vec3 type and operations
│   │   ├── ray.go                   # Ray type and methods
│   │   └── aabb.go                  # Axis-Aligned Bounding Box
│   ├── geometry/                    # Scene objects (shapes)
│   │   ├── shape.go                 # Shape interface and hit record
│   │   ├── sphere.go                # Sphere primitive
│   │   ├── plane.go                 # Infinite plane
│   │   ├── quad.go                  # Quadrilateral
│   │   └── triangle.go              # Triangle primitive
│   ├── material/                    # Surface materials
│   │   ├── material.go              # Material interface
│   │   ├── lambert.go               # Lambertian (diffuse)
│   │   ├── metal.go                 # Metallic reflection
│   │   └── glass.go                 # Dielectric (glass)
│   ├── acceleration/                # Spatial data structures
│   │   ├── bvh.go                   # Bounding Volume Hierarchy
│   │   └── bvh_node.go              # BVH tree nodes
│   ├── scene/                       # Scene management
│   │   ├── scene.go                 # Scene container and hit testing
│   │   └── presets.go               # Pre-built scene configurations
│   └── renderer/                    # Core rendering logic
│       ├── raytracer.go             # Main raytracer with progressive passes
│       ├── camera.go                # Camera with ray generation
│       └── image.go                 # Image buffer and PNG output
```

## Core Components

### 1. Raytracer (`pkg/renderer/raytracer.go`)
- **Responsibility**: Orchestrate rendering process, manage progressive passes
- **Key Methods**: 
  - `NewRaytracer(scene, camera) *Raytracer`
  - `RenderPass() *Image` - Single rendering pass
  - `RenderProgressive(passes int) <-chan *Image` - Stream progressive passes via channel

### 2. Camera (`pkg/renderer/camera.go`)
- **Responsibility**: Generate rays through image plane, handle viewport
- **Key Methods**:
  - `NewCamera(lookFrom, lookAt, vUp Vec3, vfov float64, aspectRatio float64) *Camera`
  - `GetRay(s, t float64) Ray` - Generate ray for pixel coordinates

### 3. Scene Objects (`pkg/geometry/`)
- **Responsibility**: Implement shape primitives for ray intersection
- **Interface**: `Shape` with `Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool)`
- **Primitives**: Sphere, Plane, Quad, Triangle

### 4. Materials (`pkg/material/`)
- **Responsibility**: Handle light scattering and color calculation
- **Interface**: `Material` with `Scatter(ray Ray, hit HitRecord) (attenuation Vec3, scattered Ray, ok bool)`
- **Types**: Lambert, Metal, Glass

### 5. Geometric Utilities (`pkg/math/`)
- **Vec3**: 3D vector with math operations (add, multiply, dot, cross, normalize)
- **Ray**: Origin + direction with `At(t float64) Vec3` method
- **AABB**: Bounding box for acceleration structures

### 6. BVH Tree (`pkg/acceleration/`)
- **Responsibility**: Spatial partitioning for fast ray-object intersection
- **Key Types**: `BVH` tree and `BVHNode` 
- **Interface**: Implements `Shape` for hierarchical hit testing

### 7. Scene Configuration (`pkg/scene/`)
- **Scene**: Container for all shape objects, implements `Shape`
- **Presets**: Functions to generate common scenes (e.g., `RandomSpheres(n int)`)

## Progressive Rendering Design

The raytracer supports progressive enhancement through streaming passes:

1. **Channel-based streaming**: `RenderProgressive()` returns a channel that yields images
2. **Real-time output**: Each pass is available immediately when completed
3. **Flexible consumption**: CLI writes files, web server streams to clients
4. **Tile-based parallelization**: Each pass can use parallel tile rendering

## Usage Patterns

**CLI Application:**
```go
imageChannel := raytracer.RenderProgressive(10)
passNumber := 1
for image := range imageChannel {
    filename := fmt.Sprintf("render_%02d.png", passNumber)
    image.SavePNG(filename)
    fmt.Printf("Completed pass %d\n", passNumber)
    passNumber++
}
```

**Web Server (future):**
```go
imageChannel := raytracer.RenderProgressive(50)
for image := range imageChannel {
    websocket.WriteJSON(map[string]interface{}{
        "pass": passNumber,
        "image": image.ToBase64(),
    })
}
```

## Data Flow

```
main.go → Scene Setup → Camera → Raytracer → Progressive Passes → PNG Output
```

1. Parse CLI arguments for scene selection
2. Create scene with objects and materials  
3. Configure camera with desired viewport
4. Initialize raytracer with scene and camera
5. Execute progressive rendering passes
6. Output PNG images after each pass 