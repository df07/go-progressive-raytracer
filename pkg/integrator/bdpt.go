package integrator

import (
	"fmt"
	"math"
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Vertex represents a single vertex in a light transport path
type Vertex struct {
	Point    core.Vec3     // 3D position
	Normal   core.Vec3     // Surface normal
	Light    core.Light    // Light at this vertex
	Material core.Material // Material at this vertex
	Camera   core.Camera   // Camera at this vertex

	// Path tracing information
	IncomingDirection core.Vec3 // Direction ray arrived from

	// MIS probability densities
	AreaPdfForward float64 // PDF for generating this vertex forward
	AreaPdfReverse float64 // PDF for generating this vertex reverse

	// Vertex classification
	IsLight         bool // On light source
	IsCamera        bool // On camera
	IsSpecular      bool // Specular interaction
	IsInfiniteLight bool // On infinite area light (background)

	// Transport quantities
	Beta         core.Vec3 // Accumulated throughput from path start to this vertex
	EmittedLight core.Vec3 // Light emitted from this vertex
}

// Path represents a sequence of vertices in a light transport path
type Path struct {
	Vertices []Vertex
	Length   int
}

// BDPTIntegrator implements bidirectional path tracing
type BDPTIntegrator struct {
	*PathTracingIntegrator
	Verbose bool
}

// bdptStrategy represents a single BDPT path construction strategy
type bdptStrategy struct {
	s, t         int             // Light path length, camera path length
	contribution core.Vec3       // Radiance contribution
	misWeight    float64         // MIS weight
	splatRays    []core.SplatRay // Splat rays for t=1 strategies
}

// NewBDPTIntegrator creates a new BDPT integrator
func NewBDPTIntegrator(config core.SamplingConfig) *BDPTIntegrator {
	return &BDPTIntegrator{
		PathTracingIntegrator: NewPathTracingIntegrator(config),
		Verbose:               false,
	}
}

// RayColorWithSplats computes color with support for ray-based splatting
// Returns (pixel color, splat rays)
func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, sampleIndex int) (core.Vec3, []core.SplatRay) {
	// for now, both paths have the same max depth
	cameraMaxDepth := bdpt.config.MaxDepth
	lightMaxDepth := bdpt.config.MaxDepth

	// Generate camera path with vertices
	cameraPath := bdpt.generateCameraSubpath(ray, scene, random, cameraMaxDepth)

	// Generate a light path
	lightPath := bdpt.generateLightSubpath(scene, random, lightMaxDepth)

	// Evaluate all BDPT strategies with proper MIS weighting
	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, scene, random)

	// evaluateBDPTStrategies now returns both color and splats
	return bdpt.evaluateBDPTStrategies(strategies)
}

// generateCameraSubpath generates a camera subpath with proper PDF tracking for BDPT
// Each vertex stores forward/reverse PDFs needed for MIS weight calculation
func (bdpt *BDPTIntegrator) generateCameraSubpath(ray core.Ray, scene core.Scene, random *rand.Rand, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Calculate camera PDFs for proper BDPT balancing
	camera := scene.GetCamera()
	_, directionPDF := camera.CalculateRayPDFs(ray)

	// Calculate cosine term for perspective projection (like light emission cosine)
	rayDir := ray.Direction.Normalize()
	cameraForward := camera.GetCameraForward()
	cosTheta := rayDir.Dot(cameraForward)
	if cosTheta <= 0 {
		cosTheta = 0.001 // Avoid division by zero
	}

	// Create the initial camera vertex (like light path does for light sources)
	cameraVertex := Vertex{
		Point:             ray.Origin,
		Normal:            ray.Direction.Multiply(-1),  // Camera "normal" points back along ray
		Material:          nil,                         // Cameras don't have materials
		Light:             nil,                         // Cameras are not lights
		Camera:            camera,                      // Store camera reference for PDF calculations
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0}, // Camera is the starting point
		AreaPdfForward:    0.0,                         // Initial camera PDF is always 0.0
		AreaPdfReverse:    0.0,                         // Cannot generate reverse direction to camera
		IsLight:           false,
		IsCamera:          true,
		IsSpecular:        false,
		Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
		EmittedLight:      core.Vec3{X: 0, Y: 0, Z: 0}, // Cameras don't emit light
	}

	path.Vertices = append(path.Vertices, cameraVertex)
	path.Length++

	// Continue the camera path by tracing the ray through the scene
	beta := core.Vec3{X: 1, Y: 1, Z: 1}
	bdpt.extendPath(&path, ray, beta, directionPDF, scene, random, maxDepth, true) // Pass maxDepth through because camera doesn't count as a vertex

	return path
}

// generateLightSubpath generates a light subpath with proper PDF tracking for BDPT
// Starting from light emission, each vertex stores forward/reverse PDFs for MIS
func (bdpt *BDPTIntegrator) generateLightSubpath(scene core.Scene, random *rand.Rand, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Sample emission from a light in the scene
	lights := scene.GetLights()
	if len(lights) == 0 {
		return path
	}

	sampledLight := lights[random.Intn(len(lights))]
	emissionSample := sampledLight.SampleEmission(random)
	lightSelectionPdf := 1.0 / float64(len(lights))
	cosTheta := emissionSample.Direction.Dot(emissionSample.Normal)

	lightVertex := Vertex{
		Point:             emissionSample.Point,
		Normal:            emissionSample.Normal,
		Material:          nil, // Lights don't have materials in our current system
		Light:             sampledLight,
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0}, // Light is the starting point
		AreaPdfForward:    emissionSample.AreaPDF * lightSelectionPdf,
		AreaPdfReverse:    0.0, // Cannot generate reverse direction to light
		IsLight:           true,
		IsCamera:          false,
		IsSpecular:        false,
		Beta:              emissionSample.Emission, // Include emission in throughput
		EmittedLight:      emissionSample.Emission, // Already properly scaled
	}

	path.Vertices = append(path.Vertices, lightVertex)
	path.Length++

	// Continue the light path by bouncing off surfaces using the common path extension logic
	currentRay := core.NewRay(emissionSample.Point, emissionSample.Direction)

	// Use the common path extension logic (maxDepth-1 because light counts as a vertex in pt)
	// PBRT formula: beta = Le * |cos(theta)| / (lightPdf * pdfPos * pdfDir)
	forwardThroughput := emissionSample.Emission.Multiply(math.Abs(cosTheta) / (lightSelectionPdf * emissionSample.AreaPDF * emissionSample.DirectionPDF))
	bdpt.logf("generateLightSubpath: forwardThroughput=%v, cosTheta=%f, lightSelectionPdf=%f, AreaPDF=%f, DirectionPDF=%f\n", forwardThroughput, math.Abs(cosTheta), lightSelectionPdf, emissionSample.AreaPDF, emissionSample.DirectionPDF)
	bdpt.extendPath(&path, currentRay, forwardThroughput, emissionSample.DirectionPDF, scene, random, maxDepth-1, false)

	return path
}

// extendPath extends a path by tracing a ray through the scene, handling intersections and scattering
// This is the common logic shared between camera and light path generation after the initial vertex
func (bdpt *BDPTIntegrator) extendPath(path *Path, currentRay core.Ray, beta core.Vec3, pdfDir float64, scene core.Scene, random *rand.Rand, maxBounces int, isCameraPath bool) {
	for bounces := 0; bounces < maxBounces; bounces++ {
		vertexPrev := path.Vertices[path.Length-1]

		// Check for intersections
		hit, isHit := scene.GetBVH().Hit(currentRay, 0.001, 1e100)
		if !isHit {
			if !isCameraPath {
				break
			}
			// Hit background - create a background vertex with captured light
			bgColor := bdpt.BackgroundGradient(currentRay, scene)

			// For background (infinite area light), we should use solid angle PDF directly
			// Don't convert to area PDF since background is at infinite distance
			vertex := Vertex{
				Point:             currentRay.Origin.Add(currentRay.Direction.Multiply(1000.0)), // Far background
				Normal:            currentRay.Direction.Multiply(-1),                            // Reverse direction
				Material:          nil,
				IncomingDirection: currentRay.Direction.Multiply(-1),
				AreaPdfForward:    pdfDir,                  // Keep as solid angle PDF for infinite area light
				AreaPdfReverse:    0.0,                     // Cannot generate rays towards background
				IsLight:           bgColor.Luminance() > 0, // Only mark as light if background actually emits
				IsCamera:          false,                   // No camera vertices in path extension
				IsSpecular:        false,
				IsInfiniteLight:   true, // Mark as infinite area light
				Beta:              beta,
				EmittedLight:      bgColor, // Capture background light
			}

			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Capture emitted light from this vertex
		emittedLight := bdpt.GetEmittedLight(currentRay, hit)

		// Create vertex for the intersection
		vertex := Vertex{
			Point:             hit.Point,
			Normal:            hit.Normal,
			Material:          hit.Material,
			IncomingDirection: currentRay.Direction.Multiply(-1),
			AreaPdfForward:    1.0, // Will be updated by setVertexPDFs if material scatters
			AreaPdfReverse:    0.0, // Will be updated by setVertexPDFs if material scatters
			IsLight:           emittedLight.Luminance() > 0,
			IsCamera:          false, // No camera vertices in path extension
			IsSpecular:        false, // Will be set below if material scatters
			Beta:              beta,
			EmittedLight:      emittedLight, // Captured during path generation
		}

		// Set forward direction PDF into this vertex (PBRT: prev.ConvertDensity(pdf, v))
		vertex.AreaPdfForward = vertexPrev.convertPDFDensity(vertex, pdfDir)

		// Try to scatter the ray
		scatter, didScatter := hit.Material.Scatter(currentRay, *hit, random)
		if !didScatter {
			// Material absorbed the ray - add vertex and terminate path
			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Material scattered - capture the scatter information
		vertex.IsSpecular = scatter.IsSpecular()
		pdfDir = scatter.PDF // PDF for the direction we scattered, also used in next bounce

		// Handle specular vs diffuse materials differently (like path tracer does)
		cosTheta := scatter.Scattered.Direction.AbsDot(hit.Normal)
		if scatter.IsSpecular() {
			// For specular materials: no PDF division (deterministic reflection/refraction)
			beta = beta.MultiplyVec(scatter.Attenuation)
		} else {
			// For diffuse materials: standard Monte Carlo integration with PDF
			beta = beta.MultiplyVec(scatter.Attenuation).Multiply(cosTheta / pdfDir)
		}

		pdfRev, isReverseDelta := hit.Material.PDF(scatter.Scattered.Direction, currentRay.Direction.Multiply(-1), hit.Normal)
		// For delta functions in BDPT, set reverse PDF to 0 (like PBRT)
		if isReverseDelta {
			vertex.IsSpecular = true
			pdfRev = 0.0
			pdfDir = 0.0
		}
		vertexPrev.AreaPdfReverse = vertex.convertPDFDensity(vertexPrev, pdfRev)

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
	}
}

// Return probability per unit area at vertex to
func (v *Vertex) convertPDFDensity(next Vertex, pdfDir float64) float64 {
	// For infinite area lights (background), keep solid angle PDF as-is
	if next.IsInfiniteLight {
		return pdfDir
	}

	direction := next.Point.Subtract(v.Point)
	distanceSquared := direction.LengthSquared()
	if distanceSquared == 0 { // prevent division by zero
		return 0.0
	}
	invDist2 := 1.0 / distanceSquared

	// Follow PBRT's ConvertDensity exactly
	pdf := pdfDir
	// Only multiply by cosine if next vertex is on a surface
	if next.Material != nil { // IsOnSurface equivalent
		cosTheta := direction.Multiply(math.Sqrt(invDist2)).Dot(next.Normal)
		pdf *= math.Abs(cosTheta) // Use absolute value like PBRT
	}

	return pdf * invDist2
}

// calculateMISWeight implements PBRT's MIS weighting for BDPT strategies
// This compares forward vs reverse PDFs to properly weight different path construction strategies
func (bdpt *BDPTIntegrator) calculateMISWeight(cameraPath, lightPath Path, sampledVertex *Vertex, s, t int, scene core.Scene) float64 {
	if s+t == 2 {
		return 1.0
	}

	// For path tracing strategies that hit infinite lights (background),
	// return MIS weight 1.0 since we can't actually sample infinite lights directly
	if s == 0 && t > 1 {
		lastVertex := cameraPath.Vertices[t-1]
		if lastVertex.IsInfiniteLight {
			bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight: infinite light hit, weight=1.0\n", s, t)
			return 1.0
		}
	}

	// Helper function equivalent to PBRT's remap0 - deals with delta functions
	remap0 := func(f float64) float64 {
		if f != 0 {
			return f
		}
		return 1.0
	}

	sumRi := 0.0

	// Look up connection vertices and their predecessors
	var qs, pt, qsMinus, ptMinus *Vertex
	if s > 0 {
		qs = &lightPath.Vertices[s-1]
	}
	if t > 0 {
		pt = &cameraPath.Vertices[t-1]
	}
	if s > 1 {
		qsMinus = &lightPath.Vertices[s-2]
	}
	if t > 1 {
		ptMinus = &cameraPath.Vertices[t-2]
	}

	// Store original values to restore later (Go's defer equivalent of PBRT's ScopedAssignment)
	var originalPtPdfRev, originalPtMinusPdfRev, originalQsPdfRev, originalQsMinusPdfRev float64
	var originalPtDelta, originalQsDelta bool

	defer func() {
		// Restore original values
		if pt != nil {
			pt.AreaPdfReverse = originalPtPdfRev
			pt.IsSpecular = originalPtDelta
		}
		if ptMinus != nil {
			ptMinus.AreaPdfReverse = originalPtMinusPdfRev
		}
		if qs != nil {
			qs.AreaPdfReverse = originalQsPdfRev
			qs.IsSpecular = originalQsDelta
		}
		if qsMinus != nil {
			qsMinus.AreaPdfReverse = originalQsMinusPdfRev
		}
	}()

	// Update sampled vertex for s=1 or t=1 strategy
	if s == 1 && qs != nil && sampledVertex != nil {
		*qs = *sampledVertex
	} else if t == 1 && pt != nil && sampledVertex != nil {
		*pt = *sampledVertex
	}

	// Mark connection vertices as non-degenerate and store originals
	if pt != nil {
		originalPtDelta = pt.IsSpecular
		pt.IsSpecular = false
	}
	if qs != nil {
		originalQsDelta = qs.IsSpecular
		qs.IsSpecular = false
	}

	// Update reverse density of vertex pt_{t-1}
	if pt != nil {
		originalPtPdfRev = pt.AreaPdfReverse
		if s > 0 {
			// pt.AreaPdfReverse = qs.Pdf(scene, qsMinus, *pt)
			pt.AreaPdfReverse = bdpt.calculateVertexPdf(*qs, qsMinus, *pt, scene)
			bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight 1 remap pt: pt.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtPdfRev, pt.AreaPdfReverse)
		} else {
			// pt.AreaPdfReverse = pt.PdfLightOrigin(scene, *ptMinus, lightPdf, lightToIndex)
			pt.AreaPdfReverse = bdpt.calculateLightOriginPdf(*pt, *ptMinus, scene)
			bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight 2 remap pt: pt.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtPdfRev, pt.AreaPdfReverse)
		}
	}

	// Update reverse density of vertex pt_{t-2}
	if ptMinus != nil {
		originalPtMinusPdfRev = ptMinus.AreaPdfReverse
		if s > 0 {
			// ptMinus.AreaPdfReverse = pt.Pdf(scene, qs, *ptMinus)
			ptMinus.AreaPdfReverse = bdpt.calculateVertexPdf(*pt, qs, *ptMinus, scene)
			bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight 1 remap ptMinus: ptMinus.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtMinusPdfRev, ptMinus.AreaPdfReverse)
		} else {
			// ptMinus.AreaPdfReverse = pt.PdfLight(scene, *ptMinus)
			ptMinus.AreaPdfReverse = bdpt.calculateLightPdf(*pt, *ptMinus, scene)
			bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight 2 remap ptMinus: ptMinus.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtMinusPdfRev, ptMinus.AreaPdfReverse)
		}
	}

	// Update reverse density of vertices qs_{s-1} and qs_{s-2}
	if qs != nil {
		originalQsPdfRev = qs.AreaPdfReverse
		if pt != nil {
			// qs.AreaPdfReverse = pt.Pdf(scene, ptMinus, *qs)
			qs.AreaPdfReverse = bdpt.calculateVertexPdf(*pt, ptMinus, *qs, scene)
		}
	}
	if qsMinus != nil {
		originalQsMinusPdfRev = qsMinus.AreaPdfReverse
		if qs != nil && pt != nil {
			// qsMinus.AreaPdfReverse = qs.Pdf(scene, pt, *qsMinus)
			qsMinus.AreaPdfReverse = bdpt.calculateVertexPdf(*qs, pt, *qsMinus, scene)
		}
	}

	// Consider hypothetical connection strategies along the camera subpath
	ri := 1.0
	for i := t - 1; i > 0; i-- {
		vertex := &cameraPath.Vertices[i]
		ri *= remap0(vertex.AreaPdfReverse) / remap0(vertex.AreaPdfForward)

		// Check if there's a specular vertex later in the path
		hasSpecularAfter := false
		for j := i + 1; j < t; j++ {
			if cameraPath.Vertices[j].IsSpecular {
				hasSpecularAfter = true
				break
			}
		}

		// HACK: Exclude connection strategies that would require connecting through specular vertices
		// This compensates for not implementing t=1 strategies (light tracing to camera)
		// TODO: Remove this hack once t=1 strategies are implemented
		//
		// The issue: MIS heavily downweights path tracing strategies expecting t=1 to be more efficient
		// for specular reflection paths. Since we skip t=1, we need to prevent MIS from considering
		// impossible connection strategies that would connect through specular vertices.
		//
		// Only add to sumRi if no specular vertex follows (meaning connection is viable)
		if !vertex.IsSpecular && !cameraPath.Vertices[i-1].IsSpecular && !hasSpecularAfter {
			sumRi += ri
		}
		bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight cameraPath[%d]: pdfFwd=%.3g, pdfRev=%.3g, ri=%.3g, sumRi=%.3g, hasSpecularAfter=%t\n", s, t, i, remap0(vertex.AreaPdfForward), remap0(vertex.AreaPdfReverse), ri, sumRi, hasSpecularAfter)
	}

	// Consider hypothetical connection strategies along the light subpath
	ri = 1.0
	for i := s - 1; i >= 0; i-- {
		vertex := &lightPath.Vertices[i]
		ri *= remap0(vertex.AreaPdfReverse) / remap0(vertex.AreaPdfForward)

		var deltaLightVertex bool
		if i > 0 {
			deltaLightVertex = lightPath.Vertices[i-1].IsSpecular
		} else {
			deltaLightVertex = vertex.IsLight && vertex.IsSpecular // TODO: light needs to tell if it is delta
		}

		if !vertex.IsSpecular && !deltaLightVertex {
			sumRi += ri
		}
		bdpt.logf(" (s=%d,t=%d) calculatePBRTMISWeight lightPath[%d]: pdfFwd=%.3g, pdfRev=%.3g, ri=%.3g, sumRi=%.3g\n", s, t, i, remap0(vertex.AreaPdfForward), remap0(vertex.AreaPdfReverse), ri, sumRi)
	}

	return 1.0 / (1.0 + sumRi)
}

// Helper functions for PBRT MIS calculations

// calculateVertexPdf implements PBRT's Vertex::Pdf
func (bdpt *BDPTIntegrator) calculateVertexPdf(curr Vertex, prev *Vertex, next Vertex, scene core.Scene) float64 {
	if curr.IsLight {
		return bdpt.calculateLightPdf(curr, next, scene)
	}

	// Compute directions to preceding and next vertex
	wn := next.Point.Subtract(curr.Point)
	if wn.LengthSquared() == 0 {
		return 0
	}
	wn = wn.Normalize()

	var wp core.Vec3
	if prev != nil {
		wp = prev.Point.Subtract(curr.Point)
		if wp.LengthSquared() == 0 {
			return 0
		}
		wp = wp.Normalize()
	} else {
		// CHECK(type == VertexType::Camera) equivalent
		if !curr.IsCamera {
			return 0
		}
	}

	var pdf float64
	if curr.IsCamera {
		// ei.camera->Pdf_We(ei.SpawnRay(wn), &unused, &pdf);
		// Use our camera PDF implementation
		ray := core.NewRay(curr.Point, wn)
		if curr.Camera != nil {
			_, pdf = curr.Camera.CalculateRayPDFs(ray)
		}
		if pdf == 0 {
			return 0
		}
	} else if curr.Material != nil {
		// pdf = si.bsdf->Pdf(wp, wn);
		materialPdf, isDelta := curr.Material.PDF(wp, wn, curr.Normal)
		if isDelta {
			return 0
		}
		pdf = materialPdf
	} else {
		// Medium case - TODO: implement if needed
		return 0
	}

	// Return probability per unit area at vertex _next_
	// return ConvertDensity(pdf, next);
	return curr.convertPDFDensity(next, pdf)
}

// calculateLightPdf implements PBRT's Vertex::PdfLight
func (bdpt *BDPTIntegrator) calculateLightPdf(curr Vertex, to Vertex, scene core.Scene) float64 {
	w := to.Point.Subtract(curr.Point)
	invDist2 := 1.0 / w.LengthSquared()
	w = w.Multiply(math.Sqrt(invDist2))

	var pdf float64
	if curr.IsLight {
		// Handle infinite area lights (background)
		if curr.IsInfiniteLight {
			// PBRT: Compute planar sampling density for infinite light sources
			worldRadius := bdpt.getWorldRadius(scene)
			pdf = 1.0 / (math.Pi * worldRadius * worldRadius)
		} else if curr.Light != nil {
			// Use the light's EmissionPDF which gives area PDF
			areaPdf := curr.Light.EmissionPDF(curr.Point, w)

			// Convert to directional PDF: pdfDir = areaPdf / cos(theta)
			// where cos(theta) is angle between light normal and emission direction
			cosTheta := w.Dot(curr.Normal)
			if cosTheta <= 0 {
				return 0
			}

			pdfDir := areaPdf / cosTheta
			pdf = pdfDir * invDist2
		}
	}

	// if (v.IsOnSurface()) pdf *= AbsDot(v.ng(), w);
	if !to.IsLight && !to.IsCamera {
		cosine := math.Abs(to.Normal.Dot(w))
		pdf *= cosine
	}

	return pdf
}

// calculateLightOriginPdf implements PBRT's Vertex::PdfLightOrigin
func (bdpt *BDPTIntegrator) calculateLightOriginPdf(lightVertex Vertex, to Vertex, scene core.Scene) float64 {
	w := to.Point.Subtract(lightVertex.Point)
	if w.LengthSquared() == 0 {
		return 0
	}
	w = w.Normalize()

	// Handle infinite area lights (background)
	if lightVertex.IsInfiniteLight {
		// PBRT: Return solid angle density for infinite light sources
		// For our simple background, use uniform solid angle distribution
		// In PBRT this would be InfiniteLightDensity(scene, lightDistr, lightToDistrIndex, w)

		// Light selection probability (uniform among all lights)
		lightSelectionPdf := 1.0 // no light selection for infinite light

		// For uniform background, PDF is uniform over sphere
		infiniteLightPdf := 1.0 / (4.0 * math.Pi)

		bdpt.logf(" (s=?,t=?) calculateLightOriginPdf: infinite light, infiniteLightPdf=%0.3g, lightSelectionPdf=%0.3g\n", infiniteLightPdf, lightSelectionPdf)
		return infiniteLightPdf / lightSelectionPdf
	}

	if !lightVertex.IsLight || lightVertex.Light == nil {
		return 0
	}

	// Compute the discrete probability of sampling this light
	lights := scene.GetLights()
	if len(lights) == 0 {
		return 0
	}
	pdfChoice := 1.0 / float64(len(lights)) // Uniform light selection

	// Get position PDF from the light's EmissionPDF
	// This is equivalent to PBRT's light->Pdf_Le(..., &pdfPos, &pdfDir)
	pdfPos := lightVertex.Light.EmissionPDF(lightVertex.Point, w)

	return pdfPos * pdfChoice
}

// evaluateBDPTStrategies evaluates all BDPT path construction strategies with MIS weighting.
//
// BDPT works by generating two subpaths:
// - Camera subpath: starts from camera, bounces through scene
// - Light subpath: starts from light sources, bounces through scene
//
// These can be connected in multiple ways to form complete light transport paths:
// - (s=0, t=n): Pure path tracing - camera path only
// - (s=1, t=n-1): Direct lighting - connect camera path to light
// - (s=2, t=n-2): One-bounce indirect - light bounces once before connecting
// - etc.
//
// Multiple Importance Sampling (MIS) optimally combines all strategies using
// the power heuristic to minimize variance.
func (bdpt *BDPTIntegrator) evaluateBDPTStrategies(strategies []bdptStrategy) (core.Vec3, []core.SplatRay) {
	// Apply MIS weighting to all strategies
	totalContribution := core.Vec3{X: 0, Y: 0, Z: 0}
	var allSplatRays []core.SplatRay

	for _, strategy := range strategies {
		if strategy.t > 1 { // t=1 strategies hit the camera directly, not necessarily at the point we're calculating
			// Use PBRT MIS weight calculation
			bdpt.logf(" (s=%d,t=%d) evaluateBDPTStrategies: contribution=%v, PBRT weight=%0.3g\n", strategy.s, strategy.t, strategy.contribution, strategy.misWeight)
			totalContribution = totalContribution.Add(strategy.contribution.Multiply(strategy.misWeight))
		} else if strategy.t == 1 && len(strategy.splatRays) > 0 {
			// t=1 strategies contribute via splats
			for _, splatRay := range strategy.splatRays {
				// Apply MIS weighting to splat contribution
				weightedSplat := core.SplatRay{
					Ray:   splatRay.Ray,
					Color: splatRay.Color.Multiply(strategy.misWeight),
				}
				allSplatRays = append(allSplatRays, weightedSplat)
			}
		}
	}

	return totalContribution, allSplatRays
}

// generateBDPTStrategies generates all valid BDPT strategies for the given camera and light paths
func (bdpt *BDPTIntegrator) generateBDPTStrategies(cameraPath, lightPath Path, scene core.Scene, random *rand.Rand) []bdptStrategy {
	strategies := make([]bdptStrategy, 0)

	for s := 0; s <= lightPath.Length; s++ {
		for t := 1; t <= cameraPath.Length; t++ {
			var contribution core.Vec3
			var sampledVertex *Vertex

			if s == 0 {
				// s=0: Pure camera path
				contribution = bdpt.evaluatePathTracingStrategy(cameraPath, t)
				if contribution.Luminance() > 0 {
					bdpt.logf(" (s=%d,t=%d) evaluatePathTracingStrategy returned contribution=%0.3g\n", s, t, contribution)
				}
			} else if t == 1 {
				// t=1 is light path direct to camera, which might hit a different pixel
				// skip it for now
				continue
			} else if s == 1 {
				// s=1: Direct lighting
				// Use direct light sampling to avoid challenges with choosing a light point on the wrong side of the light
				contribution, sampledVertex = bdpt.evaluateDirectLightingStrategy(cameraPath, s, t, scene, random)
				bdpt.logf(" (s=%d,t=%d) evaluateDirectLightingStrategy returned contribution=%0.3g\n", s, t, contribution)
			} else {
				// All other cases: Connection strategies (including s=0, t<last)
				contribution = bdpt.evaluateConnectionStrategy(cameraPath, lightPath, s, t, scene)
			}

			if contribution.Luminance() > 0 {
				misWeight := bdpt.calculateMISWeight(cameraPath, lightPath, sampledVertex, s, t, scene)

				strategies = append(strategies, bdptStrategy{
					s:            s,
					t:            t,
					contribution: contribution,
					misWeight:    misWeight,
				})
			}
		}
	}

	return strategies
}

// evaluatePathTracingStrategy evaluates the BDPT path tracing strategy
// This is the camera-only path that accumulates radiance from surface emission and background
func (bdpt *BDPTIntegrator) evaluatePathTracingStrategy(cameraPath Path, t int) core.Vec3 {
	// Only evaluate for the complete cameraPath
	if t == 0 || t < cameraPath.Length {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// For s=0,t=n strategy, we evaluate the camera path's accumulated radiance
	// The last vertex in the camera path determines the final contribution
	lastVertex := cameraPath.Vertices[t-1]

	// The contribution is the emitted light at the last vertex weighted by path throughput
	// pbrt: L = pt.Le(scene, cameraVertices[t - 2]) * pt.beta
	contribution := lastVertex.EmittedLight.MultiplyVec(lastVertex.Beta)
	bdpt.logf(" (s=0,t=%d) evaluatePathTracingStrategy: contribution:%v = lastVertex.EmittedLight:%v * lastVertex.Beta:%v\n", t, contribution, lastVertex.EmittedLight, lastVertex.Beta)
	return contribution
}

func (bdpt *BDPTIntegrator) evaluateDirectLightingStrategy(cameraPath Path, s, t int, scene core.Scene, random *rand.Rand) (core.Vec3, *Vertex) {
	if s != 1 {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil // Only s=1 is valid for direct lighting
	}

	cameraVertex := cameraPath.Vertices[t-1]

	if cameraVertex.IsSpecular || cameraVertex.Material == nil {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	lights := scene.GetLights()
	lightSample, sampledLight, hasLight := core.SampleLight(lights, cameraVertex.Point, random)
	if !hasLight || lightSample.Emission.Luminance() <= 0 || lightSample.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Check if light is visible (shadow ray)
	shadowRay := core.NewRay(cameraVertex.Point, lightSample.Direction)
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, lightSample.Distance-0.001)
	if blocked {
		// Light is blocked, no direct contribution
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Calculate the cosine factor
	cosine := lightSample.Direction.Dot(cameraVertex.Normal)
	if cosine <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil // Light is behind the surface
	}

	// pbrt: L = pt.beta * pt.f(sampled, TransportMode::Radiance) * sampled.beta
	// pbrt sampled.beta: light->Sample_Li / (Sample_Li&pdf * lightDistr&lightPdf)
	// pbrt pdfFwd: sampled.PdfLightOrigin(scene, pt, lightDistr, lightToIndex)
	//          => pdfPos * pdfChoice // Return solid angle density for non-infinite light sources
	//          => pdfDir for light sample not used
	brdf := cameraVertex.Material.EvaluateBRDF(core.Vec3{}, lightSample.Direction, cameraVertex.Normal)
	lightBeta := lightSample.Emission.Multiply(1 / lightSample.PDF) // light sample pdf contains light selection pdf
	lightContribution := brdf.MultiplyVec(cameraVertex.Beta).MultiplyVec(lightBeta).Multiply(cosine)

	// Create sampled light vertex for PBRT MIS calculation
	sampledVertex := &Vertex{
		Point:             lightSample.Point,
		Normal:            lightSample.Normal,
		Light:             sampledLight,
		Material:          nil, // Lights don't have materials
		Camera:            nil,
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0},
		AreaPdfForward:    lightSample.PDF, // Light sampling PDF
		AreaPdfReverse:    0.0,
		IsLight:           true,
		IsCamera:          false,
		IsSpecular:        false,
		Beta:              lightBeta,
		EmittedLight:      lightSample.Emission,
	}

	bdpt.logf(" (s=%d,t=%d) evaluateDirectLightingStrategy: brdf=%v * beta=%v * emission=%v * (cosine=%f / pdf=%f)\n", s, t, brdf, cameraVertex.Beta, lightSample.Emission, cosine, lightSample.PDF)

	return lightContribution, sampledVertex
}

// evaluateLightTracingStrategy evaluates light tracing (light path hits camera)
func (bdpt *BDPTIntegrator) evaluateLightTracingStrategy(lightPath Path) core.Vec3 {
	// For now, return zero since we don't implement camera sampling from light paths
	// Full implementation would trace light path and check if it hits the camera
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// evaluateBRDF evaluates the BRDF at a vertex for a given outgoing direction
func (bdpt *BDPTIntegrator) evaluateBRDF(vertex Vertex, outgoingDirection core.Vec3) core.Vec3 {
	// For light sources, we don't evaluate BRDF - they emit directly
	if vertex.IsLight && vertex.Material == nil {
		// Light sources contribute their emission directly, not through BRDF
		// For connections, we use identity (1.0) since the light emission is handled separately
		return core.Vec3{X: 1, Y: 1, Z: 1}
	}

	if vertex.Material == nil {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Use the new EvaluateBRDF method from the material interface
	return vertex.Material.EvaluateBRDF(vertex.IncomingDirection, outgoingDirection, vertex.Normal)
}

// evaluateConnection computes the contribution from connecting two specific vertices.
//
// This implements the BDPT connection formula:
// L = f_camera(x) * G(x,y) * f_light(y) * T_camera * T_light
//
// Where:
// - f_camera(x): BRDF at camera vertex for connection direction
// - f_light(y): BRDF at light vertex for connection direction
// - G(x,y): geometric term = cos(θx) * cos(θy) / distance²
// - T_camera: accumulated throughput along camera subpath
// - T_light: accumulated throughput along light subpath
//
// The connection is only valid if both vertices are non-specular and
// there is an unoccluded line of sight between them.
func (bdpt *BDPTIntegrator) evaluateConnectionStrategy(cameraPath, lightPath Path, s, t int, scene core.Scene) core.Vec3 {
	if s < 1 || t < 1 || s > lightPath.Length || t > cameraPath.Length {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Get the vertices to connect. s and t are 0-based indices, so we need to subtract 1
	lightVertex := lightPath.Vertices[s-1]   // s-1 vertex in light path
	cameraVertex := cameraPath.Vertices[t-1] // t-1 vertex in camera path

	// Skip connections involving specular vertices (can't connect through delta functions)
	if lightVertex.IsSpecular || cameraVertex.IsSpecular {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Calculate direction from camera vertex to light vertex
	direction := lightVertex.Point.Subtract(cameraVertex.Point)
	distance := direction.Length()
	if distance < 0.001 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}
	direction = direction.Multiply(1.0 / distance)

	// Visibility test
	shadowRay := core.NewRay(cameraVertex.Point, direction)
	hit, blocked := scene.GetBVH().Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
		bdpt.logf(" (s=%d,t=%d) evaluateConnectionStrategy: blocked hit=%v\n", s, t, hit)
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Geometric term: G(x,y) = cos(theta_x) * cos(theta_y) / distance^2
	cosAtCamera := direction.Dot(cameraVertex.Normal)
	cosAtLight := direction.Multiply(-1).Dot(lightVertex.Normal)
	if cosAtCamera <= 0 || cosAtLight <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}
	geometricTerm := (cosAtCamera * cosAtLight) / (distance * distance)

	// Evaluate BRDF at camera vertex
	cameraBRDF := bdpt.evaluateBRDF(cameraVertex, direction)

	// Calculate path throughputs up to the connection vertices (not including them)
	// For connection, we need throughput up to but not including the connection vertex
	cameraPathThroughput := cameraPath.Vertices[t-1].Beta
	lightPathThroughput := lightPath.Vertices[s-1].Beta

	// Calculate light vertex BRDF for connection (PBRT: qs.f(pt, TransportMode::Importance))
	var lightBRDF core.Vec3
	if lightVertex.IsLight {
		// Light sources have identity BRDF (emission is already in throughput)
		lightBRDF = core.NewVec3(1, 1, 1)
	} else {
		// Surface vertex BRDF toward camera vertex
		lightBRDF = bdpt.evaluateBRDF(lightVertex, direction.Multiply(-1))
	}

	// PBRT formula: L = qs.beta * qs.f(pt, TransportMode::Importance) * pt.f(qs, TransportMode::Radiance) * pt.beta * G
	// Which translates to: lightThroughput * lightBRDF * cameraBRDF * cameraThroughput * G
	bdpt.logf(" (s=%d,t=%d) evaluateConnectionStrategy: cameraBRDF=%v * lightBRDF=%v * G=%v * cameraThroughput=%v * lightThroughput=%v\n", s, t, cameraBRDF, lightBRDF, geometricTerm, cameraPathThroughput, lightPathThroughput)
	contribution := lightPathThroughput.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraPathThroughput).Multiply(geometricTerm)

	return contribution
}

// getWorldRadius calculates the radius of the scene's bounding sphere
// This is used for infinite light PDF calculations following PBRT
func (bdpt *BDPTIntegrator) getWorldRadius(scene core.Scene) float64 {
	bb := scene.GetBVH().Root.BoundingBox

	// For now, use a simple heuristic based on scene shapes
	// In a full implementation, this would use the actual scene bounds
	shapes := scene.GetShapes()
	if len(shapes) == 0 {
		return 1000.0 // Default radius for empty scenes
	}

	// Calculate center and radius of bounding sphere
	center := bb.Min.Add(bb.Max).Multiply(0.5)
	radius := bb.Max.Subtract(center).Length()

	// Ensure minimum radius for numerical stability
	if radius < 100.0 {
		radius = 100.0
	}

	return radius
}

func (bdpt *BDPTIntegrator) logf(format string, a ...interface{}) {
	if bdpt.Verbose {
		fmt.Printf(format, a...)
	}
}
