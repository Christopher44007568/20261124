package offaxis

import "math"

// Probe is one sampled eye-space point with all three independent verdicts
// attached: clip-space edge markers, the analytic frustum classification,
// and (for inside probes) the depth value derived two separate ways.
type Probe struct {
	Point        Vec3
	EdgeMask     uint8   // rasterizer-style homogeneous clip markers
	Violated     uint8   // analytic plane bitmask
	Boundary     uint8   // analytic boundary plane bitmask
	WindowDepth  float64 // depth from the projection matrix, NaN when outside
	DepthFromEye float64 // depth rebuilt analytically from eye-space z
	Inside       bool
}

// ProbeGrid samples a rectangular NDC lattice at one fixed eye-space depth.
// Rows run top to bottom so callers can print the result directly. Points
// deliberately overscan [-1,1] so outside and boundary cases are included.
func ProbeGrid(p Mat4, b Bounds, depth float64, cols, rows int) ([]Probe, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	planes, err := b.AnalyticPlanes()
	if err != nil {
		return nil, err
	}
	const span = 1.25
	probes := make([]Probe, 0, cols*rows)
	for row := 0; row < rows; row++ {
		ndcY := span - 2*span*float64(row)/float64(rows-1)
		for col := 0; col < cols; col++ {
			ndcX := -span + 2*span*float64(col)/float64(cols-1)
			eye, ok := p.Unproject(ndcX, ndcY, depth)
			if !ok {
				return nil, ErrCenterRay
			}
			probes = append(probes, classifyProbe(p, planes, eye))
		}
	}
	return probes, nil
}

func classifyProbe(p Mat4, planes []Plane, eye Vec3) Probe {
	violated, boundary := Classify(planes, eye, 1e-7)
	clip := p.TransformPoint(eye)
	probe := Probe{
		Point:    eye,
		EdgeMask: MarkEdges(clip),
		Violated: violated,
		Boundary: boundary,
		Inside:   violated == 0,
	}
	if probe.Inside && clip.W > 0 {
		ndc := clip.NDC()
		probe.WindowDepth = ndc.Z*0.5 + 0.5
		probe.DepthFromEye = analyticWindowDepth(eye, planes)
	} else {
		probe.WindowDepth = math.NaN()
		probe.DepthFromEye = math.NaN()
	}
	return probe
}

// analyticWindowDepth converts an inside eye-space point to window depth
// using only the near/far planes, independently of the projection matrix.
func analyticWindowDepth(eye Vec3, planes []Plane) float64 {
	n := -planes[PlaneNear].D // plane D is -near
	f := planes[PlaneFar].D   // plane D is +far
	d := -eye.Z
	return (1/n - 1/d) / (1/n - 1/f)
}

// FrustumCorners returns the eight eye-space corners of the frustum.
func (b Bounds) FrustumCorners() ([]Vec3, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	xs := [2]float64{b.Left, b.Right}
	ys := [2]float64{b.Bottom, b.Top}
	zs := [2]float64{-b.Near, -b.Far}
	corners := make([]Vec3, 0, 8)
	for _, z := range zs {
		scale := -z / b.Near
		for _, y := range ys {
			for _, x := range xs {
				corners = append(corners, Vec3{X: x * scale, Y: y * scale, Z: z})
			}
		}
	}
	return corners, nil
}
