package offaxis

import "math"

// Mismatch describes one probe where the independent pipelines disagree.
type Mismatch struct {
	Index    int
	Point    Vec3
	Edge     uint8
	Analytic uint8
	Kind     string
}

// DepthCheck compares the matrix-derived window depth against the analytic
// 1/z depth for every inside probe.
type DepthCheck struct {
	Count       int
	MaxAbsError float64
}

// CrossCheck runs the three-way comparison for a probe slice and returns
// any disagreements together with the depth reconciliation statistics.
func CrossCheck(probes []Probe) ([]Mismatch, DepthCheck) {
	var mismatches []Mismatch
	var depth DepthCheck
	for i, pr := range probes {
		if pr.EdgeMask != pr.Violated {
			mismatches = append(mismatches, Mismatch{
				Index: i, Point: pr.Point,
				Edge: pr.EdgeMask, Analytic: pr.Violated,
				Kind: "edge-mask vs analytic planes",
			})
		}
		if !pr.Inside {
			continue
		}
		depth.Count++
		if err := math.Abs(pr.WindowDepth - pr.DepthFromEye); err > depth.MaxAbsError {
			depth.MaxAbsError = err
		}
		if pr.WindowDepth < -1e-9 || pr.WindowDepth > 1+1e-9 {
			mismatches = append(mismatches, Mismatch{
				Index: i, Point: pr.Point, Kind: "depth outside [0,1]",
			})
		}
	}
	return mismatches, depth
}

// CompareFramebufferDepth reconciles rasterized window depths against the
// analytic formula for every covered pixel whose eye-space point is known.
// It returns the raw covered count, the number of pixels reconciled inside
// the frustum, and the maximum depth error over those pixels.
func CompareFramebufferDepth(fb *Framebuffer, eyeAt func(x, y int) (Vec3, bool), planes []Plane) (covered, reconciled int, maxErr float64) {
	for y := fb.Viewport.Y; y < fb.Viewport.Y+fb.Viewport.Height; y++ {
		for x := fb.Viewport.X; x < fb.Viewport.X+fb.Viewport.Width; x++ {
			idx := fb.index(x, y)
			if !fb.Covered[idx] {
				continue
			}
			covered++
			eye, ok := eyeAt(x, y)
			// Clipped fragments can sit a few ulps outside the near plane,
			// so use a slightly looser epsilon than the point classifier.
			if !ok || !Inside(planes, eye, 1e-6) {
				continue
			}
			want := analyticWindowDepth(eye, planes)
			reconciled++
			if err := math.Abs(want - fb.Depth[idx]); err > maxErr {
				maxErr = err
			}
		}
	}
	return covered, reconciled, maxErr
}

// PlaneSetAgreement reports the maximum normalized difference between the
// bound-derived planes and the matrix-extracted planes.
func PlaneSetAgreement(b Bounds, p Mat4) (float64, error) {
	analytic, err := b.AnalyticPlanes()
	if err != nil {
		return 0, err
	}
	for i := range analytic {
		n := analytic[i].Normal.Length()
		analytic[i].Normal = analytic[i].Normal.Scale(1 / n)
		analytic[i].D /= n
	}
	extracted := p.ExtractPlanes()
	worst := 0.0
	for i := range analytic {
		a := analytic[i]
		e := extracted[i]
		diff := math.Abs(a.Normal.X-e.Normal.X) + math.Abs(a.Normal.Y-e.Normal.Y) +
			math.Abs(a.Normal.Z-e.Normal.Z) + math.Abs(a.D-e.D)
		if diff > worst {
			worst = diff
		}
	}
	return worst, nil
}
