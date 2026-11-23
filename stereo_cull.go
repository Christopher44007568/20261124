package stereocull

// CullStereo walks the BVH once for both eyes.
//
// The top levels use the binocular union frustum as a cheap quick screen:
// one AABB test rejects whole subtrees invisible to both eyes. Deeper nodes
// switch to independent per-eye AABB tests so an eye is disabled for a
// subtree it cannot see, while the other eye keeps traversing. Leaves run
// precise sphere tests only for active eyes; a leaf box fully inside an eye
// frustum accepts every sphere without a per-sphere test.
//
// The result is mathematically the same as two independent exact per-eye
// traversals (asserted in the tests), but the tree is walked once.
func (b *BVH) CullStereo(unionFr, leftFr, rightFr Frustum) (leftAcc, rightAcc, screenAcc IDSet, stats StereoStats) {
	leftAcc = NewIDSet(len(b.Items))
	rightAcc = NewIDSet(len(b.Items))
	screenAcc = NewIDSet(len(b.Items))
	const unionLevels = 3

	var walk func(idx, depth int, testLeft, testRight bool)
	walk = func(idx, depth int, testLeft, testRight bool) {
		node := b.Nodes[idx]
		stats.NodeTests++

		if depth < unionLevels {
			// Quick screen: single union AABB test near the root.
			if unionFr.TestAABB(node.Box) == CullOutside {
				stats.NodeRejectedBoth++
				return
			}
		} else {
			// Precise per-eye AABB pruning in the lower tree.
			if testLeft && leftFr.TestAABB(node.Box) == CullOutside {
				testLeft = false
			}
			if testRight && rightFr.TestAABB(node.Box) == CullOutside {
				testRight = false
			}
			if !testLeft && !testRight {
				stats.NodeRejectedBoth++
				return
			}
		}

		if node.Left < 0 {
			// Leaves always need per-eye tests to be exact.
			leftBox := leftFr.TestAABB(node.Box)
			rightBox := rightFr.TestAABB(node.Box)
			for _, ins := range b.Items[node.LeafStart : node.LeafStart+node.LeafCount] {
				stats.Union.Stats.Candidates++
				screenAcc.Add(ins.ID)
				stats.Union.Stats.Accepted++

				if leftBox != CullOutside {
					stats.PerEyeSphereTests++
					stats.Left.Candidates++
					if leftBox == CullInside ||
						leftFr.TestSphere(ins.Center, ins.Radius) != CullOutside {
						leftAcc.Add(ins.ID)
						stats.Left.Accepted++
					} else {
						stats.Left.Rejected++
					}
				}
				if rightBox != CullOutside {
					stats.PerEyeSphereTests++
					stats.Right.Candidates++
					if rightBox == CullInside ||
						rightFr.TestSphere(ins.Center, ins.Radius) != CullOutside {
						rightAcc.Add(ins.ID)
						stats.Right.Accepted++
					} else {
						stats.Right.Rejected++
					}
				}
			}
			return
		}
		walk(node.Left, depth+1, testLeft, testRight)
		walk(node.Right, depth+1, testLeft, testRight)
	}
	walk(0, 0, true, true)

	stats.Union.Stats.NodeTests = stats.NodeTests
	stats.Union.Stats.NodeRejected = stats.NodeRejectedBoth
	return leftAcc, rightAcc, screenAcc, stats
}
