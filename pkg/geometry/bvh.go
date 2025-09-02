package geometry

import (
	"sort"

	"github.com/df07/go-progressive-raytracer/pkg/core"
	"github.com/df07/go-progressive-raytracer/pkg/material"
)

// BVHNode represents a node in the Bounding Volume Hierarchy
type BVHNode struct {
	BoundingBox AABB
	Left        *BVHNode
	Right       *BVHNode
	Shapes      []Shape // Multiple shapes for leaf nodes (nil for internal nodes)
}

// BVH represents a Bounding Volume Hierarchy for fast ray-object intersection
type BVH struct {
	Root   *BVHNode
	Center core.Vec3 // Precomputed finite scene center for infinite light calculations
	Radius float64   // Precomputed world radius for infinite light PDF calculations
}

// NewBVH constructs a BVH from a slice of shapes
func NewBVH(shapes []Shape) *BVH {
	if len(shapes) == 0 {
		return &BVH{Root: nil, Center: core.Vec3{}, Radius: 0}
	}

	// Make a copy of the shapes slice to avoid modifying the original
	// This is crucial for thread safety when multiple workers build BVHs concurrently
	shapesCopy := make([]Shape, len(shapes))
	copy(shapesCopy, shapes)

	root := buildBVH(shapesCopy, 0)

	// Use the root BVH node's bounding box for world bounds (no need to recalculate)
	var worldCenter core.Vec3
	var worldRadius float64
	if root != nil {
		worldCenter = root.BoundingBox.Center()
		worldRadius = root.BoundingBox.Max.Subtract(worldCenter).Length()
	} else {
		// Empty scene fallback
		worldCenter = core.Vec3{}
		worldRadius = 100.0
	}

	return &BVH{
		Root:   root,
		Center: worldCenter,
		Radius: worldRadius,
	}
}

// Leaf threshold: if we have this many or fewer shapes, store them in a leaf node
const leafThreshold = 8

// buildBVH recursively builds the BVH using fast median splitting
// This approach avoids the expensive O(n² log n) sorting bottleneck by using
// simple median splits along the longest axis, providing ~7-8x speedup over
// the previous sorting-based approach while maintaining good ray intersection performance.
func buildBVH(shapes []Shape, depth int) *BVHNode {
	// Calculate bounding box for all shapes
	var boundingBox AABB
	if len(shapes) > 0 {
		boundingBox = shapes[0].BoundingBox()
		for i := 1; i < len(shapes); i++ {
			boundingBox = boundingBox.Union(shapes[i].BoundingBox())
		}
	}

	// Base case: few shapes - create leaf node with all shapes
	if len(shapes) <= leafThreshold {
		return &BVHNode{
			BoundingBox: boundingBox,
			Shapes:      shapes,
		}
	}

	// Find best split using simplified binned approach (much faster than sorting)
	bestAxis, splitPos := findBestSplitSimple(shapes, boundingBox)

	// If we couldn't find a good split, create a leaf
	if bestAxis == -1 {
		return &BVHNode{
			BoundingBox: boundingBox,
			Shapes:      shapes,
		}
	}

	// Partition shapes based on the best split
	leftShapes, rightShapes := partitionShapesSimple(shapes, bestAxis, splitPos)

	// Ensure we don't create empty partitions
	if len(leftShapes) == 0 || len(rightShapes) == 0 {
		return &BVHNode{
			BoundingBox: boundingBox,
			Shapes:      shapes,
		}
	}

	return &BVHNode{
		BoundingBox: boundingBox,
		Left:        buildBVH(leftShapes, depth+1),
		Right:       buildBVH(rightShapes, depth+1),
	}
}

// findBestSplitSimple finds the best axis and split position using simple binned median
func findBestSplitSimple(shapes []Shape, boundingBox AABB) (bestAxis int, splitPos float64) {
	bestAxis = boundingBox.LongestAxis()

	// Get the extent along the best axis
	var minVal, maxVal float64
	switch bestAxis {
	case 0:
		minVal, maxVal = boundingBox.Min.X, boundingBox.Max.X
	case 1:
		minVal, maxVal = boundingBox.Min.Y, boundingBox.Max.Y
	case 2:
		minVal, maxVal = boundingBox.Min.Z, boundingBox.Max.Z
	}

	// Skip if no extent along this axis
	if maxVal <= minVal {
		return -1, 0
	}

	// Use simple median split
	splitPos = (minVal + maxVal) * 0.5
	return bestAxis, splitPos
}

// partitionShapesSimple partitions shapes based on the chosen axis and split position
func partitionShapesSimple(shapes []Shape, axis int, splitPos float64) ([]Shape, []Shape) {
	var leftShapes, rightShapes []Shape

	for _, shape := range shapes {
		center := shape.BoundingBox().Center()
		var centerVal float64
		switch axis {
		case 0:
			centerVal = center.X
		case 1:
			centerVal = center.Y
		case 2:
			centerVal = center.Z
		}

		if centerVal < splitPos {
			leftShapes = append(leftShapes, shape)
		} else {
			rightShapes = append(rightShapes, shape)
		}
	}

	return leftShapes, rightShapes
}

// sortShapesByAxis sorts shapes by their bounding box center along the specified axis
// This function is kept for compatibility but should no longer be used in the main BVH construction
func sortShapesByAxis(shapes []Shape, axis int) {
	sort.Slice(shapes, func(i, j int) bool {
		centerI := shapes[i].BoundingBox().Center()
		centerJ := shapes[j].BoundingBox().Center()

		switch axis {
		case 0:
			return centerI.X < centerJ.X
		case 1:
			return centerI.Y < centerJ.Y
		case 2:
			return centerI.Z < centerJ.Z
		default:
			return false
		}
	})
}

// Hit tests if a ray intersects any shape in the BVH
func (bvh *BVH) Hit(ray core.Ray, tMin, tMax float64, hit *material.HitRecord) bool {
	if bvh.Root == nil {
		return false
	}
	return bvh.hitNode(bvh.Root, ray, tMin, tMax, hit)
}

// hitNode recursively tests ray intersection with BVH nodes
func (bvh *BVH) hitNode(node *BVHNode, ray core.Ray, tMin, tMax float64, hit *material.HitRecord) bool {
	// First check if ray hits the bounding box
	if !node.BoundingBox.Hit(ray, tMin, tMax) {
		return false
	}

	// If this is a leaf node, test against all shapes using linear search
	if node.Shapes != nil {
		hitAnything := false
		closestSoFar := tMax

		// Linear search through all shapes in the leaf
		for _, shape := range node.Shapes {
			if shape.Hit(ray, tMin, closestSoFar, hit) {
				hitAnything = true
				closestSoFar = hit.T
				// hit is already filled by the shape - no copying needed!
			}
		}

		return hitAnything
	}

	// Internal node - test both children
	hitAnything := false
	closestSoFar := tMax

	// Test left child
	if node.Left != nil {
		if bvh.hitNode(node.Left, ray, tMin, closestSoFar, hit) {
			hitAnything = true
			closestSoFar = hit.T
			// hit is already filled by the recursive call
		}
	}

	// Test right child
	if node.Right != nil {
		if bvh.hitNode(node.Right, ray, tMin, closestSoFar, hit) {
			hitAnything = true
			closestSoFar = hit.T
			// hit is already filled by the recursive call
		}
	}

	return hitAnything
}

// BoundingBox implements the Shape interface - returns the overall bounding box of the BVH
func (bvh *BVH) BoundingBox() AABB {
	if bvh.Root == nil {
		return AABB{}
	}
	return bvh.Root.BoundingBox
}

// getStats returns statistics about the BVH structure
func (bvh *BVH) getStats() bvhStats {
	if bvh.Root == nil {
		return bvhStats{}
	}

	stats := bvhStats{}
	bvh.collectStats(bvh.Root, 0, &stats)

	// Calculate average depth after collecting all data
	if stats.leafNodes > 0 {
		stats.avgDepth = stats.avgDepth / float64(stats.leafNodes)
	}

	return stats
}

// bvhStats contains statistics about the BVH structure
type bvhStats struct {
	totalNodes  int
	leafNodes   int
	maxDepth    int
	avgDepth    float64
	totalShapes int
}

// collectStats recursively collects statistics about the BVH
func (bvh *BVH) collectStats(node *BVHNode, depth int, stats *bvhStats) {
	stats.totalNodes++

	if depth > stats.maxDepth {
		stats.maxDepth = depth
	}

	if node.Shapes != nil {
		// Leaf node
		stats.leafNodes++
		stats.totalShapes += len(node.Shapes)
		stats.avgDepth += float64(depth) // Accumulate depth for average calculation
	} else {
		// Internal node
		if node.Left != nil {
			bvh.collectStats(node.Left, depth+1, stats)
		}
		if node.Right != nil {
			bvh.collectStats(node.Right, depth+1, stats)
		}
	}
}
