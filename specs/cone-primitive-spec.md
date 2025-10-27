# Cone Primitive Specification

## Overview

This document specifies the implementation of a cone/frustum primitive for the progressive raytracer. The implementation supports both pointed cones (apex at top) and truncated cones/frustums (flat top).

## Parameterization

The cone is defined by two circular cross-sections:

```go
type Cone struct {
    BaseCenter   core.Vec3          // Center of the base circle
    BaseRadius   float64            // Radius at the base
    TopCenter    core.Vec3          // Center of the top circle
    TopRadius    float64            // Radius at top (0 for pointed, >0 for truncum)
    Material     material.Material  // Surface material

    // Cached derived values (computed in constructor)
    axis         core.Vec3          // Unit vector from base to top
    height       float64            // Distance between base and top
    tanAngle     float64            // tan(cone angle) for intersection math
}
```

### Derived Values

From the base parameters, we derive:
- **axis** = normalize(TopCenter - BaseCenter)
- **height** = |TopCenter - BaseCenter|
- **tanAngle** = (BaseRadius - TopRadius) / height

### Special Cases

- **Pointed cone**: TopRadius = 0
- **Frustum**: TopRadius > 0
- **Cylinder**: TopRadius = BaseRadius (degenerate cone, may want special handling)

## Mathematical Formulation

### Cone Surface Equation

For a cone with apex at origin, axis along +Z, and half-angle α, the implicit surface equation is:
```
x² + y² = (z·tan(α))²
```

For our generalized cone/frustum, we transform the problem:
1. Work in cone-local space where apex is at origin and axis is along +Z
2. The apex of the infinite cone extended from our frustum is at distance `d` from base:
   ```
   d = BaseRadius / tan(α) = BaseRadius * height / (BaseRadius - TopRadius)
   ```

### Ray-Cone Intersection Algorithm

Given ray `P(t) = O + tD` and cone parameters:

#### Step 1: Transform to Cone Space

Define apex position in world space:
```
apex = BaseCenter - axis * d
where d = BaseRadius * height / (BaseRadius - TopRadius)
```

Special case: If BaseRadius == TopRadius (cylinder), handle separately.

#### Step 2: Set Up Quadratic Equation

Let:
- CO = O - apex (ray origin relative to apex)
- k = tan²(α) = (BaseRadius - TopRadius)² / height²

The ray-cone intersection yields quadratic `at² + bt + c = 0`:

```
DdotV = D · axis
COdotV = CO · axis

a = D·D - DdotV² - k·DdotV²
b = 2(D·CO - DdotV·COdotV - k·DdotV·COdotV)
c = CO·CO - COdotV² - k·COdotV²
```

Simplified (factoring):
```
a = D·D - (1 + k)·DdotV²
b = 2[D·CO - (1 + k)·DdotV·COdotV]
c = CO·CO - (1 + k)·COdotV²
```

#### Step 3: Solve Quadratic

Compute discriminant: `Δ = b² - 4ac`

- If Δ < 0: No intersection, return nil/false
- If Δ ≥ 0:
  ```
  t₁ = (-b - √Δ) / (2a)
  t₂ = (-b + √Δ) / (2a)
  ```

#### Step 4: Validate Solutions

For each solution t (try t₁ first, then t₂):

1. **Check ray parameter bounds**: `tMin ≤ t ≤ tMax`

2. **Compute intersection point**: `P = O + t·D`

3. **Check height bounds**:
   ```
   h = (P - BaseCenter) · axis
   if h < 0 or h > height: reject
   ```

4. **Verify correct cone side** (not shadow cone):
   ```
   r = BaseRadius - (h / height) * (BaseRadius - TopRadius)
   distFromAxis = |(P - BaseCenter) - h·axis|
   if distFromAxis > r + ε: reject (ε is small tolerance)
   ```

   Alternatively: Check that we're on the correct cone nappe:
   ```
   if (P - apex) · axis < 0: reject (for pointed cone with θ < 90°)
   ```

5. **First valid intersection**: Use this t value

## Surface Normal Calculation

For intersection point P on the cone surface:

```
// Vector from apex to intersection point
apexToP = P - apex

// Project onto axis to get axial component
axialComponent = (apexToP · axis) * axis

// Perpendicular component points radially outward
radialComponent = apexToP - axialComponent

// Normal is perpendicular to surface (mix of radial and axial)
// For a cone, the normal makes angle (90° - α) with the radial direction
normal = normalize(radialComponent - tan(α) * axialComponent)
```

Alternatively, using the parameterization directly:
```
// Height along cone
h = (P - BaseCenter) · axis

// Center point on axis at this height
centerPoint = BaseCenter + h * axis

// Radial vector from axis to point
radial = P - centerPoint

// Normal calculation (perpendicular to cone surface)
normal = normalize(radial + (BaseRadius - TopRadius) / height * axis)
```

Use `SurfaceInteraction.SetFaceNormal(ray, normal)` to automatically handle front/back face orientation.

## Bounding Box Calculation

The AABB must contain both circular cross-sections:

```
// Find the radius extent at base
baseExtent = core.Vec3{BaseRadius, BaseRadius, BaseRadius}
baseMin = BaseCenter - baseExtent
baseMax = BaseCenter + baseExtent

// Find the radius extent at top
topExtent = core.Vec3{TopRadius, TopRadius, TopRadius}
topMin = TopCenter - topExtent
topMax = TopCenter + topExtent

// Union of both AABBs
return NewAABB(
    core.NewVec3(
        min(baseMin.X, topMin.X),
        min(baseMin.Y, topMin.Y),
        min(baseMin.Z, topMin.Z),
    ),
    core.NewVec3(
        max(baseMax.X, topMax.X),
        max(baseMax.Y, topMax.Y),
        max(baseMax.Z, topMax.Z),
    ),
)
```

Note: This is conservative (larger than necessary) but correct. A tighter bound would consider the actual cone geometry, but this simpler approach is sufficient for BVH construction.

## Edge Cases and Special Handling

### 1. Cylinder (BaseRadius == TopRadius)

When `BaseRadius ≈ TopRadius` (within tolerance), the cone degenerates to a cylinder. Options:
- Implement cylinder-specific intersection logic
- OR reject during construction and suggest using a dedicated Cylinder primitive
- OR handle with special-case math (tanAngle → ∞)

### 2. Nearly Parallel Rays

When `a ≈ 0` in the quadratic equation, the ray is nearly parallel to the cone surface. Handle carefully:
- If `|a| < ε`, check if it's a linear equation: `bt + c = 0`
- May produce tangent intersections or miss entirely

### 3. Numerical Precision

- Use small epsilon (e.g., 1e-8) for floating-point comparisons
- Height bounds checking should have small tolerance: `if h < -ε or h > height + ε`
- Discriminant checking: `if Δ < -ε` to handle near-tangent cases

### 4. Pointed Cone Apex

When TopRadius = 0, the apex is singular (zero normal). In practice:
- Intersections extremely close to apex may have numerical issues
- Consider rejecting hits within small epsilon of apex
- Or handle apex normal specially (average of surrounding normals)

### 5. Shadow Cone

The quadratic equation naturally includes the "shadow cone" extending opposite to the apex direction. This is filtered by:
- Height bounds check (rejects points outside [0, height])
- Axial direction check (rejects points on wrong side of apex)

## Implementation Checklist

- [ ] Define `Cone` struct with base/top centers and radii
- [ ] Constructor that validates parameters and caches derived values
- [ ] Implement `Hit()` method:
  - [ ] Set up and solve quadratic equation
  - [ ] Validate both solutions against bounds
  - [ ] Return first valid intersection with proper SurfaceInteraction
- [ ] Implement `BoundingBox()` method
- [ ] Calculate surface normals correctly using SetFaceNormal
- [ ] Handle edge cases (parallel rays, apex singularity, numerical precision)
- [ ] Unit tests:
  - [ ] Ray hits cone body
  - [ ] Ray misses cone
  - [ ] Ray parallel to cone axis
  - [ ] Ray grazing/tangent to cone
  - [ ] Truncated cone (TopRadius > 0)
  - [ ] Pointed cone (TopRadius = 0)
  - [ ] Normal calculation verification
  - [ ] Bounding box correctness

## Testing Strategy

### Unit Tests

1. **Basic Intersection Tests**
   - Ray perpendicular to axis hitting cone side
   - Ray parallel to axis (should miss or graze)
   - Ray from inside cone
   - Ray from outside cone at various angles

2. **Pointed Cone Tests**
   - TopRadius = 0
   - Rays intersecting near apex
   - Verify shadow cone rejection

3. **Truncated Cone Tests**
   - TopRadius > 0
   - Rays hitting upper portion
   - Verify both intersections work correctly

4. **Height Bounds Tests**
   - Rays that would hit infinite cone but miss truncated version
   - Intersection points exactly at height boundaries

5. **Normal Tests**
   - Verify normals point outward
   - Check normal angle relative to surface
   - Front face vs back face handling

6. **Bounding Box Tests**
   - Verify AABB contains entire cone
   - Test with various orientations

### Integration Tests

- Render scenes with cones using path tracing integrator
- Compare against reference images if available
- Visual inspection of shading (normals) and shadows

### Performance Considerations

- Profile intersection code with high cone count scenes
- Verify BVH acceleration works effectively
- Compare performance to similar primitives (cylinder, sphere)

## References

- "Intersection of a ray and a cone" - https://lousodrome.net/blog/light/2017/01/03/intersection-of-a-ray-and-a-cone/
- "Ray tracing primitives" - Cambridge University Computer Graphics course
- Stack Overflow discussions on cone normal calculation
- Existing sphere.go and quad.go implementations in this codebase
