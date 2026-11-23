package main

import "math"

// Camera describes one eye of the stereo rig.
type Camera struct {
	Eye    Vec3
	View   Mat4
	Proj   Mat4
	Near   float64
	Far    float64
	FovY   float64
	Aspect float64
}

// Framebuffer holds the depth buffer and per-pixel winning instance.
type Framebuffer struct {
	W, H  int
	Depth []float64
	Owner []int
}

func newFramebuffer(w, h int) *Framebuffer {
	fb := &Framebuffer{W: w, H: h, Depth: make([]float64, w*h), Owner: make([]int, w*h)}
	for i := range fb.Depth {
		fb.Depth[i] = 1e30
		fb.Owner[i] = -1
	}
	return fb
}

// boxTriangles lists the 12 triangles of a unit box as corner indices
// (corner i = (sx, sy, sz) bits: x = i>>2&1, y = i>>1&1, z = i&1).
var boxTriangles = [12][3]int{
	{4, 6, 7}, {4, 7, 5}, // x-
	{0, 1, 3}, {0, 3, 2}, // x+
	{0, 4, 5}, {0, 5, 1}, // y-
	{2, 3, 7}, {2, 7, 6}, // y+
	{0, 2, 6}, {0, 6, 4}, // z-
	{1, 5, 7}, {1, 7, 3}, // z+
}

// clipNear clips a camera-space polygon against the near plane (keep z <= -near).
func clipNear(poly []Vec3, near float64) []Vec3 {
	var out []Vec3
	n := len(poly)
	for i := 0; i < n; i++ {
		cur, prev := poly[i], poly[(i+n-1)%n]
		curIn, prevIn := cur.Z <= -near, prev.Z <= -near
		if curIn != prevIn {
			t := (-near - prev.Z) / (cur.Z - prev.Z)
			out = append(out, prev.Add(cur.Sub(prev).Scale(t)))
		}
		if curIn {
			out = append(out, cur)
		}
	}
	return out
}

// RasterizeEye renders all instances into a small framebuffer and returns
// per-instance visible pixel counts (ground-truth coverage for one eye).
func RasterizeEye(cam Camera, bound []BoundInstance, w, h int) (map[int]int, *Framebuffer) {
	fb := newFramebuffer(w, h)
	coverage := map[int]int{}
	f := 1 / math.Tan(cam.FovY/2)
	for _, b := range bound {
		corners := b.Corners()
		var camPts [8]Vec3
		for i, p := range corners {
			camPts[i] = cam.View.MulPoint(p)
		}
		for _, tri := range boxTriangles {
			poly := []Vec3{camPts[tri[0]], camPts[tri[1]], camPts[tri[2]]}
			poly = clipNear(poly, cam.Near)
			for k := 1; k+1 < len(poly); k++ {
				rasterTri(fb, cam, f, poly[0], poly[k], poly[k+1], b.Inst.ID, coverage)
			}
		}
	}
	return coverage, fb
}

// rasterTri rasterizes one camera-space triangle with a z-buffer test.
func rasterTri(fb *Framebuffer, cam Camera, f float64, a, b, c Vec3, id int, coverage map[int]int) {
	project := func(p Vec3) (sx, sy, invZ float64) {
		invZ = 1 / -p.Z
		ndcX := f / cam.Aspect * p.X * invZ
		ndcY := f * p.Y * invZ
		sx = (ndcX*0.5 + 0.5) * float64(fb.W)
		sy = (0.5 - ndcY*0.5) * float64(fb.H)
		return sx, sy, invZ
	}
	ax, ay, az := project(a)
	bx, by, bz := project(b)
	cx, cy, cz := project(c)
	edge := func(x0, y0, x1, y1, px, py float64) float64 {
		return (x1-x0)*(py-y0) - (y1-y0)*(px-x0)
	}
	area := edge(ax, ay, bx, by, cx, cy)
	if area > -1e-12 && area < 1e-12 {
		return
	}
	minX, maxX := min3(ax, bx, cx), max3(ax, bx, cx)
	minY, maxY := min3(ay, by, cy), max3(ay, by, cy)
	x0 := clampi(int(minX)-1, 0, fb.W-1)
	x1 := clampi(int(maxX)+1, 0, fb.W-1)
	y0 := clampi(int(minY)-1, 0, fb.H-1)
	y1 := clampi(int(maxY)+1, 0, fb.H-1)
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			fx, fy := float64(px)+0.5, float64(py)+0.5
			w0 := edge(bx, by, cx, cy, fx, fy) / area
			w1 := edge(cx, cy, ax, ay, fx, fy) / area
			w2 := edge(ax, ay, bx, by, fx, fy) / area
			if w0 < 0 || w1 < 0 || w2 < 0 {
				continue
			}
			invZ := w0*az + w1*bz + w2*cz
			if invZ <= 0 {
				continue
			}
			depth := 1 / invZ
			if depth > cam.Far {
				continue
			}
			idx := py*fb.W + px
			if depth < fb.Depth[idx] {
				fb.Depth[idx] = depth
				if fb.Owner[idx] >= 0 {
					coverage[fb.Owner[idx]]--
				}
				fb.Owner[idx] = id
				coverage[id]++
			}
		}
	}
}

func min3(a, b, c float64) float64 { return minf(minf(a, b), c) }
func max3(a, b, c float64) float64 { return maxf(maxf(a, b), c) }
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
