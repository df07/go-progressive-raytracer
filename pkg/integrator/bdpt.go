package integrator

import (
	"math/rand"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Vertex represents a single vertex in a light transport path
type Vertex struct {
	Point    core.Vec3     // 3D position
	Normal   core.Vec3     // Surface normal
	Material core.Material // Material at this vertex

	// Path tracing information
	IncomingDirection core.Vec3 // Direction ray arrived from
	OutgoingDirection core.Vec3 // Direction ray continues to
	IncomingRay       core.Ray  // The actual ray that hit this vertex

	// MIS probability densities
	ForwardPDF float64 // PDF for generating this vertex forward
	ReversePDF float64 // PDF for generating this vertex reverse

	// Vertex classification
	IsLight    bool // On light source
	IsCamera   bool // On camera
	IsSpecular bool // Specular interaction

	// Transport quantities
	Throughput    core.Vec3           // Accumulated throughput from path start to this vertex
	Beta          core.Vec3           // Unweighted throughput (BRDF * cos without PDF division)
	EmittedLight  core.Vec3           // Light emitted from this vertex
	ScatterResult *core.ScatterResult // Material scatter result for BRDF evaluation
}

// Path represents a sequence of vertices in a light transport path
type Path struct {
	Vertices []Vertex
	Length   int
}

// BDPTIntegrator implements bidirectional path tracing
type BDPTIntegrator struct {
	*PathTracingIntegrator
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
	}
}

// RayColor computes the color for a single ray using BDPT
func (bdpt *BDPTIntegrator) RayColor(ray core.Ray, scene core.Scene, random *rand.Rand, depth int, throughput core.Vec3, sampleIndex int) core.Vec3 {
	// Generate camera path with vertices
	cameraPath := bdpt.generateCameraSubpath(ray, scene, random, depth, throughput, sampleIndex)

	// Generate a light path
	lightPath := bdpt.generateLightSubpath(scene, random, depth)

	// Evaluate all BDPT strategies with proper MIS weighting
	return bdpt.evaluateBDPTStrategies(cameraPath, lightPath, scene, random, sampleIndex)
}

// generateCameraSubpath generates a camera subpath with proper PDF tracking for BDPT
// Each vertex stores forward/reverse PDFs needed for MIS weight calculation
func (bdpt *BDPTIntegrator) generateCameraSubpath(ray core.Ray, scene core.Scene, random *rand.Rand, depth int, throughput core.Vec3, sampleIndex int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, depth),
		Length:   0,
	}

	currentRay := ray
	currentThroughput := throughput

	for bounces := 0; bounces < depth; bounces++ {
		// No Russian Roulette during path generation - we'll handle it during lighting evaluation

		// Check for intersections
		hit, isHit := scene.GetBVH().Hit(currentRay, 0.001, 1000.0)
		if !isHit {
			// Hit background - create a background vertex with captured light
			bgColor := bdpt.BackgroundGradient(currentRay, scene)

			vertex := Vertex{
				Point:             currentRay.Origin.Add(currentRay.Direction.Multiply(1000.0)), // Far background
				Normal:            currentRay.Direction.Multiply(-1),                            // Reverse direction
				Material:          nil,
				IncomingDirection: currentRay.Direction.Multiply(-1),
				OutgoingDirection: core.Vec3{X: 0, Y: 0, Z: 0},
				IncomingRay:       currentRay,
				ForwardPDF:        1.0,  // Background PDF
				ReversePDF:        0.0,  // Cannot generate rays towards background
				IsLight:           true, // Background acts as light source
				IsCamera:          bounces == 0,
				IsSpecular:        false,
				Throughput:        currentThroughput,
				Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
				EmittedLight:      bgColor, // Capture background light
				ScatterResult:     nil,     // Background doesn't scatter
			}

			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Capture emitted light from this vertex
		emittedLight := bdpt.GetEmittedLight(currentRay, hit)

		// Try to scatter the ray
		scatter, didScatter := hit.Material.Scatter(currentRay, *hit, random)

		// Create vertex for the intersection with all captured information
		vertex := Vertex{
			Point:             hit.Point,
			Normal:            hit.Normal,
			Material:          hit.Material,
			IncomingDirection: currentRay.Direction.Multiply(-1),
			OutgoingDirection: core.Vec3{X: 0, Y: 0, Z: 0}, // Will be set if ray continues
			IncomingRay:       currentRay,
			ForwardPDF:        1.0, // Will be updated by setVertexPDFs if material scatters
			ReversePDF:        1.0, // Will be updated by setVertexPDFs if material scatters
			IsLight:           emittedLight.Luminance() > 0,
			IsCamera:          bounces == 0,
			IsSpecular:        false, // Will be set below if material scatters
			Throughput:        currentThroughput,
			Beta:              core.Vec3{X: 1, Y: 1, Z: 1}, // Will be calculated properly later
			EmittedLight:      emittedLight,                // Captured during path generation
			ScatterResult:     nil,                         // Will be set below if material scatters
		}

		if !didScatter {
			// Material absorbed the ray - add vertex and terminate path
			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Material scattered - capture the scatter information
		vertex.IsSpecular = scatter.IsSpecular()
		vertex.OutgoingDirection = scatter.Scattered.Direction
		vertex.ScatterResult = &scatter // Capture scatter result for later use

		// Set PDFs and Beta using the helper function
		bdpt.setVertexPDFs(&vertex, scatter, currentRay, hit)

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
		currentThroughput = currentThroughput.MultiplyVec(scatter.Attenuation)
	}

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
	emissionSample, hasLight := core.SampleLightEmission(scene.GetLights(), random)
	if !hasLight || emissionSample.PDF <= 0 {
		return path // No lights or invalid emission sample
	}

	// Create the initial light vertex
	lightVertex := Vertex{
		Point:             emissionSample.Point,
		Normal:            emissionSample.Normal,
		Material:          nil,                         // Lights don't have materials in our current system
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0}, // Light is the starting point
		OutgoingDirection: emissionSample.Direction,
		IncomingRay:       core.Ray{},         // No incoming ray for light
		ForwardPDF:        emissionSample.PDF, // Already includes light selection PDF
		ReversePDF:        0.0,                // Cannot generate reverse direction to light
		IsLight:           true,
		IsCamera:          false,
		IsSpecular:        false,
		Throughput:        core.Vec3{X: 1, Y: 1, Z: 1},
		Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
		EmittedLight:      emissionSample.Emission, // Already properly scaled
		ScatterResult:     nil,                     // Lights don't scatter
	}

	path.Vertices = append(path.Vertices, lightVertex)
	path.Length++

	// Continue the light path by bouncing off surfaces
	currentRay := core.NewRay(emissionSample.Point, emissionSample.Direction)
	currentThroughput := emissionSample.Emission

	for bounces := 1; bounces < maxDepth; bounces++ {
		// Check for intersections
		hit, isHit := scene.GetBVH().Hit(currentRay, 0.001, 1000.0)
		if !isHit {
			// Light ray escaped to environment - create background vertex
			bgColor := bdpt.BackgroundGradient(currentRay, scene)

			vertex := Vertex{
				Point:             currentRay.Origin.Add(currentRay.Direction.Multiply(1000.0)),
				Normal:            currentRay.Direction.Multiply(-1),
				Material:          nil,
				IncomingDirection: currentRay.Direction.Multiply(-1),
				OutgoingDirection: core.Vec3{X: 0, Y: 0, Z: 0},
				IncomingRay:       currentRay,
				ForwardPDF:        1.0,
				ReversePDF:        1.0,
				IsLight:           true, // Background is treated as light
				IsCamera:          false,
				IsSpecular:        false,
				Throughput:        currentThroughput,
				Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
				EmittedLight:      bgColor,
				ScatterResult:     nil,
			}

			path.Vertices = append(path.Vertices, vertex)
			path.Length++
			break
		}

		// Create vertex for the intersection
		vertex := Vertex{
			Point:             hit.Point,
			Normal:            hit.Normal,
			Material:          hit.Material,
			IncomingDirection: currentRay.Direction.Multiply(-1),
			OutgoingDirection: core.Vec3{X: 0, Y: 0, Z: 0},
			IncomingRay:       currentRay,
			ForwardPDF:        1.0, // Will be updated by setVertexPDFs if material scatters
			ReversePDF:        1.0, // Will be updated by setVertexPDFs if material scatters
			IsLight:           false,
			IsCamera:          false,
			IsSpecular:        false,
			Throughput:        currentThroughput,
			Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
			EmittedLight:      core.Vec3{X: 0, Y: 0, Z: 0}, // Surface vertices don't emit (usually)
			ScatterResult:     nil,
		}

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
		vertex.OutgoingDirection = scatter.Scattered.Direction
		vertex.ScatterResult = &scatter

		// Set PDFs and Beta using the helper function
		bdpt.setVertexPDFs(&vertex, scatter, currentRay, hit)

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
		currentThroughput = currentThroughput.MultiplyVec(scatter.Attenuation)
	}

	return path
}

// setVertexPDFs sets the forward and reverse PDFs for a vertex based on scattering
func (bdpt *BDPTIntegrator) setVertexPDFs(vertex *Vertex, scatter core.ScatterResult, incomingRay core.Ray, hit *core.HitRecord) {
	if scatter.IsSpecular() {
		// For specular materials, use the new PDF method that returns 1.0 for perfect reflection/refraction
		vertex.ForwardPDF = hit.Material.PDF(incomingRay.Direction.Multiply(-1), scatter.Scattered.Direction, hit.Normal)
		vertex.ReversePDF = hit.Material.PDF(scatter.Scattered.Direction, incomingRay.Direction.Multiply(-1), hit.Normal)
		// For specular, Beta = attenuation (no cos/PDF division)
		vertex.Beta = scatter.Attenuation
	} else {
		vertex.ForwardPDF = scatter.PDF // PDF for the direction we scattered
		// Calculate reverse PDF using the material's PDF method
		vertex.ReversePDF = hit.Material.PDF(scatter.Scattered.Direction, incomingRay.Direction.Multiply(-1), hit.Normal)
		// Beta = BRDF * cos(theta) for non-specular materials
		cosTheta := scatter.Scattered.Direction.Dot(hit.Normal)
		if cosTheta > 0 {
			vertex.Beta = scatter.Attenuation.Multiply(cosTheta)
		} else {
			vertex.Beta = core.Vec3{X: 0, Y: 0, Z: 0}
		}
	}
}

// calculatePathPDF calculates the PDF for a complete path construction strategy
func (bdpt *BDPTIntegrator) calculatePathPDF(cameraPath Path, lightPath Path, s, t int) float64 {
	pdf := 1.0

	// Camera path contribution (first t vertices)
	for i := 0; i < t && i < len(cameraPath.Vertices); i++ {
		vertex := cameraPath.Vertices[i]
		if vertex.ForwardPDF > 0 {
			pdf *= vertex.ForwardPDF
		}
	}

	// Light path contribution (first s vertices)
	for i := 0; i < s && i < len(lightPath.Vertices); i++ {
		vertex := lightPath.Vertices[i]
		if vertex.ForwardPDF > 0 {
			pdf *= vertex.ForwardPDF
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

	return currentWeight / sumWeights
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
func (bdpt *BDPTIntegrator) evaluateBDPTStrategies(cameraPath, lightPath Path, scene core.Scene, random *rand.Rand, sampleIndex int) core.Vec3 {

	totalContribution := core.Vec3{X: 0, Y: 0, Z: 0}

	// Calculate all valid strategies and their PDFs for proper MIS
	strategies := make([]bdptStrategy, 0)

	// Strategy: Pure path tracing (s=0, t=camera_length)
	if len(cameraPath.Vertices) > 0 {
		contribution := bdpt.evaluatePathTracingStrategy(cameraPath, scene, random, sampleIndex)
		pdf := bdpt.calculatePathPDF(cameraPath, lightPath, 0, len(cameraPath.Vertices))
		if contribution.Luminance() > 0 && pdf > 0 {
			strategies = append(strategies, bdptStrategy{
				s: 0, t: len(cameraPath.Vertices),
				contribution: contribution,
				pdf:          pdf,
			})
		}
	}

	// Strategies: Connection strategies (s>0, t>0)
	maxS := lightPath.Length
	if maxS > 3 {
		maxS = 3 // Reasonable limit to avoid too many strategies
	}
	maxT := cameraPath.Length
	if maxT > 4 {
		maxT = 4 // Reasonable limit
	}

	for s := 1; s <= maxS; s++ {
		for t := 1; t <= maxT; t++ {
			contribution := bdpt.evaluateConnectionStrategy(cameraPath, lightPath, s, t, scene)
			if contribution.Luminance() > 0 {
				pdf := bdpt.calculatePathPDF(cameraPath, lightPath, s, t)
				if pdf > 0 {
					strategies = append(strategies, bdptStrategy{
						s: s, t: t,
						contribution: contribution,
						pdf:          pdf,
					})
				}
			}
		}
	}

	// Apply MIS weighting to all strategies
	for _, strategy := range strategies {
		// Calculate MIS weight by comparing against all other strategies
		weight := bdpt.calculateMISWeight(strategy, strategies)
		totalContribution = totalContribution.Add(strategy.contribution.Multiply(weight))
	}

	return totalContribution
}

// evaluatePathTracingStrategy evaluates the standard path tracing strategy (camera path only)
func (bdpt *BDPTIntegrator) evaluatePathTracingStrategy(cameraPath Path, scene core.Scene, random *rand.Rand, sampleIndex int) core.Vec3 {
	// Just use the original path tracing integrator for this - it's proven to work
	if len(cameraPath.Vertices) > 0 {
		firstVertex := cameraPath.Vertices[0]
		originalRay := firstVertex.IncomingRay
		return bdpt.PathTracingIntegrator.RayColor(originalRay, scene, random, bdpt.config.MaxDepth, core.Vec3{X: 1, Y: 1, Z: 1}, sampleIndex)
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// evaluateLightTracingStrategy evaluates light tracing (light path hits camera)
func (bdpt *BDPTIntegrator) evaluateLightTracingStrategy(lightPath Path, scene core.Scene) core.Vec3 {
	// For now, return zero since we don't implement camera sampling from light paths
	// Full implementation would trace light path and check if it hits the camera
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// evaluateBRDF evaluates the BRDF at a vertex for a given outgoing direction
func (bdpt *BDPTIntegrator) evaluateBRDF(vertex Vertex, outgoingDirection core.Vec3) core.Vec3 {
	if vertex.Material == nil {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Use the new EvaluateBRDF method from the material interface
	return vertex.Material.EvaluateBRDF(vertex.IncomingDirection, outgoingDirection, vertex.Normal)
}

// evaluateConnectionStrategy evaluates connecting s light vertices with t camera vertices
func (bdpt *BDPTIntegrator) evaluateConnectionStrategy(cameraPath, lightPath Path, s, t int, scene core.Scene) core.Vec3 {
	if s < 1 || t < 1 || s > lightPath.Length || t > cameraPath.Length {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Get the vertices to connect
	lightVertex := lightPath.Vertices[s-1]   // s-th vertex in light path (0-indexed)
	cameraVertex := cameraPath.Vertices[t-1] // t-th vertex in camera path (0-indexed)

	// Skip connections involving specular vertices (can't connect through delta functions)
	if lightVertex.IsSpecular || cameraVertex.IsSpecular {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Calculate connection
	return bdpt.evaluateConnection(cameraVertex, lightVertex, cameraPath, lightPath, s, t, scene)
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
func (bdpt *BDPTIntegrator) evaluateConnection(cameraVertex, lightVertex Vertex, cameraPath, lightPath Path, s, t int, scene core.Scene) core.Vec3 {
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

	// Evaluate BRDFs at both connection vertices
	cameraBRDF := bdpt.evaluateBRDF(cameraVertex, direction)
	lightBRDF := bdpt.evaluateBRDF(lightVertex, direction.Multiply(-1)) // Opposite direction for light vertex

	// Calculate path throughputs
	cameraPathThroughput := bdpt.calculateCameraPathThroughput(cameraPath, t)
	lightPathThroughput := bdpt.calculateLightPathThroughput(lightPath, s)

	// Combine everything: f_camera(x) * G(x,y) * f_light(y) * T_camera * T_light
	contribution := cameraBRDF.MultiplyVec(lightBRDF).Multiply(geometricTerm).MultiplyVec(cameraPathThroughput).MultiplyVec(lightPathThroughput)

	return contribution
}

// calculateCameraPathThroughput calculates the throughput along a camera subpath
func (bdpt *BDPTIntegrator) calculateCameraPathThroughput(path Path, length int) core.Vec3 {
	if length <= 0 || length > path.Length {
		return core.Vec3{X: 1, Y: 1, Z: 1}
	}

	// Use the stored throughput from the vertex (much more efficient)
	if length <= path.Length && length > 0 {
		return path.Vertices[length-1].Throughput
	}

	return core.Vec3{X: 1, Y: 1, Z: 1}
}

// calculateLightPathThroughput calculates the throughput along a light subpath
func (bdpt *BDPTIntegrator) calculateLightPathThroughput(path Path, length int) core.Vec3 {
	if length <= 0 || length > path.Length {
		return core.Vec3{X: 1, Y: 1, Z: 1}
	}

	// Use the stored throughput from the vertex (much more efficient)
	if length <= path.Length && length > 0 {
		return path.Vertices[length-1].Throughput
	}

	return core.Vec3{X: 1, Y: 1, Z: 1}
}
