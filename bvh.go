package stereocull

import "sort"

// Instance is a renderable object approximated by a bounding sphere.
type Instance struct {
	ID     int
	Name   string
	Center Vec3
	Radius float64
}

// Bounds returns the axis-aligned box of the bounding sphere.
func (ins Instance) Bounds() AABB {
	h := Vec3{X: ins.Radius, Y: ins.Radius, Z: ins.Radius}
	return AABB{Min: ins.Center.Sub(h), Max: ins.Center.Add(h)}
}

// BVH is an implicit-ish binary tree stored as a slice for cache locality.
type BVH struct {
	Nodes []BVHNode
	Items []Instance
	index map[int]int
}

// BVHNode is either an internal node (Leaf < 0) or a leaf holding a contiguous
// range of BVH.Items.
type BVHNode struct {
	Box       AABB
	Left      int
	Right     int
	LeafStart int
	LeafCount int
}

// BuildBVH constructs the tree using longest-axis median splits.
func BuildBVH(items []Instance, leafSize int) *BVH {
	if leafSize < 1 {
		leafSize = 1
	}
	b := &BVH{Items: append([]Instance(nil), items...)}
	b.Nodes = make([]BVHNode, 0, len(items)*2)
	b.build(0, len(b.Items), leafSize)
	b.index = make(map[int]int, len(b.Items))
	for i, ins := range b.Items {
		b.index[ins.ID] = i
	}
	return b
}

// ByID returns the instance with the given ID.
func (b *BVH) ByID(id int) Instance { return b.Items[b.index[id]] }

func (b *BVH) build(start, count, leafSize int) int {
	box := boundsOf(b.Items[start : start+count])
	nodeIdx := len(b.Nodes)
	b.Nodes = append(b.Nodes, BVHNode{Box: box, Left: -1, Right: -1, LeafStart: start, LeafCount: count})
	if count <= leafSize {
		return nodeIdx
	}

	axis := longestAxis(box)
	slice := b.Items[start : start+count]
	sort.SliceStable(slice, func(i, j int) bool {
		return axisCoord(slice[i].Center, axis) < axisCoord(slice[j].Center, axis)
	})
	mid := count / 2
	left := b.build(start, mid, leafSize)
	right := b.build(start+mid, count-mid, leafSize)
	b.Nodes[nodeIdx].Left = left
	b.Nodes[nodeIdx].Right = right
	b.Nodes[nodeIdx].LeafStart = 0
	b.Nodes[nodeIdx].LeafCount = 0
	return nodeIdx
}

// TraversalStats records how much work one cull pass performed.
type TraversalStats struct {
	NodeTests    int
	NodeRejected int
	SphereTests  int
	Candidates   int // leaf instances that reached a frustum test
	Accepted     int // instances not outside (inside or intersecting)
	Rejected     int // instances fully outside
}

// StereoStats holds per-eye work counters for a union-guided stereo pass.
type StereoStats struct {
	Left              TraversalStats
	Right             TraversalStats
	Union             EyeResult
	NodeTests         int
	NodeRejectedBoth  int
	PerEyeSphereTests int
}

// Cull runs one frustum over the BVH and returns accepted instance IDs plus
// traversal statistics.
func (b *BVH) Cull(fr Frustum) (IDSet, TraversalStats) {
	stats := TraversalStats{}
	accepted := NewIDSet(len(b.Items))
	var walk func(idx int)
	walk = func(idx int) {
		node := b.Nodes[idx]
		stats.NodeTests++
		if fr.TestAABB(node.Box) == CullOutside {
			stats.NodeRejected++
			return
		}
		if node.Left < 0 {
			for _, ins := range b.Items[node.LeafStart : node.LeafStart+node.LeafCount] {
				stats.SphereTests++
				stats.Candidates++
				if fr.TestSphere(ins.Center, ins.Radius) == CullOutside {
					stats.Rejected++
					continue
				}
				stats.Accepted++
				accepted.Add(ins.ID)
			}
			return
		}
		walk(node.Left)
		walk(node.Right)
	}
	walk(0)
	return accepted, stats
}

func boundsOf(items []Instance) AABB {
	box := items[0].Bounds()
	for _, ins := range items[1:] {
		b := ins.Bounds()
		box.Min = minVec(box.Min, b.Min)
		box.Max = maxVec(box.Max, b.Max)
	}
	return box
}

func minVec(a, b Vec3) Vec3 {
	return Vec3{X: minFloat(a.X, b.X), Y: minFloat(a.Y, b.Y), Z: minFloat(a.Z, b.Z)}
}

func maxVec(a, b Vec3) Vec3 {
	return Vec3{X: maxFloat(a.X, b.X), Y: maxFloat(a.Y, b.Y), Z: maxFloat(a.Z, b.Z)}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func longestAxis(b AABB) int {
	ext := b.Max.Sub(b.Min)
	switch {
	case ext.X >= ext.Y && ext.X >= ext.Z:
		return 0
	case ext.Y >= ext.Z:
		return 1
	default:
		return 2
	}
}

func axisCoord(v Vec3, axis int) float64 {
	switch axis {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}
