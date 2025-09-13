# PBRT v4 Scene Format Specification

This document outlines the subset of PBRT v4 format that we support in our raytracer.

## File Structure

PBRT scene files are plain text with two main sections:

1. **Pre-WorldBegin**: Global rendering options (camera, film, integrator, sampler)
2. **WorldBegin block**: Scene content (geometry, materials, lights)

```
# Comments start with #
LookAt 3 4 1.5  0.5 0.5 0  0 0 1  # Camera position
Camera "perspective" "float fov" 45

Film "rgb" "string filename" "output.png"
     "integer xresolution" 400 "integer yresolution" 400

WorldBegin
    # Scene content goes here
    Material "diffuse" "rgb reflectance" [0.7 0.2 0.2]
    Shape "sphere" "float radius" 1.0
WorldEnd
```

## Syntax Rules

- **Parameters**: `"type name" value` or `"type name" [array values]`
- **Comments**: Lines starting with `#`
- **Blocks**: Use `WorldBegin`/`WorldEnd`, `AttributeBegin`/`AttributeEnd`
- **Strings**: Quoted strings for names and types
- **Arrays**: Square brackets `[val1 val2 val3]`

## Camera Definition

```pbrt
# Set camera position and orientation
LookAt eyex eyey eyez   # Camera position
       atx aty atz      # Look at point  
       upx upy upz      # Up vector

# Camera type and parameters
Camera "perspective" 
    "float fov" 45                    # Field of view in degrees
    "float shutteropen" 0.0           # Shutter open time
    "float shutterclose" 1.0          # Shutter close time
```

## Film/Output Configuration

```pbrt
Film "rgb" 
    "string filename" "output.png"
    "integer xresolution" 800
    "integer yresolution" 600
```

## Materials

### Lambertian (Diffuse)
```pbrt
Material "diffuse" 
    "rgb reflectance" [0.7 0.3 0.1]          # RGB albedo
    "spectrum reflectance" "filename.spd"     # Spectral reflectance
```

### Metal (Conductor)
```pbrt
Material "conductor"
    "rgb eta" [0.2 0.9 1.0]           # Index of refraction (RGB)
    "rgb k" [3.1 2.3 1.8]             # Absorption coefficient  
    "float roughness" 0.01            # Surface roughness
```

### Glass (Dielectric) 
```pbrt
Material "dielectric"
    "float eta" 1.5                   # Index of refraction
    "rgb transmittance" [1 1 1]       # Transmission color
    "float roughness" 0.0             # Surface roughness
```

## Shapes

### Sphere
```pbrt
Shape "sphere"
    "float radius" 1.0                # Sphere radius
    "float zmin" -1.0                 # Clipping Z minimum
    "float zmax" 1.0                  # Clipping Z maximum
```

### Quad (Rectangle)
```pbrt
# Define quad by corner point and two edge vectors
Shape "bilinearPatch"
    "point3 P00" [x1 y1 z1]          # Corner 1
    "point3 P01" [x2 y2 z2]          # Corner 2  
    "point3 P10" [x3 y3 z3]          # Corner 3
    "point3 P11" [x4 y4 z4]          # Corner 4
```

### Triangle Mesh
```pbrt
Shape "trianglemesh"
    "integer indices" [0 1 2  2 3 0]  # Triangle vertex indices
    "point3 P" [                      # Vertex positions
        x1 y1 z1  x2 y2 z2  x3 y3 z3  x4 y4 z4
    ]
    "normal N" [                      # Vertex normals (optional)
        nx1 ny1 nz1  nx2 ny2 nz2  nx3 ny3 nz3  nx4 ny4 nz4
    ]
```

## Lights

### Point Light
```pbrt
LightSource "point"
    "rgb I" [10 10 10]               # Intensity (RGB)
    "point3 from" [0 5 0]            # Position
```

### Distant Light (Directional)
```pbrt
LightSource "distant"
    "rgb L" [3 3 3]                  # Radiance
    "point3 from" [0 0 0]            # Direction from
    "point3 to" [0 0 1]              # Direction to
```

### Area Light
Area lights are created by adding emission to materials:

```pbrt
Material "diffuse" 
    "rgb reflectance" [0 0 0]        # Non-reflective
AreaLightSource "diffuse"
    "rgb L" [15 15 15]               # Emission
Shape "bilinearPatch" 
    # ... quad definition
```

### Infinite Environment Light
```pbrt
LightSource "infinite"
    "rgb L" [0.4 0.45 0.5]           # Background radiance
    "string filename" "envmap.exr"    # Environment map (optional)
```

## Transformations

```pbrt
# Apply before shapes/lights
Translate x y z                      # Translation
Rotate angle x y z                   # Rotation (angle in degrees, axis)
Scale x y z                          # Non-uniform scaling
Scale s                              # Uniform scaling

# Matrix transformation
Transform [m00 m01 m02 m03  m10 m11 m12 m13  m20 m21 m22 m23  m30 m31 m32 m33]
```

## Attribute Blocks

Group transformations and materials:

```pbrt
AttributeBegin
    # Transformations and materials apply only within this block
    Material "diffuse" "rgb reflectance" [0.8 0.2 0.2]
    Translate 0 1 0
    Shape "sphere" "float radius" 0.5
AttributeEnd
```

## Cornell Box Example

```pbrt
# Cornell Box Scene
LookAt 278 278 -800   278 278 0   0 1 0
Camera "perspective" "float fov" 40

Film "rgb" "string filename" "cornell.png"
     "integer xresolution" 400 "integer yresolution" 400

WorldBegin

# White material for most surfaces
Material "diffuse" "rgb reflectance" [0.73 0.73 0.73]

# Floor
Shape "bilinearPatch"
    "point3 P00" [0 0 0]
    "point3 P01" [556 0 0]  
    "point3 P10" [0 0 556]
    "point3 P11" [556 0 556]

# Ceiling with area light
AttributeBegin
    Material "diffuse" "rgb reflectance" [0 0 0]  # Non-reflective
    AreaLightSource "diffuse" "rgb L" [15 15 15]  # Emission
    Shape "bilinearPatch"
        "point3 P00" [213 548.8 227]
        "point3 P01" [343 548.8 227]
        "point3 P10" [213 548.8 332]  
        "point3 P11" [343 548.8 332]
AttributeEnd

WorldEnd
```

## Implementation Notes

For our initial implementation, we'll focus on:

- **Camera**: Perspective only, LookAt positioning
- **Materials**: Diffuse (lambertian), conductor (metal), dielectric (glass)
- **Shapes**: Spheres, bilinear patches (quads), triangle meshes
- **Lights**: Point, distant, area (via emissive materials), infinite
- **Transformations**: Translate, rotate, scale
- **Attributes**: Material and transformation grouping

Advanced features like textures, participating media, and complex sampling can be added later.