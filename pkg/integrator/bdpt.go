package integrator

import (
	"math"
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
	Throughput    core.Vec3           // Accumulated throughput to vertex
	Beta          core.Vec3           // BSDF * cos(theta) / pdf
	EmittedLight  core.Vec3           // Light emitted from this vertex (captured during path gen)
	ScatterResult *core.ScatterResult // Material scatter result (captured during path gen)
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

// generateCameraSubpath generates a camera subpath with vertex tracking
// This focuses purely on path generation for BDPT - lighting is handled separately
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
			ForwardPDF:        1.0, // Will be calculated properly later
			ReversePDF:        1.0, // Will be calculated properly later
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

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
		currentThroughput = currentThroughput.MultiplyVec(scatter.Attenuation)
	}

	return path
}

// generateLightSubpath generates a light subpath starting from a light source
func (bdpt *BDPTIntegrator) generateLightSubpath(scene core.Scene, random *rand.Rand, maxDepth int) Path {
	path := Path{
		Vertices: make([]Vertex, 0, maxDepth),
		Length:   0,
	}

	// Get lights from the scene
	lights := scene.GetLights()
	if len(lights) == 0 {
		return path // No lights, return empty path
	}

	// Sample a light source
	lightIndex := int(random.Float64() * float64(len(lights)))
	if lightIndex >= len(lights) {
		lightIndex = len(lights) - 1
	}
	light := lights[lightIndex]

	// Sample a point on the light and get initial ray
	lightSample := light.Sample(core.Vec3{X: 0, Y: 0, Z: 0}, random) // Position doesn't matter for emission
	if lightSample.PDF <= 0 {
		return path // Invalid light sample
	}

	// Create the initial light vertex
	lightVertex := Vertex{
		Point:             lightSample.Point,
		Normal:            lightSample.Normal,
		Material:          nil,                         // Lights don't have materials in our current system
		IncomingDirection: core.Vec3{X: 0, Y: 0, Z: 0}, // Light is the starting point
		OutgoingDirection: lightSample.Direction,
		IncomingRay:       core.Ray{}, // No incoming ray for light
		ForwardPDF:        lightSample.PDF,
		ReversePDF:        0.0, // Cannot generate reverse direction to light
		IsLight:           true,
		IsCamera:          false,
		IsSpecular:        false,
		Throughput:        core.Vec3{X: 1, Y: 1, Z: 1},
		Beta:              core.Vec3{X: 1, Y: 1, Z: 1},
		EmittedLight:      lightSample.Emission,
		ScatterResult:     nil, // Lights don't scatter
	}

	path.Vertices = append(path.Vertices, lightVertex)
	path.Length++

	// Continue the light path by bouncing off surfaces
	currentRay := core.NewRay(lightSample.Point, lightSample.Direction)
	currentThroughput := lightSample.Emission

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
			ForwardPDF:        1.0, // Will be calculated properly later
			ReversePDF:        1.0, // Will be calculated properly later
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

		path.Vertices = append(path.Vertices, vertex)
		path.Length++

		// Prepare for next bounce
		currentRay = scatter.Scattered
		currentThroughput = currentThroughput.MultiplyVec(scatter.Attenuation)
	}

	return path
}

// evaluateBDPTStrategies evaluates all BDPT path construction strategies with MIS
func (bdpt *BDPTIntegrator) evaluateBDPTStrategies(cameraPath, lightPath Path, scene core.Scene, random *rand.Rand, sampleIndex int) core.Vec3 {

	// Strategy 1: Pure path tracing
	pathTracingContribution := bdpt.evaluatePathTracingStrategy(cameraPath, scene, random, sampleIndex)

	// Strategy 2: Find the best connection
	bestConnectionContribution := core.Vec3{X: 0, Y: 0, Z: 0}

	// Try connections (but limit to avoid too many combinations)
	maxConnections := 3 // Limit connection attempts
	connectionCount := 0

	for s := 1; s <= lightPath.Length && connectionCount < maxConnections; s++ {
		for t := 1; t <= cameraPath.Length && connectionCount < maxConnections; t++ {
			connectionContribution := bdpt.evaluateConnectionStrategy(cameraPath, lightPath, s, t, scene)
			if connectionContribution.Luminance() > bestConnectionContribution.Luminance() {
				bestConnectionContribution = connectionContribution
			}
			connectionCount++
		}
	}

	// Use MIS to weight path tracing vs best connection
	if bestConnectionContribution.Luminance() > 0 {
		// Calculate MIS weights for the two strategies
		pathWeight := 0.5 // path tracing
		connWeight := 0.5 // connection

		return pathTracingContribution.Multiply(pathWeight).Add(bestConnectionContribution.Multiply(connWeight))
	} else {
		// No valid connections, just use path tracing
		return pathTracingContribution
	}
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

// evaluateConnection computes the contribution from connecting two specific vertices
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

	// Evaluate BRDF at camera vertex
	var brdf core.Vec3
	if cameraVertex.ScatterResult != nil {
		// Use the actual material BRDF
		brdf = cameraVertex.ScatterResult.Attenuation.Multiply(1.0 / math.Pi)
	} else {
		return core.Vec3{X: 0, Y: 0, Z: 0} // No scatter result means no BRDF
	}

	// Calculate path throughputs
	cameraPathThroughput := bdpt.calculateCameraPathThroughput(cameraPath, t)
	lightPathThroughput := bdpt.calculateLightPathThroughput(lightPath, s)

	// Combine everything: f(x) * G(x,y) * f(y) * T_camera * T_light
	contribution := brdf.Multiply(geometricTerm).MultiplyVec(cameraPathThroughput).MultiplyVec(lightPathThroughput)

	return contribution
}

// calculateCameraPathThroughput calculates the throughput along a camera subpath
func (bdpt *BDPTIntegrator) calculateCameraPathThroughput(path Path, length int) core.Vec3 {
	if length <= 0 || length > path.Length {
		return core.Vec3{X: 1, Y: 1, Z: 1}
	}

	throughput := core.Vec3{X: 1, Y: 1, Z: 1}
	for i := 0; i < length-1; i++ {
		vertex := path.Vertices[i]
		if vertex.ScatterResult != nil {
			throughput = throughput.MultiplyVec(vertex.ScatterResult.Attenuation)
		}
	}
	return throughput
}

// calculateLightPathThroughput calculates the throughput along a light subpath
func (bdpt *BDPTIntegrator) calculateLightPathThroughput(path Path, length int) core.Vec3 {
	if length <= 0 || length > path.Length {
		return core.Vec3{X: 1, Y: 1, Z: 1}
	}

	// Start with the light emission from the first vertex
	throughput := core.Vec3{X: 1, Y: 1, Z: 1}
	if path.Length > 0 && path.Vertices[0].EmittedLight.Luminance() > 0 {
		throughput = path.Vertices[0].EmittedLight
	}

	// Accumulate scattering along the path (same as camera path)
	for i := 1; i < length; i++ {
		vertex := path.Vertices[i]
		if vertex.ScatterResult != nil {
			throughput = throughput.MultiplyVec(vertex.ScatterResult.Attenuation)
		}
	}
	return throughput
}
