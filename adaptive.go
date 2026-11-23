package main

import "math"

// quad 四叉树节点，覆盖显示空间 [x0,y0]×[x1,y1]。
type quad struct {
	x0, y0, x1, y1 float64
	level          int
	children       [4]*quad // 为空则为叶子
}

func (q *quad) leaf() bool { return q.children[0] == nil }

// AdaptiveStats 自适应加密过程的统计。
type AdaptiveStats struct {
	Leaves     int // 叶子四边形数
	TJunctions int // 若不缝合会产生的 T 型结点（潜在插值空洞）数
	MaxDepth   int
}

// BuildAdaptive 弦高误差自适应加密。
// 判据：在每个四边形边中点与中心处，比较真实畸变 UV 与角点 UV 的
// 双线性插值，偏差（弦高误差）超过 tol 即四分。宽视野边缘与局部
// 突变处曲率大，会自然获得更高密度。
// minN 为初始网格，maxDepth 限制细分深度，边界始终为 [-1,1]²。
func BuildAdaptive(d Distortion, minN int, tol float64, maxDepth int) (*Mesh, AdaptiveStats) {
	var st AdaptiveStats
	var roots []*quad
	step := 2.0 / float64(minN)
	for j := 0; j < minN; j++ {
		for i := 0; i < minN; i++ {
			q := &quad{x0: -1 + float64(i)*step, y0: -1 + float64(j)*step,
				x1: -1 + float64(i+1)*step, y1: -1 + float64(j+1)*step}
			refine(q, d, tol, maxDepth, &st)
			roots = append(roots, q)
		}
	}
	m := &Mesh{}
	// 全局顶点表：叶子角点去重（相邻叶子共享角点）。
	vertIdx := map[[2]int64]int{}
	cornerOf := func(x, y float64) int {
		key := [2]int64{int64(math.Round(x * 1e12)), int64(math.Round(y * 1e12))}
		if id, ok := vertIdx[key]; ok {
			return id
		}
		id := m.addVert(d, x, y)
		vertIdx[key] = id
		return id
	}
	var leaves []*quad
	for _, r := range roots {
		collectLeaves(r, &leaves)
	}
	st.Leaves = len(leaves)
	// 先登记全部叶子角点，再检查每条边中点是否被更细的邻居占据：
	// 被占据即潜在 T 型结点，朴素逐格三角化会在此处产生插值裂缝。
	for _, l := range leaves {
		cornerOf(l.x0, l.y0)
		cornerOf(l.x1, l.y0)
		cornerOf(l.x0, l.y1)
		cornerOf(l.x1, l.y1)
	}
	for _, l := range leaves {
		st.TJunctions += triangulateLeaf(m, l, d, cornerOf, vertIdx)
	}
	return m, st
}

// chordErr 四边形的弦高误差：边中点与中心处真实 UV 相对双线性插值的偏差。
func chordErr(d Distortion, q *quad) float64 {
	u00, v00 := d.Map(q.x0, q.y0)
	u10, v10 := d.Map(q.x1, q.y0)
	u01, v01 := d.Map(q.x0, q.y1)
	u11, v11 := d.Map(q.x1, q.y1)
	mx, my := 0.5*(q.x0+q.x1), 0.5*(q.y0+q.y1)
	pts := [][2]float64{
		{mx, q.y0}, {mx, q.y1}, {q.x0, my}, {q.x1, my}, {mx, my},
	}
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	worst := 0.0
	for _, p := range pts {
		tx := (p[0] - q.x0) / (q.x1 - q.x0)
		ty := (p[1] - q.y0) / (q.y1 - q.y0)
		uEst := lerp(lerp(u00, u10, tx), lerp(u01, u11, tx), ty)
		vEst := lerp(lerp(v00, v10, tx), lerp(v01, v11, tx), ty)
		uT, vT := d.Map(p[0], p[1])
		e := math.Hypot(uT-uEst, vT-vEst)
		if e > worst {
			worst = e
		}
	}
	return worst
}

func refine(q *quad, d Distortion, tol float64, maxDepth int, st *AdaptiveStats) {
	if q.level >= maxDepth || chordErr(d, q) <= tol {
		if q.level > st.MaxDepth {
			st.MaxDepth = q.level
		}
		return
	}
	mx, my := 0.5*(q.x0+q.x1), 0.5*(q.y0+q.y1)
	lv := q.level + 1
	q.children = [4]*quad{
		{x0: q.x0, y0: q.y0, x1: mx, y1: my, level: lv},
		{x0: mx, y0: q.y0, x1: q.x1, y1: my, level: lv},
		{x0: q.x0, y0: my, x1: mx, y1: q.y1, level: lv},
		{x0: mx, y0: my, x1: q.x1, y1: q.y1, level: lv},
	}
	for _, c := range q.children {
		refine(c, d, tol, maxDepth, st)
	}
}

func collectLeaves(q *quad, out *[]*quad) {
	if q.leaf() {
		*out = append(*out, q)
		return
	}
	for _, c := range q.children {
		collectLeaves(c, out)
	}
}

// triangulateLeaf 把叶子四边形保守三角化。
// 若某条边的中点已被更细的邻居占用（T 型结点），则把该中点纳入
// 边界多边形，绕中心顶点扇形三角化。这样粗细交界处共享真实顶点，
// 插值连续、无裂缝，且低密度大格仍完整覆盖自身区域（保守覆盖）。
// 返回该叶子上的 T 型结点数。
func triangulateLeaf(m *Mesh, q *quad, d Distortion,
	cornerOf func(x, y float64) int, vertIdx map[[2]int64]int) int {
	midOccupied := func(x, y float64) bool {
		key := [2]int64{int64(math.Round(x * 1e12)), int64(math.Round(y * 1e12))}
		_, ok := vertIdx[key]
		return ok
	}
	cx := [4]float64{q.x0, q.x1, q.x1, q.x0}
	cy := [4]float64{q.y0, q.y0, q.y1, q.y1}
	var poly []int
	tj := 0
	for e := 0; e < 4; e++ {
		poly = append(poly, cornerOf(cx[e], cy[e]))
		nx, ny := cx[(e+1)%4], cy[(e+1)%4]
		mx, my := 0.5*(cx[e]+nx), 0.5*(cy[e]+ny)
		if midOccupied(mx, my) {
			poly = append(poly, cornerOf(mx, my))
			tj++
		}
	}
	center := m.addVert(d, 0.5*(q.x0+q.x1), 0.5*(q.y0+q.y1))
	for i := 0; i < len(poly); i++ {
		m.Tris = append(m.Tris, [3]int{center, poly[i], poly[(i+1)%len(poly)]})
	}
	return tj
}
