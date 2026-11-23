package main

import "math"

// scenario is a named scene builder.
type scenario struct {
	name  string
	build func(nextID *int) []*Node
}

func leaf(id *int, name string, local Mat4, center, half Vec3) *Node {
	n := &Node{Name: name, Local: local, Instance: &Instance{ID: *id, Name: name, Box: OBB{Center: center, Half: half}}}
	*id++
	return n
}

// scenarioEdgeGrazing places boxes tangent to the side planes of the
// left eye's frustum, slightly inside and slightly outside (擦边).
func scenarioEdgeGrazing() scenario {
	return scenario{"擦边-侧缘", func(id *int) []*Node {
		const eyeLx = -ipd / 2
		const half, epsIn, epsOut, zd = 0.5, 0.4, 0.9, 5.0
		hh := tanHalfH * zd // horizontal half-extent at depth zd
		hv := tanHalfV * zd // vertical half-extent at depth zd
		box := func(name string, c Vec3) *Node {
			return leaf(id, name, Identity(), c, Vec3{half, half, half})
		}
		return []*Node{
			box("graze-left-in", Vec3{eyeLx - hh + half - epsIn, 0, -zd}),
			box("graze-left-out", Vec3{eyeLx - hh - half - epsOut, 0, -zd}),
			box("graze-right-in", Vec3{eyeLx + hh - half + epsIn, 0, -zd}),
			box("graze-right-out", Vec3{eyeLx + hh + half + epsOut, 0, -zd}),
			box("graze-top-in", Vec3{0, hv - half + epsIn, -zd}),
			box("graze-top-out", Vec3{0, hv + half + epsOut, -zd}),
			box("graze-bottom-in", Vec3{0, -hv + half - epsIn, -zd}),
			box("graze-bottom-out", Vec3{0, -hv - half - epsOut, -zd}),
		}
	}}
}

// scenarioNearFarGrazing grazes the near and far planes. Kept apart from
// the side-plane boxes: anything crossing the near plane subtends a huge
// angle and would occlude the other grazing boxes.
func scenarioNearFarGrazing() scenario {
	return scenario{"擦边-近远平面", func(id *int) []*Node {
		const eps = 0.4
		small := func(name string, c Vec3, h float64) *Node {
			return leaf(id, name, Identity(), c, Vec3{h, h, h})
		}
		return []*Node{
			small("graze-near-in", Vec3{0, 0, -(nearDist + 0.05 - 0.02)}, 0.05),
			small("graze-near-out", Vec3{0, 0, -(nearDist - 0.05 - 0.02)}, 0.05),
			// Far boxes are big and high up so the near box (which fans
			// out over the screen center) cannot hide them.
			small("graze-far-in", Vec3{60, 73, -(farDist - 8 + eps)}, 8),
			small("graze-far-out", Vec3{60, 73, -(farDist + 8 + eps)}, 8),
		}
	}}
}

// scenarioSingleEyeOcclusion hides a target from the left eye behind a
// wall while the right eye sees around it, and adds frustum-only boxes.
func scenarioSingleEyeOcclusion() scenario {
	return scenario{"单眼遮挡", func(id *int) []*Node {
		return []*Node{
			leaf(id, "occluder-wall", Identity(), Vec3{-0.42, 0, -2}, Vec3{0.14, 2, 0.05}),
			leaf(id, "hidden-target", Identity(), Vec3{-1.0, 0, -6}, Vec3{0.3, 0.3, 0.3}),
			// Thin slivers sitting in the wedge between the two eyes'
			// frusta: fully inside one eye's frustum, fully outside
			// the other's (the wedge is only ~ipd wide).
			leaf(id, "left-frustum-only", Identity(), Vec3{-4.5, 0, -3}, Vec3{0.03, 0.5, 0.03}),
			leaf(id, "right-frustum-only", Identity(), Vec3{4.5, 0, -3}, Vec3{0.03, 0.5, 0.03}),
			leaf(id, "reference-box", Identity(), Vec3{0.5, 0.3, -4}, Vec3{0.3, 0.3, 0.3}),
		}
	}}
}

// scenarioLargeCrossEye uses oversized instances that span both eyes and
// straddle the near plane (大对象跨眼/跨近平面).
func scenarioLargeCrossEye() scenario {
	return scenario{"大对象跨眼", func(id *int) []*Node {
		return []*Node{
			leaf(id, "huge-bar-cross-near", Identity(), Vec3{0, 0.1, -0.05}, Vec3{10, 0.15, 0.2}),
			leaf(id, "ground-slab", Identity(), Vec3{0, -2, -10}, Vec3{20, 0.1, 20}),
			leaf(id, "tall-pillar", Identity(), Vec3{2.5, 0, -3}, Vec3{0.8, 5, 0.8}),
		}
	}}
}

// scenarioHierarchy exercises hierarchical transforms and rotated OBBs.
func scenarioHierarchy() scenario {
	return scenario{"层级变换+旋转盒", func(id *int) []*Node {
		rig := &Node{Name: "rig", Local: Mul(Translation(1.5, 0, -5), RotationAxis(Vec3{0, 1, 0}, math.Pi/4.5))}
		rig.Children = append(rig.Children,
			leaf(id, "hinge-box", Identity(), Vec3{0, 0, 0}, Vec3{0.6, 0.2, 0.2}))
		arm := &Node{Name: "arm", Local: Mul(Translation(1.2, 0, 0), RotationAxis(Vec3{0, 0, 1}, math.Pi/6))}
		arm.Children = append(arm.Children,
			leaf(id, "arm-tip", Identity(), Vec3{0.5, 0, 0}, Vec3{0.25, 0.25, 0.25}))
		rig.Children = append(rig.Children, arm)
		return []*Node{
			rig,
			leaf(id, "spinning-top", Mul(Translation(-1.5, 1, -4), RotationAxis(Vec3{1, 1, 0}, math.Pi/3)), Vec3{0, 0, 0}, Vec3{0.4, 0.1, 0.4}),
			leaf(id, "behind-camera", Identity(), Vec3{0, 0, 2}, Vec3{0.3, 0.3, 0.3}),
			leaf(id, "far-left-out", Identity(), Vec3{-20, 0, -5}, Vec3{0.5, 0.5, 0.5}),
		}
	}}
}

func allScenarios() []scenario {
	return []scenario{
		scenarioEdgeGrazing(),
		scenarioNearFarGrazing(),
		scenarioSingleEyeOcclusion(),
		scenarioLargeCrossEye(),
		scenarioHierarchy(),
	}
}
