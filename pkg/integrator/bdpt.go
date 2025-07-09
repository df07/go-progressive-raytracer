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
	s, t         int       // Light path length, camera path length
	contribution core.Vec3 // Radiance contribution
	pdf          float64   // Path construction PDF
}

// NewBDPTIntegrator creates a new BDPT integrator
func NewBDPTIntegrator(config core.SamplingConfig) *BDPTIntegrator {
	return &BDPTIntegrator{
		PathTracingIntegrator: NewPathTracingIntegrator(config),
		Verbose:               false,
	}
}

// RayColor computes the color for a single ray using BDPT
func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, maxDepth int, throughput core.Vec3, sampleIndex int) core.Vec3 {
	// Generate camera path with vertices
	cameraPath := bdpt.generateCameraSubpath(ray, scene, random, maxDepth, throughput, sampleIndex)

	// Generate a light path
	lightPath := bdpt.generateLightSubpath(scene, random, maxDepth)

	// Evaluate all BDPT strategies with proper MIS weighting
	strategies := bdpt.generateBDPTStrategies(cameraPath, lightPath, scene)
	return bdpt.weightBDPTStrategies(strategies)
}

// generateCameraSubpath generates a camera subpath with proper PDF tracking for BDPT
// Each vertex stores forward/reverse PDFs needed for MIS weight calculation
func (bdpt *BDPTIntegrator) generateCameraSubpath(ray core.Ray, scene core.Scene, random *rand.Rand, maxDepth int, throughput core.Vec3, sampleIndex int) Path {
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
	bdpt.extendPath(&path, ray, beta, directionPDF, scene, random, maxDepth) // Pass maxDepth through because camera doesn't count as a vertex

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
	forwardThroughput := emissionSample.Emission.Multiply(cosTheta / emissionSample.AreaPDF * emissionSample.DirectionPDF * lightSelectionPdf)
	bdpt.extendPath(&path, currentRay, forwardThroughput, emissionSample.DirectionPDF, scene, random, maxDepth-1)

	return path
}

// extendPath extends a path by tracing a ray through the scene, handling intersections and scattering
// This is the common logic shared between camera and light path generation after the initial vertex
func (bdpt *BDPTIntegrator) extendPath(path *Path, currentRay core.Ray, beta core.Vec3, pdfDir float64, scene core.Scene, random *rand.Rand, maxBounces int) {
	for bounces := 0; bounces < maxBounces; bounces++ {
		vertexPrev := path.Vertices[path.Length-1]

		// Check for intersections
		hit, isHit := scene.GetBVH().Hit(currentRay, 0.001, 1e100)
		if !isHit {
			// Hit background - create a background vertex with captured light
			bgColor := bdpt.BackgroundGradient(currentRay, scene)

			vertex := Vertex{
				Point:             currentRay.Origin.Add(currentRay.Direction.Multiply(1000.0)), // Far background
				Normal:            currentRay.Direction.Multiply(-1),                            // Reverse direction
				Material:          nil,
				IncomingDirection: currentRay.Direction.Multiply(-1),
				AreaPdfForward:    1.0,                     // Background PDF
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

		// Set forward direction PDF into this vertex
		vertex.AreaPdfForward = vertex.convertPDFDensity(vertexPrev, pdfDir)

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
			pdfRev = 0.0
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
	direction := next.Point.Subtract(v.Point)
	distanceSquared := direction.LengthSquared()
	if distanceSquared == 0 { // prevent division by zero
		return 0.0
	}
	invDist2 := 1.0 / distanceSquared
	cosTheta := direction.Multiply(math.Sqrt(invDist2)).Dot(next.Normal)
	return pdfDir * cosTheta * invDist2
}

// calculatePathPDF calculates the PDF for a complete path construction strategy
func (bdpt *BDPTIntegrator) calculatePathPDF(cameraPath, lightPath Path, s, t int) float64 {
	pdf := 1.0

	// Camera path contribution: PDFs up to but not including connection vertex
	// For connection strategies, we exclude the final scatter PDF since it's replaced by connection PDF
	for i := 0; i < t-1 && i < cameraPath.Length; i++ {
		vertex := cameraPath.Vertices[i]
		if vertex.AreaPdfForward > 0 {
			pdf *= vertex.AreaPdfForward
		}
	}

	// Light path contribution: PDFs up to but not including connection vertex
	// For connection strategies, we exclude the final scatter PDF since it's replaced by connection PDF
	for i := 0; i < s-1 && i < lightPath.Length; i++ {
		vertex := lightPath.Vertices[i]
		if vertex.AreaPdfForward > 0 {
			pdf *= vertex.AreaPdfForward
		}
	}

	// Connection PDF: Only for actual connection strategies, not pure camera/light paths
	if s > 0 && t > 0 {
		cameraVertex := cameraPath.Vertices[t-1]
		lightVertex := lightPath.Vertices[s-1]

		// Calculate connection direction
		direction := lightVertex.Point.Subtract(cameraVertex.Point)
		distance := direction.Length()

		if distance > 0.001 { // Avoid division by zero for very close vertices
			direction = direction.Multiply(1.0 / distance)

			// Connection PDF from camera vertex: probability of scattering toward light vertex
			if cameraVertex.Material != nil {
				cameraPDF, isCameraDelta := cameraVertex.Material.PDF(cameraVertex.IncomingDirection, direction, cameraVertex.Normal)
				// Skip connections through delta (specular) vertices
				if isCameraDelta {
					return 0.0 // Cannot connect through delta functions
				}
				if cameraPDF <= 0 {
					return 0.0 // Invalid connection direction
				}
				pdf *= cameraPDF
			}

			// Connection PDF from light vertex: probability of scattering toward camera vertex
			if lightVertex.Material != nil {
				lightPDF, isLightDelta := lightVertex.Material.PDF(lightVertex.IncomingDirection, direction.Multiply(-1), lightVertex.Normal)
				// Skip connections through delta (specular) vertices
				if isLightDelta {
					return 0.0 // Cannot connect through delta functions
				}
				if lightPDF <= 0 {
					return 0.0 // Invalid connection direction
				}
				pdf *= lightPDF
			}
		}
	}

	return pdf
}

// calculateMISWeight calculates the MIS weight for a strategy against all competing strategies
func (bdpt *BDPTIntegrator) calculateMISWeight(currentStrategy bdptStrategy, allStrategies []bdptStrategy) float64 {
	if currentStrategy.pdf <= 0 {
		return 0.0
	}

	// Power heuristic with β = 2
	sumWeights := 0.0
	currentWeight := currentStrategy.pdf * currentStrategy.pdf

	for _, strategy := range allStrategies {
		if strategy.pdf > 0 {
			sumWeights += strategy.pdf * strategy.pdf
		}
	}

	if sumWeights <= 0 {
		return 0.0
	}

	bdpt.logf(" (s=%d,t=%d) calculateMISWeight: weight=%f (%f / %f)\n", currentStrategy.s, currentStrategy.t, currentWeight/sumWeights, currentWeight, sumWeights)
	return currentWeight / sumWeights
}

// weightBDPTStrategies evaluates all BDPT path construction strategies with MIS weighting.
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
func (bdpt *BDPTIntegrator) weightBDPTStrategies(strategies []bdptStrategy) core.Vec3 {
	// Apply MIS weighting to all strategies
	totalContribution := core.Vec3{X: 0, Y: 0, Z: 0}
	for _, strategy := range strategies {
		// Calculate MIS weight by comparing against all other strategies
		weight := bdpt.calculateMISWeight(strategy, strategies)
		bdpt.logf(" (s=%d,t=%d) weightBDPTStrategies: contribution=%v * weight=%f\n", strategy.s, strategy.t, strategy.contribution, weight)
		totalContribution = totalContribution.Add(strategy.contribution.Multiply(weight))
	}

	return totalContribution
}

// generateBDPTStrategies generates all valid BDPT strategies for the given camera and light paths
func (bdpt *BDPTIntegrator) generateBDPTStrategies(cameraPath, lightPath Path, scene core.Scene) []bdptStrategy {
	strategies := make([]bdptStrategy, 0)

	for s := 0; s <= lightPath.Length; s++ {
		for t := 1; t <= cameraPath.Length; t++ {
			var contribution core.Vec3

			if s == 0 {
				// s=0: Pure camera path
				contribution = bdpt.evaluatePathTracingStrategy(cameraPath, t)
			} else if t == 1 {
				// t=1 is light path direct to camera, which might hit a different pixel
				// skip it for now
				continue
			} else {
				// All other cases: Connection strategies (including s=0, t<last)
				contribution = bdpt.evaluateConnectionStrategy(cameraPath, lightPath, s, t, scene)
			}

			if contribution.Luminance() > 0 {
				pdf := bdpt.calculatePathPDF(cameraPath, lightPath, s, t)
				if pdf > 0 {
					strategies = append(strategies, bdptStrategy{
						s:            s,
						t:            t,
						contribution: contribution,
						pdf:          pdf,
					})
				}
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
	contribution := lastVertex.EmittedLight.MultiplyVec(lastVertex.Beta)
	bdpt.logf(" (s=0,t=%d) evaluatePathTracingStrategy: contribution:%v = lastVertex.EmittedLight:%v * lastVertex.Beta:%v\n", t, contribution, lastVertex.EmittedLight, lastVertex.Beta)
	return contribution
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
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, distance-0.001)
	if blocked {
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

	// For BDPT connections, use appropriate scaling
	var lightContribution core.Vec3
	if lightVertex.IsLight {
		// When connecting to a light source, emission is in the beta but need to divide by pdf
		lightContribution = core.NewVec3(1, 1, 1).Multiply(1 / lightVertex.AreaPdfForward)
	} else {
		// When connecting to a surface hit by light, the surface acts as a secondary light source
		// We need BRDF * cos_theta, but cos_theta is already in the geometric term
		// So we just need the BRDF value multiplied by the surface's received radiance
		// The received radiance is encoded in the light path throughput
		lightContribution = bdpt.evaluateBRDF(lightVertex, direction.Multiply(-1)).Multiply(1 / lightVertex.AreaPdfForward)
	}

	// pbrt: L = qs.beta * qs.f(pt, TransportMode::Importance) * pt.f(qs, TransportMode::Radiance) * pt.beta;

	// Combine everything: f_camera(x) * G(x,y) * [Le(light) or f_light(y)] * T_camera * T_light
	// L = qs.beta * qs.f(pt, TransportMode::Importance) * pt.f(qs, TransportMode::Radiance) * pt.beta * G(qs, pt)
	bdpt.logf("bdpt: brdf=%v * light=%v * G=%v * cameraThroughput=%v * lightThroughput=%v\n", cameraBRDF, lightContribution, geometricTerm, cameraPathThroughput, lightPathThroughput)
	contribution := cameraBRDF.MultiplyVec(lightContribution).Multiply(geometricTerm).MultiplyVec(cameraPathThroughput).MultiplyVec(lightPathThroughput)

	return contribution
}

func (bdpt *BDPTIntegrator) logf(format string, a ...interface{}) {
	if bdpt.Verbose {
		fmt.Printf(format, a...)
	}
}
