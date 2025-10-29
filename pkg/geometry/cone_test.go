package geometry

import (
	"math"
	"testing"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

func TestNewCone(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	baseCenter := core.NewVec3(0, 0, 0)
	topCenter := core.NewVec3(0, 2, 0)
	baseRadius := 1.0
	topRadius := 0.0 // Pointed cone

	cone, err := NewCone(baseCenter, baseRadius, topCenter, topRadius, false, mat)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	if cone == nil {
		t.Fatal("NewCone returned nil")
	}

	// Check that axis is normalized and points in the right direction
	expectedAxis := core.NewVec3(0, 1, 0)
	if !cone.axis.Equals(expectedAxis) {
		t.Errorf("Expected axis %v, got %v", expectedAxis, cone.axis)
	}

	// Check height
	expectedHeight := 2.0
	if math.Abs(cone.height-expectedHeight) > 1e-9 {
		t.Errorf("Expected height %f, got %f", expectedHeight, cone.height)
	}

	// Check tanAngle
	expectedTanAngle := 1.0 / 2.0 // (1.0 - 0.0) / 2.0
	if math.Abs(cone.tanAngle-expectedTanAngle) > 1e-9 {
		t.Errorf("Expected tanAngle %f, got %f", expectedTanAngle, cone.tanAngle)
	}

	// Check apex position (for pointed cone, apex = topCenter)
	expectedApex := core.NewVec3(0, 2, 0)
	if !cone.apex.Equals(expectedApex) {
		t.Errorf("Expected apex %v, got %v", expectedApex, cone.apex)
	}
}

func TestNewCone_Frustum(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	baseCenter := core.NewVec3(0, 0, 0)
	topCenter := core.NewVec3(0, 4, 0)
	baseRadius := 2.0
	topRadius := 1.0

	cone, err := NewCone(baseCenter, baseRadius, topCenter, topRadius, false, mat)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Check tanAngle
	expectedTanAngle := (2.0 - 1.0) / 4.0 // 0.25
	if math.Abs(cone.tanAngle-expectedTanAngle) > 1e-9 {
		t.Errorf("Expected tanAngle %f, got %f", expectedTanAngle, cone.tanAngle)
	}

	// Check apex position
	// For frustum, apex is beyond the top
	// dFromTop = topRadius * height / (baseRadius - topRadius) = 1.0 * 4.0 / (2.0 - 1.0) = 4.0
	// apex = topCenter + axis * dFromTop = (0,4,0) + (0,1,0) * 4.0 = (0,8,0)
	expectedApex := core.NewVec3(0, 8, 0)
	if !cone.apex.Equals(expectedApex) {
		t.Errorf("Expected apex %v, got %v", expectedApex, cone.apex)
	}
}

func TestNewCone_Validation(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))

	tests := []struct {
		name       string
		baseCenter core.Vec3
		baseRadius float64
		topCenter  core.Vec3
		topRadius  float64
		wantErr    bool
	}{
		{
			name:       "valid pointed cone",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  0.0,
			wantErr:    false,
		},
		{
			name:       "valid frustum",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 2.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  1.0,
			wantErr:    false,
		},
		{
			name:       "negative base radius",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: -1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  0.0,
			wantErr:    true,
		},
		{
			name:       "negative top radius",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  -0.5,
			wantErr:    true,
		},
		{
			name:       "equal radii (cylinder)",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  1.0,
			wantErr:    true,
		},
		{
			name:       "top radius larger than base",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  2.0,
			wantErr:    true,
		},
		{
			name:       "same base and top centers",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 0, 0),
			topRadius:  0.5,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCone(tt.baseCenter, tt.baseRadius, tt.topCenter, tt.topRadius, false, mat)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCone() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCone_BoundingBox(t *testing.T) {
	tests := []struct {
		name       string
		baseCenter core.Vec3
		baseRadius float64
		topCenter  core.Vec3
		topRadius  float64
		wantMin    core.Vec3
		wantMax    core.Vec3
	}{
		{
			name:       "axis-aligned Y pointed cone",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.0,
			topCenter:  core.NewVec3(0, 2, 0),
			topRadius:  0.0,
			wantMin:    core.NewVec3(-1, 0, -1),
			wantMax:    core.NewVec3(1, 2, 1),
		},
		{
			name:       "axis-aligned Y frustum",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 2.0,
			topCenter:  core.NewVec3(0, 4, 0),
			topRadius:  1.0,
			wantMin:    core.NewVec3(-2, 0, -2),
			wantMax:    core.NewVec3(2, 4, 2),
		},
		{
			name:       "axis-aligned Z",
			baseCenter: core.NewVec3(0, 0, 0),
			baseRadius: 1.5,
			topCenter:  core.NewVec3(0, 0, 3),
			topRadius:  0.5,
			wantMin:    core.NewVec3(-1.5, -1.5, 0),
			wantMax:    core.NewVec3(1.5, 1.5, 3),
		},
	}

	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cone, err := NewCone(tt.baseCenter, tt.baseRadius, tt.topCenter, tt.topRadius, false, mat)
			if err != nil {
				t.Fatalf("NewCone failed: %v", err)
			}

			bbox := cone.BoundingBox()

			tolerance := 1e-9
			if !approxEqualVec(bbox.Min, tt.wantMin, tolerance) {
				t.Errorf("Expected min %v, got %v", tt.wantMin, bbox.Min)
			}

			if !approxEqualVec(bbox.Max, tt.wantMax, tolerance) {
				t.Errorf("Expected max %v, got %v", tt.wantMax, bbox.Max)
			}
		})
	}
}

// Helper function for approximate vector equality
func approxEqualVec(a, b core.Vec3, tolerance float64) bool {
	return math.Abs(a.X-b.X) < tolerance &&
		math.Abs(a.Y-b.Y) < tolerance &&
		math.Abs(a.Z-b.Z) < tolerance
}

func TestCone_Hit_SimpleHit(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Cone along Y axis, from Y=0 (radius 1) to Y=2 (radius 0 - pointed)
	cone, err := NewCone(
		core.NewVec3(0, 0, 0), // base center
		1.0,                   // base radius
		core.NewVec3(0, 2, 0), // top center
		0.0,                   // top radius (pointed)
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from outside hitting the cone perpendicular to axis
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit, but got miss")
	}

	// At Y=1, the cone radius should be 0.5 (halfway up from base to apex)
	// So we should hit at X=0.5
	expectedPoint := core.NewVec3(0.5, 1, 0)
	tolerance := 1e-6
	if !approxEqualVec(hit.Point, expectedPoint, tolerance) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	if !hit.FrontFace {
		t.Error("Expected front face hit")
	}
}

func TestCone_Hit_Miss(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray that misses the cone entirely
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if isHit {
		t.Errorf("Expected miss, but got hit at %v", hit.Point)
	}
}

func TestCone_Hit_HeightBounds(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Cone from Y=0 to Y=2
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	tests := []struct {
		name    string
		origin  core.Vec3
		dir     core.Vec3
		wantHit bool
	}{
		{
			name:    "hit within bounds",
			origin:  core.NewVec3(2, 1, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: true,
		},
		{
			name:    "miss above cone",
			origin:  core.NewVec3(2, 3, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: false,
		},
		{
			name:    "miss below cone",
			origin:  core.NewVec3(2, -1, 0),
			dir:     core.NewVec3(-1, 0, 0),
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cone.Hit(ray, 0.001, 1000.0)

			if isHit != tt.wantHit {
				t.Errorf("Expected hit=%v, got hit=%v", tt.wantHit, isHit)
			}
		})
	}
}

func TestCone_Hit_Frustum(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Frustum: base radius 2, top radius 1, height 4
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		2.0, // base radius
		core.NewVec3(0, 4, 0),
		1.0, // top radius
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray hitting at Y=2 (halfway up)
	// At Y=2, radius should be 1.5 (linear interpolation)
	ray := core.NewRay(core.NewVec3(3, 2, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on frustum")
	}

	// Should hit at X=1.5 (the radius at Y=2)
	expectedPoint := core.NewVec3(1.5, 2, 0)
	tolerance := 1e-5
	if !approxEqualVec(hit.Point, expectedPoint, tolerance) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestCone_Hit_FromInside(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from inside the cone shooting outward
	ray := core.NewRay(core.NewVec3(0, 1, 0), core.NewVec3(1, 0, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit from inside")
	}

	// Should be a back face hit
	if hit.FrontFace {
		t.Error("Expected back face hit when shooting from inside")
	}
}

func TestCone_Hit_TBounds(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,
		false,
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))

	// Test tMax bound
	hit, isHit := cone.Hit(ray, 0.001, 1.0)
	if isHit {
		t.Errorf("Expected miss due to tMax bound, but got hit at t=%f", hit.T)
	}

	// Test tMin bound - intersection should be rejected if both are outside range
	hit, isHit = cone.Hit(ray, 10.0, 1000.0)
	if isHit {
		t.Errorf("Expected miss due to tMin bound, but got hit at t=%f", hit.T)
	}
}

func TestCone_Capped_HitBaseCap_Pointed(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Pointed cone with base cap
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,  // pointed
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from below hitting base cap
	ray := core.NewRay(core.NewVec3(0.5, -1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on base cap, but got miss")
	}

	// Should hit at Y=0 (base of cone)
	expectedPoint := core.NewVec3(0.5, 0, 0)
	tolerance := 1e-6
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in -Y direction (downward)
	expectedNormal := core.NewVec3(0, -1, 0)
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCone_Capped_HitBaseCap_Frustum(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Frustum with both caps
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.3,  // frustum
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from below hitting base cap
	ray := core.NewRay(core.NewVec3(0.3, -1, 0), core.NewVec3(0, 1, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on base cap, but got miss")
	}

	// Should hit at Y=0
	expectedPoint := core.NewVec3(0.3, 0, 0)
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}
}

func TestCone_Capped_HitTopCap_Frustum(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	// Frustum with both caps
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.3,  // frustum - has top cap
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from above hitting top cap
	ray := core.NewRay(core.NewVec3(0.2, 3, 0), core.NewVec3(0, -1, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on top cap, but got miss")
	}

	// Should hit at Y=2 (top of frustum)
	expectedPoint := core.NewVec3(0.2, 2, 0)
	tolerance := 1e-6
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Normal should point in +Y direction (upward)
	expectedNormal := core.NewVec3(0, 1, 0)
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v, got %v", expectedNormal, hit.Normal)
	}
}

func TestCone_Capped_MissOutsideCapRadius(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.3,  // frustum
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	tests := []struct {
		name   string
		origin core.Vec3
		dir    core.Vec3
	}{
		{
			name:   "miss base cap outside radius",
			origin: core.NewVec3(1.5, -1, 0),
			dir:    core.NewVec3(0, 1, 0),
		},
		{
			name:   "miss top cap outside radius (ray misses entire cone)",
			origin: core.NewVec3(2.0, 3, 0), // Far outside, won't hit body either
			dir:    core.NewVec3(0, -1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cone.Hit(ray, 0.001, 1000.0)

			if isHit {
				t.Error("Expected miss outside cap radius, but got hit")
			}
		})
	}
}

func TestCone_Capped_HitBodyNotCap(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,  // pointed
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray hitting the side (body), not the cap
	ray := core.NewRay(core.NewVec3(2, 1, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on cone body")
	}

	// Normal should have radial component, not purely axial
	// For a cone, the normal mixes radial and axial directions
	tolerance := 1e-6
	if math.Abs(hit.Normal.Y) > 1.0-tolerance {
		t.Errorf("Expected non-axial normal (cone body), got %v", hit.Normal)
	}
}

func TestCone_Uncapped_NoCapHits(t *testing.T) {
	mat := material.NewLambertian(core.NewVec3(0.5, 0.5, 0.5))

	tests := []struct {
		name      string
		topRadius float64
		origin    core.Vec3
		dir       core.Vec3
	}{
		{
			name:      "pointed cone - ray misses completely",
			topRadius: 0.0,
			origin:    core.NewVec3(2.0, -1, 0),
			dir:       core.NewVec3(0, 1, 0),
		},
		{
			name:      "frustum - ray misses completely",
			topRadius: 0.3,
			origin:    core.NewVec3(2.0, -1, 0),
			dir:       core.NewVec3(0, 1, 0),
		},
		{
			name:      "frustum - ray through top opening from far above",
			topRadius: 0.3,
			origin:    core.NewVec3(0.2, 3, 0),
			dir:       core.NewVec3(0, -1, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cone, err := NewCone(
				core.NewVec3(0, 0, 0),
				1.0,
				core.NewVec3(0, 2, 0),
				tt.topRadius,
				false, // uncapped
				mat,
			)
			if err != nil {
				t.Fatalf("NewCone failed: %v", err)
			}

			ray := core.NewRay(tt.origin, tt.dir)
			_, isHit := cone.Hit(ray, 0.001, 1000.0)

			// Should pass through without hitting
			if isHit {
				t.Error("Expected miss on uncapped cone, but got hit")
			}
		})
	}
}

func TestCone_Capped_GlassRefraction_BaseCap(t *testing.T) {
	// Test that rays traveling INSIDE a glass cone hit the base cap correctly
	// This is important for glass materials where rays refract in and then out
	mat := material.NewDielectric(1.5) // Glass
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,  // pointed
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// Ray from INSIDE the cone shooting downward toward base cap
	// This simulates a ray that has already entered the glass and is exiting
	ray := core.NewRay(core.NewVec3(0.3, 0.5, 0), core.NewVec3(0, -1, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on base cap from inside")
	}

	// Should hit at Y=0 (base cap)
	expectedPoint := core.NewVec3(0.3, 0, 0)
	if !hit.Point.Equals(expectedPoint) {
		t.Errorf("Expected hit point %v, got %v", expectedPoint, hit.Point)
	}

	// Critical: Should be a BACK FACE hit (hitting from inside)
	if hit.FrontFace {
		t.Error("Expected back face hit when ray travels from inside cone toward base cap")
	}

	// Critical: Normal should point UPWARD (back toward ray origin) for proper refraction
	// When hitting from inside, the normal should be flipped to point toward the ray
	expectedNormal := core.NewVec3(0, 1, 0) // Should point up (inward from glass perspective)
	tolerance := 1e-6
	if math.Abs(hit.Normal.X-expectedNormal.X) > tolerance ||
		math.Abs(hit.Normal.Y-expectedNormal.Y) > tolerance ||
		math.Abs(hit.Normal.Z-expectedNormal.Z) > tolerance {
		t.Errorf("Expected normal %v for back-face hit, got %v", expectedNormal, hit.Normal)
		t.Errorf("This is critical for glass refraction - ray should refract from glass->air with upward normal")
	}
}

func TestCone_Capped_GlassRefraction_ConeSurface_ThenCap(t *testing.T) {
	// More complete test: ray hits cone surface, enters glass, then should hit base cap from inside
	mat := material.NewDielectric(1.5) // Glass
	cone, err := NewCone(
		core.NewVec3(0, 0, 0),
		1.0,
		core.NewVec3(0, 2, 0),
		0.0,  // pointed
		true, // capped
		mat,
	)
	if err != nil {
		t.Fatalf("NewCone failed: %v", err)
	}

	// First hit: ray from outside hits cone body
	ray := core.NewRay(core.NewVec3(2, 0.5, 0), core.NewVec3(-1, 0, 0))
	hit, isHit := cone.Hit(ray, 0.001, 1000.0)

	if !isHit {
		t.Fatal("Expected hit on cone body")
	}

	if !hit.FrontFace {
		t.Error("First hit should be front face (entering glass)")
	}

	// Now simulate a refracted ray traveling inside the glass downward
	// (In reality this would be computed by the material, but we're testing geometry)
	insideRay := core.NewRay(hit.Point, core.NewVec3(0, -1, 0))
	hit2, isHit2 := cone.Hit(insideRay, 0.001, 1000.0)

	if !isHit2 {
		t.Fatal("Expected hit on base cap from inside")
	}

	// Second hit should be back face (exiting glass through base cap)
	if hit2.FrontFace {
		t.Error("Second hit should be back face (exiting glass through base cap)")
	}

	// Normal should point back toward the inside of the cone (upward)
	if hit2.Normal.Y <= 0 {
		t.Errorf("Expected upward normal for base cap back-face hit, got %v", hit2.Normal)
	}
}
