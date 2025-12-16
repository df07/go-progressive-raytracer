package server

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/geometry"
	"github.com/df07/go-progressive-raytracer/pkg/lights"
	"github.com/df07/go-progressive-raytracer/pkg/material"
	"github.com/df07/go-progressive-raytracer/pkg/scene"
)

// InspectResponse represents the JSON response for object inspection
type InspectResponse struct {
	Hit          bool                   `json:"hit"`
	MaterialType string                 `json:"materialType"`
	GeometryType string                 `json:"geometryType"`
	Point        [3]float64             `json:"point"`
	Normal       [3]float64             `json:"normal"`
	Distance     float64                `json:"distance"`
	FrontFace    bool                   `json:"frontFace"`
	Properties   map[string]interface{} `json:"properties"`
}

// extractMaterialInfo extracts detailed material information with type assertions
func (s *Server) extractMaterialInfo(mat material.Material) (string, map[string]interface{}) {
	properties := make(map[string]interface{})

	// Check specific material types using type assertions
	switch m := mat.(type) {
	case *material.Lambertian:
		properties["albedo"] = [3]float64{m.Albedo.X, m.Albedo.Y, m.Albedo.Z}
		properties["color"] = fmt.Sprintf("#%02x%02x%02x",
			int(m.Albedo.X*255), int(m.Albedo.Y*255), int(m.Albedo.Z*255))
		return "lambertian", properties

	case *material.Metal:
		properties["albedo"] = [3]float64{m.Albedo.X, m.Albedo.Y, m.Albedo.Z}
		properties["color"] = fmt.Sprintf("#%02x%02x%02x",
			int(m.Albedo.X*255), int(m.Albedo.Y*255), int(m.Albedo.Z*255))
		properties["fuzzness"] = m.Fuzzness
		return "metal", properties

	case *material.Dielectric:
		properties["refractiveIndex"] = m.RefractiveIndex
		properties["color"] = "#ffffff" // Clear glass
		return "dielectric", properties

	case *material.Emissive:
		properties["emission"] = [3]float64{m.Emission.X, m.Emission.Y, m.Emission.Z}
		properties["color"] = fmt.Sprintf("#%02x%02x%02x",
			int(m.Emission.X*255), int(m.Emission.Y*255), int(m.Emission.Z*255))
		return "emissive", properties

	case *material.Layered:
		// For layered materials, show info about both layers
		outerType, outerProps := s.extractMaterialInfo(m.Outer)
		innerType, innerProps := s.extractMaterialInfo(m.Inner)
		properties["outer"] = map[string]interface{}{
			"type":       outerType,
			"properties": outerProps,
		}
		properties["inner"] = map[string]interface{}{
			"type":       innerType,
			"properties": innerProps,
		}
		return "layered", properties

	case *material.Mix:
		// For mixed materials, show info about both materials and the mix ratio
		material1Type, material1Props := s.extractMaterialInfo(m.Material1)
		material2Type, material2Props := s.extractMaterialInfo(m.Material2)
		properties["material1"] = map[string]interface{}{
			"type":       material1Type,
			"properties": material1Props,
		}
		properties["material2"] = map[string]interface{}{
			"type":       material2Type,
			"properties": material2Props,
		}
		properties["ratio"] = m.Ratio
		properties["description"] = fmt.Sprintf("%.0f%% %s, %.0f%% %s",
			(1-m.Ratio)*100, material1Type, m.Ratio*100, material2Type)
		return "mixed", properties

	default:
		// Check if it's emissive using interface
		if emitter, ok := mat.(material.Emitter); ok {
			emission := emitter.Emit(core.NewRay(core.NewVec3(0, 0, 0), core.NewVec3(1, 0, 0)), nil)
			properties["emission"] = [3]float64{emission.X, emission.Y, emission.Z}
			properties["color"] = fmt.Sprintf("#%02x%02x%02x",
				int(emission.X*255), int(emission.Y*255), int(emission.Z*255))
			return "emissive", properties
		}
		return "unknown", properties
	}
}

// InspectResult contains rich information about an object hit by an inspection ray
type InspectResult struct {
	Hit       bool
	HitRecord *material.SurfaceInteraction // Full hit record with material reference
	Shape     geometry.Shape               // The actual shape that was hit
}

// inspectPixel casts a ray through the specified pixel coordinates and returns information about the first object hit
func inspectPixel(sceneObj *scene.Scene, width, height, pixelX, pixelY int) InspectResult {
	// Get camera and create a ray for the pixel center (no jitter for inspection)
	camera := sceneObj.Camera

	// Preprocess scene to build the bvh
	if err := sceneObj.Preprocess(); err != nil {
		return InspectResult{Hit: false}
	}

	// Create a deterministic random generator for ray generation
	// This ensures we get a consistent ray through the pixel center
	sampler := core.NewRandomSampler(rand.New(rand.NewSource(0)))
	ray := camera.GetRay(pixelX, pixelY, sampler.Get2D(), sampler.Get2D())

	// Cast the ray and find the first intersection using scene's BVH
	hit, isHit := sceneObj.BVH.Hit(ray, 0.001, math.Inf(1))
	if !isHit {
		return InspectResult{Hit: false}
	}

	// Find the specific shape that was hit by testing all shapes
	// (BVH doesn't return the shape, just the hit record)
	shapes := sceneObj.Shapes
	for _, shape := range shapes {
		if shapeHit, shapeIsHit := shape.Hit(ray, 0.001, hit.T+0.001); shapeIsHit {
			if shapeHit.T == hit.T { // Same intersection
				return InspectResult{
					Hit:       true,
					HitRecord: hit,
					Shape:     shape,
				}
			}
		}
	}

	// Fallback: return hit without specific shape
	return InspectResult{
		Hit:       true,
		HitRecord: hit,
		Shape:     nil,
	}
}

// extractGeometryInfo extracts detailed geometry information
func (s *Server) extractGeometryInfo(shape geometry.Shape) (string, map[string]interface{}) {
	properties := make(map[string]interface{})

	switch geom := shape.(type) {
	case *geometry.Sphere:
		properties["center"] = [3]float64{geom.Center.X, geom.Center.Y, geom.Center.Z}
		properties["radius"] = geom.Radius
		return "sphere", properties

	case *geometry.Quad:
		properties["corner"] = [3]float64{geom.Corner.X, geom.Corner.Y, geom.Corner.Z}
		properties["u"] = [3]float64{geom.U.X, geom.U.Y, geom.U.Z}
		properties["v"] = [3]float64{geom.V.X, geom.V.Y, geom.V.Z}
		properties["normal"] = [3]float64{geom.Normal.X, geom.Normal.Y, geom.Normal.Z}
		return "quad", properties

	// Note: Planes removed, replaced with finite quads

	case *lights.SphereLight:
		properties["center"] = [3]float64{geom.Center.X, geom.Center.Y, geom.Center.Z}
		properties["radius"] = geom.Radius
		return "sphere_light", properties

	case *lights.QuadLight:
		properties["corner"] = [3]float64{geom.Corner.X, geom.Corner.Y, geom.Corner.Z}
		properties["u"] = [3]float64{geom.U.X, geom.U.Y, geom.U.Z}
		properties["v"] = [3]float64{geom.V.X, geom.V.Y, geom.V.Z}
		properties["normal"] = [3]float64{geom.Normal.X, geom.Normal.Y, geom.Normal.Z}
		properties["area"] = geom.Area
		return "quad_light", properties

	case *geometry.TriangleMesh:
		properties["triangleCount"] = geom.GetTriangleCount()
		bbox := geom.BoundingBox()
		properties["boundingBox"] = map[string]interface{}{
			"min": [3]float64{bbox.Min.X, bbox.Min.Y, bbox.Min.Z},
			"max": [3]float64{bbox.Max.X, bbox.Max.Y, bbox.Max.Z},
		}
		return "triangle_mesh", properties

	case *geometry.Cylinder:
		properties["baseCenter"] = [3]float64{geom.BaseCenter.X, geom.BaseCenter.Y, geom.BaseCenter.Z}
		properties["topCenter"] = [3]float64{geom.TopCenter.X, geom.TopCenter.Y, geom.TopCenter.Z}
		properties["radius"] = geom.Radius
		return "cylinder", properties

	case *geometry.Cone:
		properties["baseCenter"] = [3]float64{geom.BaseCenter.X, geom.BaseCenter.Y, geom.BaseCenter.Z}
		properties["baseRadius"] = geom.BaseRadius
		properties["topCenter"] = [3]float64{geom.TopCenter.X, geom.TopCenter.Y, geom.TopCenter.Z}
		properties["topRadius"] = geom.TopRadius
		if geom.TopRadius == 0 {
			properties["type"] = "pointed"
		} else {
			properties["type"] = "frustum"
		}
		return "cone", properties

	default:
		return "unknown", properties
	}
}

// handleInspect handles ray casting inspection requests
func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create request object for parameter parsing
	inspectReq := &RenderRequest{}

	// Parse common scene parameters using shared function
	if err := s.parseCommonSceneParams(r, inspectReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid scene parameters: " + err.Error()})
		return
	}

	// Parse pixel coordinates
	pixelX, err := strconv.Atoi(r.URL.Query().Get("x"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid x coordinate"})
		return
	}

	pixelY, err := strconv.Atoi(r.URL.Query().Get("y"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid y coordinate"})
		return
	}

	// Validate pixel coordinates
	if pixelX < 0 || pixelX >= inspectReq.Width || pixelY < 0 || pixelY >= inspectReq.Height {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Pixel coordinates out of bounds"})
		return
	}

	const configOnly = true
	sceneObj := s.createScene(inspectReq, configOnly, nil)
	if sceneObj == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Unknown scene: " + inspectReq.Scene})
		return
	}

	// Perform the inspection using the scene directly
	result := inspectPixel(sceneObj, inspectReq.Width, inspectReq.Height, pixelX, pixelY)

	// Convert to JSON response
	if !result.Hit {
		response := InspectResponse{Hit: false}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Extract detailed information
	materialType, materialProps := s.extractMaterialInfo(result.HitRecord.Material)
	geometryType, geometryProps := s.extractGeometryInfo(result.Shape)

	// Combine properties
	allProperties := make(map[string]interface{})
	allProperties["material"] = materialProps
	allProperties["geometry"] = geometryProps

	response := InspectResponse{
		Hit:          true,
		MaterialType: materialType,
		GeometryType: geometryType,
		Point:        [3]float64{result.HitRecord.Point.X, result.HitRecord.Point.Y, result.HitRecord.Point.Z},
		Normal:       [3]float64{result.HitRecord.Normal.X, result.HitRecord.Normal.Y, result.HitRecord.Normal.Z},
		Distance:     result.HitRecord.T,
		FrontFace:    result.HitRecord.FrontFace,
		Properties:   allProperties,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
