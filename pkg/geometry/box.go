package geometry

import (
	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// Box represents a rectangular box made up of 6 quads with optional rotation
type Box struct {
	Center   core.Vec3         // Center point of the box
	Size     core.Vec3         // Size along each axis (width, height, depth)
	Rotation core.Vec3         // Rotation angles in radians (X, Y, Z)
	Material material.Material // Material for all faces
	faces    [6]*Quad          // The 6 quad faces
	bbox     AABB              // Cached bounding box
}

// NewBox creates a new box with the given center, size, rotation, and material
// Size represents half-extents (so a size of (1,1,1) creates a 2x2x2 box)
// Rotation is in radians around X, Y, Z axes (applied in that order)
func NewBox(center, size, rotation core.Vec3, material material.Material) *Box {
	box := &Box{
		Center:   center,
		Size:     size,
		Rotation: rotation,
		Material: material,
	}

	// Generate the 6 faces
	box.generateFaces()

	return box
}

// NewAxisAlignedBox creates a new axis-aligned box (no rotation)
func NewAxisAlignedBox(center, size core.Vec3, material material.Material) *Box {
	return NewBox(center, size, core.NewVec3(0, 0, 0), material)
}

// generateFaces creates the 6 quad faces of the box
func (b *Box) generateFaces() {
	// Define the 8 corners of a unit box centered at origin
	corners := [8]core.Vec3{
		core.NewVec3(-1, -1, -1), // 0: left-bottom-back
		core.NewVec3(1, -1, -1),  // 1: right-bottom-back
		core.NewVec3(1, 1, -1),   // 2: right-top-back
		core.NewVec3(-1, 1, -1),  // 3: left-top-back
		core.NewVec3(-1, -1, 1),  // 4: left-bottom-front
		core.NewVec3(1, -1, 1),   // 5: right-bottom-front
		core.NewVec3(1, 1, 1),    // 6: right-top-front
		core.NewVec3(-1, 1, 1),   // 7: left-top-front
	}

	// Scale corners by size and apply rotation
	for i := range corners {
		// Scale by size
		corners[i] = core.NewVec3(
			corners[i].X*b.Size.X,
			corners[i].Y*b.Size.Y,
			corners[i].Z*b.Size.Z,
		)
		// Apply rotation
		corners[i] = corners[i].Rotate(b.Rotation)
		// Translate to center
		corners[i] = corners[i].Add(b.Center)
	}

	// Create the 6 faces using the transformed corners
	// Each face is defined by a corner and two edge vectors

	// Front face (Z+): 4-5-6-7
	b.faces[0] = NewQuad(
		corners[4],                      // corner
		corners[5].Subtract(corners[4]), // u vector (right)
		corners[7].Subtract(corners[4]), // v vector (up)
		b.Material,
	)

	// Back face (Z-): 1-0-3-2
	b.faces[1] = NewQuad(
		corners[1],                      // corner
		corners[0].Subtract(corners[1]), // u vector (left)
		corners[2].Subtract(corners[1]), // v vector (up)
		b.Material,
	)

	// Right face (X+): 5-1-2-6
	b.faces[2] = NewQuad(
		corners[5],                      // corner
		corners[1].Subtract(corners[5]), // u vector (back)
		corners[6].Subtract(corners[5]), // v vector (up)
		b.Material,
	)

	// Left face (X-): 0-4-7-3
	b.faces[3] = NewQuad(
		corners[0],                      // corner
		corners[4].Subtract(corners[0]), // u vector (front)
		corners[3].Subtract(corners[0]), // v vector (up)
		b.Material,
	)

	// Top face (Y+): 3-7-6-2
	b.faces[4] = NewQuad(
		corners[3],                      // corner
		corners[7].Subtract(corners[3]), // u vector (front)
		corners[2].Subtract(corners[3]), // v vector (right)
		b.Material,
	)

	// Bottom face (Y-): 4-0-1-5
	b.faces[5] = NewQuad(
		corners[4],                      // corner
		corners[0].Subtract(corners[4]), // u vector (back)
		corners[5].Subtract(corners[4]), // v vector (right)
		b.Material,
	)

	// Calculate bounding box from all corners
	b.bbox = NewAABBFromPoints(corners[0], corners[1], corners[2], corners[3],
		corners[4], corners[5], corners[6], corners[7])
}

// Hit tests if a ray intersects with any face of the box
func (b *Box) Hit(ray core.Ray, tMin, tMax float64) (*material.HitRecord, bool) {
	var closestHit *material.HitRecord
	closestT := tMax

	// Test intersection with all 6 faces
	for _, face := range b.faces {
		if hit, isHit := face.Hit(ray, tMin, closestT); isHit {
			if hit.T < closestT {
				closestT = hit.T
				closestHit = hit
			}
		}
	}

	return closestHit, closestHit != nil
}

// BoundingBox returns the axis-aligned bounding box for this box
func (b *Box) BoundingBox() AABB {
	return b.bbox
}
