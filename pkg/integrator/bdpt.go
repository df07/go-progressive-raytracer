package integrator

import (
	"fmt"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// Vertex represents a single vertex in a light transport path
type Vertex struct {
	Point      core.Vec3     // 3D position
	Normal     core.Vec3     // Surface normal
	Light      core.Light    // Light at this vertex (TODO: remove after cleanup)
	LightIndex int           // Index of light in scene's light array (-1 if not a light vertex)
	Material   core.Material // Material at this vertex

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
func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene *scene.Scene, sampler core.Sampler) (core.Vec3, []SplatRay) {

	// Generate random camera and light paths
	cameraPath := bdpt.generateCameraPath(ray, scene, sampler, bdpt.config.MaxDepth)
	lightPath := bdpt.generateLightPath(scene, sampler, bdpt.config.MaxDepth)

	// Evaluate all combinations of camera and light paths with MIS weighting
	var totalLight core.Vec3
	var totalSplats []SplatRay

	for s := 0; s <= lightPath.Length; s++ { // s is the number of vertices from the light path
		for t := 1; t <= cameraPath.Length; t++ { // t is the number of vertices from the camera path

			// evaluate BDPT strategy for s vertexes from light path and t vertexes from camera path
			light, splats, sample := bdpt.evaluateBDPTStrategy(cameraPath, lightPath, s, t, scene, sampler)

			// apply MIS weight to contribution and splat rays
			if !light.IsZero() || len(splats) > 0 {
				misWeight := bdpt.calculateMISWeight(&cameraPath, &lightPath, sample, s, t, scene)
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

// generateCameraPath generates a camera path with proper PDF tracking for BDPT
// Each vertex stores forward/reverse PDFs needed for MIS weight calculation
func (bdpt *BDPTIntegrator) generateCameraPath(ray core.Ray, scene *scene.Scene, sampler core.Sampler, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Calculate camera PDFs for proper BDPT balancing
	camera := scene.Camera
	_, directionPDF := camera.CalculateRayPDFs(ray)

	// Create the initial camera vertex (like light path does for light sources)
	cameraVertex := Vertex{
		Point:    ray.Origin,
		Normal:   ray.Direction.Multiply(-1), // Camera "normal" points back along ray
		IsCamera: true,
		Beta:     core.Vec3{X: 1, Y: 1, Z: 1},
	}

	path.Vertices = append(path.Vertices, cameraVertex)
	path.Length++

	// Continue the camera path by tracing the ray through the scene
	beta := core.Vec3{X: 1, Y: 1, Z: 1}
	// bdpt.logf("generateCameraSubpath: ray=%v, beta=%v, directionPDF=%0.6g\n", ray, beta, directionPDF)
	bdpt.extendPath(&path, ray, beta, directionPDF, scene, sampler, maxDepth, true) // Pass maxDepth through because camera doesn't count as a vertex

	return path
}

// generateLightPath generates a light path with proper PDF tracking for BDPT
// Starting from light emission, each vertex stores forward/reverse PDFs for MIS
func (bdpt *BDPTIntegrator) generateLightPath(scene *scene.Scene, sampler core.Sampler, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Sample emission from a light in the scene
	lights := scene.Lights
	if len(lights) == 0 {
		return path
	}

	lightSampler := scene.LightSampler
	sampledLight, lightSelectionPdf, lightIndex := lightSampler.SampleLightEmission(sampler.Get1D())
	emissionSample := sampledLight.SampleEmission(sampler.Get2D(), sampler.Get2D())
	cosTheta := emissionSample.Direction.AbsDot(emissionSample.Normal)

	lightVertex := Vertex{
		Point:           emissionSample.Point,
		Normal:          emissionSample.Normal,
		Light:           sampledLight, // TODO: remove after cleanup
		LightIndex:      lightIndex,
		AreaPdfForward:  emissionSample.AreaPDF * lightSelectionPdf, // probability of generating this point is the light sampling pdf
		AreaPdfReverse:  0.0,                                        // probability of generating this point in reverse is set by MIS weight calculation
		IsLight:         true,
		IsInfiniteLight: sampledLight.Type() == core.LightTypeInfinite, // Detect infinite lights for proper MIS handling
		Beta:            emissionSample.Emission,                       // PBRT: light vertex stores raw emission, transport beta used for path continuation
		EmittedLight:    emissionSample.Emission,                       // Already properly scaled
	}

	path.Vertices = append(path.Vertices, lightVertex)
	path.Length++

	// Use the common path extension logic (maxDepth-1 because light counts as a vertex in pt)
	// PBRT formula: beta = Le * |cos(theta)| / (lightSelectionPdf * areaPdf * pdfDir)
	ray := core.NewRay(emissionSample.Point, emissionSample.Direction)
	beta := emissionSample.Emission.Multiply(cosTheta / (lightSelectionPdf * emissionSample.AreaPDF * emissionSample.DirectionPDF))
	// bdpt.logf("generateLightSubpath: forwardThroughput=%v, cosTheta=%f, lightSelectionPdf=%f, AreaPDF=%f, DirectionPDF=%f\n", beta, cosTheta, lightSelectionPdf, emissionSample.AreaPDF, emissionSample.DirectionPDF)
	bdpt.extendPath(&path, ray, beta, emissionSample.DirectionPDF, scene, sampler, maxDepth-1, false)

	// PBRT: Correct subpath sampling densities for infinite area lights
	if path.Vertices[0].IsInfiniteLight {
		// Set spatial density of path[1] for infinite area light (first bounce after infinite light)
		if path.Length > 1 {
			firstBounceVertex := &path.Vertices[1]
			firstBounceVertex.AreaPdfForward = emissionSample.AreaPDF * lightSelectionPdf

			// Apply cosine term if the vertex is on a surface
			if firstBounceVertex.Material != nil {
				cosineAtFirstBounce := ray.Direction.AbsDot(firstBounceVertex.Normal)
				firstBounceVertex.AreaPdfForward *= cosineAtFirstBounce
			}
		}

		// Set spatial density of path[0] for infinite area light (use directional density)
		// PBRT: Use InfiniteLightDensity to account for all infinite lights in this direction
		// Use direct lighting PDF (cosine-weighted) to match what our Sample() function does
		path.Vertices[0].AreaPdfForward = bdpt.calculateInfiniteLightDensity(emissionSample.Point, emissionSample.Normal, emissionSample.Direction, scene)
	}

	return path
}

// extendPath extends a path by tracing a ray through the scene, handling intersections and scattering
// This is the common logic shared between camera and light path generation after the initial vertex
func (bdpt *BDPTIntegrator) extendPath(path *Path, currentRay core.Ray, beta core.Vec3, pdfFwd float64, scene *scene.Scene, sampler core.Sampler, maxBounces int, isCameraPath bool) {
	for bounces := 0; bounces < maxBounces; bounces++ {
		vertexPrevIndex := path.Length - 1
		vertexPrev := &path.Vertices[vertexPrevIndex] // Still need copy for calculations

		// Check for intersections
		hit, isHit := scene.BVH.Hit(currentRay, 0.001, math.Inf(1))
		if !isHit {
			if isCameraPath {
				// Hit background - check for infinite light emission
				lights := scene.Lights
				var totalEmission core.Vec3
				for _, light := range lights {
					// Only check infinite lights when we miss all geometry
					if light.Type() == core.LightTypeInfinite {
						emission := light.Emit(currentRay)
						totalEmission = totalEmission.Add(emission)
					}
				}

				vertex := createBackgroundVertex(currentRay, totalEmission, beta, pdfFwd)
				path.Vertices = append(path.Vertices, *vertex)
				path.Length++
			}
			break
		}

		// Create vertex for the intersection
		vertex := Vertex{
			Point:             hit.Point,
			Normal:            hit.Normal,
			Material:          hit.Material,
			IncomingDirection: currentRay.Direction.Multiply(-1),
			Beta:              beta,
		}

		// Capture emitted light from this vertex
		vertex.EmittedLight = bdpt.GetEmittedLight(currentRay, hit)
		vertex.IsLight = !vertex.EmittedLight.IsZero()

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

// evaluateBDPTStrategy evaluates a single BDPT strategy
func (bdpt *BDPTIntegrator) evaluateBDPTStrategy(cameraPath, lightPath Path, s, t int, scene *scene.Scene, sampler core.Sampler) (core.Vec3, []SplatRay, *Vertex) {
	var light core.Vec3
	var sample *Vertex         // needed for MIS weight calculation for strategies that sample a new vertex
	var splats []SplatRay // returned by light tracing strategy

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

func (bdpt *BDPTIntegrator) evaluateDirectLightingStrategy(cameraPath Path, t int, scene *scene.Scene, sampler core.Sampler) (core.Vec3, *Vertex) {
	cameraVertex := &cameraPath.Vertices[t-1]

	if cameraVertex.IsSpecular || cameraVertex.Material == nil {
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	lightSample, sampledLight, hasLight := core.SampleLight(scene.Lights, scene.LightSampler, cameraVertex.Point, cameraVertex.Normal, sampler)
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
	_, blocked := scene.BVH.Hit(shadowRay, 0.001, lightSample.Distance-0.001)
	if blocked {
		// Light is blocked, no direct contribution
		return core.Vec3{X: 0, Y: 0, Z: 0}, nil
	}

	// Create sampled light vertex for PBRT MIS calculation
	sampledVertex := &Vertex{
		Point:           lightSample.Point,
		Normal:          lightSample.Normal,
		Light:           sampledLight,    // TODO: remove after cleanup
		LightIndex:      -1,              // TODO: get actual light index from geometry.SampleLight
		AreaPdfForward:  lightSample.PDF, // probability of generating this point is the light sampling pdf
		AreaPdfReverse:  0.0,             // probability of generating this point in reverse is set by MIS weight calculation
		IsLight:         true,
		IsInfiniteLight: sampledLight.Type() == core.LightTypeInfinite, // Properly mark infinite lights
		Beta:            lightBeta,
		EmittedLight:    lightSample.Emission,
	}

	//bdpt.logf(" (s=1,t=%d) evaluateDirectLightingStrategy: L=%v => brdf=%v * beta=%v * emission=%v * (cosTheta=%f / pdf=%f)\n", t, lightContribution, brdf, cameraVertex.Beta, lightSample.Emission, cosTheta, lightSample.PDF)

	return lightContribution, sampledVertex
}

// evaluateLightTracingStrategy evaluates light tracing (light path hits camera)
// Returns (direct contribution, splat rays, sampled camera vertex)
func (bdpt *BDPTIntegrator) evaluateLightTracingStrategy(lightPath Path, s int, scene *scene.Scene, sampler core.Sampler) ([]SplatRay, *Vertex) {
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
	camera := scene.Camera
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
	_, blocked := scene.BVH.Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
		return nil, nil
	}

	// NOTE: PBRT includes a scaling factor for crop windows here (see https://github.com/mmp/pbrt-v4/issues/347)
	// This compensates for splats from rays outside the crop window. We can ignore this because we don't implement crop windows.

	// Create the sampled camera vertex for MIS weight calculation
	// This represents the dynamically sampled camera vertex
	sampledCameraVertex := &Vertex{
		Point:    cameraSample.Ray.Origin,
		Normal:   cameraSample.Ray.Direction.Multiply(-1), // Camera "normal" points back along ray
		IsCamera: true,
		Beta:     cameraBeta, // Wi / pdf from camera sampling
	}

	// Create splat ray for this contribution
	splatRay := SplatRay{
		Ray:   cameraSample.Ray,
		Color: lightContribution,
	}

	// bdpt.logf(" (s=%d,t=1) evaluateLightTracingStrategy: L=%v => brdf=%v * cameraBeta=%v * lightVertex.Beta=%v * cosine=%f\n", s, lightContribution, brdf, cameraBeta, lightVertex.Beta, cosine)

	return []SplatRay{splatRay}, sampledCameraVertex
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
func (bdpt *BDPTIntegrator) evaluateConnectionStrategy(cameraPath, lightPath Path, s, t int, scene *scene.Scene) core.Vec3 {
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
	_, blocked := scene.BVH.Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
		// bdpt.logf(" (s=%d,t=%d) evaluateConnectionStrategy: blocked hit=%v\n", s, t, hit)
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	return contribution
}

// evaluateBRDF evaluates the BRDF at a vertex for a given outgoing direction
func (bdpt *BDPTIntegrator) evaluateBRDF(vertex *Vertex, outgoingDirection core.Vec3) core.Vec3 {
	// Infinite lights don't scatter - they only emit (no BRDF)
	if vertex.IsInfiniteLight {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// For other light sources, we don't evaluate BRDF - they emit directly
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

func createBackgroundVertex(ray core.Ray, bgColor core.Vec3, beta core.Vec3, pdfFwd float64) *Vertex {
	// For background (infinite area light), we should use solid angle PDF directly
	// Don't convert to area PDF since background is at infinite distance
	return &Vertex{
		Point:             ray.Origin.Add(ray.Direction.Multiply(1000.0)), // Far background
		Normal:            ray.Direction.Multiply(-1),                     // Reverse direction
		IncomingDirection: ray.Direction.Multiply(-1),
		AreaPdfForward:    pdfFwd,            // Keep as solid angle PDF for infinite area light
		AreaPdfReverse:    0.0,               // Cannot generate rays towards background
		IsLight:           !bgColor.IsZero(), // Only mark as light if background actually emits
		IsInfiniteLight:   true,              // Mark as infinite area light
		Beta:              beta,
		EmittedLight:      bgColor, // Capture background light
	}
}

func (bdpt *BDPTIntegrator) logf(format string, a ...interface{}) {
	if bdpt.Verbose {
		fmt.Printf(format, a...)
	}
}
