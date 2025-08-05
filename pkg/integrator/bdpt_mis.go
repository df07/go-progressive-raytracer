package integrator

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// calculateMISWeight implements PBRT's MIS weighting for BDPT strategies
// This compares forward vs reverse PDFs to properly weight different path construction strategies
func (bdpt *BDPTIntegrator) calculateMISWeight(cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene core.Scene) float64 {
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

// calculateMISWeightAlt3 implements zero-allocation MIS weighting using on-demand PDF calculation
// Directly copies calculateMISWeightAlt2 logic but eliminates intermediate arrays
func (bdpt *BDPTIntegrator) calculateMISWeightAlt3(cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene core.Scene) float64 {
	// Handle same early returns as original
	if s+t == 2 {
		return 1.0
	}

	// For path tracing strategies that hit infinite lights, return weight 1.0
	if s == 0 && t > 1 {
		lastVertex := &cameraPath.Vertices[t-1]
		if lastVertex.IsInfiniteLight {
			return 1.0
		}
	}

	sumRi := 0.0

	// Camera path alternatives: start from connection vertex and work backward
	ri := 1.0
	for i := t - 1; i > 0; i-- { // t-1 down to 1
		forwardPdf, reversePdf, isConnectible := bdpt.calculateMISCameraVertexPdfs(i, cameraPath, lightPath, sampledVertex, s, t, scene)
		ri *= remap0(reversePdf) / remap0(forwardPdf)

		// isConnectible now includes both vertex and predecessor connectibility
		if isConnectible {
			sumRi += ri
		}
	}

	// Light path alternatives: start from connection vertex and work backward
	ri = 1.0
	for i := s - 1; i >= 0; i-- { // s-1 down to 0
		forwardPdf, reversePdf, isConnectible := bdpt.calculateMISLightVertexPdfs(i, cameraPath, lightPath, sampledVertex, s, t, scene)
		ri *= remap0(reversePdf) / remap0(forwardPdf)

		// isConnectible now includes both vertex and predecessor connectibility
		if isConnectible {
			sumRi += ri
		}
	}

	return 1.0 / (1.0 + sumRi)
}

// calculateMISCameraVertexPdfs returns PDF values for a camera path vertex at index i
// Returns (forwardPdf, reversePdf, isConnectible) where isConnectible includes predecessor checks
func (bdpt *BDPTIntegrator) calculateMISCameraVertexPdfs(cameraIdx int, cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene core.Scene) (float64, float64, bool) {
	// Camera path vertex at index i
	vertex := &cameraPath.Vertices[cameraIdx]
	forwardPdf := vertex.AreaPdfForward
	reversePdf := vertex.AreaPdfReverse
	isConnectible := !vertex.IsSpecular

	// Check predecessor connectibility for camera path
	if cameraIdx > 0 {
		isConnectible = isConnectible && !cameraPath.Vertices[cameraIdx-1].IsSpecular
	}

	// Apply strategy-specific PDF corrections for camera path vertices
	switch {
	case s == 0:
		// Path tracing strategy
		if cameraIdx == t-1 && t > 1 {
			// Vertex t-1 should be a light, calculate reverse pdf from light origin
			reversePdf = bdpt.calculateLightOriginPdf(&cameraPath.Vertices[t-1], &cameraPath.Vertices[t-2], scene)
			isConnectible = true
		} else if cameraIdx == t-2 && t > 2 {
			reversePdf = bdpt.calculateVertexPdf(&cameraPath.Vertices[t-1], nil, &cameraPath.Vertices[t-2], scene)
		}

	case t == 1:
		// Light tracing strategy: camera path has only camera vertex
		if cameraIdx == 0 {
			// Camera vertex gets sampled vertex PDFs
			forwardPdf = sampledVertex.AreaPdfForward
			reversePdf = sampledVertex.AreaPdfReverse
			isConnectible = true
		}

	case s == 1:
		// Direct lighting strategy
		if cameraIdx == t-1 && t > 0 {
			// Camera vertex reverse PDF from sampled light
			reversePdf = bdpt.calculateVertexPdf(sampledVertex, nil, &cameraPath.Vertices[t-1], scene)
			isConnectible = true
		} else if cameraIdx == t-2 && t > 1 {
			// Camera predecessor reverse PDF
			reversePdf = bdpt.calculateVertexPdf(&cameraPath.Vertices[t-1], sampledVertex, &cameraPath.Vertices[t-2], scene)
		}

	default:
		// Connection strategy
		if cameraIdx == t-1 {
			// Camera connection vertex
			reversePdf = bdpt.calculateVertexPdf(&lightPath.Vertices[s-1], &lightPath.Vertices[s-2], &cameraPath.Vertices[t-1], scene)
			isConnectible = true
		} else if cameraIdx == t-2 && t > 1 {
			// Camera predecessor
			reversePdf = bdpt.calculateVertexPdf(&cameraPath.Vertices[t-1], &lightPath.Vertices[s-1], &cameraPath.Vertices[t-2], scene)
		}
	}

	return forwardPdf, reversePdf, isConnectible
}

// calculateMISLightVertexPdfs returns PDF values for a light path vertex at lightIdx
// Returns (forwardPdf, reversePdf, isConnectible) where isConnectible includes predecessor checks
func (bdpt *BDPTIntegrator) calculateMISLightVertexPdfs(lightIdx int, cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene core.Scene) (float64, float64, bool) {
	// Light path vertex at index lightIdx
	vertex := &lightPath.Vertices[lightIdx]
	forwardPdf := vertex.AreaPdfForward
	reversePdf := vertex.AreaPdfReverse

	// Light vertices: connectible if not specular AND not delta light (point light)
	isDeltaLight := vertex.IsLight && vertex.Light != nil && vertex.Light.Type() == core.LightTypePoint
	isConnectible := !vertex.IsSpecular && !isDeltaLight

	// Check predecessor connectibility for light path
	if lightIdx > 0 {
		predecessor := &lightPath.Vertices[lightIdx-1]
		predecessorIsDeltaLight := predecessor.IsLight && predecessor.Light != nil && predecessor.Light.Type() == core.LightTypePoint
		predecessorConnectible := !predecessor.IsSpecular && !predecessorIsDeltaLight
		isConnectible = isConnectible && predecessorConnectible
	}

	// Apply strategy-specific PDF corrections for light path vertices

	switch {
	case s == 0:
		// Path tracing strategy - no light path vertices in this strategy
		// This should never be called for s=0

	case t == 1:
		// Light tracing strategy
		if lightIdx == s-1 && s > 1 {
			// Reverse PDF: from camera (sampledVertex) to this light vertex
			reversePdf = bdpt.calculateVertexPdf(sampledVertex, nil, &lightPath.Vertices[s-1], scene)
			isConnectible = true
		} else if lightIdx == s-2 && s > 1 {
			// Reverse PDF: from light connection vertex to this predecessor
			reversePdf = bdpt.calculateVertexPdf(&lightPath.Vertices[s-1], sampledVertex, &lightPath.Vertices[s-2], scene)
		}

	case s == 1:
		// Direct lighting strategy
		if lightIdx == s-1 && sampledVertex != nil {
			// Sampled light vertex
			forwardPdf = sampledVertex.AreaPdfForward
			reversePdf = bdpt.calculateVertexPdf(&cameraPath.Vertices[t-1], &cameraPath.Vertices[t-2], sampledVertex, scene)
			isConnectible = true
		}

	default:
		// Connection strategy
		if lightIdx == s-1 {
			// Light connection vertex
			reversePdf = bdpt.calculateVertexPdf(&cameraPath.Vertices[t-1], &cameraPath.Vertices[t-2], &lightPath.Vertices[s-1], scene)
			isConnectible = true
		} else if lightIdx == s-2 && s > 1 {
			// Light predecessor
			reversePdf = bdpt.calculateVertexPdf(&lightPath.Vertices[s-1], &cameraPath.Vertices[t-1], &lightPath.Vertices[s-2], scene)
		}
	}

	return forwardPdf, reversePdf, isConnectible
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
