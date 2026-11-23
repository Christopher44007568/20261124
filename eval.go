package main

import "math"

// locator 用均匀分桶加速点定位。
type locator struct {
	mesh *Mesh
	n    int
	bins [][]int
}

func newLocator(m *Mesh, n int) *locator {
	l := &locator{mesh: m, n: n, bins: make([][]int, n*n)}
	cell := 2.0 / float64(n)
	for ti, t := range m.Tris {
		x0, x1 := math.Inf(1), math.Inf(-1)
		y0, y1 := math.Inf(1), math.Inf(-1)
		for _, vi := range t {
			v := m.Verts[vi]
			x0, x1 = math.Min(x0, v.X), math.Max(x1, v.X)
			y0, y1 = math.Min(y0, v.Y), math.Max(y1, v.Y)
		}
		i0 := clampi(int((x0+1)/cell), 0, n-1)
		i1 := clampi(int((x1+1)/cell), 0, n-1)
		j0 := clampi(int((y0+1)/cell), 0, n-1)
		j1 := clampi(int((y1+1)/cell), 0, n-1)
		for j := j0; j <= j1; j++ {
			for i := i0; i <= i1; i++ {
				l.bins[j*n+i] = append(l.bins[j*n+i], ti)
			}
		}
	}
	return l
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

// bary 返回点在三角形中的重心坐标；全部 >= -eps 即在三角形内。
func bary(m *Mesh, t [3]int, x, y float64) (w0, w1, w2 float64) {
	a, b, c := m.Verts[t[0]], m.Verts[t[1]], m.Verts[t[2]]
	d := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
	if math.Abs(d) < 1e-18 {
		return -1, -1, -1
	}
	w0 = ((b.Y-c.Y)*(x-c.X) + (c.X-b.X)*(y-c.Y)) / d
	w1 = ((c.Y-a.Y)*(x-c.X) + (a.X-c.X)*(y-c.Y)) / d
	w2 = 1 - w0 - w1
	return
}

// interp 在 (x,y) 处插值 UV；未覆盖返回 ok=false。
func (l *locator) interp(x, y float64) (u, v float64, ok bool) {
	i := clampi(int((x+1)/2*float64(l.n)), 0, l.n-1)
	j := clampi(int((y+1)/2*float64(l.n)), 0, l.n-1)
	for _, ti := range l.bins[j*l.n+i] {
		t := l.mesh.Tris[ti]
		w0, w1, w2 := bary(l.mesh, t, x, y)
		const eps = 1e-9
		if w0 >= -eps && w1 >= -eps && w2 >= -eps {
			a, b, c := l.mesh.Verts[t[0]], l.mesh.Verts[t[1]], l.mesh.Verts[t[2]]
			return w0*a.U + w1*b.U + w2*c.U, w0*a.V + w1*b.V + w2*c.V, true
		}
	}
	return 0, 0, false
}

// Metrics 一种网格策略的评估结果。
type Metrics struct {
	Name        string
	Verts       int
	Tris        int
	MaxErr      float64 // 最大几何误差（归一化 UV）
	MaxErrTexel float64 // 折算 1024 纹理的 texel 数
	Holes       int     // 采样点中无三角形覆盖的数量（插值空洞）
	TJunctions  int     // 自适应网格缝合的 T 型结点数
	BoundaryOK  bool    // 显示边界是否保持 [-1,1]²
	BuildMs     float64 // 网格生成耗时（毫秒）
}

// Evaluate 采样网格，统计最大几何误差与插值空洞。
func Evaluate(name string, m *Mesh, d Distortion, tj int, buildMs float64, samples int) Metrics {
	loc := newLocator(m, 128)
	mt := Metrics{Name: name, Verts: len(m.Verts), Tris: len(m.Tris),
		TJunctions: tj, BuildMs: buildMs, BoundaryOK: checkBoundary(m)}
	for j := 0; j < samples; j++ {
		y := -1 + 2*(float64(j)+0.5)/float64(samples)
		for i := 0; i < samples; i++ {
			x := -1 + 2*(float64(i)+0.5)/float64(samples)
			u, v, ok := loc.interp(x, y)
			if !ok {
				mt.Holes++
				continue
			}
			uT, vT := d.Map(x, y)
			if e := math.Hypot(uT-u, vT-v); e > mt.MaxErr {
				mt.MaxErr = e
			}
		}
	}
	mt.MaxErrTexel = mt.MaxErr * 1024
	return mt
}

// checkBoundary 验证网格边界仍是 [-1,1]²：极值恰好为 ±1，
// 且四条边上都存在落在边界上的顶点。
func checkBoundary(m *Mesh) bool {
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	onL, onR, onB, onT := false, false, false, false
	for _, v := range m.Verts {
		minX, maxX = math.Min(minX, v.X), math.Max(maxX, v.X)
		minY, maxY = math.Min(minY, v.Y), math.Max(maxY, v.Y)
		const e = 1e-12
		onL = onL || math.Abs(v.X+1) < e
		onR = onR || math.Abs(v.X-1) < e
		onB = onB || math.Abs(v.Y+1) < e
		onT = onT || math.Abs(v.Y-1) < e
	}
	return minX == -1 && maxX == 1 && minY == -1 && maxY == 1 &&
		onL && onR && onB && onT
}
