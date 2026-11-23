package main

import (
	"math"
	"sort"
)

type kdNode struct {
	point       Vec3
	index       int
	axis        int
	left, right *kdNode
}

type kdTree struct {
	root *kdNode
}

type pointIndex struct {
	point Vec3
	index int
}

func buildKDTree(points []Vec3) *kdTree {
	items := make([]pointIndex, len(points))
	for index, point := range points {
		items[index] = pointIndex{point: point, index: index}
	}
	return &kdTree{root: buildKDNode(items, 0)}
}

func buildKDNode(items []pointIndex, depth int) *kdNode {
	if len(items) == 0 {
		return nil
	}
	axis := depth % 3
	sort.Slice(items, func(i, j int) bool {
		return dimensionAt(items[i].point, axis) < dimensionAt(items[j].point, axis)
	})
	middle := len(items) / 2
	return &kdNode{
		point: items[middle].point,
		index: items[middle].index,
		axis:  axis,
		left:  buildKDNode(items[:middle], depth+1),
		right: buildKDNode(items[middle+1:], depth+1),
	}
}

func dimensionAt(point Vec3, axis int) float64 {
	switch axis {
	case 0:
		return point.X
	case 1:
		return point.Y
	default:
		return point.Z
	}
}

func (tree *kdTree) nearest(point Vec3) (Vec3, int, float64) {
	bestPoint := Vec3{}
	bestIndex := -1
	bestDistance := math.Inf(1)
	var visit func(node *kdNode)
	visit = func(node *kdNode) {
		if node == nil {
			return
		}
		distance := point.Sub(node.point).Length()
		if distance < bestDistance {
			bestDistance = distance
			bestPoint = node.point
			bestIndex = node.index
		}

		delta := dimensionAt(point, node.axis) - dimensionAt(node.point, node.axis)
		first, second := node.left, node.right
		if delta > 0 {
			first, second = node.right, node.left
		}
		visit(first)
		if math.Abs(delta) < bestDistance {
			visit(second)
		}
	}
	visit(tree.root)
	return bestPoint, bestIndex, bestDistance
}
