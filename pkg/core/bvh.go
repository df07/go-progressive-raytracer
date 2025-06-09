package core

import (
	"sort"
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
	Root *BVHNode
}

// NewBVH constructs a BVH from a slice of shapes
func NewBVH(shapes []Shape) *BVH {
	if len(shapes) == 0 {
		return &BVH{Root: nil}
	}

	// Make a copy of the shapes slice to avoid modifying the original
	// This is crucial for thread safety when multiple workers build BVHs concurrently
	shapesCopy := make([]Shape, len(shapes))
	copy(shapesCopy, shapes)

	return &BVH{
		Root: buildBVH(shapesCopy, 0),
	}
}

// Leaf threshold: if we have this many or fewer shapes, store them in a leaf node
const leafThreshold = 8

// buildBVH recursively builds the BVH using a simple but fast method with leaf thresholding
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
	// This uses efficient linear search for small groups
	if len(shapes) <= leafThreshold {
		return &BVHNode{
			BoundingBox: boundingBox,
			Shapes:      shapes, // Store all shapes in leaf for linear search
		}
	}

	// For larger groups, use simple median split along longest axis
	// This is much faster than SAH and still gives good results for regular grids
	axis := boundingBox.LongestAxis()
	sortShapesByAxis(shapes, axis)

	// Split in the middle
	mid := len(shapes) / 2
	leftShapes := shapes[:mid]
	rightShapes := shapes[mid:]

	return &BVHNode{
		BoundingBox: boundingBox,
		Left:        buildBVH(leftShapes, depth+1),
		Right:       buildBVH(rightShapes, depth+1),
	}
}

// sortShapesByAxis sorts shapes by their bounding box center along the specified axis
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
func (bvh *BVH) Hit(ray Ray, tMin, tMax float64) (*HitRecord, bool) {
	if bvh.Root == nil {
		return nil, false
	}
	return bvh.hitNode(bvh.Root, ray, tMin, tMax)
}

// hitNode recursively tests ray intersection with BVH nodes
func (bvh *BVH) hitNode(node *BVHNode, ray Ray, tMin, tMax float64) (*HitRecord, bool) {
	// First check if ray hits the bounding box
	if !node.BoundingBox.Hit(ray, tMin, tMax) {
		return nil, false
	}

	// If this is a leaf node, test against all shapes using linear search
	if node.Shapes != nil {
		var closestHit *HitRecord
		hitAnything := false
		closestSoFar := tMax

		// Linear search through all shapes in the leaf
		for _, shape := range node.Shapes {
			if hit, isHit := shape.Hit(ray, tMin, closestSoFar); isHit {
				hitAnything = true
				closestSoFar = hit.T
				closestHit = hit
			}
		}

		return closestHit, hitAnything
	}

	// Internal node - test both children
	var closestHit *HitRecord
	hitAnything := false
	closestSoFar := tMax

	// Test left child
	if node.Left != nil {
		if hit, isHit := bvh.hitNode(node.Left, ray, tMin, closestSoFar); isHit {
			hitAnything = true
			closestSoFar = hit.T
			closestHit = hit
		}
	}

	// Test right child
	if node.Right != nil {
		if hit, isHit := bvh.hitNode(node.Right, ray, tMin, closestSoFar); isHit {
			hitAnything = true
			closestSoFar = hit.T
			closestHit = hit
		}
	}

	return closestHit, hitAnything
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
