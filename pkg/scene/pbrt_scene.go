package scene

import (
	"fmt"
	"strconv"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/loaders"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// NewPBRTScene creates a scene from a PBRT file
func NewPBRTScene(filepath string, cameraOverrides ...geometry.CameraConfig) (*Scene, error) {
	// Parse PBRT file
	pbrtScene, err := loaders.LoadPBRT(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load PBRT file: %v", err)
	}

	// Create scene
	scene := &Scene{
		Shapes:         make([]geometry.Shape, 0),
		Lights:         make([]lights.Light, 0),
		SamplingConfig: createDefaultPBRTSamplingConfig(),
	}

	// Convert camera
	if err := convertCamera(pbrtScene, scene, cameraOverrides...); err != nil {
		return nil, fmt.Errorf("failed to convert camera: %v", err)
	}

	// Convert all materials first
	materials := make([]material.Material, len(pbrtScene.Materials))
	for i, matStmt := range pbrtScene.Materials {
		mat, err := convertMaterial(&matStmt)
		if err != nil {
			return nil, fmt.Errorf("failed to convert material: %v", err)
		}
		materials[i] = mat
	}

	// Process shapes using their assigned material index
	for _, shapeStmt := range pbrtScene.Shapes {
		var shapeMaterial material.Material
		if shapeStmt.MaterialIndex >= 0 && shapeStmt.MaterialIndex < len(materials) {
			shapeMaterial = materials[shapeStmt.MaterialIndex]
		} else {
			return nil, fmt.Errorf("shape has no valid material (MaterialIndex: %d)", shapeStmt.MaterialIndex)
		}

		shape, err := convertShape(&shapeStmt, shapeMaterial)
		if err != nil {
			return nil, fmt.Errorf("failed to convert shape: %v", err)
		}
		if shape != nil {
			scene.Shapes = append(scene.Shapes, shape)
		}
	}

	// Process top-level lights
	for _, lightStmt := range pbrtScene.LightSources {
		light, err := convertLight(&lightStmt)
		if err != nil {
			return nil, fmt.Errorf("failed to convert light: %v", err)
		}
		if light != nil {
			scene.Lights = append(scene.Lights, light)
		}
	}

	// Process attribute blocks
	for _, attrBlock := range pbrtScene.Attributes {
		if err := processAttributeBlock(&attrBlock, scene, materials); err != nil {
			return nil, fmt.Errorf("failed to process attribute block: %v", err)
		}
	}

	return scene, nil
}

// createDefaultPBRTSamplingConfig creates default sampling configuration
func createDefaultPBRTSamplingConfig() SamplingConfig {
	return SamplingConfig{
		Width:                     400,
		Height:                    400,
		SamplesPerPixel:           100,
		MaxDepth:                  5,
		RussianRouletteMinBounces: 3,
		AdaptiveMinSamples:        0.25,
		AdaptiveThreshold:         0.01,
	}
}

// convertCamera converts PBRT camera to our camera system
func convertCamera(pbrtScene *loaders.PBRTScene, scene *Scene, cameraOverrides ...geometry.CameraConfig) error {
	// Default camera config
	cameraConfig := geometry.CameraConfig{
		Center:        core.NewVec3(0, 0, 0),
		LookAt:        core.NewVec3(0, 0, -1),
		Up:            core.NewVec3(0, 1, 0),
		Width:         400,
		AspectRatio:   1.0,
		VFov:          90.0,
		Aperture:      0.0,
		FocusDistance: 1.0,
	}

	// Apply LookAt if present
	if pbrtScene.LookAt != nil && pbrtScene.LookAtTo != nil && pbrtScene.LookAtUp != nil {
		cameraConfig.Center = *pbrtScene.LookAt
		cameraConfig.LookAt = *pbrtScene.LookAtTo
		cameraConfig.Up = *pbrtScene.LookAtUp
	}

	// Apply camera parameters if present
	if pbrtScene.Camera != nil {
		if pbrtScene.Camera.Subtype == "perspective" {
			if fov, ok := pbrtScene.Camera.GetFloatParam("fov"); ok {
				if fov <= 0 || fov >= 180 {
					return fmt.Errorf("invalid camera FOV %f: must be between 0 and 180 degrees", fov)
				}
				cameraConfig.VFov = fov
			}
		}
	}

	// Apply film parameters if present
	if pbrtScene.Film != nil {
		if width, ok := pbrtScene.Film.GetFloatParam("xresolution"); ok {
			if width <= 0 || width > 8192 {
				return fmt.Errorf("invalid image width %f: must be between 1 and 8192", width)
			}
			cameraConfig.Width = int(width)
			scene.SamplingConfig.Width = int(width)
		}
		if height, ok := pbrtScene.Film.GetFloatParam("yresolution"); ok {
			if height <= 0 || height > 8192 {
				return fmt.Errorf("invalid image height %f: must be between 1 and 8192", height)
			}
			scene.SamplingConfig.Height = int(height)
			cameraConfig.AspectRatio = float64(cameraConfig.Width) / height
		}
	}

	// Apply camera overrides if provided
	if len(cameraOverrides) > 0 {
		cameraConfig = geometry.MergeCameraConfig(cameraConfig, cameraOverrides[0])
		// Update sampling config dimensions if width/aspect ratio were overridden
		if cameraOverrides[0].Width > 0 {
			scene.SamplingConfig.Width = cameraOverrides[0].Width
			scene.SamplingConfig.Height = int(float64(cameraOverrides[0].Width) / cameraConfig.AspectRatio)
		}
	}

	scene.CameraConfig = cameraConfig
	scene.Camera = geometry.NewCamera(cameraConfig)

	return nil
}

// convertMaterial converts a PBRT material to our material system
func convertMaterial(stmt *loaders.PBRTStatement) (material.Material, error) {
	switch stmt.Subtype {
	case "diffuse":
		// Get reflectance (albedo)
		if rgb, ok := stmt.GetRGBParam("reflectance"); ok {
			return material.NewLambertian(*rgb), nil
		}
		// Default white diffuse
		return material.NewLambertian(core.NewVec3(0.7, 0.7, 0.7)), nil

	case "conductor":
		// Metal material
		albedo := core.NewVec3(0.7, 0.6, 0.5) // Default metal color
		if rgb, ok := stmt.GetRGBParam("eta"); ok {
			albedo = *rgb
		}

		fuzz := 0.0
		if roughness, ok := stmt.GetFloatParam("roughness"); ok {
			if roughness < 0 || roughness > 1 {
				return nil, fmt.Errorf("invalid metal roughness %f: must be between 0 and 1", roughness)
			}
			fuzz = roughness
		}

		return material.NewMetal(albedo, fuzz), nil

	case "dielectric":
		// Glass material
		ior := 1.5 // Default glass IOR
		if eta, ok := stmt.GetFloatParam("eta"); ok {
			if eta <= 0 {
				return nil, fmt.Errorf("invalid dielectric IOR %f: must be positive", eta)
			}
			ior = eta
		}
		return material.NewDielectric(ior), nil

	default:
		return nil, fmt.Errorf("unsupported material type: %s", stmt.Subtype)
	}
}

// convertShape converts a PBRT shape to our shape system
func convertShape(stmt *loaders.PBRTStatement, mat material.Material) (geometry.Shape, error) {
	if mat == nil {
		return nil, fmt.Errorf("shape has no material")
	}

	switch stmt.Subtype {
	case "sphere":
		radius := 1.0
		if r, ok := stmt.GetFloatParam("radius"); ok {
			if r <= 0 {
				return nil, fmt.Errorf("invalid sphere radius %f: must be positive", r)
			}
			radius = r
		}

		center := core.NewVec3(0, 0, 0)
		return geometry.NewSphere(center, radius, mat), nil

	case "bilinearPatch":
		// PBRT bilinear patch -> our Quad
		p00, ok1 := stmt.GetPoint3Param("P00")
		p01, ok2 := stmt.GetPoint3Param("P01")
		p10, ok3 := stmt.GetPoint3Param("P10")
		_, ok4 := stmt.GetPoint3Param("P11")

		if !ok1 || !ok2 || !ok3 || !ok4 {
			return nil, fmt.Errorf("bilinearPatch missing corner points")
		}

		// Convert to our quad format: corner + two edge vectors
		// P00 is corner, u = P01-P00, v = P10-P00
		corner := *p00
		u := p01.Subtract(*p00)
		v := p10.Subtract(*p00)

		return geometry.NewQuad(corner, u, v, mat), nil

	case "trianglemesh":
		// Get vertices
		param, exists := stmt.Parameters["P"]
		if !exists || len(param.Values)%3 != 0 {
			return nil, fmt.Errorf("trianglemesh missing or invalid vertices")
		}

		vertices := make([]core.Vec3, 0, len(param.Values)/3)
		for i := 0; i < len(param.Values); i += 3 {
			x, err1 := strconv.ParseFloat(param.Values[i], 64)
			if err1 != nil {
				return nil, fmt.Errorf("invalid vertex X coordinate '%s': %v", param.Values[i], err1)
			}
			y, err2 := strconv.ParseFloat(param.Values[i+1], 64)
			if err2 != nil {
				return nil, fmt.Errorf("invalid vertex Y coordinate '%s': %v", param.Values[i+1], err2)
			}
			z, err3 := strconv.ParseFloat(param.Values[i+2], 64)
			if err3 != nil {
				return nil, fmt.Errorf("invalid vertex Z coordinate '%s': %v", param.Values[i+2], err3)
			}
			vertices = append(vertices, core.NewVec3(x, y, z))
		}

		// Get indices
		indicesParam, exists := stmt.Parameters["indices"]
		if !exists || len(indicesParam.Values)%3 != 0 {
			return nil, fmt.Errorf("trianglemesh missing or invalid indices")
		}

		indices := make([]int, 0, len(indicesParam.Values))
		for _, idxStr := range indicesParam.Values {
			idx, _ := strconv.Atoi(idxStr)
			indices = append(indices, idx)
		}

		return geometry.NewTriangleMesh(vertices, indices, mat, nil), nil

	default:
		return nil, fmt.Errorf("unsupported shape type: %s", stmt.Subtype)
	}
}

// convertLight converts a PBRT light to our light system
func convertLight(stmt *loaders.PBRTStatement) (lights.Light, error) {
	switch stmt.Subtype {
	case "point":
		intensity := core.NewVec3(10, 10, 10) // Default intensity
		if rgb, ok := stmt.GetRGBParam("I"); ok {
			intensity = *rgb
		}

		position := core.NewVec3(0, 5, 0) // Default position
		if pos, ok := stmt.GetPoint3Param("from"); ok {
			position = *pos
		}

		// Use sphere light as point light approximation with emissive material
		emissiveMat := material.NewEmissive(intensity)
		return lights.NewSphereLight(position, 0.1, emissiveMat), nil

	case "distant":
		radiance := core.NewVec3(3, 3, 3) // Default radiance
		if rgb, ok := stmt.GetRGBParam("L"); ok {
			radiance = *rgb
		}

		// Use uniform infinite light as distant light approximation
		// (Direction is ignored since we don't have directional lights)
		return lights.NewUniformInfiniteLight(radiance), nil

	case "infinite":
		radiance := core.NewVec3(1, 1, 1) // Default background
		if rgb, ok := stmt.GetRGBParam("L"); ok {
			radiance = *rgb
		}

		return lights.NewUniformInfiniteLight(radiance), nil

	case "infinite-gradient":
		topColor := core.NewVec3(0.5, 0.7, 1.0)    // Default blue sky
		bottomColor := core.NewVec3(1.0, 1.0, 1.0) // Default white horizon

		if rgb, ok := stmt.GetRGBParam("topColor"); ok {
			topColor = *rgb
		}
		if rgb, ok := stmt.GetRGBParam("bottomColor"); ok {
			bottomColor = *rgb
		}

		return lights.NewGradientInfiniteLight(topColor, bottomColor), nil

	default:
		return nil, fmt.Errorf("unsupported light type: %s", stmt.Subtype)
	}
}

// processAttributeBlock processes an AttributeBegin/AttributeEnd block
func processAttributeBlock(block *loaders.AttributeBlock, scene *Scene, globalMaterials []material.Material) error {
	// Convert local materials in this block
	localMaterials := make([]material.Material, len(block.Materials))
	for i, matStmt := range block.Materials {
		mat, err := convertMaterial(&matStmt)
		if err != nil {
			return fmt.Errorf("failed to convert material in attribute block: %v", err)
		}
		localMaterials[i] = mat
	}

	// Process shapes in this block
	for _, shapeStmt := range block.Shapes {
		var shapeMaterial material.Material

		// Determine which material to use based on MaterialIndex
		if shapeStmt.MaterialIndex >= 0 && shapeStmt.MaterialIndex < len(localMaterials) {
			// Use local material from this attribute block
			shapeMaterial = localMaterials[shapeStmt.MaterialIndex]
		} else if shapeStmt.MaterialIndex >= 0 && shapeStmt.MaterialIndex < len(globalMaterials) {
			// Use global material
			shapeMaterial = globalMaterials[shapeStmt.MaterialIndex]
		} else {
			return fmt.Errorf("shape has no valid material (MaterialIndex: %d, local materials: %d, global materials: %d)",
				shapeStmt.MaterialIndex, len(localMaterials), len(globalMaterials))
		}

		// Check for area lights and override material if present
		for _, lightStmt := range block.LightSources {
			if lightStmt.Type == "AreaLightSource" {
				if rgb, ok := lightStmt.GetRGBParam("L"); ok {
					shapeMaterial = material.NewEmissive(*rgb)
					break // Only handle one area light per attribute block
				}
			}
		}

		shape, err := convertShape(&shapeStmt, shapeMaterial)
		if err != nil {
			return fmt.Errorf("failed to convert shape in attribute block: %v", err)
		}
		if shape != nil {
			scene.Shapes = append(scene.Shapes, shape)
		}
	}

	// Process lights in this block (including AreaLightSource)
	for _, lightStmt := range block.LightSources {
		if lightStmt.Type == "AreaLightSource" {
			// Area light - we need to process this BEFORE shapes to set emissive material
			// For now, this is handled in the main processing flow above
			// AreaLightSource creates emissive shapes, not separate light objects
		} else {
			light, err := convertLight(&lightStmt)
			if err != nil {
				return fmt.Errorf("failed to convert light in attribute block: %v", err)
			}
			if light != nil {
				scene.Lights = append(scene.Lights, light)
			}
		}
	}

	return nil
}
