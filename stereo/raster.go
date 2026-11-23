package stereo

import "math"

// cv 是裁剪空间顶点：齐次坐标 + 顶点色。
type cv struct {
	P     Vec4
	Color Vec3
}

// sv 是屏幕空间顶点，保留 1/w 供透视校正插值。
type sv struct {
	Sx, Sy float64
	Depth  float64 // NDC z
	InvW   float64
	Color  Vec3
}

type plane int

const (
	pLeft plane = iota
	pRight
	pBottom
	pTop
	pNear
	pFar
)

func dist(v cv, p plane) float64 {
	switch p {
	case pLeft:
		return v.P.W + v.P.X
	case pRight:
		return v.P.W - v.P.X
	case pBottom:
		return v.P.W + v.P.Y
	case pTop:
		return v.P.W - v.P.Y
	case pNear:
		return v.P.W + v.P.Z
	default:
		return v.P.W - v.P.Z
	}
}

func lerpCV(a, b cv, t float64) cv {
	return cv{
		P:     Vec4{a.P.X + (b.P.X-a.P.X)*t, a.P.Y + (b.P.Y-a.P.Y)*t, a.P.Z + (b.P.Z-a.P.Z)*t, a.P.W + (b.P.W-a.P.W)*t},
		Color: a.Color.Add(b.Color.Sub(a.Color).Scale(t)),
	}
}

// clipTri 对六个半空间做 Sutherland-Hodgman 裁剪，返回 0..n 个三角形。
func clipTri(in []cv) [][3]cv {
	poly := in
	for _, p := range []plane{pLeft, pRight, pBottom, pTop, pNear, pFar} {
		if len(poly) == 0 {
			return nil
		}
		var out []cv
		for i := range poly {
			cur := poly[i]
			prev := poly[(i+len(poly)-1)%len(poly)]
			dc, dp := dist(cur, p), dist(prev, p)
			curIn, prevIn := dc >= 0, dp >= 0
			if curIn != prevIn {
				out = append(out, lerpCV(prev, cur, dp/(dp-dc)))
			}
			if curIn {
				out = append(out, cur)
			}
		}
		poly = out
	}
	if len(poly) < 3 {
		return nil
	}
	var tris [][3]cv
	for i := 1; i+1 < len(poly); i++ {
		tris = append(tris, [3]cv{poly[0], poly[i], poly[i+1]})
	}
	return tris
}

func toScreen(v cv, w, h int) sv {
	inv := 1 / v.P.W
	ndc := Vec3{v.P.X * inv, v.P.Y * inv, v.P.Z * inv}
	return sv{
		Sx:    (ndc.X*0.5 + 0.5) * float64(w),
		Sy:    (1 - (ndc.Y*0.5 + 0.5)) * float64(h),
		Depth: ndc.Z,
		InvW:  inv,
		Color: v.Color,
	}
}

var lightDir = norm(Vec3{-0.4, -0.7, -1.0})

// shade 做固定方向光的平面着色。
func shade(normal, base, tint Vec3) Vec3 {
	d := math.Max(0, normal.Dot(lightDir.Scale(-1)))
	c := base.Mul(tint).Scale(0.25 + 0.75*d)
	return clampV(c)
}
func (v Vec3) Mul(o Vec3) Vec3 { return Vec3{v.X * o.X, v.Y * o.Y, v.Z * o.Z} }
func clampV(v Vec3) Vec3 {
	return Vec3{math.Min(1, math.Max(0, v.X)), math.Min(1, math.Max(0, v.Y)), math.Min(1, math.Max(0, v.Z))}
}

// raster 将一个已裁剪三角形写入帧缓冲。
func raster(f *Frame, tri [3]cv, transparent bool, alpha float64, orderTag string) {
	w, h := f.Width, f.Height
	s := [3]sv{toScreen(tri[0], w, h), toScreen(tri[1], w, h), toScreen(tri[2], w, h)}
	minX := math.Max(0, math.Floor(math.Min(s[0].Sx, math.Min(s[1].Sx, s[2].Sx))))
	maxX := math.Min(float64(w-1), math.Ceil(math.Max(s[0].Sx, math.Max(s[1].Sx, s[2].Sx))))
	minY := math.Max(0, math.Floor(math.Min(s[0].Sy, math.Min(s[1].Sy, s[2].Sy))))
	maxY := math.Min(float64(h-1), math.Ceil(math.Max(s[0].Sy, math.Max(s[1].Sy, s[2].Sy))))

	den := (s[1].Sy-s[2].Sy)*(s[0].Sx-s[2].Sx) + (s[2].Sx-s[1].Sx)*(s[0].Sy-s[2].Sy)
	if math.Abs(den) < 1e-12 {
		return
	}
	invDen := 1 / den
	for py := minY; py <= maxY; py++ {
		for px := minX; px <= maxX; px++ {
			cx, cy := px+0.5, py+0.5
			b0 := ((s[1].Sy-s[2].Sy)*(cx-s[2].Sx) + (s[2].Sx-s[1].Sx)*(cy-s[2].Sy)) * invDen
			b1 := ((s[2].Sy-s[0].Sy)*(cx-s[2].Sx) + (s[0].Sx-s[2].Sx)*(cy-s[2].Sy)) * invDen
			b2 := 1 - b0 - b1
			if b0 < 0 || b1 < 0 || b2 < 0 {
				continue
			}
			// 透视校正权重
			w0, w1, w2 := b0*s[0].InvW, b1*s[1].InvW, b2*s[2].InvW
			wsum := w0 + w1 + w2
			w0, w1, w2 = w0/wsum, w1/wsum, w2/wsum
			depth := w0*s[0].Depth + w1*s[1].Depth + w2*s[2].Depth
			col := s[0].Color.Scale(w0).Add(s[1].Color.Scale(w1)).Add(s[2].Color.Scale(w2))
			idx := int(py)*w + int(px)
			if depth > f.Depth[idx] {
				continue
			}
			if transparent {
				dst := f.Color[idx]
				f.Color[idx] = dst.Scale(1 - alpha).Add(col.Scale(alpha))
				// 不写深度：维持画家算法语义
				continue
			}
			f.Depth[idx] = depth
			f.Color[idx] = col
		}
	}
	if transparent {
		f.TransOrder = append(f.TransOrder, orderTag)
	}
}
