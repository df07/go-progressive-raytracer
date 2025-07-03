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

	// Store configuration for PDF calculations
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

	imageHeight := int(float64(config.Width) / config.AspectRatio) // Calculate height from width

	// Calculate viewport dimensions
	theta := config.VFov * math.Pi / 180.0 // Convert degrees to radians
	h := math.Tan(theta / 2.0)
	viewportHeight := 2.0 * h * focusDistance
	viewportWidth := viewportHeight * config.AspectRatio

	// Calculate the vectors across the horizontal and down the vertical viewport edges
	viewportU := u.Multiply(viewportWidth)  // Vector across viewport horizontal edge
	viewportV := v.Multiply(viewportHeight) // Vector up viewport vertical edge

	// Calculate the horizontal and vertical delta vectors from pixel to pixel
	pixelDeltaU := viewportU.Multiply(1.0 / float64(config.Width))
	pixelDeltaV := viewportV.Multiply(1.0 / float64(imageHeight))

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

	return &Camera{
		center:       config.Center,
		pixel00Loc:   pixel00Loc,
		pixelDeltaU:  pixelDeltaU,
		pixelDeltaV:  pixelDeltaV,
		defocusDiskU: defocusDiskU,
		defocusDiskV: defocusDiskV,
		config:       config,
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

	rayDirection := pixelSample.Subtract(rayOrigin)
	return core.NewRay(rayOrigin, rayDirection)
}

// CalculateRayPDFs calculates the area and direction PDFs for a camera ray
// This is needed for BDPT to properly balance camera and light path PDFs
func (c *Camera) CalculateRayPDFs(ray core.Ray) (areaPDF, directionPDF float64) {
	// Calculate image dimensions
	imageHeight := int(float64(c.config.Width) / c.config.AspectRatio)

	// Camera sensor area (in world units)
	// The sensor area is the area of one pixel times the number of pixels
	pixelAreaU := c.pixelDeltaU.Length()
	pixelAreaV := c.pixelDeltaV.Length()
	totalSensorArea := pixelAreaU * pixelAreaV * float64(c.config.Width) * float64(imageHeight)

	// Area PDF: uniform sampling over the sensor area
	areaPDF = 1.0 / totalSensorArea

	// Direction PDF: based on solid angle subtended by the pixel
	// For a pinhole camera, this is proportional to cos^3(theta) / distance^2
	// where theta is angle from optical axis

	// Calculate focus distance
	focusDistance := c.config.FocusDistance
	if focusDistance <= 0 {
		focusDistance = c.config.Center.Subtract(c.config.LookAt).Length()
	}

	// Ray direction from camera center (normalized)
	rayDir := ray.Direction.Normalize()

	// Camera forward direction (toward LookAt)
	cameraForward := c.config.LookAt.Subtract(c.config.Center).Normalize()

	// Cosine of angle between ray and camera forward direction
	cosTheta := rayDir.Dot(cameraForward)
	if cosTheta <= 0 {
		// Ray pointing away from camera forward direction - invalid
		return 0, 0
	}

	// Direction PDF includes cos^3 term for perspective projection
	// and normalization by solid angle of the entire image plane
	theta := c.config.VFov * math.Pi / 180.0
	h := math.Tan(theta / 2.0)
	viewportHeight := 2.0 * h * focusDistance
	viewportWidth := viewportHeight * c.config.AspectRatio

	// Total solid angle subtended by the viewport
	totalSolidAngle := viewportWidth * viewportHeight / (focusDistance * focusDistance)

	// Direction PDF: cos^3(theta) normalized by total solid angle
	directionPDF = (cosTheta * cosTheta * cosTheta) / totalSolidAngle

	return areaPDF, directionPDF
}

// GetCameraForward returns the camera's forward direction (toward LookAt)
func (c *Camera) GetCameraForward() core.Vec3 {
	return c.config.LookAt.Subtract(c.config.Center).Normalize()
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
