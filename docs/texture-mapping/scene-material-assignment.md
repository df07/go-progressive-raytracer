# Scene and Material Assignment

## Overview
Scenes are defined programmatically in Go source files (one file per scene in `pkg/scene/`). Materials are created with constructors and assigned directly to geometry. External resources are loaded via the PLY loader pattern. No scene file format exists except for experimental PBRT support.

## Scene Definition Pattern

### Scene Structure (`pkg/scene/scene.go`)

```go
type Scene struct {
    Camera         *geometry.Camera       // View configuration
    Shapes         []geometry.Shape       // All renderable objects
    Lights         []lights.Light         // Light sources
    LightSampler   lights.LightSampler    // Light sampling strategy
    SamplingConfig SamplingConfig         // Render settings
    CameraConfig   geometry.CameraConfig  // Camera parameters
    BVH            *geometry.BVH          // Acceleration structure (built in Preprocess)
}
```

### Scene Creation Function

Each scene is a Go function returning `*Scene`:

```go
// pkg/scene/my_scene.go
func NewMyScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // 1. Configure camera
    cameraConfig := geometry.CameraConfig{
        Center:      core.NewVec3(0, 1, 5),
        LookAt:      core.NewVec3(0, 0, 0),
        Up:          core.NewVec3(0, 1, 0),
        Width:       800,
        AspectRatio: 16.0 / 9.0,
        VFov:        45.0,
        Aperture:    0.1,
    }
    camera := geometry.NewCamera(cameraConfig)

    // 2. Configure sampling
    samplingConfig := SamplingConfig{
        SamplesPerPixel:           200,
        MaxDepth:                  50,
        RussianRouletteMinBounces: 20,
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
    blueMetal := material.NewMetal(core.NewVec3(0.3, 0.5, 0.8), 0.1)
    glass := material.NewDielectric(1.5)

    // 5. Create and add geometry
    sphere1 := geometry.NewSphere(core.NewVec3(-2, 1, 0), 1.0, redDiffuse)
    sphere2 := geometry.NewSphere(core.NewVec3(0, 1, 0), 1.0, glass)
    sphere3 := geometry.NewSphere(core.NewVec3(2, 1, 0), 1.0, blueMetal)

    scene.Shapes = append(scene.Shapes, sphere1, sphere2, sphere3)

    // 6. Add lights
    scene.AddSphereLight(
        core.NewVec3(0, 10, 0),     // position
        2.0,                         // radius
        core.NewVec3(10, 10, 10),   // emission
    )

    // 7. Add background light
    scene.AddGradientInfiniteLight(
        core.NewVec3(0.5, 0.7, 1.0),  // sky color
        core.NewVec3(1.0, 1.0, 1.0),  // horizon color
    )

    return scene
}
```

## Material Assignment Patterns

### Direct Assignment (Most Common)

Create material, create geometry with material:

```go
mat := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
sphere := geometry.NewSphere(center, radius, mat)
```

### Material Reuse

One material shared by multiple objects:

```go
goldMaterial := material.NewMetal(core.NewVec3(0.8, 0.6, 0.2), 0.1)

sphere1 := geometry.NewSphere(pos1, r1, goldMaterial)
sphere2 := geometry.NewSphere(pos2, r2, goldMaterial)
sphere3 := geometry.NewSphere(pos3, r3, goldMaterial)
```

### Per-Triangle Materials in Meshes

TriangleMesh supports different material per triangle:

```go
materials := make([]material.Material, numTriangles)
for i := 0; i < numTriangles; i++ {
    materials[i] = selectMaterialForTriangle(i)
}

options := &geometry.TriangleMeshOptions{
    Materials: materials,
}

mesh := geometry.NewTriangleMesh(vertices, faces, defaultMat, options)
```

### Layered/Composite Materials

Complex materials built from combinations:

```go
// Clear coat over diffuse base
base := material.NewLambertian(core.NewVec3(0.7, 0.1, 0.1))
coating := material.NewDielectric(1.5)
clearCoated := material.NewLayered(coating, base)

sphere := geometry.NewSphere(center, radius, clearCoated)
```

## Scene Discovery and Registration

### Scene Registry (`pkg/scene/scene_discovery.go`)

Scenes are registered in a map:

```go
var AvailableScenes = map[string]func(...geometry.CameraConfig) *Scene{
    "default":       NewDefaultScene,
    "cornell":       NewCornellScene,
    "spheregrid":    NewSphereGridScene,
    "dragon":        NewDragonScene,
    "caustic-glass": NewCausticGlassScene,
    // Add new scenes here
}
```

### Adding a New Scene

1. Create `pkg/scene/my_scene.go` with `NewMyScene()` function
2. Add to `AvailableScenes` map in `scene_discovery.go`
3. Update CLI scene flag in `main.go`
4. Update web server scene options in `web/server/server.go`
5. Update web UI dropdown in `web/static/index.html`

## External Resource Loading

### PLY Mesh Loading Pattern

**Loader**: `pkg/loaders/ply.go` - `LoadPLY(filename string) (*PLYData, error)`

**Usage Example** (`pkg/scene/dragon.go`):

```go
func NewDragonScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // Load PLY file
    plyData, err := loaders.LoadPLY("assets/dragon.ply")
    if err != nil {
        log.Printf("Failed to load dragon.ply: %v", err)
        return fallbackScene()
    }

    // Create material
    dragonMaterial := material.NewLambertian(core.NewVec3(0.8, 0.3, 0.1))

    // Optional: per-triangle normals from PLY data
    var options *geometry.TriangleMeshOptions
    if len(plyData.Normals) > 0 {
        options = &geometry.TriangleMeshOptions{
            Normals: plyData.Normals,
        }
    }

    // Create triangle mesh
    mesh := geometry.NewTriangleMesh(
        plyData.Vertices,
        plyData.Faces,
        dragonMaterial,
        options,
    )

    scene.Shapes = append(scene.Shapes, mesh)
    // ... rest of scene setup
}
```

### PLY Data Structure

```go
type PLYData struct {
    Vertices   []core.Vec3  // Vertex positions
    Faces      []int        // Triangle indices (3 per triangle)
    Normals    []core.Vec3  // Per-vertex normals (if present in file)
    Colors     []core.Vec3  // Per-vertex colors (if present, normalized to [0,1])
    TexCoords  []core.Vec2  // Per-vertex UVs (if present) ← LOADED BUT NOT USED
    // Additional properties...
}
```

### Asset Path Resolution

**Current Pattern**: Relative paths from execution directory.

**Example**:
```go
// Assumes `assets/dragon.ply` exists relative to where binary is run
plyData, err := loaders.LoadPLY("assets/dragon.ply")
```

**Error Handling**: Scenes provide fallback if file not found:
```go
if err != nil {
    log.Printf("Failed to load mesh: %v", err)
    return NewDefaultScene()  // Fallback to simple scene
}
```

## Configuration and Overrides

### Camera Overrides

Scene functions accept optional camera config override:

```go
func NewMyScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    defaultConfig := geometry.CameraConfig{/* defaults */}

    if len(cameraOverrides) > 0 {
        config = geometry.MergeCameraConfig(defaultConfig, cameraOverrides[0])
    }
    // ...
}
```

**Usage**:
```go
// Use scene defaults
scene := NewMyScene()

// Override camera position
override := geometry.CameraConfig{
    Center: core.NewVec3(0, 5, 10),
}
scene := NewMyScene(override)
```

### Sampling Configuration

Each scene specifies render quality settings:

```go
samplingConfig := SamplingConfig{
    SamplesPerPixel:           200,   // Rays per pixel
    MaxDepth:                  50,    // Max bounce depth
    RussianRouletteMinBounces: 20,    // Min bounces before termination
    AdaptiveMinSamples:        0.15,  // 15% min samples for adaptive
    AdaptiveThreshold:         0.01,  // 1% error threshold
}
```

These can be overridden from CLI or web interface.

## Texture Integration Strategy

### Current State
- No image loading infrastructure
- No texture file specification in scenes
- Materials use solid colors only

### Proposed Pattern for Texture Loading

**Option 1: Similar to PLY Loading**

Create `pkg/loaders/image.go`:
```go
package loaders

type ImageData struct {
    Width  int
    Height int
    Pixels []core.Vec3  // RGB pixels
}

func LoadImage(filename string) (*ImageData, error) {
    // Load PNG/JPG/etc and convert to Vec3 array
}
```

**Usage in Scene**:
```go
func NewTexturedScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    // Load texture image
    imageData, err := loaders.LoadImage("assets/brick_albedo.png")
    if err != nil {
        log.Printf("Failed to load texture: %v", err)
        // Fallback to solid color
    }

    // Create texture sampler
    texture := NewImageTexture(imageData)

    // Create textured material
    mat := material.NewTexturedLambertian(texture)

    // Create geometry with textured material
    quad := geometry.NewQuad(corner, u, v, mat)

    scene.Shapes = append(scene.Shapes, quad)
    // ...
}
```

### Asset Management Considerations

**Texture Paths**: Like PLY files, use relative paths from execution directory.

**Error Handling**: Provide solid color fallback if texture fails to load.

**Memory Management**: Large textures could consume significant memory - consider:
- Lazy loading
- Mipmapping for different detail levels
- Texture compression

**File Formats**: Use Go's `image` package for standard formats:
- `image/png` - PNG support
- `image/jpeg` - JPEG support
- Built into Go standard library (no external dependencies)

## Web Interface Integration

### Scene Selection (`web/server/server.go`)

Web server provides scene list to frontend:

```go
// In server initialization
sceneNames := make([]string, 0, len(scene.AvailableScenes))
for name := range scene.AvailableScenes {
    sceneNames = append(sceneNames, name)
}
```

### Adding Textures to Web Interface

Would need to:
1. Upload texture files to server (or serve from assets directory)
2. Add texture selection UI in web interface
3. Pass texture paths through render API
4. Server loads textures and builds scene with textured materials

**Alternative**: Predefined textured scenes (like current scene presets).

## PBRT Scene Loader

### Experimental Support (`pkg/loaders/pbrt.go`)

Basic PBRT file format parser exists but not fully integrated:

```go
func LoadPBRTScene(filename string) (*Scene, error)
```

**Status**: Experimental, not production-ready.

**Capabilities**: Parse PBRT scene files (geometry, materials, lights).

**Limitations**: Subset of PBRT features supported.

**Texture Support**: PBRT format includes texture specifications - could serve as reference for texture system design.

## Key Takeaways for Texture System

1. **Scene Creation**: Add texture loading to scene constructors (like PLY loading)
2. **Asset Paths**: Use relative paths from execution directory
3. **Error Handling**: Fallback to solid colors if texture fails
4. **Material Assignment**: Works the same - just use textured material types
5. **Web Integration**: Extend scene presets or add texture upload feature
6. **No Breaking Changes**: Textured materials should coexist with solid color materials

## Example: Complete Textured Scene

Hypothetical implementation:

```go
func NewTexturedRoomScene(cameraOverrides ...geometry.CameraConfig) *Scene {
    scene := createBaseScene(cameraOverrides...)

    // Load textures
    brickAlbedo, _ := loaders.LoadImage("assets/brick_albedo.png")
    woodAlbedo, _ := loaders.LoadImage("assets/wood_albedo.png")

    // Create textured materials
    brickMat := material.NewTexturedLambertian(NewImageTexture(brickAlbedo))
    woodMat := material.NewTexturedLambertian(NewImageTexture(woodAlbedo))

    // Create geometry with UVs
    // (Quad already computes UVs from barycentric coords)
    wall := geometry.NewQuad(
        core.NewVec3(-5, 0, -5),
        core.NewVec3(10, 0, 0),
        core.NewVec3(0, 10, 0),
        brickMat,
    )

    floor := geometry.NewQuad(
        core.NewVec3(-5, 0, -5),
        core.NewVec3(10, 0, 0),
        core.NewVec3(0, 0, 10),
        woodMat,
    )

    scene.Shapes = append(scene.Shapes, wall, floor)
    return scene
}
```

This follows existing patterns while adding texture support.
