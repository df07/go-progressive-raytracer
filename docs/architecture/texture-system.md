# Texture System

## Overview

The texture system provides spatially-varying material properties through the ColorSource interface. Supports image textures (PNG/JPEG) and procedural textures with zero external dependencies (Go standard library only). Textures are sampled using UV coordinates computed by geometry primitives.

## ColorSource Interface

**File**: `pkg/material/color_source.go`

```go
type ColorSource interface {
    Evaluate(uv core.Vec2, point core.Vec3) core.Vec3
}
```

**Purpose**: Abstraction for color computation from surface coordinates

**Parameters**:
- `uv` - Texture coordinates in [0,1] range (used by image textures)
- `point` - 3D world position (used by procedural textures)

**Usage in Materials**: Materials call `ColorSource.Evaluate(hit.UV, hit.Point)` to get surface color at intersection point

## SolidColor Implementation

**File**: `pkg/material/color_source.go`

```go
type SolidColor struct {
    Color core.Vec3
}
```

**Constructor**: `material.NewSolidColor(color Vec3)`

**Behavior**: Returns constant color regardless of UV or position

**Purpose**: Backward compatibility - all solid-color materials use SolidColor internally

**Example**:
```go
solid := material.NewSolidColor(core.NewVec3(0.7, 0.3, 0.1))
color := solid.Evaluate(anyUV, anyPoint)  // Always returns (0.7, 0.3, 0.1)
```

## ImageTexture Implementation

**File**: `pkg/material/image_texture.go`

```go
type ImageTexture struct {
    Width  int
    Height int
    Pixels []core.Vec3  // Row-major: Pixels[y*Width + x]
}
```

**Constructor**: `material.NewImageTexture(width, height int, pixels []core.Vec3)`

**Sampling Algorithm**: Nearest-neighbor filtering
- Converts UV [0,1] to pixel coordinates
- V-flip: V=0 is bottom, V=1 is top (compensates for image coordinate systems)
- UV wrapping: Repeat mode for UVs outside [0,1]
- Pixel access: `pixels[y*width + x]`

**Example**:
```go
imageData, _ := loaders.LoadImage("brick.png")
texture := material.NewImageTexture(
    imageData.Width,
    imageData.Height,
    imageData.Pixels,
)
mat := material.NewTexturedLambertian(texture)
```

## Image Loading

**File**: `pkg/loaders/image.go`

**Function**: `LoadImage(filename string) (*ImageData, error)`

**Supported Formats**:
- PNG (via `image/png`)
- JPEG (via `image/jpeg`)
- Auto-detects format from file header

**ImageData Structure**:
```go
type ImageData struct {
    Width  int
    Height int
    Pixels []core.Vec3
}
```

**Color Conversion**: RGBA [0, 65535] → Vec3 [0, 1]
```go
pixels[y*width+x] = core.NewVec3(
    float64(r)/65535.0,
    float64(g)/65535.0,
    float64(b)/65535.0,
)
```

**Error Handling**: Returns error for missing/invalid files

**Example**:
```go
imageData, err := loaders.LoadImage("assets/earth.png")
if err != nil {
    log.Printf("Failed to load texture: %v", err)
    // Fallback to solid color or default scene
}
```

## Procedural Textures

**File**: `pkg/material/procedural_textures.go`

All procedural textures are implemented as ImageTexture instances with generated pixel data.

### Checkerboard Pattern

**Function**: `NewCheckerboardTexture(width, height, checkSize int, color1, color2 Vec3)`

**Behavior**:
- Alternating rectangular pattern
- `checkSize` controls size of each square in pixels
- Checkerboard formula: `(checkX + checkY) % 2 == 0`

**Example**:
```go
checkerboard := material.NewCheckerboardTexture(
    256, 256, 32,
    core.NewVec3(0.9, 0.9, 0.9),  // White
    core.NewVec3(0.2, 0.2, 0.8),  // Blue
)
mat := material.NewTexturedLambertian(checkerboard)
```

### Gradient Texture

**Function**: `NewGradientTexture(width, height int, color1, color2 Vec3)`

**Behavior**:
- Vertical color interpolation
- `color1` at top (y=0)
- `color2` at bottom (y=height-1)
- Linear interpolation: `color = color1*(1-t) + color2*t`

**Example**:
```go
gradient := material.NewGradientTexture(
    256, 256,
    core.NewVec3(1.0, 0.2, 0.2),  // Red (top)
    core.NewVec3(0.2, 1.0, 0.2),  // Green (bottom)
)
```

### UV Debug Texture

**Function**: `NewUVDebugTexture(width, height int)`

**Behavior**:
- Visualizes UV coordinates as colors
- U coordinate → Red channel
- V coordinate → Green channel
- Blue channel = 0

**Purpose**: Debugging UV mapping on geometry primitives

**Example**:
```go
uvDebug := material.NewUVDebugTexture(256, 256)
debugMat := material.NewTexturedLambertian(uvDebug)
sphere := geometry.NewSphere(center, radius, debugMat)
// Sphere will show red gradient horizontally, green vertically
```

## Custom Procedural Textures

To create custom procedural textures, generate pixel array and wrap in ImageTexture:

```go
// Create custom pattern
width, height := 512, 512
pixels := make([]core.Vec3, width*height)

for y := 0; y < height; y++ {
    for x := 0; x < width; x++ {
        // Custom color computation
        u := float64(x) / float64(width-1)
        v := float64(y) / float64(height-1)

        // Example: Radial gradient
        dx := u - 0.5
        dy := v - 0.5
        dist := math.Sqrt(dx*dx + dy*dy)

        pixels[y*width+x] = core.NewVec3(dist, dist, dist)
    }
}

texture := material.NewImageTexture(width, height, pixels)
```

## Usage Patterns

### Loading and Using Image Textures

```go
// Load image file
imageData, err := loaders.LoadImage("assets/brick.png")
if err != nil {
    log.Printf("Failed to load texture: %v", err)
    return  // Or use fallback
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
```

### Mixing Textured and Solid Materials

```go
// Textured floor
floorTexture := material.NewCheckerboardTexture(512, 512, 64, white, black)
floorMat := material.NewTexturedLambertian(floorTexture)
floor := geometry.NewQuad(corner, u, v, floorMat)

// Solid color sphere
sphereMat := material.NewLambertian(core.NewVec3(0.8, 0.2, 0.2))
sphere := geometry.NewSphere(center, radius, sphereMat)

scene.Shapes = append(scene.Shapes, floor, sphere)
```

### Textured Mesh from PLY File

```go
// Load mesh with UVs
plyData, err := loaders.LoadPLY("assets/model.ply")
if err != nil {
    return err
}

// Load texture
imageData, err := loaders.LoadImage("assets/model_texture.png")
if err != nil {
    return err
}

// Create textured material
texture := material.NewImageTexture(imageData.Width, imageData.Height, imageData.Pixels)
mat := material.NewTexturedLambertian(texture)

// Create mesh with UVs
options := &geometry.TriangleMeshOptions{
    Normals:   plyData.Normals,
    VertexUVs: plyData.TexCoords,  // Connect PLY UVs
}

mesh := geometry.NewTriangleMesh(
    plyData.Vertices,
    plyData.Faces,
    mat,
    options,
)
```

## Performance Characteristics

**Memory Usage**:
- Each pixel: 24 bytes (3 × float64)
- 512×512 texture: ~6 MB
- 1024×1024 texture: ~24 MB
- No mipmaps or compressed formats

**Runtime Performance**:
- Texture sampling: O(1) nearest-neighbor lookup
- No caching (evaluates on every material query)
- Negligible overhead vs solid colors

**Limitations**:
- No bilinear/trilinear filtering
- No mipmapping (may alias on distant surfaces)
- No texture compression
- Entire texture loaded into memory

## Integration with Rendering Pipeline

**Data Flow**:
```
Geometry.Hit() → SurfaceInteraction.UV
                 ↓
Material.Scatter() → ColorSource.Evaluate(UV, Point)
                     ↓
ImageTexture → Pixel lookup → Vec3 color
                     ↓
BRDF evaluation → Ray attenuation
```

**Key Points**:
- Geometry primitives compute UV coordinates
- Materials receive UV via SurfaceInteraction
- ColorSource.Evaluate() called for every ray-surface interaction
- Texture sampling happens during path tracing (not pre-baked)

## Example Scene

See `pkg/scene/texture_test_scene.go` for comprehensive demonstration:
- 7 geometry types with different textures
- Checkerboard, gradient, and UV debug textures
- Shows texture mapping on sphere, cylinder, cone, box, disc, quad, triangle
2025-12-26T15:59:31Z +1 Accessed to understand texture system for debugging texture inconsistency bug

## Access Log
2025-12-26T16:17:00Z +1 Accessed to understand material BRDF and texture evaluation for PT vs BDPT bug investigation
2025-12-26T16:27:53Z +1 Accessed to understand texture evaluation in ColorSource interface for PT vs BDPT bug
