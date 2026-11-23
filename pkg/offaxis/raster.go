package offaxis

import "math"

// Viewport is the per-eye screen rectangle. Asymmetric frusta are allowed to
// use arbitrary, even zero-offset, viewports because the off-axis terms live
// in the projection matrix itself, not in the viewport.
type Viewport struct {
	X, Y          int
	Width, Height int
}

// ScreenVertex is a post-viewport vertex.
type ScreenVertex struct {
	SX, SY   float64
	Depth    float64 // window depth in [0,1]
	EdgeMask uint8
	InvW     float64 // 1 / clip-space w, for perspective-correct interpolation
}

// Framebuffer stores one eye's rasterization result.
type Framebuffer struct {
	Viewport Viewport
	Depth    []float64 // window depth, +Inf where nothing was drawn
	Covered  []bool
	Boundary []bool // pixels touching a triangle edge
}

// NewFramebuffer allocates a framebuffer for the given viewport.
func NewFramebuffer(vp Viewport) *Framebuffer {
	n := vp.Width * vp.Height
	fb := &Framebuffer{
		Viewport: vp,
		Depth:    make([]float64, n),
		Covered:  make([]bool, n),
		Boundary: make([]bool, n),
	}
	for i := range fb.Depth {
		fb.Depth[i] = math.Inf(1)
	}
	return fb
}

func (fb *Framebuffer) index(x, y int) int {
	return (y-fb.Viewport.Y)*fb.Viewport.Width + (x - fb.Viewport.X)
}

// IndexOf returns the backing-slice index for a viewport pixel.
func (fb *Framebuffer) IndexOf(x, y int) int { return fb.index(x, y) }

// ToScreen converts a clipped homogeneous vertex into screen coordinates.
func (vp Viewport) ToScreen(clip Vec4) ScreenVertex {
	ndc := clip.NDC()
	return ScreenVertex{
		SX:    float64(vp.X) + (ndc.X*0.5+0.5)*float64(vp.Width),
		SY:    float64(vp.Y) + (1-(ndc.Y*0.5+0.5))*float64(vp.Height),
		Depth: ndc.Z*0.5 + 0.5,
		InvW:  1 / clip.W,
	}
}

// DrawTriangle clips one eye-space triangle, projects the survivors and
// fills them with the Direct3D-style top-left fill rule.
func (fb *Framebuffer) DrawTriangle(p Mat4, vp Viewport, a, b, c Vec3) {
	verts := ClipPolygon([]ClipVertex{
		{Pos: p.TransformPoint(a)},
		{Pos: p.TransformPoint(b)},
		{Pos: p.TransformPoint(c)},
	})
	if len(verts) < 3 {
		return
	}
	screen := make([]ScreenVertex, len(verts))
	for i, v := range verts {
		sv := vp.ToScreen(v.Pos)
		sv.EdgeMask = v.EdgeMask
		screen[i] = sv
	}
	for i := 1; i < len(screen)-1; i++ {
		fb.fillTriangle(screen[0], screen[i], screen[i+1])
	}
}

// edgeFunc is the signed area function; positive means p is on the
// interior (left) side of the directed edge a->b.
func edgeFunc(a, b ScreenVertex, px, py float64) float64 {
	return (px-a.SX)*(b.SY-a.SY) - (py-a.SY)*(b.SX-a.SX)
}

// isTopLeft reports whether a directed edge is a top or left edge, which
// receives the inclusive half of the fill rule.
func isTopLeft(a, b ScreenVertex) bool {
	dx := b.SX - a.SX
	dy := b.SY - a.SY
	return dy > 0 || (dy == 0 && dx > 0)
}

func (fb *Framebuffer) fillTriangle(a, b, c ScreenVertex) {
	minX := int(math.Floor(math.Min(math.Min(a.SX, b.SX), c.SX)))
	maxX := int(math.Ceil(math.Max(math.Max(a.SX, b.SX), c.SX)))
	minY := int(math.Floor(math.Min(math.Min(a.SY, b.SY), c.SY)))
	maxY := int(math.Ceil(math.Max(math.Max(a.SY, b.SY), c.SY)))

	signedArea := edgeFunc(a, b, c.SX, c.SY)
	if signedArea == 0 {
		return
	}
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			w0 := edgeFunc(b, c, px, py)
			w1 := edgeFunc(c, a, px, py)
			w2 := edgeFunc(a, b, px, py)
			// Winding-independent inside test: every edge function must have
			// the same sign as the signed area. A zero value is accepted only
			// on a top or left edge (the inclusive half of the fill rule).
			if !insideEdge(w0, signedArea, b, c) ||
				!insideEdge(w1, signedArea, c, a) ||
				!insideEdge(w2, signedArea, a, b) {
				continue
			}
			if x < fb.Viewport.X || x >= fb.Viewport.X+fb.Viewport.Width ||
				y < fb.Viewport.Y || y >= fb.Viewport.Y+fb.Viewport.Height {
				continue
			}
			alpha := w0 / signedArea
			beta := w1 / signedArea
			gamma := w2 / signedArea
			// Perspective-correct: interpolate depth/w and 1/w, then divide.
			invW := alpha*a.InvW + beta*b.InvW + gamma*c.InvW
			depthOverW := alpha*(a.Depth*a.InvW) + beta*(b.Depth*b.InvW) +
				gamma*(c.Depth*c.InvW)
			depth := depthOverW / invW
			idx := fb.index(x, y)
			if depth < fb.Depth[idx] {
				fb.Depth[idx] = depth
				fb.Covered[idx] = true
			}
			if nearEdge(px, py, a, b) || nearEdge(px, py, b, c) ||
				nearEdge(px, py, c, a) {
				fb.Boundary[idx] = true
			}
		}
	}
}

// insideEdge implements a winding-independent half-space test. A pixel is on
// the interior side when its edge value w shares the sign of the triangle's
// signed area. The zero case is accepted exclusively for top/left edges.
func insideEdge(w, signedArea float64, a, b ScreenVertex) bool {
	if signedArea > 0 {
		if w > 0 {
			return true
		}
		return w == 0 && isTopLeft(a, b)
	}
	if w < 0 {
		return true
	}
	return w == 0 && isTopLeft(a, b)
}

func nearEdge(px, py float64, a, b ScreenVertex) bool {
	e := edgeFunc(a, b, px, py)
	length := math.Hypot(b.SX-a.SX, b.SY-a.SY)
	if length == 0 {
		return false
	}
	return math.Abs(e)/length < 1.0
}
