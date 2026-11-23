package main

import (
	"errors"
	"fmt"

	"offaxis/pkg/offaxis"
)

type scenario struct {
	name   string
	bounds offaxis.Bounds
	depth  float64 // probe depth
	vp     offaxis.Viewport
	// triangles exercise near-plane penetration and off-screen geometry.
	triangles [][3]offaxis.Vec3
}

func scenarios() []scenario {
	return []scenario{
		{
			name:   "1. asymmetric off-axis frustum (left/right FOV differ)",
			bounds: offaxis.Bounds{Left: -1.7, Right: 0.9, Bottom: -1.1, Top: 0.6, Near: 2, Far: 12},
			depth:  5,
			vp:     offaxis.Viewport{Width: 20, Height: 12},
			triangles: [][3]offaxis.Vec3{
				{
					{X: -1.2, Y: -0.9, Z: -5},
					{X: 1.0, Y: -0.9, Z: -5},
					{X: 0.2, Y: 0.8, Z: -5},
				},
			},
		},
		{
			name:   "2. near-plane penetration (one vertex behind the camera)",
			bounds: offaxis.Bounds{Left: -1, Right: 1, Bottom: -1, Top: 1, Near: 2, Far: 10},
			depth:  4,
			vp:     offaxis.Viewport{Width: 16, Height: 16},
			triangles: [][3]offaxis.Vec3{
				{
					{X: -1.5, Y: 1.5, Z: 1.5},
					{X: -1.5, Y: -1.5, Z: -6},
					{X: 1.5, Y: -1.5, Z: -6},
				},
			},
		},
		{
			name:   "3. aspect ratio change (wide frustum, non-square viewport)",
			bounds: offaxis.Bounds{Left: -2.4, Right: 2.0, Bottom: -0.6, Top: 0.8, Near: 1, Far: 20},
			depth:  6,
			vp:     offaxis.Viewport{Width: 28, Height: 8},
			triangles: [][3]offaxis.Vec3{
				{
					{X: -2, Y: -0.4, Z: -6},
					{X: 1.8, Y: -0.4, Z: -6},
					{X: 0, Y: 0.6, Z: -6},
				},
			},
		},
		{
			name:   "4. degenerate corner (left == right): must be rejected",
			bounds: offaxis.Bounds{Left: 1, Right: 1, Bottom: -1, Top: 1, Near: 2, Far: 10},
			depth:  4,
			vp:     offaxis.Viewport{Width: 12, Height: 12},
		},
	}
}

func runScenario(s scenario) bool {
	fmt.Printf("\n=== %s ===\n", s.name)
	printBounds(s.bounds)
	m, err := s.bounds.ProjectionMatrix()
	if err != nil {
		fmt.Printf("REJECTED: %v\n", err)
		return errors.Is(err, offaxis.ErrDegenerate) ||
			errors.Is(err, offaxis.ErrReversedDepth) ||
			errors.Is(err, offaxis.ErrInvalidDepth)
	}
	printMatrix("projection matrix (row-major, glFrustum convention):", m)

	planes, _ := s.bounds.AnalyticPlanes()
	agreement, err := offaxis.PlaneSetAgreement(s.bounds, m)
	if err != nil {
		fmt.Printf("plane agreement failed: %v\n", err)
		return false
	}
	fmt.Printf("analytic vs matrix-extracted plane max diff: %.2e\n", agreement)

	probes, err := offaxis.ProbeGrid(m, s.bounds, s.depth, s.vp.Width, s.vp.Height)
	if err != nil {
		fmt.Printf("probe grid failed: %v\n", err)
		return false
	}
	printProbeGrid(probes, s.vp.Width)
	printDepthGrid(probes, s.vp.Width)

	mismatches, depth := offaxis.CrossCheck(probes)
	if len(mismatches) != 0 {
		fmt.Printf("CROSS-CHECK FAILED: %d mismatches\n", len(mismatches))
		return false
	}
	fmt.Printf("cross-check: edge masks == analytic planes on %d probes; depth max err %.2e\n",
		len(probes), depth.MaxAbsError)

	fb := offaxis.NewFramebuffer(s.vp)
	for _, tri := range s.triangles {
		fb.DrawTriangle(m, s.vp, tri[0], tri[1], tri[2])
	}
	printFramebuffer(fb)
	eyeAt := func(x, y int) (offaxis.Vec3, bool) {
		ndcX := (float64(x)+0.5-float64(s.vp.X))/float64(s.vp.Width)*2 - 1
		ndcY := 1 - (float64(y)+0.5-float64(s.vp.Y))/float64(s.vp.Height)*2
		idx := fb.IndexOf(x, y)
		// Recover the actual eye-space depth of this fragment from its
		// window depth instead of assuming the probe-plane depth.
		ndcZ := fb.Depth[idx]*2 - 1
		n, f := s.bounds.Near, s.bounds.Far
		depth := 2 * n * f / (f + n - ndcZ*(f-n))
		return m.Unproject(ndcX, ndcY, depth)
	}
	covered, reconciled, maxErr := offaxis.CompareFramebufferDepth(fb, eyeAt, planes)
	fmt.Printf("rasterizer: %d covered, %d reconciled; depth vs analytic max err %.2e\n",
		covered, reconciled, maxErr)
	return maxErr < 1e-9 && agreement < 1e-9
}

func printFramebuffer(fb *offaxis.Framebuffer) {
	vp := fb.Viewport
	fmt.Println("framebuffer (# = covered fragment, o = edge boundary, . = empty):")
	for y := vp.Y; y < vp.Y+vp.Height; y++ {
		var line []byte
		line = append(line, ' ', ' ')
		for x := vp.X; x < vp.X+vp.Width; x++ {
			idx := fb.IndexOf(x, y)
			switch {
			case fb.Boundary[idx] && fb.Covered[idx]:
				line = append(line, 'o')
			case fb.Covered[idx]:
				line = append(line, '#')
			default:
				line = append(line, '.')
			}
			line = append(line, ' ')
		}
		fmt.Println(string(line))
	}
}
