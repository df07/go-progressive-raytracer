package integrator

import (
	"fmt"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Vertex represents a single vertex in a light transport path
type Vertex struct {
	Point    core.Vec3     // 3D position
	Normal   core.Vec3     // Surface normal
	Light    core.Light    // Light at this vertex
	Material core.Material // Material at this vertex

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

// IsOnSurface returns true if this vertex is on a surface with meaningful geometry
// Matches PBRT's Vertex::IsOnSurface() which checks if geometric normal is non-zero
// For light vertices: only area lights (not point lights) are considered "on surface"
// For surface vertices: check if vertex has material (surface interaction)
func (v *Vertex) IsOnSurface() bool {
	if v.IsLight && v.Light != nil {
		// Light vertices: only area lights are "on surface", not point lights
		return v.Light.Type() == core.LightTypeArea
	}
	// Non-light vertices: check if has material (surface interaction)
	return v.Material != nil
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

// NewBDPTIntegrator creates a new BDPT integrator
func NewBDPTIntegrator(config core.SamplingConfig) *BDPTIntegrator {
	return &BDPTIntegrator{
		PathTracingIntegrator: NewPathTracingIntegrator(config),
		Verbose:               false,
	}
}

// RayColor computes color with support for ray-based splatting
// Returns (pixel color, splat rays)
func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene core.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay) {

	// Generate random camera and light paths
	cameraPath := bdpt.generateCameraSubpath(ray, scene, sampler, bdpt.config.MaxDepth)
	lightPath := bdpt.generateLightSubpath(scene, sampler, bdpt.config.MaxDepth)

	// Evaluate all combinations of camera and light paths with MIS weighting
	var totalLight core.Vec3
	var totalSplats []core.SplatRay

	for s := 0; s <= lightPath.Length; s++ { // s is the number of vertices from the light path
		for t := 1; t <= cameraPath.Length; t++ { // t is the number of vertices from the camera path

			// evaluate BDPT strategy for s vertexes from light path and t vertexes from camera path
			light, splats, sample := bdpt.evaluateBDPTStrategy(cameraPath, lightPath, s, t, scene, sampler)

			// apply MIS weight to contribution and splat rays
			if !light.IsZero() || len(splats) > 0 {
				misWeight := bdpt.calculateMISWeight(cameraPath, lightPath, sample, s, t, scene)
				totalLight = totalLight.Add(light.Multiply(misWeight))
				for i := range splats {
					splats[i].Color = splats[i].Color.Multiply(misWeight)
				}
				totalSplats = append(totalSplats, splats...)
			}
		}
	}

	return totalLight, totalSplats
}

// evaluateBDPTStrategy evaluates a single BDPT strategy
func (bdpt *BDPTIntegrator) evaluateBDPTStrategy(cameraPath, lightPath Path, s, t int, scene core.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay, *Vertex) {
	var light core.Vec3
	var sample *Vertex         // needed for MIS weight calculation for strategies that sample a new vertex
	var splats []core.SplatRay // returned by light tracing strategy

	switch {
	case s == 1 && t == 1:
		return core.Vec3{}, nil, nil // pbrt does not implement s=1,t=1 strategy. These paths are captured by s=0,t=1
	case s == 0:
		light = bdpt.evaluatePathTracingStrategy(cameraPath, t) // s=0: Pure camera path
	case t == 1:
		splats, sample = bdpt.evaluateLightTracingStrategy(lightPath, s, scene, sampler) // t=1: Light path direct to camera (light tracing)
	case s == 1:
		light, sample = bdpt.evaluateDirectLightingStrategy(cameraPath, t, scene, sampler) // s=1: Direct lighting
	default:
		light = bdpt.evaluateConnectionStrategy(cameraPath, lightPath, s, t, scene) // All other cases: Connection strategies (including s=0, t<last)
	}

	return light, splats, sample
}

// generateCameraSubpath generates a camera subpath with proper PDF tracking for BDPT
// Each vertex stores forward/reverse PDFs needed for MIS weight calculation
func (bdpt *BDPTIntegrator) generateCameraSubpath(ray core.Ray, scene core.Scene, sampler core.Sampler, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Calculate camera PDFs for proper BDPT balancing
	camera := scene.GetCamera()
	_, directionPDF := camera.CalculateRayPDFs(ray)

	// Create the initial camera vertex (like light path does for light sources)
	cameraVertex := Vertex{
		Point:             ray.Origin,
		Normal:            ray.Direction.Multiply(-1),  // Camera "normal" points back along ray
		Material:          nil,                         // Cameras don't have materials
		Light:             nil,                         // Cameras are not lights
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
	// bdpt.logf("generateCameraSubpath: ray=%v, beta=%v, directionPDF=%0.6g\n", ray, beta, directionPDF)
	bdpt.extendPath(&path, ray, beta, directionPDF, scene, sampler, maxDepth, true) // Pass maxDepth through because camera doesn't count as a vertex

	return path
}

// generateLightSubpath generates a light subpath with proper PDF tracking for BDPT
// Starting from light emission, each vertex stores forward/reverse PDFs for MIS
func (bdpt *BDPTIntegrator) generateLightSubpath(scene core.Scene, sampler core.Sampler, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Sample emission from a light in the scene
	lights := scene.GetLights()
	if len(lights) == 0 {
		return path
	}

	sampledLight := lights[int(sampler.Get1D()*float64(len(lights)))]
	emissionSample := sampledLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
	lightSelectionPdf := 1.0 / float64(len(lights))
	cosTheta := emissionSample.Direction.AbsDot(emissionSample.Normal)

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
		Beta:              emissionSample.Emission, // PBRT: light vertex stores raw emission, transport beta used for path continuation
		EmittedLight:      emissionSample.Emission, // Already properly scaled
	}

	path.Vertices = append(path.Vertices, lightVertex)
	path.Length++

	// Use the common path extension logic (maxDepth-1 because light counts as a vertex in pt)
	// PBRT formula: beta = Le * |cos(theta)| / (lightSelectionPdf * areaPdf * pdfDir)
	ray := core.NewRay(emissionSample.Point, emissionSample.Direction)
	beta := emissionSample.Emission.Multiply(cosTheta / (lightSelectionPdf * emissionSample.AreaPDF * emissionSample.DirectionPDF))
	// bdpt.logf("generateLightSubpath: forwardThroughput=%v, cosTheta=%f, lightSelectionPdf=%f, AreaPDF=%f, DirectionPDF=%f\n", beta, cosTheta, lightSelectionPdf, emissionSample.AreaPDF, emissionSample.DirectionPDF)
	bdpt.extendPath(&path, ray, beta, emissionSample.DirectionPDF, scene, sampler, maxDepth-1, false)

	return path
}

// extendPath extends a path by tracing a ray through the scene, handling intersections and scattering
// This is the common logic shared between camera and light path generation after the initial vertex
func (bdpt *BDPTIntegrator) extendPath(path *Path, currentRay core.Ray, beta core.Vec3, pdfFwd float64, scene core.Scene, sampler core.Sampler, maxBounces int, isCameraPath bool) {
	for bounces := 0; bounces < maxBounces; bounces++ {
		vertexPrevIndex := path.Length - 1
		vertexPrev := &path.Vertices[vertexPrevIndex] // Still need copy for calculations

		// Check for intersections
		hit, isHit := scene.GetBVH().Hit(currentRay, 0.001, math.Inf(1))
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
				AreaPdfForward:    pdfFwd,            // Keep as solid angle PDF for infinite area light
				AreaPdfReverse:    0.0,               // Cannot generate rays towards background
				IsLight:           !bgColor.IsZero(), // Only mark as light if background actually emits
				IsCamera:          false,             // No camera vertices in path extension
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
			IsLight:           !emittedLight.IsZero(),
			IsCamera:          false, // No camera vertices in path extension
			IsSpecular:        false, // Will be set below if material scatters
			Beta:              beta,
			EmittedLight:      emittedLight, // Captured during path generation
		}

		// Set forward PDF into this vertex, from the pdf of the previous vertex
		// pbrt: prev.ConvertDensity(pdf, v)
		vertex.AreaPdfForward = vertexPrev.convertSolidAngleToAreaPdf(&vertex, pdfFwd)

		// Try to scatter the ray
		scatter, didScatter := hit.Material.Scatter(currentRay, *hit, sampler)
		if !didScatter {
			// Material absorbed the ray - add vertex and terminate path
			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Material scattered - capture the scatter information
		pdfFwd = scatter.PDF
		vertex.IsSpecular = scatter.IsSpecular()

		// Handle specular vs diffuse materials differently (like path tracer does)
		cosTheta := scatter.Scattered.Direction.AbsDot(hit.Normal)
		if scatter.IsSpecular() {
			// For specular materials: no PDF division (deterministic reflection/refraction)
			beta = beta.MultiplyVec(scatter.Attenuation)
		} else {
			// For diffuse materials: standard Monte Carlo integration with PDF
			beta = beta.MultiplyVec(scatter.Attenuation).Multiply(cosTheta / scatter.PDF)
		}

		// pbrt: Float pdfRev = bsdf.PDF(bs->wi, wo, !mode)
		pdfRev, isReverseDelta := hit.Material.PDF(scatter.Scattered.Direction, currentRay.Direction.Multiply(-1), hit.Normal)

		// For delta functions in BDPT, set reverse PDF to 0 (like PBRT)
		if isReverseDelta {
			vertex.IsSpecular = true
			pdfRev = 0.0
			pdfFwd = 0.0
		}

		// Set reverse PDF into the previous vertex, from the pdf of the current vertex
		// pbrt: prev.pdfRev = vertex.ConvertDensity(pdfRev, prev);
		vertexPrev.AreaPdfReverse = vertex.convertSolidAngleToAreaPdf(vertexPrev, pdfRev)

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
	}
}

// convertSolidAngleToAreaPdf converts a directional PDF to an area PDF
// PBRT equivalent: Vertex::ConvertDensity
// Converts from solid angle PDF (per steradian) to area PDF (per unit area)
// Note: special case for infinite area lights (background): returns solid angle pdf
func (v *Vertex) convertSolidAngleToAreaPdf(next *Vertex, pdf float64) float64 {
	if next.IsInfiniteLight {
		return pdf
	}

	direction := next.Point.Subtract(v.Point)
	distanceSquared := direction.LengthSquared()
	if distanceSquared == 0 { // prevent division by zero
		return 0.0
	}
	invDist2 := 1.0 / distanceSquared

	// Follow PBRT's ConvertDensity exactly
	// Formula: area_pdf = solid_angle_pdf * distance² / |cos(theta)|

	// Only multiply by cosTheta if next vertex is on a surface (PBRT's IsOnSurface)
	if next.IsOnSurface() {
		cosTheta := direction.Multiply(math.Sqrt(invDist2)).AbsDot(next.Normal)
		pdf *= cosTheta
	}

	return pdf * invDist2
}

// calculateMISWeight implements PBRT's MIS weighting for BDPT strategies
// This compares forward vs reverse PDFs to properly weight different path construction strategies
func (bdpt *BDPTIntegrator) calculateMISWeight(cameraPath, lightPath Path, sampledVertex *Vertex, s, t int, scene core.Scene) float64 {
	disableMISWeight := false
	if disableMISWeight {
		return 1.0 / float64(s+t-1)
	}

	if s+t == 2 {
		// bdpt.logf(" (s=%d,t=%d) calculateMISWeight: s+t==2, weight=1.0\n", s, t)
		return 1.0
	}

	// For path tracing strategies that hit infinite lights (background),
	// return MIS weight 1.0 since we can't actually sample infinite lights directly
	if s == 0 && t > 1 {
		lastVertex := &cameraPath.Vertices[t-1]
		if lastVertex.IsInfiniteLight {
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight: infinite light hit, weight=1.0\n", s, t)
			return 1.0
		}
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
			pt.AreaPdfReverse = bdpt.calculateVertexPdf(qs, qsMinus, pt, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 1 remap pt: pt.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtPdfRev, pt.AreaPdfReverse)
		} else {
			// pt.AreaPdfReverse = pt.PdfLightOrigin(scene, *ptMinus, lightPdf, lightToIndex)
			pt.AreaPdfReverse = bdpt.calculateLightOriginPdf(pt, ptMinus, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 2 remap pt: pt.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtPdfRev, pt.AreaPdfReverse)
		}
	}

	// Update reverse density of vertex pt_{t-2}
	if ptMinus != nil {
		originalPtMinusPdfRev = ptMinus.AreaPdfReverse
		if s > 0 {
			// ptMinus.AreaPdfReverse = pt.Pdf(scene, qs, *ptMinus)
			ptMinus.AreaPdfReverse = bdpt.calculateVertexPdf(pt, qs, ptMinus, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 1 remap ptMinus: ptMinus.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtMinusPdfRev, ptMinus.AreaPdfReverse)
		} else {
			// ptMinus.AreaPdfReverse = pt.PdfLight(scene, *ptMinus)
			ptMinus.AreaPdfReverse = bdpt.calculateLightPdf(pt, ptMinus, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 2 remap ptMinus: ptMinus.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalPtMinusPdfRev, ptMinus.AreaPdfReverse)
		}
	}

	// Update reverse density of vertices qs_{s-1} and qs_{s-2}
	if qs != nil {
		originalQsPdfRev = qs.AreaPdfReverse
		if pt != nil {
			// qs.AreaPdfReverse = pt.Pdf(scene, ptMinus, *qs)
			qs.AreaPdfReverse = bdpt.calculateVertexPdf(pt, ptMinus, qs, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 3 remap qs: qs.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalQsPdfRev, qs.AreaPdfReverse)
		}
	}
	if qsMinus != nil {
		originalQsMinusPdfRev = qsMinus.AreaPdfReverse
		if qs != nil && pt != nil {
			// qsMinus.AreaPdfReverse = qs.Pdf(scene, pt, *qsMinus)
			qsMinus.AreaPdfReverse = bdpt.calculateVertexPdf(qs, pt, qsMinus, scene)
			// bdpt.logf(" (s=%d,t=%d) calculateMISWeight 4 remap qsMinus: qsMinus.AreaPdfReverse=%0.3g -> %0.3g\n", s, t, originalQsMinusPdfRev, qsMinus.AreaPdfReverse)
		}
	}

	// Consider hypothetical connection strategies along the camera subpath
	ri := 1.0
	for i := t - 1; i > 0; i-- {
		vertex := &cameraPath.Vertices[i]
		ri *= remap0(vertex.AreaPdfReverse) / remap0(vertex.AreaPdfForward)

		// Only add to sumRi if no specular vertex follows (meaning connection is viable)
		if !vertex.IsSpecular && !cameraPath.Vertices[i-1].IsSpecular {
			sumRi += ri
		}
		// bdpt.logf(" (s=%d,t=%d) calculateMISWeight cameraPath[%d]: pdfFwd=%.3g, pdfRev=%.3g, ri=%.3g, sumRi=%.3g\n", s, t, i, remap0(vertex.AreaPdfForward), remap0(vertex.AreaPdfReverse), ri, sumRi)
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
			deltaLightVertex = vertex.IsLight && vertex.Light.Type() == core.LightTypePoint
		}

		if !vertex.IsSpecular && !deltaLightVertex {
			sumRi += ri
		}
		// bdpt.logf(" (s=%d,t=%d) calculateMISWeight lightPath[%d]: pdfFwd=%.3g, pdfRev=%.3g, ri=%.3g, sumRi=%.3g\n", s, t, i, remap0(vertex.AreaPdfForward), remap0(vertex.AreaPdfReverse), ri, sumRi)
	}

	// bdpt.logf(" (s=%d,t=%d) calculateMISWeight: sumRi=%0.3g, weight=%0.3f\n", s, t, sumRi, 1.0/(1.0+sumRi))
	return 1.0 / (1.0 + sumRi)
}

// Helper functions for PBRT MIS calculations

// calculateVertexPdf implements PBRT's Vertex::Pdf
func (bdpt *BDPTIntegrator) calculateVertexPdf(curr *Vertex, prev *Vertex, next *Vertex, scene core.Scene) float64 {
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
		_, pdf = scene.GetCamera().CalculateRayPDFs(ray)
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
	return curr.convertSolidAngleToAreaPdf(next, pdf)
}

// calculateLightPdf implements PBRT's Vertex::PdfLight
func (bdpt *BDPTIntegrator) calculateLightPdf(curr *Vertex, to *Vertex, scene core.Scene) float64 {
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
			//fmt.Printf("infinite light pdf: %f\n", pdf)
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
		cosTheta := to.Normal.AbsDot(w)
		pdf *= cosTheta
	}

	return pdf
}

// calculateLightOriginPdf implements PBRT's Vertex::PdfLightOrigin
func (bdpt *BDPTIntegrator) calculateLightOriginPdf(lightVertex *Vertex, to *Vertex, scene core.Scene) float64 {
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

		// bdpt.logf(" (s=?,t=?) calculateLightOriginPdf: infinite light, infiniteLightPdf=%0.3g, lightSelectionPdf=%0.3g\n", infiniteLightPdf, lightSelectionPdf)
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

// evaluatePathTracingStrategy evaluates the BDPT path tracing strategy
// This is the camera-only path that accumulates radiance from surface emission and background
func (bdpt *BDPTIntegrator) evaluatePathTracingStrategy(cameraPath Path, t int) core.Vec3 {
	// Only evaluate for the complete cameraPath
	if t == 0 || t < cameraPath.Length {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// For s=0,t=n strategy, we evaluate the camera path's accumulated radiance
	// The last vertex in the camera path determines the final contribution
	lastVertex := &cameraPath.Vertices[t-1]

	// The contribution is the emitted light at the last vertex weighted by path throughput
	// pbrt: L = pt.Le(scene, cameraVertices[t - 2]) * pt.beta
	contribution := lastVertex.EmittedLight.MultiplyVec(lastVertex.Beta)
	// bdpt.logf(" (s=0,t=%d) evaluatePathTracingStrategy: contribution:%v = lastVertex.EmittedLight:%v * lastVertex.Beta:%v\n", t, contribution, lastVertex.EmittedLight, lastVertex.Beta)
	return contribution
}

func (bdpt *BDPTIntegrator) evaluateDirectLightingStrategy(cameraPath Path, t int, scene core.Scene, sampler core.Sampler) (core.Vec3, *Vertex) {
	cameraVertex := &cameraPath.Vertices[t-1]

	if cameraVertex.IsSpecular || cameraVertex.Material == nil {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	lights := scene.GetLights()
	lightSample, sampledLight, hasLight := core.SampleLight(lights, cameraVertex.Point, sampler)
	if !hasLight || lightSample.Emission.IsZero() || lightSample.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Calculate the cosTheta factor
	cosTheta := lightSample.Direction.AbsDot(cameraVertex.Normal)
	if cosTheta <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil // Light is behind the surface
	}

	// pbrt: L = pt.beta * pt.f(sampled, TransportMode::Radiance) * sampled.beta
	// pbrt sampled.beta: light->Sample_Li / (Sample_Li &pdf * lightDistr &lightPdf)
	// pbrt pdfFwd: sampled.PdfLightOrigin(scene, pt, lightDistr, lightToIndex)
	//          => pdfPos * pdfChoice // Return solid angle density for non-infinite light sources
	//          => pdfDir for light sample not used
	brdf := cameraVertex.Material.EvaluateBRDF(cameraVertex.IncomingDirection, lightSample.Direction, cameraVertex.Normal)
	lightBeta := lightSample.Emission.Multiply(1 / lightSample.PDF) // light sample pdf contains light selection pdf
	lightContribution := brdf.MultiplyVec(cameraVertex.Beta).MultiplyVec(lightBeta).Multiply(cosTheta)

	if lightContribution.IsZero() {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Check if light is visible (shadow ray)
	shadowRay := core.NewRay(cameraVertex.Point, lightSample.Direction)
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, lightSample.Distance-0.001)
	if blocked {
		// Light is blocked, no direct contribution
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Create sampled light vertex for PBRT MIS calculation
	sampledVertex := &Vertex{
		Point:             lightSample.Point,
		Normal:            lightSample.Normal,
		Light:             sampledLight,
		Material:          nil, // Lights don't have materials
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0},
		AreaPdfForward:    lightSample.PDF, // Light sampling PDF
		AreaPdfReverse:    0.0,
		IsLight:           true,
		IsCamera:          false,
		IsSpecular:        false,
		Beta:              lightBeta,
		EmittedLight:      lightSample.Emission,
	}

	// bdpt.logf(" (s=1,t=%d) evaluateDirectLightingStrategy: L=%v => brdf=%v * beta=%v * emission=%v * (cosine=%f / pdf=%f)\n", t, lightContribution, brdf, cameraVertex.Beta, lightSample.Emission, cosine, lightSample.PDF)

	return lightContribution, sampledVertex
}

// evaluateLightTracingStrategy evaluates light tracing (light path hits camera)
// Returns (direct contribution, splat rays, sampled camera vertex)
func (bdpt *BDPTIntegrator) evaluateLightTracingStrategy(lightPath Path, s int, scene core.Scene, sampler core.Sampler) ([]core.SplatRay, *Vertex) {
	if s <= 1 || s > lightPath.Length {
		return nil, nil
	}

	// Get the light path vertex we're connecting to the camera
	lightVertex := &lightPath.Vertices[s-1]

	// Skip specular vertices (can't connect through delta functions)
	if lightVertex.IsSpecular {
		return nil, nil
	}

	// Sample the camera from this light vertex
	camera := scene.GetCamera()
	cameraSample := camera.SampleCameraFromPoint(lightVertex.Point, sampler.Get2D())
	if cameraSample == nil {
		return nil, nil
	}

	// bdpt.logf(" (s=%d,t=1) SampleCameraFromPoint: Weight=%v, PDF=%f\n", s, cameraSample.Weight, cameraSample.PDF)

	// PBRT formula: L = qs.beta * qs.f(sampled, TransportMode::Importance) * sampled.beta;
	// where sampled.beta = Wi / pdf

	// NOTE: PBRT uses TransportMode::Importance vs ::Radiance to handle the mathematical difference between
	// radiance transport and importance transport in bidirectional path tracing. This matters for:
	// 1. Non-symmetric BSDFs (especially dielectric materials with refraction)
	// 2. Shading normals vs geometry normals
	// Since we use symmetric materials with geometry normals, we can ignore the transport mode distinction.
	// TODO: We do have dielectric materials with refraction - need to implement proper transport mode handling for those.

	brdf := bdpt.evaluateBRDF(lightVertex, cameraSample.Ray.Direction.Multiply(-1))
	cameraBeta := cameraSample.Weight.Multiply(1 / cameraSample.PDF)

	cosine := cameraSample.Ray.Direction.Multiply(-1).Dot(lightVertex.Normal)
	if cosine <= 0 {
		return nil, nil
	}

	lightContribution := brdf.MultiplyVec(cameraBeta).MultiplyVec(lightVertex.Beta)

	if lightVertex.IsOnSurface() {
		lightContribution = lightContribution.Multiply(cosine)
	}

	if lightContribution.IsZero() {
		// bdpt.logf(" Light contribution is <= 0: %v\n", lightContribution)
		return nil, nil
	}

	// Visibility test
	shadowRay := core.NewRay(lightVertex.Point, cameraSample.Ray.Direction.Multiply(-1))
	distance := lightVertex.Point.Subtract(cameraSample.Ray.Origin).Length()
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
		return nil, nil
	}

	// NOTE: PBRT includes a scaling factor for crop windows here (see https://github.com/mmp/pbrt-v4/issues/347)
	// This compensates for splats from rays outside the crop window. We can ignore this because we don't implement crop windows.

	// Create the sampled camera vertex for MIS weight calculation
	// This represents the dynamically sampled camera vertex
	sampledCameraVertex := &Vertex{
		Point:             cameraSample.Ray.Origin,
		Normal:            cameraSample.Ray.Direction.Multiply(-1), // Camera "normal" points back along ray
		Material:          nil,                                     // Cameras don't have materials
		Light:             nil,                                     // Cameras are not lights
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0},             // Camera is the starting point
		AreaPdfForward:    0.0,                                     // Will be computed by MIS weight calculation
		AreaPdfReverse:    0.0,                                     // Will be computed by MIS weight calculation
		IsLight:           false,
		IsCamera:          true,
		IsSpecular:        false,
		Beta:              cameraBeta,                  // Wi / pdf from camera sampling
		EmittedLight:      core.Vec3{X: 0, Y: 0, Z: 0}, // Cameras don't emit light
	}

	// Create splat ray for this contribution
	splatRay := core.SplatRay{
		Ray:   cameraSample.Ray,
		Color: lightContribution,
	}

	// bdpt.logf(" (s=%d,t=1) evaluateLightTracingStrategy: L=%v => brdf=%v * cameraBeta=%v * lightVertex.Beta=%v * cosine=%f\n", s, lightContribution, brdf, cameraBeta, lightVertex.Beta, cosine)

	return []core.SplatRay{splatRay}, sampledCameraVertex
}

// evaluateBRDF evaluates the BRDF at a vertex for a given outgoing direction
func (bdpt *BDPTIntegrator) evaluateBRDF(vertex *Vertex, outgoingDirection core.Vec3) core.Vec3 {
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
	lightVertex := &lightPath.Vertices[s-1]   // s-1 vertex in light path
	cameraVertex := &cameraPath.Vertices[t-1] // t-1 vertex in camera path

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
	lightBRDF := bdpt.evaluateBRDF(lightVertex, direction.Multiply(-1))

	// PBRT formula: L = qs.beta * qs.f(pt, TransportMode::Importance) * pt.f(qs, TransportMode::Radiance) * pt.beta * G
	// Which translates to: lightThroughput * lightBRDF * cameraBRDF * cameraThroughput * G
	contribution := lightPathThroughput.MultiplyVec(lightBRDF).MultiplyVec(cameraBRDF).MultiplyVec(cameraPathThroughput).Multiply(geometricTerm)
	// bdpt.logf(" (s=%d,t=%d) evaluateConnectionStrategy: L=%v => cameraBRDF=%v * lightBRDF=%v * G=%v * cameraThroughput=%v * lightThroughput=%v\n", s, t, contribution, cameraBRDF, lightBRDF, geometricTerm, cameraPathThroughput, lightPathThroughput)

	if contribution.IsZero() {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Visibility test
	shadowRay := core.NewRay(cameraVertex.Point, direction)
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
		// bdpt.logf(" (s=%d,t=%d) evaluateConnectionStrategy: blocked hit=%v\n", s, t, hit)
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	return contribution
}

// getWorldRadius calculates the radius of the scene's bounding sphere
// This is used for infinite light PDF calculations following PBRT
func (bdpt *BDPTIntegrator) getWorldRadius(scene core.Scene) float64 {
	bb := scene.GetBVH().Root.BoundingBox

	// For now, use a simple heuristic based on scene shapes
	// In a full implementation, this would use the actual scene bounds
	// TODO: Implement this properly
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

// remap0 is equivalent to PBRT's remap0 - deals with delta functions
// Returns 1.0 for zero values to avoid division by zero in MIS weight calculations
func remap0(f float64) float64 {
	if f != 0 {
		return f
	}
	return 1.0
}

func (bdpt *BDPTIntegrator) logf(format string, a ...interface{}) {
	if bdpt.Verbose {
		fmt.Printf(format, a...)
	}
}
