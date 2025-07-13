package renderer

import (
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// CameraConfig contains all camera configuration parameters
type CameraConfig struct {
	// Camera positioning
	Center core.Vec3 // Camera position
	LookAt core.Vec3 // Point the camera is looking at
	Up     core.Vec3 // Up direction (usually (0,1,0))

	// Image properties
	Width       int     // Image width in pixels
	AspectRatio float64 // Aspect ratio (width/height)
	VFov        float64 // Vertical field of view in degrees

	// Focus properties
	Aperture      float64 // Angle of defocus blur (0 = no blur)
	FocusDistance float64 // Distance to focus plane (0 = auto-calculate from LookAt)
}

// Camera generates rays for rendering with configurable positioning and depth of field
type Camera struct {
	center       core.Vec3 // Camera position
	pixel00Loc   core.Vec3 // Location of pixel (0,0)
	pixelDeltaU  core.Vec3 // Offset to pixel to the right
	pixelDeltaV  core.Vec3 // Offset to pixel below
	defocusDiskU core.Vec3 // Defocus disk horizontal radius
	defocusDiskV core.Vec3 // Defocus disk vertical radius

	// Camera coordinate system (computed once)
	u, v, w       core.Vec3 // Orthonormal basis vectors
	cameraForward core.Vec3 // Camera forward direction

	// Computed camera properties
	aspectRatio     float64 // Aspect ratio (width/height)
	focusDistance   float64 // Effective focus distance
	lensRadius      float64 // Effective lens radius
	lensArea        float64 // Lens area
	totalSensorArea float64 // Total sensor area
	imagePlaneArea  float64 // Image plane area
	imageWidth      int     // Image width in pixels
	imageHeight     int     // Image height in pixels
	theta           float64 // Field of view in radians
	viewportWidth   float64 // Width of the viewport
	viewportHeight  float64 // Height of the viewport
	cosTotalWidth   float64 // Cosine of half the field of view width
	cosTotalHeight  float64 // Cosine of half the field of view height

	// Store configuration for reference
	config CameraConfig
}

// NewCamera creates a camera with the given configuration
func NewCamera(config CameraConfig) *Camera {
	// Calculate camera coordinate system
	w := config.Center.Subtract(config.LookAt).Normalize() // Camera looks along -w
	u := w.Cross(config.Up).Normalize()                    // Right vector (fixed: was config.Up.Cross(w))
	v := w.Cross(u)                                        // Up vector

	// Calculate focus distance - auto-calculate from LookAt if not specified
	focusDistance := config.FocusDistance
	if focusDistance <= 0 {
		focusDistance = config.Center.Subtract(config.LookAt).Length()
	}

	// Calculate lens properties
	lensRadius := config.Aperture / 2
	lensArea := math.Pi * lensRadius * lensRadius
	if config.Aperture == 0 {
		lensArea = 1.0 // Pinhole camera
	}

	// Calculate image dimensions and plane area
	imageWidth := config.Width
	imageHeight := int(float64(imageWidth) / config.AspectRatio)
	theta := config.VFov * math.Pi / 180.0
	h := math.Tan(theta / 2.0)
	viewportHeight := 2.0 * h * focusDistance
	viewportWidth := viewportHeight * config.AspectRatio

	// pbrt: Compute image plane area at z=1 for camera importance function
	// This matches PBRT's approach of normalizing to unit distance
	hAtZ1 := math.Tan(theta / 2.0)
	viewportHeightAtZ1 := 2.0 * hAtZ1 * 1.0 // At z=1
	viewportWidthAtZ1 := viewportHeightAtZ1 * config.AspectRatio
	imagePlaneArea := viewportWidthAtZ1 * viewportHeightAtZ1

	// Calculate the vectors across the horizontal and down the vertical viewport edges
	viewportU := u.Multiply(viewportWidth)  // Vector across viewport horizontal edge
	viewportV := v.Multiply(viewportHeight) // Vector up viewport vertical edge

	// Calculate the horizontal and vertical delta vectors from pixel to pixel
	pixelDeltaU := viewportU.Multiply(1.0 / float64(imageWidth))
	pixelDeltaV := viewportV.Multiply(1.0 / float64(imageHeight))

	// Camera sensor area (in world units)
	// The sensor area is the area of one pixel times the number of pixels
	pixelAreaU := pixelDeltaU.Length()
	pixelAreaV := pixelDeltaV.Length()
	totalSensorArea := pixelAreaU * pixelAreaV * float64(imageWidth) * float64(imageHeight)

	// Calculate the location of the upper left pixel
	halfViewportU := viewportU.Multiply(0.5)
	halfViewportV := viewportV.Multiply(0.5)
	viewportUpperLeft := config.Center.
		Subtract(w.Multiply(focusDistance)).
		Subtract(halfViewportU).
		Subtract(halfViewportV)

	pixel00Loc := viewportUpperLeft.Add(pixelDeltaU.Add(pixelDeltaV).Multiply(0.5))

	// Calculate defocus disk basis vectors
	defocusDiskU := u.Multiply(config.Aperture / 2)
	defocusDiskV := v.Multiply(config.Aperture / 2)

	// calculate the camera forward direction
	cameraForward := config.LookAt.Subtract(config.Center).Normalize()

	// Compute cosTotalWidth like PBRT: direction to corner of image
	// Find corner of viewport in camera space
	cornerPoint := core.NewVec3(-viewportWidth/2, -viewportHeight/2, -focusDistance)
	cornerDirection := cornerPoint.Normalize()
	// Cosine of angle between forward direction (0,0,-1) and corner direction
	forwardDirection := core.NewVec3(0, 0, -1)
	cosTotalWidth := cornerDirection.Dot(forwardDirection)
	cosTotalHeight := cosTotalWidth // Same for both since we check corners

	return &Camera{
		center:          config.Center,
		pixel00Loc:      pixel00Loc,
		pixelDeltaU:     pixelDeltaU,
		pixelDeltaV:     pixelDeltaV,
		defocusDiskU:    defocusDiskU,
		defocusDiskV:    defocusDiskV,
		u:               u,
		v:               v,
		w:               w,
		focusDistance:   focusDistance,
		lensRadius:      lensRadius,
		lensArea:        lensArea,
		viewportWidth:   viewportWidth,
		viewportHeight:  viewportHeight,
		totalSensorArea: totalSensorArea,
		imagePlaneArea:  imagePlaneArea,
		imageWidth:      config.Width,
		imageHeight:     imageHeight,
		theta:           theta,
		cosTotalWidth:   cosTotalWidth,
		cosTotalHeight:  cosTotalHeight,
		cameraForward:   cameraForward,
		config:          config,
	}
}

// GetRay generates a ray for pixel coordinates (i, j) with sub-pixel sampling using the provided random generator
func (c *Camera) GetRay(i, j int, random *rand.Rand) core.Ray {
	// Add random offset for anti-aliasing
	jitter := core.NewVec3(random.Float64()-0.5, random.Float64()-0.5, 0)
	pixelSample := c.pixel00Loc.
		Add(c.pixelDeltaU.Multiply(float64(i) + jitter.X)).
		Add(c.pixelDeltaV.Multiply(float64(j) + jitter.Y))

	// Determine ray origin (with defocus blur if enabled)
	rayOrigin := c.center
	if c.defocusDiskU.Length() > 0 {
		p := core.RandomInUnitDisk(random)
		offset := c.defocusDiskU.Multiply(p.X).Add(c.defocusDiskV.Multiply(p.Y))
		rayOrigin = c.center.Add(offset)
	}

	rayDirection := pixelSample.Subtract(rayOrigin).Normalize()
	return core.NewRay(rayOrigin, rayDirection)
}

// CalculateRayPDFs calculates the area and direction PDFs for a camera ray
// This is needed for BDPT to properly balance camera and light path PDFs
func (c *Camera) CalculateRayPDFs(ray core.Ray) (areaPDF, dirPDF float64) {

	// Cosine of angle between ray and camera forward direction
	cosTheta := ray.Direction.Dot(c.cameraForward)
	if cosTheta <= c.cosTotalWidth {
		return 0, 0
	}

	// Check if ray hits within image bounds using existing MapRayToPixel
	_, _, inBounds := c.MapRayToPixel(ray)
	if !inBounds {
		return 0, 0
	}

	// pbrt: *pdfPos = 1 / lensArea;
	// pbrt: *pdfDir = 1 / (A * Pow<3>(cosTheta));
	// pbrt: A = std::abs((pMax.x - pMin.x) * (pMax.y - pMin.y)); // image plane area

	areaPDF = 1.0 / c.lensArea
	dirPDF = 1.0 / (c.imagePlaneArea * math.Pow(cosTheta, 3))

	return areaPDF, dirPDF
}

// GetCameraForward returns the camera's forward direction (toward LookAt)
func (c *Camera) GetCameraForward() core.Vec3 {
	return c.cameraForward
}

// SampleCameraFromPoint samples the camera from a reference point for t=1 strategies
// Camera handles lens sampling internally, returns complete sample
// Equivalent to pbrt PerspectiveCamera::SampleWi
func (c *Camera) SampleCameraFromPoint(refPoint core.Vec3, random *rand.Rand) *core.CameraSample {
	// Sample lens coordinates using concentric disk sampling
	lensCoords := core.RandomInUnitDisk(random).Multiply(c.lensRadius)

	// Transform lens point to world space using stored camera basis vectors
	lensPoint := c.center.Add(c.u.Multiply(lensCoords.X)).Add(c.v.Multiply(lensCoords.Y))

	// Create ray from lens toward reference point
	direction := refPoint.Subtract(lensPoint).Normalize()
	distance := refPoint.Subtract(lensPoint).Length()
	ray := core.NewRay(lensPoint, direction)

	// Calculate PDF for camera sampling
	// pbrt: Float lensArea = lensRadius != 0 ? (Pi * Sqr(lensRadius)) : 1;
	// pbrt: Float pdf = Sqr(dist) / (AbsDot(lensIntr.n, wi) * lensArea);
	cosine := c.cameraForward.AbsDot(direction)
	pdf := (distance * distance) / (cosine * c.lensArea)

	// Calculate camera importance weight
	importance := c.EvaluateRayImportance(ray)
	if importance.Luminance() == 0 {
		return nil // Ray doesn't contribute
	}

	return &core.CameraSample{
		Ray:    ray,
		Weight: importance,
		PDF:    pdf,
	}
}

// EvaluateRayImportance calculates the camera importance function for a ray
func (c *Camera) EvaluateRayImportance(ray core.Ray) core.Vec3 {
	// Check if ray is forward-facing with respect to the camera
	cosine := ray.Direction.Dot(c.cameraForward)

	if cosine < c.cosTotalWidth {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Return importance for point on image plane
	// PBRT formula: We = 1 / (A * lensArea * cos^4(theta))
	importance := 1.0 / (c.imagePlaneArea * c.lensArea * math.Pow(cosine, 4))

	return core.Vec3{X: importance, Y: importance, Z: importance}
}

// MapRayToPixel maps a ray back to pixel coordinates (for splat placement)
func (c *Camera) MapRayToPixel(ray core.Ray) (int, int, bool) {
	// Find intersection with image plane
	// Ray: origin + t * direction
	// Image plane: center - w * focusDistance
	imagePlaneCenter := c.center.Subtract(c.w.Multiply(c.focusDistance))

	// Solve for t: (origin + t * direction - imagePlaneCenter) dot w = 0
	toPlane := imagePlaneCenter.Subtract(ray.Origin)
	t := toPlane.Dot(c.w) / ray.Direction.Dot(c.w)

	if t <= 0 {
		return 0, 0, false // Ray going wrong direction
	}

	// Find hit point on image plane
	hitPoint := ray.Origin.Add(ray.Direction.Multiply(t))

	// Convert to image plane coordinates
	relativePoint := hitPoint.Subtract(imagePlaneCenter)
	planeX := relativePoint.Dot(c.u)
	planeY := relativePoint.Dot(c.v)

	// Convert to viewport coordinates and normalize to [0,1] range
	normalizedX := (planeX + c.viewportWidth/2) / c.viewportWidth
	normalizedY := (planeY + c.viewportHeight/2) / c.viewportHeight

	// Convert to pixel coordinates
	pixelX := int(normalizedX * float64(c.imageWidth))
	pixelY := int(normalizedY * float64(c.imageHeight))

	// Check bounds
	if pixelX >= 0 && pixelX < c.imageWidth && pixelY >= 0 && pixelY < c.imageHeight {
		return pixelX, pixelY, true
	}

	return 0, 0, false
}

// MergeCameraConfig merges camera configuration overrides with defaults
// Only non-zero values in the override will replace the default values
func MergeCameraConfig(defaultConfig CameraConfig, override CameraConfig) CameraConfig {
	result := defaultConfig

	if override.Width != 0 {
		result.Width = override.Width
	}
	if override.AspectRatio != 0 {
		result.AspectRatio = override.AspectRatio
	}
	if override.Center.Length() != 0 {
		result.Center = override.Center
	}
	if override.LookAt.Length() != 0 {
		result.LookAt = override.LookAt
	}
	if override.Up.Length() != 0 {
		result.Up = override.Up
	}
	if override.VFov != 0 {
		result.VFov = override.VFov
	}
	if override.Aperture != 0 {
		result.Aperture = override.Aperture
	}
	if override.FocusDistance != 0 {
		result.FocusDistance = override.FocusDistance
	}

	return result
}
