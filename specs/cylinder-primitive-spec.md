# Cylinder Primitive Specification

## Overview

This document specifies the implementation of a finite cylinder primitive for the progressive raytracer. The cylinder is defined by two circular cross-sections of equal radius (unlike the cone, which has different radii).

## Parameterization

The cylinder is defined by its two end caps and constant radius:

```go
type Cylinder struct {
    BaseCenter   core.Vec3          // Center of the base circle
    TopCenter    core.Vec3          // Center of the top circle
    Radius       float64            // Constant radius along entire length
    Material     material.Material  // Surface material

    // Cached derived values (computed in constructor)
    axis         core.Vec3          // Unit vector from base to top
    height       float64            // Distance between base and top
}
```

### Derived Values

From the base parameters, we derive:
- **axis** = normalize(TopCenter - BaseCenter)
- **height** = |TopCenter - BaseCenter|

### Validation

Constructor should validate:
- Radius > 0
- BaseCenter ≠ TopCenter (height > 0)

## Mathematical Formulation

### Cylinder Surface Equation

For an infinite cylinder with axis along direction **V̂** passing through point **C**, and radius **r**, a point **P** lies on the surface when:

```
|(P - C) - ((P - C) · V̂)V̂| = r
```

This states that the distance from **P** to the axis equals the radius.

### Ray-Cylinder Intersection Algorithm

Given ray `P(t) = O + tD` and cylinder parameters:

#### Method 1: Direct Quadratic (Simpler for Arbitrary Orientation)

Let:
- **Δ** = O - BaseCenter (ray origin relative to base)
- **V̂** = axis (unit vector)

The perpendicular distance from a point on the ray to the cylinder axis must equal the radius:

```
|(Δ + tD) - ((Δ + tD) · V̂)V̂| = r
```

Expanding the magnitude squared:
```
|(Δ + tD) - ((Δ + tD) · V̂)V̂|² = r²
```

Let:
- **DV** = D · V̂
- **ΔV** = Δ · V̂

The perpendicular component of (Δ + tD) from the axis is:
```
perpComponent = (Δ + tD) - (ΔV + t·DV)V̂
```

Expanding the magnitude:
```
|perpComponent|² = |Δ + tD|² - (ΔV + t·DV)²
                 = (Δ·Δ + 2t(Δ·D) + t²(D·D)) - (ΔV² + 2t·ΔV·DV + t²·DV²)
```

Setting this equal to r² and rearranging into standard form `at² + bt + c = 0`:

```
a = D·D - DV²
b = 2(Δ·D - ΔV·DV)
c = Δ·Δ - ΔV² - r²
```

Simplified alternative notation:
```
a = |D|² - (D·V̂)²
b = 2[Δ·D - (Δ·V̂)(D·V̂)]
c = |Δ|² - (Δ·V̂)² - r²
```

#### Method 2: Using Cross Product (Alternative)

The distance from a point to a line can be expressed using cross products:
```
distance = |(O - BaseCenter) × axis + t(D × axis)| / |axis|
```

Since axis is a unit vector, |axis| = 1, and this simplifies to:
```
|(O - BaseCenter) × axis + t(D × axis)| = r
```

This also yields the same quadratic equation with:
```
a = |D × axis|²
b = 2(D × axis) · ((O - BaseCenter) × axis)
c = |(O - BaseCenter) × axis|² - r²
```

Both methods are equivalent; use Method 1 for implementation as it avoids the cross product computation.

#### Step 1: Set Up and Solve Quadratic

Compute coefficients a, b, c using Method 1 formulas above.

Compute discriminant: `Δ = b² - 4ac`

- If Δ < 0: No intersection, return nil/false
- If Δ ≥ 0:
  ```
  t₁ = (-b - √Δ) / (2a)
  t₂ = (-b + √Δ) / (2a)
  ```

#### Step 2: Validate Solutions

For each solution t (try t₁ first, then t₂):

1. **Check ray parameter bounds**: `tMin ≤ t ≤ tMax`

2. **Compute intersection point**: `P = O + t·D`

3. **Check height bounds**:
   ```
   h = (P - BaseCenter) · axis
   if h < 0 or h > height: reject
   ```

4. **First valid intersection**: Use this t value and create SurfaceInteraction

## Surface Normal Calculation

For intersection point **P** on the cylinder surface, the normal points radially outward from the axis:

```
// Height along cylinder axis
h = (P - BaseCenter) · axis

// Point on axis at same height as intersection
axisPoint = BaseCenter + h * axis

// Normal is radial direction from axis to surface point
normal = normalize(P - axisPoint)
```

Use `SurfaceInteraction.SetFaceNormal(ray, normal)` to automatically handle front/back face orientation.

## Bounding Box Calculation

The AABB must contain both circular cross-sections:

```go
// Create extent vectors for radius
extent := core.Vec3{Radius, Radius, Radius}

// Base circle bounds
baseMin := BaseCenter.Subtract(extent)
baseMax := BaseCenter.Add(extent)

// Top circle bounds
topMin := TopCenter.Subtract(extent)
topMax := TopCenter.Add(extent)

// Union of both AABBs
return NewAABB(
    core.NewVec3(
        math.Min(baseMin.X, topMin.X),
        math.Min(baseMin.Y, topMin.Y),
        math.Min(baseMin.Z, topMin.Z),
    ),
    core.NewVec3(
        math.Max(baseMax.X, topMax.X),
        math.Max(baseMax.Y, topMax.Y),
        math.Max(baseMax.Z, topMax.Z),
    ),
)
```

Note: This is conservative (larger than necessary for non-axis-aligned cylinders) but correct and simple. The BVH will still provide good acceleration despite the loose bounds.

## Edge Cases and Special Handling

### 1. Ray Parallel to Axis

When `D · V̂ ≈ ±1`, the ray is nearly parallel to the cylinder axis. In this case:
- The coefficient `a = D·D - (D·V̂)² ≈ 0`
- Handle carefully to avoid division by zero
- If `|a| < ε` and `|b| < ε`: ray is on or parallel to axis
- If `|a| < ε` but `b ≠ 0`: degenerate linear equation `bt + c = 0`

### 2. Ray Passes Through Axis

When the ray passes exactly through the cylinder axis:
- May produce two valid intersections on opposite sides
- Normal calculation is well-defined at all points except exactly on axis
- No special handling needed beyond standard intersection

### 3. Grazing/Tangent Intersections

When discriminant ≈ 0:
- Ray grazes the cylinder surface tangentially
- t₁ ≈ t₂
- Single intersection point
- Handle with small epsilon tolerance

### 4. Numerical Precision

- Use epsilon (e.g., 1e-8) for floating-point comparisons
- Height bounds: `if h < -ε or h > height + ε: reject`
- Discriminant: `if Δ < -ε: no intersection`
- Parallel ray check: `if |a| < ε: handle specially`

### 5. End Cap Intersections

This spec covers the **cylinder body only**, not the circular end caps at base and top. The open cylinder is a curved surface without caps.

If caps are needed in the future:
- Add a boolean parameter for capped vs. open cylinders
- Check ray-plane intersection with base and top discs
- Verify intersection point is within radius
- Choose closest valid intersection among body and caps

## Implementation Checklist

- [ ] Define `Cylinder` struct with base/top centers and radius
- [ ] Constructor that validates parameters and caches derived values
- [ ] Implement `Hit()` method:
  - [ ] Set up and solve quadratic equation using perpendicular distance formula
  - [ ] Validate both solutions against ray and height bounds
  - [ ] Return first valid intersection with proper SurfaceInteraction
  - [ ] Handle parallel ray edge case
- [ ] Implement `BoundingBox()` method
- [ ] Calculate surface normals correctly using SetFaceNormal
- [ ] Handle edge cases (parallel rays, numerical precision, grazing intersections)
- [ ] Unit tests:
  - [ ] Ray hits cylinder body from outside
  - [ ] Ray hits cylinder body from inside
  - [ ] Ray misses cylinder
  - [ ] Ray parallel to cylinder axis (misses)
  - [ ] Ray along cylinder axis (misses)
  - [ ] Ray tangent to cylinder
  - [ ] Ray at various angles to axis
  - [ ] Height bounds rejection (would hit infinite cylinder but not finite)
  - [ ] Normal calculation verification (perpendicular to axis)
  - [ ] Bounding box correctness

## Testing Strategy

### Unit Tests

1. **Basic Intersection Tests**
   - Ray perpendicular to axis hitting cylinder side
   - Ray parallel to axis (should miss unless very specific case)
   - Ray from inside cylinder
   - Ray from outside cylinder at various angles

2. **Axis Orientation Tests**
   - Axis-aligned cylinder (along X, Y, or Z)
   - Arbitrary orientation cylinder
   - Verify math works for all orientations

3. **Height Bounds Tests**
   - Rays that would hit infinite cylinder but miss finite cylinder
   - Intersections exactly at height boundaries
   - Rays entering and exiting within bounds

4. **Normal Tests**
   - Verify normals point radially outward
   - Verify normals are perpendicular to axis (normal · axis = 0)
   - Check front face vs back face handling

5. **Edge Cases**
   - Ray exactly parallel to axis
   - Ray grazing cylinder (tangent)
   - Very small or very large radii
   - Very short or very long cylinders

6. **Bounding Box Tests**
   - Verify AABB contains entire cylinder
   - Test with various orientations
   - Verify BVH traversal correctly culls/includes cylinder

### Integration Tests

- Render scenes with cylinders using path tracing integrator
- Compare cylinder images against reference if available
- Visual inspection of shading (normals) and shadows
- Test with multiple cylinders in complex arrangements

### Performance Considerations

- Profile intersection code with high cylinder count scenes
- Verify BVH acceleration works effectively
- Compare performance to sphere (should be similar)
- Benchmark parallel ray edge case handling

## Comparison to Cone

The cylinder is mathematically simpler than the cone because:
- Constant radius eliminates the tan(α) terms
- No "shadow cone" ambiguity (cone extends in one direction only)
- Normal calculation is purely radial (no axial component)
- No degenerate apex case to handle

However, both share:
- Similar parameterization (base/top centers)
- Height bounds validation
- Quadratic equation solving
- Conservative AABB strategy

## References

- "Cylinders" - PBRT Book 3rd Edition (https://pbr-book.org/3ed-2018/Shapes/Cylinders)
- Mathematics Stack Exchange: "Calculating ray-cylinder intersection points"
- "Ray tracing primitives" - Cambridge University Computer Graphics course
- Existing sphere.go and quad.go implementations in this codebase
