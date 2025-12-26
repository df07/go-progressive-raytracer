# Integrator System

## Overview

The integrator system implements light transport algorithms that compute pixel colors by tracing paths through the scene. Two integrators are available: **Path Tracing** (unidirectional) and **BDPT** (Bidirectional Path Tracing). Both integrators evaluate materials identically through the Material interface, but differ in how they construct and connect light transport paths. BDPT produces splat rays for cross-tile contributions while path tracing returns only pixel colors.

## Path Tracing Integrator

### Algorithm Overview

Path tracing (`PathTracingIntegrator`) implements **unidirectional path tracing** starting from the camera. For each pixel:

1. **Trace camera ray** into scene
2. **Test intersection** with scene geometry (BVH)
3. **Handle surface hit**:
   - Collect emitted light if surface is emissive
   - Call `Material.Scatter()` to generate random scattered direction
   - For **diffuse materials**: combine direct + indirect lighting with MIS
   - For **specular materials**: recursively trace reflection/refraction
4. **Handle background miss**: collect infinite light emission
5. **Apply Russian Roulette** termination after minimum bounces

### Material Evaluation in Path Tracing

**When materials are evaluated**:

```go
// At each surface intersection (path_tracing.go:66)
scatter, didScatter := hit.Material.Scatter(ray, *hit, sampler)
```

**For diffuse materials** (Lambertian, rough dielectrics):
- `Scatter()` samples texture **once** at UV coordinates: `albedo := l.Albedo.Evaluate(hit.UV, hit.Point)`
- Returns scattered ray direction and attenuation (BRDF / pdf)
- Direct lighting: `CalculateDirectLighting()` samples a light source
  - Casts shadow ray to check visibility
  - Evaluates material BRDF: `hit.Material.EvaluateBRDF()`
  - **EvaluateBRDF samples texture again** at same UV: `albedo := l.Albedo.Evaluate(hit.UV, hit.Point)`
  - Combines with light emission using MIS weight
- Indirect lighting: `CalculateIndirectLighting()` recursively traces scattered ray
  - Uses scattered direction from `Scatter()` result
  - Weights contribution with MIS based on material PDF vs light PDF

**For specular materials** (Metal, Glass):
- `Scatter()` samples texture **once** at UV coordinates: `albedo := m.Albedo.Evaluate(hit.UV, hit.Point)`
- Returns deterministic reflection/refraction direction (PDF=0)
- No direct lighting computation (delta functions can't be directly sampled)
- Recursively traces scattered ray with full attenuation

**Key point**: For diffuse materials, `Scatter()` and `EvaluateBRDF()` both call `texture.Evaluate(hit.UV, hit.Point)` at the **same UV coordinates**, so texture values are **consistent** within a single path vertex.

### Direct and Indirect Lighting

**Direct lighting** (`CalculateDirectLighting`):
- Samples one light from scene using light selection PDF
- Casts shadow ray from surface point to light sample point
- Evaluates `Material.EvaluateBRDF(incoming, lightDirection, hit, Radiance)`
- Applies power heuristic MIS weight: `w = (lightPDF)² / ((lightPDF)² + (materialPDF)²)`
- Contribution: `BRDF × emission × cosθ × MIS_weight / lightPDF`

**Indirect lighting** (`CalculateIndirectLighting`):
- Uses scattered direction from `Material.Scatter()`
- Recursively traces ray to get incoming radiance
- Applies power heuristic MIS weight: `w = (materialPDF)² / ((materialPDF)² + (lightPDF)²)`
- Contribution: `attenuation × cosθ × MIS_weight × incomingLight / materialPDF`

**MIS (Multiple Importance Sampling)**: Balances direct and indirect lighting to reduce variance. When both light sampling and material sampling can reach the same path, MIS weights prevent double-counting while preserving unbiasedness.

### Russian Roulette Termination

After `RussianRouletteMinBounces` (default 3), paths are probabilistically terminated:

```go
survivalProb := min(0.95, max(0.5, throughput.Luminance()))
if randomSample > survivalProb {
    terminate path
}
compensationFactor := 1.0 / survivalProb  // Energy-conserving boost
```

Surviving paths multiply final contribution by compensation factor to remain unbiased.

## BDPT Integrator

### Algorithm Overview

BDPT (`BDPTIntegrator`) implements **bidirectional path tracing** by constructing two subpaths:

1. **Light path**: starts from randomly sampled light, traces through scene
2. **Camera path**: starts from camera, traces through scene
3. **Connection**: evaluates all strategies connecting s light vertices to t camera vertices
4. **MIS weighting**: combines strategies with power heuristic to reduce variance

### Path Construction

**Camera path generation** (`generateCameraPath`):
- Creates camera vertex at ray origin
- Extends path by tracing ray through scene (`extendPath`)
- At each intersection:
  - Records surface interaction, incoming direction, emitted light
  - Calls `Material.Scatter()` to get next direction
  - **Scatter() samples texture at UV**: `albedo := l.Albedo.Evaluate(hit.UV, hit.Point)`
  - Computes forward/reverse area PDFs for MIS
  - Updates path throughput beta
  - Continues until max depth or absorption

**Light path generation** (`generateLightPath`):
- Samples light emission: `light.SampleEmission()`
- Creates light vertex with emission and surface normal
- Extends path from light through scene (`extendPath`)
- Same intersection logic as camera path
- Applies special handling for infinite lights (background)

**Path extension** (`extendPath`):
- Shared logic for both camera and light paths
- At each bounce:
  - Intersect scene geometry
  - Create vertex with `SurfaceInteraction`
  - Call `Material.Scatter()` to sample next direction
  - **Material samples texture once per vertex**: `albedo.Evaluate(hit.UV, hit.Point)`
  - For diffuse: `beta *= attenuation × cosθ / PDF`
  - For specular: `beta *= attenuation` (no PDF division)
  - Compute PDFs for MIS weight calculation
  - Continue tracing

### Material Evaluation in BDPT

**During path construction**:
- Each vertex calls `Material.Scatter()` exactly once
- **Texture sampled once per vertex**: `albedo := material.Albedo.Evaluate(hit.UV, hit.Point)`
- Attenuation stored in `ScatterResult` and used to update path throughput
- Forward/reverse PDFs computed for MIS

**During connection evaluation**:
- `evaluateBRDF()` evaluates material at vertex for connection direction
- **For Lambertian**: `EvaluateBRDF()` samples texture **again**: `albedo := l.Albedo.Evaluate(hit.UV, hit.Point)`
- **For Metal**: `EvaluateBRDF()` samples texture **again**: `albedo := m.Albedo.Evaluate(hit.UV, hit.Point)`
- Uses same UV coordinates from `SurfaceInteraction` stored in vertex
- Transport mode (`Radiance` vs `Importance`) respects light direction for non-symmetric BRDFs

**Critical consistency requirement**: `Scatter()` and `EvaluateBRDF()` must sample the **same texture value** at the same UV coordinates. Since both use `hit.UV` from the same `SurfaceInteraction`, texture values are **deterministic and consistent** for a given vertex.

### Connection Strategies

BDPT evaluates multiple strategies for each pixel, connecting s light vertices to t camera vertices:

**s=0, t≥1** (Path Tracing): Pure camera path, no light path vertices
- Evaluates camera path's accumulated radiance at last vertex
- Contribution: `lastVertex.EmittedLight × lastVertex.Beta`
- This is standard unidirectional path tracing

**s=1, t≥1** (Direct Lighting): Sample light directly from camera path endpoint
- Calls `lights.SampleLight()` to sample light position
- Evaluates `Material.EvaluateBRDF()` at camera vertex for light direction
- **EvaluateBRDF samples texture**: `albedo := material.Albedo.Evaluate(hit.UV, hit.Point)`
- Casts shadow ray for visibility
- Contribution: `cameraBeta × BRDF × lightBeta × cosθ`

**s≥2, t=1** (Light Tracing): Connect light path to camera
- Samples camera from light path vertex: `camera.SampleCameraFromPoint()`
- Evaluates BRDF with `Importance` transport mode (reverse direction)
- **EvaluateBRDF samples texture**: `albedo := material.Albedo.Evaluate(hit.UV, hit.Point)`
- Casts shadow ray for visibility
- Returns `SplatRay` for cross-tile contribution (see Splat System below)

**s≥2, t≥2** (Connection): Connect light vertex to camera vertex
- Computes connection direction between vertices
- Evaluates BRDF at both vertices:
  - Camera vertex: `evaluateBRDF(cameraVertex, direction, Radiance)`
  - Light vertex: `evaluateBRDF(lightVertex, -direction, Importance)`
  - **Both calls sample textures** at respective UV coordinates
- Geometric term: `G = cosθ_camera × cosθ_light / distance²`
- Contribution: `lightBeta × lightBRDF × cameraBRDF × cameraBeta × G`

**Specular vertices**: Connections skip vertices with `IsSpecular=true` (delta functions cannot be connected).

### MIS Weighting

BDPT uses **power heuristic** MIS to combine multiple strategies:

```
weight_i = p_i² / (p_1² + p_2² + ... + p_n²)
```

For each strategy (s,t), MIS weight is computed by:

1. **Calculate forward/reverse PDFs** for all path vertices
2. **Evaluate alternative strategies**: simulate generating same path with different s,t values
3. **Compute probability ratios** between strategies using `ri *= reversePDF / forwardPDF`
4. **Power heuristic**: `weight = 1 / (1 + sum(ri))`

Implementation details (`calculateMISWeight`):
- Walks camera path backward from connection vertex
- Walks light path backward from connection vertex
- For each alternative strategy, computes PDF ratio
- Skips strategies involving specular vertices (non-connectable)
- Uses `remap0()` to handle delta functions (maps 0→1 to avoid division by zero)

Special cases:
- **Infinite lights**: Use directional PDF for background emission
- **Light vertices**: Use `PDF_Le` for proper spatial/directional density
- **Camera vertices**: Use camera's directional PDF

### Splat System

Light tracing strategies (s≥2, t=1) produce **splat rays** that contribute to pixels other than the one being rendered:

**Why splats are needed**: When a light path connects to the camera, the connection may originate from a different tile than the pixel being sampled. The contribution must be "splatted" to the correct pixel location.

**Splat ray structure**:
```go
type SplatRay struct {
    Ray   core.Ray  // Ray from camera defining pixel location
    Color core.Vec3 // Light contribution (already MIS-weighted)
}
```

**Splat processing** (handled by renderer):
- BDPT returns `[]SplatRay` along with pixel color
- Renderer collects splats in lock-free queue
- After all tiles complete a pass, splats are applied deterministically
- Each splat ray is traced to determine pixel coordinates
- Splat color added to corresponding pixel

**Path tracing splats**: Path tracing always returns `nil` for splats (only unidirectional camera paths).

## Comparison: Path Tracing vs BDPT

### Texture Sampling

**Path Tracing**:
- `Scatter()` samples texture once per intersection
- `EvaluateBRDF()` samples texture again for direct lighting BRDF evaluation
- Both sample **same UV coordinates** from `SurfaceInteraction`
- Texture value is **consistent** for a given vertex

**BDPT**:
- `Scatter()` samples texture once during path construction
- `EvaluateBRDF()` samples texture again during connection evaluation
- Both sample **same UV coordinates** from `SurfaceInteraction` stored in vertex
- Texture value is **consistent** for a given vertex
- Multiple connections to same vertex reuse same texture sample

**Conclusion**: Both integrators sample textures **identically** through the Material interface. Any texture inconsistencies between integrators indicate a bug in material implementation or vertex data handling, not a fundamental algorithmic difference.

### When Materials Are Evaluated

**Path Tracing**:
- `Scatter()`: at each ray-surface intersection
- `EvaluateBRDF()`: during direct lighting calculation (diffuse only)

**BDPT**:
- `Scatter()`: during camera/light path construction (once per vertex)
- `EvaluateBRDF()`: during connection evaluation (potentially multiple times per vertex if multiple strategies connect through it)

### Performance Characteristics

**Path Tracing**:
- Faster per sample (simpler algorithm)
- Better for simple lighting (few indirect bounces)
- Struggles with caustics and difficult light paths (SDS paths)

**BDPT**:
- Slower per sample (multiple strategies per pixel)
- Better for complex lighting (caustics, indirect illumination)
- Produces splats requiring post-processing
- More robust for difficult transport paths

### Debugging Material Evaluation

To verify consistent texture sampling:

1. **Add logging to ColorSource.Evaluate()**:
   ```go
   func (t *ImageTexture) Evaluate(uv core.Vec2, point core.Vec3) core.Vec3 {
       color := // ... sample texture ...
       fmt.Printf("Texture eval: UV=(%.3f,%.3f) -> RGB=(%.3f,%.3f,%.3f)\n",
                  uv.X, uv.Y, color.X, color.Y, color.Z)
       return color
   }
   ```

2. **Verify UV consistency**: Same UV should produce same color regardless of integrator

3. **Check SurfaceInteraction**: Ensure geometry UV generation is deterministic

4. **Test with solid colors**: Replace texture with `SolidColor` to isolate texture sampling bugs

## Code Location

**Integrators**:
- `/pkg/integrator/path_tracing.go` - Path tracing implementation
- `/pkg/integrator/bdpt.go` - BDPT path construction and connection
- `/pkg/integrator/bdpt_mis.go` - MIS weight calculation
- `/pkg/integrator/interfaces.go` - Common interfaces (if exists)

**Materials**:
- `/pkg/material/interfaces.go` - Material, ColorSource interfaces
- `/pkg/material/lambertian.go` - Diffuse material
- `/pkg/material/metal.go` - Specular metal
- `/pkg/material/dielectric.go` - Glass/refraction
- `/pkg/material/color_source.go` - Texture interface

**Related Systems**:
- `/pkg/lights/` - Light sampling and PDF calculation
- `/pkg/geometry/` - UV coordinate generation
- `/pkg/renderer/` - Splat processing

## Usage Examples

### Selecting Integrator

```go
// Path tracing (default)
integrator := integrator.NewPathTracingIntegrator(samplingConfig)

// BDPT
integrator := integrator.NewBDPTIntegrator(samplingConfig)
```

### CLI Usage

```bash
# Path tracing
./raytracer --scene=cornell --integrator=path-tracing

# BDPT (better for caustics)
./raytracer --scene=caustic-glass --integrator=bdpt
```

### Configuring Sampling

```go
samplingConfig := scene.SamplingConfig{
    MaxDepth:                 10,   // Maximum ray bounces
    RussianRouletteMinBounces: 3,   // Start RR after 3 bounces
}
```

## Access Log
2025-12-26T16:34:49Z +1 Critical info on texture sampling consistency between PT and BDPT for bug investigation
