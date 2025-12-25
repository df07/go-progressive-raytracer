# Documentation Requests for Texture Mapping Design

This file contains requests for documentation needed to design and implement texture mapping for the raytracer.

## Context
I'm designing a texture mapping system that will allow materials to have spatially-varying properties (colors, roughness, etc.) based on surface coordinates. To write a comprehensive spec without reading the code directly, I need documentation on several key areas.

---

## Request 1: Core Types and Interfaces

**What I need to know:**
- What are the fundamental types in `pkg/core/`? (Vec3, Ray, Color, etc.)
- How are colors represented and stored?
- What intersection/hit information is available when a ray hits a surface?
- Are there existing coordinate types (e.g., UV coordinates, 2D points)?

**Why I need this:**
Texture mapping requires understanding what data is available at intersection points and how to represent texture coordinates and sampled colors. I need to know if UV coordinates are already being computed, and what the standard color/vector types are.

**What the documentation should cover:**
- List of core types with their fields and purposes
- The hit/intersection record structure - what information is available when a ray hits geometry
- Color representation (is it Vec3? separate type?)
- Any existing 2D coordinate or UV types

---

## Request 2: Material System Architecture

**What I need to know:**
- How do materials currently work? What's the interface/contract?
- What methods do materials implement?
- How are material properties (color, reflectance, etc.) currently specified?
- How do materials compute scattered rays and attenuation?
- Can materials already vary their properties, or are they currently uniform?

**Why I need this:**
Texture mapping will integrate into the material system. I need to understand the current material interface so I can design how textured materials will fit in - whether textures replace colors in existing materials, or if we need new material types.

**What the documentation should cover:**
- The material interface (what methods materials must implement)
- How materials are currently initialized (constructor patterns, configuration)
- The render pipeline interaction - when/how materials are queried
- Examples of how existing materials (lambertian, metal, glass) differ in their implementation

---

## Request 3: Geometry and UV Coordinates

**What I need to know:**
- What geometry primitives exist? (spheres, quads, triangles, meshes?)
- Do any geometry primitives currently compute UV coordinates?
- What information does geometry provide at intersection points beyond position and normal?
- How are triangles/meshes structured? Do they have per-vertex data?

**Why I need this:**
Different geometry types require different UV parameterizations. Spheres use spherical coordinates, quads use planar coordinates, triangles use barycentric interpolation. I need to know what's already implemented and what needs to be added.

**What the documentation should cover:**
- List of available geometry primitives
- The intersection result structure from geometry
- Whether UV coordinates are already computed (and if so, how)
- For meshes: what per-vertex data is available (positions, normals, UVs?)
- How the BVH system affects intersection results

---

## Request 4: Scene and Material Assignment

**What I need to know:**
- How are materials assigned to geometry in a scene?
- Is there a scene file format, or are scenes defined in code?
- How would texture file paths be specified in scene definitions?
- Are there any existing resource loading patterns (e.g., for PLY meshes)?

**Why I need this:**
Texture mapping requires loading image files and associating them with materials. I need to understand how resources are currently managed and how users specify scene configurations.

**What the documentation should cover:**
- How scenes are defined (code vs files)
- The pattern for loading external resources (refer to PLY loader if relevant)
- How materials are created and assigned to geometry
- Any existing configuration or asset management patterns

---

## Next Steps

Once I have documentation for these four areas, I should be able to:
1. Design the texture coordinate system
2. Define the texture sampling interface
3. Specify how textures integrate with materials
4. Outline the image loading requirements
5. Define UV coordinate generation for each geometry type

After the initial documentation is provided, I'll likely have follow-up questions about specific implementation details.
