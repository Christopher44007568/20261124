package hzb

import "math"

// Triangle is a world-space triangle used as occluder geometry.
type Triangle struct{ A, B, C Vec3 }

// DepthBuffer stores the nearest view-space z per pixel; +Inf means empty.
type DepthBuffer struct {
	W, H  int
	Depth []float64
}

func NewDepthBuffer(w, h int) *DepthBuffer {
	db := &DepthBuffer{W: w, H: h, Depth: make([]float64, w*h)}
	for i := range db.Depth {
		db.Depth[i] = math.Inf(1)
	}
	return db
}

// RenderDepth rasterizes triangles from cam's viewpoint, nearest z wins.
func RenderDepth(cam *Camera, tris []Triangle) *DepthBuffer {
	db := NewDepthBuffer(cam.Width, cam.Height)
	rasterize(cam, tris, func(x, y int, z float64) bool {
		i := y*db.W + x
		if z < db.Depth[i] {
			db.Depth[i] = z
		}
		return false
	})
	return db
}

// rasterize transforms triangles to view space, clips against the near plane,
// and fills them, invoking emit per covered pixel. emit returns true to stop.
func rasterize(cam *Camera, tris []Triangle, emit func(x, y int, z float64) bool) {
	for _, t := range tris {
		poly := []Vec3{cam.ToView(t.A), cam.ToView(t.B), cam.ToView(t.C)}
		poly = clipNear(poly, cam.Near)
		for i := 1; i+1 < len(poly); i++ {
			if fillTriangle(cam, poly[0], poly[i], poly[i+1], emit) {
				return
			}
		}
	}
}

// clipNear clips a view-space polygon against the plane z >= near.
func clipNear(poly []Vec3, near float64) []Vec3 {
	var out []Vec3
	n := len(poly)
	for i := 0; i < n; i++ {
		cur, prv := poly[i], poly[(i+n-1)%n]
		curIn, prvIn := cur.Z >= near, prv.Z >= near
		if curIn != prvIn {
			t := (near - prv.Z) / (cur.Z - prv.Z)
			out = append(out, prv.Add(cur.Sub(prv).Scale(t)))
		}
		if curIn {
			out = append(out, cur)
		}
	}
	return out
}

// fillTriangle fills one view-space triangle with perspective-correct z.
func fillTriangle(cam *Camera, a, b, c Vec3, emit func(x, y int, z float64) bool) bool {
	ax, ay := cam.projectView(a)
	bx, by := cam.projectView(b)
	cx, cy := cam.projectView(c)
	area := (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
	if math.Abs(area) < 1e-12 {
		return false
	}
	minX := clampi(int(math.Floor(min3(ax, bx, cx))), 0, cam.Width-1)
	maxX := clampi(int(math.Ceil(max3(ax, bx, cx))), 0, cam.Width-1)
	minY := clampi(int(math.Floor(min3(ay, by, cy))), 0, cam.Height-1)
	maxY := clampi(int(math.Ceil(max3(ay, by, cy))), 0, cam.Height-1)
	ia, ib, ic := 1/a.Z, 1/b.Z, 1/c.Z
	edge := func(x0, y0, x1, y1, px, py float64) float64 {
		return (px-x0)*(y1-y0) - (py-y0)*(x1-x0)
	}
	denom := edge(bx, by, cx, cy, ax, ay) // == edge(c,a,b) == edge(a,b,c)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			wa := edge(bx, by, cx, cy, px, py) / denom
			wb := edge(cx, cy, ax, ay, px, py) / denom
			wc := edge(ax, ay, bx, by, px, py) / denom
			if wa < 0 || wb < 0 || wc < 0 {
				continue
			}
			invZ := wa*ia + wb*ib + wc*ic
			if invZ <= 0 {
				continue
			}
			z := 1 / invZ
			if z > cam.Far {
				continue
			}
			if emit(x, y, z) {
				return true
			}
		}
	}
	return false
}

func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
