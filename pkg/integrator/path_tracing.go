package integrator

import (
	"fmt"
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// PathTracingIntegrator implements unidirectional path tracing
type PathTracingIntegrator struct {
	config  core.SamplingConfig
	Verbose bool
}

// NewPathTracingIntegrator creates a new path tracing integrator
func NewPathTracingIntegrator(config core.SamplingConfig) *PathTracingIntegrator {
	return &PathTracingIntegrator{
		config:  config,
		Verbose: false,
	}
}

// RayColor computes the color for a single ray using unidirectional path tracing
func (pt *PathTracingIntegrator) RayColor(ray core.Ray, scene core.Scene, sampler core.Sampler) (core.Vec3, []core.SplatRay) {
	depth := pt.config.MaxDepth
	throughput := core.Vec3{X: 1.0, Y: 1.0, Z: 1.0}
	return pt.rayColorRecursive(ray, scene, sampler, depth, throughput), nil
}

func (pt *PathTracingIntegrator) rayColorRecursive(ray core.Ray, scene core.Scene, sampler core.Sampler, depth int, throughput core.Vec3) core.Vec3 {
	// If we've exceeded the ray bounce limit, no more light is gathered
	if depth <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Apply Russian Roulette termination
	shouldTerminate, rrCompensation := pt.ApplyRussianRoulette(depth, throughput, sampler.Get1D())
	if shouldTerminate {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check for intersections with objects using scene's BVH
	hit, isHit := scene.GetBVH().Hit(ray, 0.001, math.Inf(1))
	if !isHit {
		bgColor := pt.BackgroundGradient(ray, scene)
		return bgColor.Multiply(rrCompensation)
	}

	// Start with emitted light from the hit material
	colorEmitted := pt.GetEmittedLight(ray, hit)

	// Try to scatter the ray
	scatter, didScatter := hit.Material.Scatter(ray, *hit, sampler)
	if !didScatter {
		// Material absorbed the ray, only return emitted light
		if colorEmitted.Luminance() > 0 {
			pt.logf("      pt[%d]    light: contribution=%v\n", pt.config.MaxDepth-depth, colorEmitted)
		} else {
			pt.logf("      pt[%d] absorbed: contribution=0\n", pt.config.MaxDepth-depth)
		}
		return colorEmitted.Multiply(rrCompensation)
	}

	// Handle scattering based on material type
	var colorScattered core.Vec3
	if scatter.IsSpecular() {
		colorScattered = pt.calculateSpecularColor(scatter, scene, depth, throughput, sampler)
	} else {
		colorScattered = pt.calculateDiffuseColor(scatter, hit, scene, depth, throughput, sampler)
	}

	// Apply Russian Roulette compensation to the final result
	finalColor := colorEmitted.Add(colorScattered)
	return finalColor.Multiply(rrCompensation)
}

// calculateSpecularColor handles specular material scattering with the provided random generator
func (pt *PathTracingIntegrator) calculateSpecularColor(scatter core.ScatterResult, scene core.Scene, depth int, throughput core.Vec3, sampler core.Sampler) core.Vec3 {
	// Update throughput with material attenuation
	newThroughput := throughput.MultiplyVec(scatter.Attenuation)
	incomingLight := pt.rayColorRecursive(scatter.Scattered, scene, sampler, depth-1, newThroughput)
	contribution := scatter.Attenuation.MultiplyVec(incomingLight)

	pt.logf("      pt[%d] specular: contribution=%v = attenuation=%v * incomingLight=%v\n", pt.config.MaxDepth-depth, contribution, scatter.Attenuation, incomingLight)

	return contribution
}

// calculateDiffuseColor handles diffuse material scattering with throughput tracking
func (pt *PathTracingIntegrator) calculateDiffuseColor(scatter core.ScatterResult, hit *core.HitRecord, scene core.Scene, depth int, throughput core.Vec3, sampler core.Sampler) core.Vec3 {
	// Combine direct lighting and indirect lighting using Multiple Importance Sampling
	directLight := pt.CalculateDirectLighting(scene, scatter, hit, sampler, depth)
	indirectLight := pt.CalculateIndirectLighting(scene, scatter, hit, depth, throughput, sampler)
	return directLight.Add(indirectLight)
}

// getEmittedLight returns the emitted light from a material if it's emissive
func (pt *PathTracingIntegrator) GetEmittedLight(ray core.Ray, hit *core.HitRecord) core.Vec3 {
	if emitter, isEmissive := hit.Material.(core.Emitter); isEmissive {
		return emitter.Emit(ray)
	}
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// calculateDirectLighting samples lights directly for direct illumination with the provided random generator
func (pt *PathTracingIntegrator) CalculateDirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord, sampler core.Sampler, depth int) core.Vec3 {
	lights := scene.GetLights()

	// Sample a light
	lightSample, _, hasLight := core.SampleLight(lights, hit.Point, sampler)
	if !hasLight || lightSample.Emission.Luminance() <= 0 || lightSample.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Check if light is visible (shadow ray)
	shadowRay := core.NewRay(hit.Point, lightSample.Direction)
	_, blocked := scene.GetBVH().Hit(shadowRay, 0.001, lightSample.Distance-0.001)
	if blocked {
		// Light is blocked, no direct contribution
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Calculate the cosine factor
	cosine := lightSample.Direction.Dot(hit.Normal)
	if cosine <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0} // Light is behind the surface
	}

	// Get material PDF for this direction (for MIS)
	// Use the material's PDF method, but for direct lighting we evaluate the light direction
	materialPDF, isDelta := hit.Material.PDF(scatter.Incoming.Direction, lightSample.Direction, hit.Normal)

	// For delta materials, skip direct lighting calculation (they can't be directly lit)
	if isDelta {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Calculate MIS weight
	misWeight := core.PowerHeuristic(1, lightSample.PDF, 1, materialPDF)

	// Calculate BRDF for the new outgoing direction
	brdf := hit.Material.EvaluateBRDF(scatter.Incoming.Direction, lightSample.Direction, hit.Normal)

	// Direct lighting contribution: BRDF * emission * cosine * MIS_weight / light_PDF
	contribution := brdf.MultiplyVec(lightSample.Emission).Multiply(cosine * misWeight / lightSample.PDF)
	pt.logf("      pt[%d]   direct: contribution=%v = brdf=%v * emission=%v * (cosine=%f * misWeight=%f / lightPDF=%f)\n", pt.config.MaxDepth-depth, contribution, brdf, lightSample.Emission, cosine, misWeight, lightSample.PDF)

	return contribution
}

// calculateIndirectLighting handles indirect illumination via material sampling with throughput tracking
func (pt *PathTracingIntegrator) CalculateIndirectLighting(scene core.Scene, scatter core.ScatterResult, hit *core.HitRecord, depth int, throughput core.Vec3, sampler core.Sampler) core.Vec3 {
	if scatter.PDF <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	scatterDirection := scatter.Scattered.Direction.Normalize()
	cosine := scatterDirection.Dot(hit.Normal)
	if cosine <= 0 {
		return core.Vec3{X: 0, Y: 0, Z: 0}
	}

	// Get light PDF for this direction (for MIS)
	lights := scene.GetLights()
	lightPDF := core.CalculateLightPDF(lights, hit.Point, scatterDirection)

	// Calculate MIS weight
	misWeight := core.PowerHeuristic(1, scatter.PDF, 1, lightPDF)

	// Update throughput for the recursive call
	newThroughput := throughput.MultiplyVec(scatter.Attenuation).Multiply(cosine / scatter.PDF)

	// Get incoming light from the scattered direction with throughput tracking
	incomingLight := pt.rayColorRecursive(scatter.Scattered, scene, sampler, depth-1, newThroughput)

	// Indirect lighting contribution with MIS
	contribution := scatter.Attenuation.Multiply(cosine * misWeight / scatter.PDF).MultiplyVec(incomingLight)

	pt.logf("      pt[%d] indirect: contribution=%v = attenuation=%v * incomingLight=%v * (cosine=%f * misWeight=%f / scatterPDF=%f)\n", pt.config.MaxDepth-depth, contribution, scatter.Attenuation, incomingLight, cosine, misWeight, scatter.PDF)

	return contribution
}

// applyRussianRoulette determines if a ray should be terminated and returns the compensation factor
// Returns (shouldTerminate, compensationFactor)
func (pt *PathTracingIntegrator) ApplyRussianRoulette(depth int, throughput core.Vec3, sample float64) (bool, float64) {
	// Apply Russian Roulette after minimum bounces AND minimum samples per pixel
	initialDepth := pt.config.MaxDepth
	currentBounce := initialDepth - depth

	shouldApplyRR := currentBounce >= pt.config.RussianRouletteMinBounces

	if !shouldApplyRR {
		return false, 1.0 // Don't terminate, no compensation needed
	}

	// Calculate survival probability based on throughput
	// Use luminance for perceptually accurate survival probability
	luminance := throughput.Luminance()

	// Conservative bounds: survivalProb between 0.5 and 0.95
	// This naturally limits compensation factor to between 1.05x and 2.0x
	survivalProb := math.Min(0.95, math.Max(0.5, luminance))

	// Russian Roulette test
	if sample > survivalProb {
		return true, 0.0 // Terminate ray
	}

	// Energy-conserving compensation (no artificial cap)
	compensationFactor := 1.0 / survivalProb
	return false, compensationFactor
}

// backgroundGradient returns a gradient color based on ray direction
func (pt *PathTracingIntegrator) BackgroundGradient(r core.Ray, scene core.Scene) core.Vec3 {
	// Get colors from the scene
	topColor, bottomColor := scene.GetBackgroundColors()

	// Normalize the ray direction to get consistent results
	unitDirection := r.Direction.Normalize()

	// Use the y-component to create a gradient (map from -1,1 to 0,1)
	t := 0.5 * (unitDirection.Y + 1.0)

	// Linear interpolation: (1-t)*bottom + t*top
	return bottomColor.Multiply(1.0 - t).Add(topColor.Multiply(t))
}

func (pt *PathTracingIntegrator) logf(format string, a ...interface{}) {
	if pt.Verbose {
		fmt.Printf(format, a...)
	}
}
