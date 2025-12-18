package integrator

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// calculateMISWeight implements zero-allocation MIS weighting using on-demand PDF calculation
// Directly copies calculateMISWeightAlt2 logic but eliminates intermediate arrays
func (bdpt *BDPTIntegrator) calculateMISWeight(cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene *scene.Scene) float64 {
	disableMISWeight := false
	if disableMISWeight {
		//return 1.0 / float64(s+t-1)
		return 1.0 / float64(cameraPath.Length+lightPath.Length-1)
	}

	// Handle same early returns as original
	if s+t == 2 {
		return 1.0
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
		// bdpt.logf(" (s=%d,t=%d) cameraPath[%d]: fwd=%.3g, rev=%.3g, conn=%v, ri=%.3g, sumRi=%.3g\n", s, t, i, forwardPdf, reversePdf, isConnectible, ri, sumRi)
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

		// bdpt.logf(" (s=%d,t=%d) lightPath[%d]: fwd=%.3g, rev=%.3g, conn=%v, ri=%.3g, sumRi=%.3g\n", s, t, i, forwardPdf, reversePdf, isConnectible, ri, sumRi)

	}

	// bdpt.logf(" (s=%d,t=%d) calculateMISWeight: sumRi=%0.3g, weight=%0.3f\n", s, t, sumRi, 1.0/(1.0+sumRi))
	return 1.0 / (1.0 + sumRi)
}

// calculateMISCameraVertexPdfs returns PDF values for a camera path vertex at index i
// Returns (forwardPdf, reversePdf, isConnectible) where isConnectible includes predecessor checks
func (bdpt *BDPTIntegrator) calculateMISCameraVertexPdfs(cameraIdx int, cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene *scene.Scene) (float64, float64, bool) {
	// Camera path vertex at index i
	vertex := &cameraPath.Vertices[cameraIdx]
	forwardPdf := vertex.AreaPdfForward
	reversePdf := vertex.AreaPdfReverse
	isConnectible := !vertex.IsSpecular

	// Apply strategy-specific PDF corrections for camera path vertices
	switch {
	case s == 0:
		// Path tracing strategy
		if cameraIdx == t-1 && t > 1 {
			// Vertex t-1 may be a light, calculate reverse pdf from light origin if so
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

	// Apply general connectibility rules after strategy-specific corrections (mirroring PBRT)
	// Check predecessor connectibility for camera path
	if cameraIdx > 0 {
		isConnectible = isConnectible && !cameraPath.Vertices[cameraIdx-1].IsSpecular
	}

	return forwardPdf, reversePdf, isConnectible
}

// calculateMISLightVertexPdfs returns PDF values for a light path vertex at lightIdx
// Returns (forwardPdf, reversePdf, isConnectible) where isConnectible includes predecessor checks
func (bdpt *BDPTIntegrator) calculateMISLightVertexPdfs(lightIdx int, cameraPath, lightPath *Path, sampledVertex *Vertex, s, t int, scene *scene.Scene) (float64, float64, bool) {
	// Light path vertex at index lightIdx
	vertex := &lightPath.Vertices[lightIdx]
	forwardPdf := vertex.AreaPdfForward
	reversePdf := vertex.AreaPdfReverse

	// Light vertices: connectible if not specular AND not delta light (point light)
	isDeltaLight := vertex.IsLight && vertex.Light != nil && vertex.Light.Type() == lights.LightTypePoint
	isConnectible := !vertex.IsSpecular && !isDeltaLight

	// Check predecessor connectibility for light path
	if lightIdx > 0 {
		predecessor := &lightPath.Vertices[lightIdx-1]
		predecessorIsDeltaLight := predecessor.IsLight && predecessor.Light != nil && predecessor.Light.Type() == lights.LightTypePoint
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
func (bdpt *BDPTIntegrator) calculateVertexPdf(curr *Vertex, prev *Vertex, next *Vertex, scene *scene.Scene) float64 {
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
		_, pdf = scene.Camera.CalculateRayPDFs(ray)
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
func (bdpt *BDPTIntegrator) calculateLightPdf(curr *Vertex, to *Vertex, scene *scene.Scene) float64 {
	w := to.Point.Subtract(curr.Point)
	invDist2 := 1.0 / w.LengthSquared()
	w = w.Multiply(math.Sqrt(invDist2))

	var pdf float64
	if curr.IsLight {
		// Handle infinite area lights (background)
		if curr.IsInfiniteLight {
			// PBRT: Compute planar sampling density for infinite light sources
			worldRadius := scene.BVH.Radius

			// Handle zero radius case (scene with no finite geometry)
			if worldRadius == 0.0 {
				worldRadius = 1.0 // Small default radius for scenes with only infinite geometry
			}

			pdf = 1.0 / (math.Pi * worldRadius * worldRadius)
		} else if curr.Light != nil {
			// Use the light's EmissionPDF
			emissionPdf := curr.Light.EmissionPDF(curr.Point, w)

			if curr.Light.Type() == lights.LightTypePoint {
				// Point Light: EmissionPDF is directional PDF p(Dir)
				// p(z1) = p(Dir) * cosThetaAtSurface / dist^2
				// cosTheta at light is 1 (handled by Sample fix)
				// We apply cosThetaAtSurface later if to.IsOnSurface()
				pdf = emissionPdf * invDist2
			} else {
				// Area Light: Compute directional PDF directly (PBRT approach)
				// For Lambertian emission: pdfDir = cosTheta / pi
				// Formula: pdf = pdfDir * invDist2 * cosThetaAtReceiver
				cosTheta := w.Dot(curr.Normal)
				if cosTheta <= 0 {
					return 0
				}

				pdfDir := cosTheta / math.Pi
				pdf = pdfDir * invDist2
			}
		}
	}

	// if (v.IsOnSurface()) pdf *= AbsDot(v.ng(), w);
	if to.IsOnSurface() {
		pdf *= to.Normal.AbsDot(w)
	}

	return pdf
}

// calculateLightOriginPdf implements PBRT's Vertex::PdfLightOrigin
func (bdpt *BDPTIntegrator) calculateLightOriginPdf(lightVertex *Vertex, to *Vertex, scene *scene.Scene) float64 {
	w := to.Point.Subtract(lightVertex.Point)
	if w.LengthSquared() == 0 {
		return 0
	}
	w = w.Normalize()

	// Handle infinite area lights (background)
	if lightVertex.IsInfiniteLight {
		// PBRT: Return infinite light density - sum PDFs of all infinite lights in direction w
		// This accounts for multiple infinite lights and direction-specific emission
		// Use direct lighting PDF (cosine-weighted) to match what our Sample() function does
		return bdpt.calculateInfiniteLightDensity(to.Point, to.Normal, w.Multiply(-1), scene) // PBRT uses -w
	}

	if !lightVertex.IsLight || lightVertex.Light == nil {
		return 0
	}

	// Compute the discrete probability of sampling this light
	lights := scene.Lights
	if len(lights) == 0 {
		return 0
	}

	// Use the actual light selection probability from the light sampler
	lightSampler := scene.LightSampler
	pdfChoice := lightSampler.GetLightProbability(lightVertex.LightIndex, lightVertex.Point, lightVertex.Normal)

	// Get position PDF from the light's EmissionPDF
	// This is equivalent to PBRT's light->Pdf_Le(..., &pdfPos, &pdfDir)
	pdfPos := lightVertex.Light.EmissionPDF(lightVertex.Point, w)

	return pdfPos * pdfChoice
}

// calculateInfiniteLightDensity implements PBRT's InfiniteLightDensity function
// Sums the directional PDFs of all infinite lights in the given direction, weighted by selection probability
// Uses cosine-weighted hemisphere PDF for consistency with direct lighting sampling
func (bdpt *BDPTIntegrator) calculateInfiniteLightDensity(point, normal, direction core.Vec3, scene *scene.Scene) float64 {
	ls := scene.Lights
	if len(ls) == 0 {
		return 0
	}

	var totalPdf float64
	lightSampler := scene.LightSampler

	// Sum PDFs of all infinite lights in this direction
	for i, light := range ls {
		if light.Type() == lights.LightTypeInfinite {
			// Use cosine-weighted hemisphere PDF (matches light.Sample behavior)
			// This corresponds to PBRT's light.PDF_Li(Interaction(), direction)
			directionalPdf := light.PDF(point, normal, direction)
			// Get the actual light selection probability from the light sampler
			lightSelectionPdf := lightSampler.GetLightProbability(i, point, normal)
			totalPdf += directionalPdf * lightSelectionPdf
		}
	}

	return totalPdf
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

// remap0 is equivalent to PBRT's remap0 - deals with delta functions
// Returns 1.0 for zero values to avoid division by zero in MIS weight calculations
func remap0(f float64) float64 {
	if f != 0 {
		return f
	}
	return 1.0
}
