// Package render is a tiny software rasterizer. It draws the synthetic scene
// without any graphics SDK: plain Go loops, a depth buffer and triangle
// rasterization only.
package render

import (
	"math"

	"stereoval/internal/geo"
	"stereoval/internal/scene"
)

// Frame is one rendered grayscale eye image plus a camera-space depth map.
type Frame struct {
	W, H  int
	Gray  []float64 // NaN where no surface was drawn (hole)
	Depth []float64 // +Inf where no surface was drawn
}

func newFrame(w, h int) *Frame {
	g := make([]float64, w*h)
	d := make([]float64, w*h)
	for i := range g {
		g[i] = math.NaN()
		d[i] = math.Inf(1)
	}
	return &Frame{W: w, H: h, Gray: g, Depth: d}
}

func (f *Frame) idx(x, y int) int { return y*f.W + x }

type vtx struct {
	u, v float64  // screen pixel
	z    float64  // camera depth
	attr geo.Vec3 // world position
}

// Render rasterizes the scene from one eye pose.
func Render(s *scene.Scene, k geo.Intrinsics, pose geo.Pose) *Frame {
	f := newFrame(k.W, k.H)

	drawPanel(f, k, pose, s.Wall)
	for _, p := range s.Panels {
		drawPanel(f, k, pose, p)
	}
	for _, m := range s.Markers {
		drawMarker(f, k, pose, m)
	}
	return f
}

func projectVtx(k geo.Intrinsics, pose geo.Pose, w geo.Vec3) (vtx, bool) {
	c := pose.ToCamera(w)
	u, v, z, ok := k.Project(c)
	if !ok {
		return vtx{}, false
	}
	return vtx{u: u, v: v, z: z, attr: w}, true
}

func drawPanel(f *Frame, k geo.Intrinsics, pose geo.Pose, p scene.Panel) {
	corners := make([]vtx, 0, 4)
	for i := 0; i < 4; i++ {
		v, ok := projectVtx(k, pose, p.Corner(i))
		if !ok {
			return // behind camera: skip whole panel for this frame
		}
		corners = append(corners, v)
	}
	tris := [][3]int{{0, 1, 2}, {0, 2, 3}}
	for _, t := range tris {
		rasterTriangle(f, k, p, corners[t[0]], corners[t[1]], corners[t[2]])
	}
}

func rasterTriangle(f *Frame, k geo.Intrinsics, p scene.Panel, a, b, c vtx) {
	minX := max(0, int(math.Floor(math.Min(a.u, math.Min(b.u, c.u)))))
	maxX := min(f.W-1, int(math.Ceil(math.Max(a.u, math.Max(b.u, c.u)))))
	minY := max(0, int(math.Floor(math.Min(a.v, math.Min(b.v, c.v)))))
	maxY := min(f.H-1, int(math.Ceil(math.Max(a.v, math.Max(b.v, c.v)))))

	den := edge(a, b, c)
	if math.Abs(den) < 1e-12 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			w0 := edge(b, c, vtx{u: px, v: py}) / den
			w1 := edge(c, a, vtx{u: px, v: py}) / den
			w2 := edge(a, b, vtx{u: px, v: py}) / den
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			// Perspective-correct interpolation of world position.
			ia := w0 / a.z
			ib := w1 / b.z
			ic := w2 / c.z
			invZ := ia + ib + ic
			w := geo.Vec3{
				X: (ia*a.attr.X + ib*b.attr.X + ic*c.attr.X) / invZ,
				Y: (ia*a.attr.Y + ib*b.attr.Y + ic*c.attr.Y) / invZ,
				Z: (ia*a.attr.Z + ib*b.attr.Z + ic*c.attr.Z) / invZ,
			}
			z := 1 / invZ
			i := f.idx(x, y)
			if z >= f.Depth[i] {
				continue
			}
			u, vv, inside := p.LocalUV(w)
			if !inside {
				continue
			}
			f.Depth[i] = z
			f.Gray[i] = scene.Shade(p, u, vv)
		}
	}
}

func edge(a, b, p vtx) float64 {
	return (p.u-a.u)*(b.v-a.v) - (p.v-a.v)*(b.u-a.u)
}

func drawMarker(f *Frame, k geo.Intrinsics, pose geo.Pose, m scene.Marker) {
	center, ok := projectVtx(k, pose, m.Center)
	if !ok {
		return
	}
	edgePt, ok := projectVtx(k, pose, geo.Add(m.Center, geo.Vec3{X: m.Radius}))
	if !ok {
		return
	}
	r := math.Abs(edgePt.u - center.u)
	if r < 1 {
		r = 1
	}
	minX := max(0, int(center.u-r))
	maxX := min(f.W-1, int(center.u+r))
	minY := max(0, int(center.v-r))
	maxY := min(f.H-1, int(center.v+r))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x) + 0.5 - center.u
			dy := float64(y) + 0.5 - center.v
			d := math.Hypot(dx, dy)
			// Sub-pixel coverage: interior pixels are fully white, boundary
			// pixels are partial so the rendered intensity centroid is
			// unbiased relative to the projected center.
			cov := 1.0
			switch {
			case d > r+0.5:
				continue
			case d > r-0.5:
				cov = clamp((r + 0.5 - d), 0, 1)
			}
			i := f.idx(x, y)
			// Markers sit on their carrier surface at the same depth, so an
			// equal-depth marker must still overwrite the panel texture.
			if center.z > f.Depth[i]+1e-4 {
				continue
			}
			f.Depth[i] = center.z
			base := f.Gray[i]
			if math.IsNaN(base) {
				base = 0
			}
			f.Gray[i] = base*(1-cov) + 255*cov
		}
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
