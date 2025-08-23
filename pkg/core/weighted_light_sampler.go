package core

import (
	"fmt"
)

// WeightedLightSampler implements light sampling with user-specified weights
// Weights must match the order of lights in the scene's Lights array
type WeightedLightSampler struct {
	lights      []Light
	weights     []float64
	sceneRadius float64
}

// NewWeightedLightSampler creates a light sampler with specified weights
// weights slice must have the same length as lights slice
// weights will be automatically normalized to sum to 1.0
func NewWeightedLightSampler(lights []Light, weights []float64, sceneRadius float64) *WeightedLightSampler {
	if len(lights) != len(weights) {
		panic(fmt.Sprintf("lights length (%d) must match weights length (%d)", len(lights), len(weights)))
	}

	// Normalize weights to sum to 1.0
	normalizedWeights := make([]float64, len(weights))
	totalWeight := 0.0
	for _, weight := range weights {
		if weight < 0 {
			panic("weights must be non-negative")
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		// All weights are zero, use uniform distribution
		uniformWeight := 1.0 / float64(len(weights))
		for i := range normalizedWeights {
			normalizedWeights[i] = uniformWeight
		}
	} else {
		// Normalize weights
		for i, weight := range weights {
			normalizedWeights[i] = weight / totalWeight
		}
	}

	return &WeightedLightSampler{
		lights:      lights,
		weights:     normalizedWeights,
		sceneRadius: sceneRadius,
	}
}

// NewUniformLightSampler creates a light sampler with equal weights for all lights
// This is a convenience function that creates a WeightedLightSampler with uniform weights
func NewUniformLightSampler(lights []Light, sceneRadius float64) *WeightedLightSampler {
	if len(lights) == 0 {
		return &WeightedLightSampler{
			lights:      lights,
			weights:     []float64{},
			sceneRadius: sceneRadius,
		}
	}

	// Create uniform weights (all equal)
	uniformWeight := 1.0 / float64(len(lights))
	weights := make([]float64, len(lights))
	for i := range weights {
		weights[i] = uniformWeight
	}

	return &WeightedLightSampler{
		lights:      lights,
		weights:     weights,
		sceneRadius: sceneRadius,
	}
}

// SampleLight selects a light using the fixed weights (independent of surface point)
// Returns the selected light, its selection probability, and its index
func (fls *WeightedLightSampler) SampleLight(point Vec3, normal Vec3, u float64) (Light, float64, int) {
	if len(fls.lights) == 0 {
		return nil, 0.0, -1
	}

	// Sample based on fixed weights using cumulative distribution
	var cumulativeProbability float64
	for i := 0; i < len(fls.lights); i++ {
		cumulativeProbability += fls.weights[i]
		if u <= cumulativeProbability {
			return fls.lights[i], fls.weights[i], i
		}
	}

	// Fallback to last light (should not happen with proper weights)
	lastIdx := len(fls.lights) - 1
	return fls.lights[lastIdx], fls.weights[lastIdx], lastIdx
}

// SampleLightEmission selects a light using the fixed weights for emission sampling
// Returns the selected light, its selection probability, and its index
func (fls *WeightedLightSampler) SampleLightEmission(u float64) (Light, float64, int) {
	if len(fls.lights) == 0 {
		return nil, 0.0, -1
	}

	// Use same fixed weights for emission sampling
	var cumulativeProbability float64
	for i := 0; i < len(fls.lights); i++ {
		cumulativeProbability += fls.weights[i]
		if u <= cumulativeProbability {
			return fls.lights[i], fls.weights[i], i
		}
	}

	// Fallback to last light
	lastIdx := len(fls.lights) - 1
	return fls.lights[lastIdx], fls.weights[lastIdx], lastIdx
}

// GetLightProbability returns the fixed probability for the light at the given index
func (fls *WeightedLightSampler) GetLightProbability(lightIndex int, point Vec3, normal Vec3) float64 {
	if lightIndex < 0 || lightIndex >= len(fls.weights) {
		return 0.0
	}
	return fls.weights[lightIndex]
}

// GetLightCount returns the number of lights in this sampler
func (fls *WeightedLightSampler) GetLightCount() int {
	return len(fls.lights)
}

// String returns a string representation for debugging
func (fls *WeightedLightSampler) String() string {
	if len(fls.lights) == 0 {
		return "WeightedLightSampler{no lights}"
	}

	result := fmt.Sprintf("WeightedLightSampler{%d lights with fixed weights:\n", len(fls.lights))
	for i, light := range fls.lights {
		result += fmt.Sprintf("  [%d] %s: %.1f%%\n", i, light.Type(), fls.weights[i]*100)
	}
	result += "}"
	return result
}
