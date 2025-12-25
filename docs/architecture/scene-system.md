# Scene System

## Overview

Scenes are defined programmatically in Go source files (one file per scene preset in `pkg/scene/`). Each scene configures camera, geometry, materials, lights, and rendering parameters. Scenes are registered in a central map for CLI and web interface access.

## Scene Structure

**File**: `pkg/scene/scene.go`

```go
type Scene struct {
    Camera         *geometry.Camera       // View configuration
    Shapes         []geometry.Shape       // All renderable geometry
    Lights         []lights.Light         // Light sources
    LightSampler   lights.LightSampler    // Light sampling strategy
    SamplingConfig SamplingConfig         // Render quality settings
    CameraConfig   geometry.CameraConfig  // Camera parameters
    BVH            *geometry.BVH          // Top-level acceleration (built in Preprocess)
}
```

### SamplingConfig

```go
type SamplingConfig struct {
    SamplesPerPixel           int     // Rays per pixel per pass
    MaxDepth                  int     // Maximum ray bounce depth
    RussianRouletteMinBounces int     // Minimum bounces before path termination
    AdaptiveMinSamples        float64 // Minimum sample fraction for adaptive sampling
    AdaptiveThreshold         float64 // Error threshold for adaptive sampling
}
```

## Scene Definition Pattern

Each scene is a Go function returning `*Scene`:

```go
// pkg/scene/example_scene.go
func NewExampleScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // 1. Configure camera
    cameraConfig := geometry.CameraConfig{
        Center:      core.NewVec3(0, 1, 5),
        LookAt:      core.NewVec3(0, 0, 0),
        Up:          core.NewVec3(0, 1, 0),
        Width:       800,
        AspectRatio: 16.0 / 9.0,
        VFov:        45.0,
        Aperture:    0.1,
        FocusDistance: 5.0,
    }

    // Apply overrides if provided
    if len(cameraOverrides) > 0 {
        cameraConfig = geometry.MergeCameraConfig(cameraConfig, cameraOverrides[0])
    }

    camera := geometry.NewCamera(cameraConfig)

    // 2. Configure rendering quality
    samplingConfig := SamplingConfig{
        SamplesPerPixel:           200,
        MaxDepth:                  50,
        RussianRouletteMinBounces: 20,
        AdaptiveMinSamples:        0.15,
        AdaptiveThreshold:         0.01,
    }

    // 3. Create scene
    scene := &Scene{
        Camera:         camera,
        Shapes:         make([]geometry.Shape, 0),
        Lights:         make([]lights.Light, 0),
        SamplingConfig: samplingConfig,
        CameraConfig:   cameraConfig,
    }

    // 4. Create materials
    redDiffuse := material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1))
    mirror := material.NewMetal(core.NewVec3(0.9, 0.9, 0.9), 0.0)
    glass := material.NewDielectric(1.5)

    // 5. Add geometry
    scene.Shapes = append(scene.Shapes,
        geometry.NewSphere(core.NewVec3(-2, 1, 0), 1.0, redDiffuse),
        geometry.NewSphere(core.NewVec3(0, 1, 0), 1.0, glass),
        geometry.NewSphere(core.NewVec3(2, 1, 0), 1.0, mirror),
    )

    // 6. Add lights
    scene.AddSphereLight(
        core.NewVec3(0, 10, 0),
        2.0,
        core.NewVec3(10, 10, 10),
    )

    return scene
}
```

## Scene Registration

**File**: `pkg/scene/scene_discovery.go`

Scenes registered in global map:

```go
var AvailableScenes = map[string]func(...geometry.CameraConfig) *Scene{
    "default":       NewDefaultScene,
    "cornell":       NewCornellScene,
    "spheregrid":    NewSphereGridScene,
    "trianglemesh":  NewTriangleMeshScene,
    "dragon":        NewDragonScene,
    "caustic-glass": NewCausticGlassScene,
}
```

## Adding New Scenes

To add a new scene to the system:

1. **Create scene file**: `pkg/scene/my_scene.go` with `NewMyScene()` function
2. **Register scene**: Add to `AvailableScenes` map in `pkg/scene/scene_discovery.go`
3. **Update CLI**: Add scene name to `--scene` flag documentation in `main.go`
4. **Update web server**: Add to scene list in `web/server/server.go`
5. **Update web UI**: Add to dropdown in `web/static/index.html`

## External Asset Loading

### Loading Mesh Files

**Pattern**: Load PLY file, create TriangleMesh, add to scene

```go
func NewMeshScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // Load mesh from file
    plyData, err := loaders.LoadPLY("assets/dragon.ply")
    if err != nil {
        log.Printf("Failed to load mesh: %v", err)
        return NewDefaultScene()  // Fallback
    }

    // Create material
    mat := material.NewLambertian(core.NewVec3(0.8, 0.3, 0.1))

    // Create mesh with optional normals
    var options *geometry.TriangleMeshOptions
    if len(plyData.Normals) > 0 {
        options = &geometry.TriangleMeshOptions{
            Normals: plyData.Normals,
        }
    }

    mesh := geometry.NewTriangleMesh(
        plyData.Vertices,
        plyData.Faces,
        mat,
        options,
    )

    scene.Shapes = append(scene.Shapes, mesh)
    // ... rest of scene setup
    return scene
}
```

### Asset Path Resolution

**Current Behavior**: Paths are relative to execution directory

**Example**: `loaders.LoadPLY("assets/dragon.ply")` expects file at `./assets/dragon.ply`

**Error Handling**: Provide fallback scene if asset loading fails

## Texture Loading

### Loading Image Textures

**Pattern**: Load image, create texture, create textured material, use with geometry

```go
func NewTexturedScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // Load texture image
    imageData, err := loaders.LoadImage("assets/brick.png")
    if err != nil {
        log.Printf("Failed to load texture: %v", err)
        // Fallback to solid color
        imageData = &loaders.ImageData{
            Width:  1,
            Height: 1,
            Pixels: []core.Vec3{core.NewVec3(0.7, 0.5, 0.3)},
        }
    }

    // Create texture
    texture := material.NewImageTexture(
        imageData.Width,
        imageData.Height,
        imageData.Pixels,
    )

    // Create textured material
    mat := material.NewTexturedLambertian(texture)

    // Use with geometry
    quad := geometry.NewQuad(corner, u, v, mat)
    scene.Shapes = append(scene.Shapes, quad)

    return scene
}
```

**Error Handling Pattern**: Fallback to 1×1 solid color texture on load failure

**Asset Paths**: Relative to execution directory (typically project root)

### Procedural Textures

**Pattern**: Create procedural texture directly without file loading

```go
// Checkerboard pattern
checkerboard := material.NewCheckerboardTexture(
    256, 256, 32,
    core.NewVec3(0.9, 0.9, 0.9),  // White
    core.NewVec3(0.2, 0.2, 0.8),  // Blue
)
mat := material.NewTexturedLambertian(checkerboard)

// Gradient
gradient := material.NewGradientTexture(
    256, 256,
    core.NewVec3(1.0, 0.2, 0.2),  // Red (top)
    core.NewVec3(0.2, 1.0, 0.2),  // Green (bottom)
)

// UV debug visualization
uvDebug := material.NewUVDebugTexture(256, 256)
```

**When to Use**:
- Procedural: Fast, no file dependencies, small memory footprint
- Image: Photorealistic detail, artist-authored content

### Combining Textures with Meshes

**Pattern**: Load mesh and texture, connect UVs via TriangleMeshOptions

```go
// Load mesh with UVs
plyData, err := loaders.LoadPLY("assets/model.ply")
if err != nil {
    log.Printf("Failed to load mesh: %v", err)
    return NewDefaultScene()  // Fallback
}

// Load texture
imageData, err := loaders.LoadImage("assets/model_texture.png")
if err != nil {
    log.Printf("Failed to load texture: %v", err)
    // Can still render mesh with solid color
    mat := material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7))
} else {
    texture := material.NewImageTexture(
        imageData.Width,
        imageData.Height,
        imageData.Pixels,
    )
    mat = material.NewTexturedLambertian(texture)
}

// Create mesh with UVs
options := &geometry.TriangleMeshOptions{
    Normals:   plyData.Normals,
    VertexUVs: plyData.TexCoords,  // Connect PLY UVs to mesh
}

mesh := geometry.NewTriangleMesh(
    plyData.Vertices,
    plyData.Faces,
    mat,
    options,
)

scene.Shapes = append(scene.Shapes, mesh)
```

**Requirements**:
- PLY file must contain texture coordinates (u/v, s/t, or texture_u/texture_v)
- VertexUVs must match vertex count
- UVs are interpolated using barycentric coordinates per triangle

## Light Setup

### Sphere Lights

```go
scene.AddSphereLight(
    position core.Vec3,
    radius float64,
    emission core.Vec3,
)
```

Creates sphere geometry with emissive material and registers as light source.

### Quad Lights

```go
scene.AddQuadLight(
    corner, u, v core.Vec3,
    emission core.Vec3,
)
```

Creates quad geometry with emissive material and registers as light source.

### Infinite Lights

**Gradient Background**:
```go
scene.AddGradientInfiniteLight(
    skyColor core.Vec3,
    horizonColor core.Vec3,
)
```

**Solid Background**:
```go
scene.AddSolidInfiniteLight(color core.Vec3)
```

### Light Sampler

After adding lights, create light sampler for importance sampling:

```go
scene.LightSampler = lights.NewUniformLightSampler(scene.Lights)
```

## Scene Preprocessing

**Method**: `scene.Preprocess()`

Called before rendering to:
1. Build top-level BVH from scene.Shapes
2. Initialize any deferred computations
3. Validate scene configuration

**Usage**:
```go
scene := NewMyScene()
scene.Preprocess()
// Scene ready for rendering
```

## Camera Configuration

### CameraConfig Fields

```go
type CameraConfig struct {
    Center        core.Vec3  // Camera position
    LookAt        core.Vec3  // Point camera looks at
    Up            core.Vec3  // Up direction (usually (0,1,0))
    Width         int        // Image width in pixels
    AspectRatio   float64    // Width/height ratio
    VFov          float64    // Vertical field of view in degrees
    Aperture      float64    // Lens aperture (0 = pinhole)
    FocusDistance float64    // Focus distance (matters when Aperture > 0)
}
```

### Camera Overrides

Scenes accept optional camera config override:

```go
// Use defaults
scene := NewCornellScene()

// Override camera position and FOV
override := geometry.CameraConfig{
    Center: core.NewVec3(0, 5, 10),
    VFov:   60.0,
}
scene := NewCornellScene(override)
```

## Example Scenes

### Default Scene (`default_scene.go`)

Mixed materials showcase: diffuse, metal, glass spheres with gradient background.

**Purpose**: Quick test scene, material variety

### Cornell Box (`cornell.go`)

Classic Cornell box with red/green walls, white floor/ceiling, mirror spheres, and quad area light.

**Purpose**: Area light testing, indirect illumination, BDPT validation

### Sphere Grid (`spheregrid.go`)

Grid of spheres for BVH performance testing.

**Purpose**: Acceleration structure benchmarking

### Triangle Mesh (`trianglemesh.go`)

Procedurally generated triangle geometry.

**Purpose**: Triangle intersection testing

### Dragon (`dragon.go`)

High-poly mesh loaded from PLY file (1.8M triangles).

**Purpose**: BVH performance demonstration, complex mesh rendering

**Note**: Requires separate `assets/dragon.ply` file download

### Caustic Glass (`caustic_glass.go`)

Glass objects with complex geometry for testing caustics and BDPT.

**Purpose**: Challenging light transport, BDPT testing

### Texture Test (`texture_test_scene.go`)

Demonstrates texture mapping on 7 different geometry types.

**Purpose**: UV coordinate system validation, texture system showcase

**Features**:
- Sphere with checkerboard
- Cylinder with gradient
- Cone with UV debug
- Box with brick pattern
- Disc with checkerboard
- Quad with gradient
- Triangle with UV debug

## Programmatic Scene Benefits

**Type Safety**: Compiler catches errors in scene construction

**Code Reuse**: Share materials, common geometry setups

**Flexibility**: Full Go language for procedural generation, loops, conditionals

**Performance**: No parsing overhead - scenes compiled into binary

**Debugging**: Can set breakpoints in scene construction

## Design Constraints

**No Scene File Format**: Scenes are Go source files (except experimental PBRT loader support)

**Recompile to Modify**: Changing scenes requires rebuilding binary (or using web interface parameters)

**Asset Dependencies**: External meshes loaded at runtime, but scene logic compiled in

**Discovery**: Scenes must be registered in `AvailableScenes` map to be accessible
