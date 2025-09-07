package material

import (
	"math"

	"github.com/df07/go-progressive-raytracer/pkg/core"
)

// Dielectric represents a transparent material like glass that can both reflect and refract
type Dielectric struct {
	RefractiveIndex float64 // Index of refraction (e.g., 1.5 for glass)
}

// NewDielectric creates a new dielectric material
func NewDielectric(refractiveIndex float64) *Dielectric {
	return &Dielectric{RefractiveIndex: refractiveIndex}
}

// Scatter implements the Material interface for dielectric scattering
func (d *Dielectric) Scatter(rayIn core.Ray, hit HitRecord, sampler core.Sampler) (ScatterResult, bool) {
	// Dielectrics always attenuate by 1.0 (no color absorption for clear glass)
	attenuation := core.NewVec3(1.0, 1.0, 1.0)

	// Determine if we're entering or exiting the material
	var refractionRatio float64
	if hit.FrontFace {
		refractionRatio = 1.0 / d.RefractiveIndex // Ray is entering the material (from air to glass)
	} else {
		refractionRatio = d.RefractiveIndex // Ray is exiting the material (from glass to air)
	}

	// Normalize the incoming ray direction
	unitDirection := rayIn.Direction.Normalize()

	// Calculate the cosine of the angle between ray and normal
	cosTheta := math.Min(-unitDirection.Dot(hit.Normal), 1.0)
	sinTheta := math.Sqrt(1.0 - cosTheta*cosTheta)

	// Check for total internal reflection
	cannotRefract := refractionRatio*sinTheta > 1.0

	var direction core.Vec3
	if cannotRefract || Reflectance(cosTheta, refractionRatio) > sampler.Get1D() {
		// Reflect
		direction = reflectVector(unitDirection, hit.Normal)
	} else {
		// Refract
		direction = refractVector(unitDirection, hit.Normal, refractionRatio)
	}

	scattered := core.Ray{Origin: hit.Point, Direction: direction}

	return ScatterResult{
		Incoming:    rayIn,
		Scattered:   scattered,
		Attenuation: attenuation,
		PDF:         0, // Specular materials have no PDF
	}, true
}

// EvaluateBRDF evaluates the BRDF for specific incoming/outgoing directions with transport mode
func (d *Dielectric) EvaluateBRDF(incomingDir, outgoingDir core.Vec3, hit *HitRecord, mode TransportMode) core.Vec3 {
	// For dielectric materials, we need to distinguish between reflection and refraction
	// and handle transport mode properly for non-symmetric scattering

	// Normalize directions
	wi := incomingDir.Normalize()
	wo := outgoingDir.Normalize()
	n := hit.Normal.Normalize()

	// Check if this is a reflection or refraction event
	wiDotN := wi.Dot(n)
	woDotN := wo.Dot(n)

	// Different hemispheres = reflection, same hemisphere = refraction
	sameHemisphere := (wiDotN > 0) == (woDotN > 0)

	if !sameHemisphere {
		// Reflection case
		reflected := reflectVector(wi, n)
		// Check if outgoing direction matches perfect reflection (within tolerance)
		if wo.Subtract(reflected).Length() < 0.001 {
			// Calculate Fresnel reflectance
			cosTheta := math.Abs(wiDotN)
			// For reflection, refractive index ratio depends on which side we're on
			var etaRatio float64
			if hit.FrontFace {
				etaRatio = 1.0 / d.RefractiveIndex // Air to glass
			} else {
				etaRatio = d.RefractiveIndex // Glass to air
			}

			fresnel := Reflectance(cosTheta, etaRatio)

			// Transport mode doesn't affect reflection for dielectrics
			return core.Vec3{X: fresnel, Y: fresnel, Z: fresnel}
		}
	} else {
		// Refraction case - this is where transport mode matters
		var etaRatio float64
		var cosTheta float64

		if hit.FrontFace {
			// Ray entering material from air side (air to glass)
			// For transport mode scaling, use material's refractive index
			etaRatio = 1.0 / d.RefractiveIndex // Still needed for Snell's law in refractVector
			cosTheta = math.Abs(wiDotN)
		} else {
			// Ray exiting material to air side (glass to air)
			// For transport mode scaling, use inverted refractive index
			etaRatio = d.RefractiveIndex // Still needed for Snell's law in refractVector
			cosTheta = math.Abs(wiDotN)
		}

		// Check if outgoing direction matches perfect refraction
		refracted := refractVector(wi, n, etaRatio)
		if wo.Subtract(refracted).Length() < 0.001 {
			fresnel := Reflectance(cosTheta, etaRatio)
			transmission := 1.0 - fresnel

			// Transport mode correction for refraction
			// PBRT: Account for non-symmetry with transmission to different medium
			if mode == Radiance {
				// Radiance transport: divide by η² to account for solid angle compression
				// Use PBRT's etap logic: material index for front face, 1/material for back face
				var etap float64
				if hit.FrontFace {
					etap = d.RefractiveIndex // Air to glass: use material index
				} else {
					etap = 1.0 / d.RefractiveIndex // Glass to air: use inverted index
				}
				transmission /= etap * etap
			}
			// For importance transport, no additional scaling needed

			return core.Vec3{X: transmission, Y: transmission, Z: transmission}
		}
	}

	// No contribution for non-perfect directions
	return core.Vec3{X: 0, Y: 0, Z: 0}
}

// PDF calculates the probability density function for specific incoming/outgoing directions
func (d *Dielectric) PDF(incomingDir, outgoingDir, normal core.Vec3) (float64, bool) {
	// For specular materials: always return (0.0, true) indicating delta function
	// This is consistent with scatter.PDF = 0 and matches PBRT approach
	return 0.0, true
}

// reflectVector calculates the reflection of a vector v off a surface with normal n
func reflectVector(v, n core.Vec3) core.Vec3 {
	// r = v - 2*dot(v,n)*n
	return v.Subtract(n.Multiply(2 * v.Dot(n)))
}

// refractVector calculates the refraction of a vector using Snell's law
func refractVector(uv, n core.Vec3, etaiOverEtat float64) core.Vec3 {
	cosTheta := math.Min(-uv.Dot(n), 1.0)
	rOutPerp := uv.Add(n.Multiply(cosTheta)).Multiply(etaiOverEtat)
	rOutParallel := n.Multiply(-math.Sqrt(math.Abs(1.0 - rOutPerp.LengthSquared())))
	return rOutPerp.Add(rOutParallel)
}

// Reflectance calculates the Fresnel reflectance using Schlick's approximation
func Reflectance(cosine, refractionRatio float64) float64 {
	// Use Schlick's approximation for reflectance
	// Calculate R0 for normal incidence
	r0 := (1 - refractionRatio) / (1 + refractionRatio)
	r0 = r0 * r0
	return r0 + (1-r0)*math.Pow(1-cosine, 5)
}
