package stereo

import "math"

// translate 构造平移矩阵。
func translate(x, y, z float64) Mat4 {
	m := identity()
	m[12], m[13], m[14] = x, y, z
	return m
}

// NewCube 构建立方体 Mesh，法线朝外，顶点带基色。
func NewCube(name string, color Vec3, transparent bool, alpha float64) *Mesh {
	c := [8]Vec3{
		{-1, -1, -1}, {1, -1, -1}, {1, 1, -1}, {-1, 1, -1},
		{-1, -1, 1}, {1, -1, 1}, {1, 1, 1}, {-1, 1, 1},
	}
	faces := []struct {
		idx        [4]int
		nx, ny, nz float64
	}{
		{[4]int{0, 3, 2, 1}, 0, 0, -1},
		{[4]int{4, 5, 6, 7}, 0, 0, 1},
		{[4]int{0, 4, 7, 3}, -1, 0, 0},
		{[4]int{1, 2, 6, 5}, 1, 0, 0},
		{[4]int{3, 7, 6, 2}, 0, 1, 0},
		{[4]int{0, 1, 5, 4}, 0, -1, 0},
	}
	m := &Mesh{Name: name, Transparent: transparent, Alpha: alpha}
	for _, f := range faces {
		base := len(m.Verts)
		n := Vec3{f.nx, f.ny, f.nz}
		for _, i := range f.idx {
			m.Verts = append(m.Verts, Vertex{Pos: c[i], Normal: n, Color: color})
		}
		m.Tris = append(m.Tris, Tri{base, base + 1, base + 2}, Tri{base, base + 2, base + 3})
	}
	return m
}

// NewQuad 构造朝 +Z 的单位正方形片，用于透明排序测试。
func NewQuad(name string, color Vec3, alpha float64) *Mesh {
	return &Mesh{
		Name:        name,
		Transparent: true,
		Alpha:       alpha,
		Verts: []Vertex{
			{Vec3{-1, -1, 0}, Vec3{0, 0, 1}, color},
			{Vec3{1, -1, 0}, Vec3{0, 0, 1}, color},
			{Vec3{1, 1, 0}, Vec3{0, 0, 1}, color},
			{Vec3{-1, 1, 0}, Vec3{0, 0, 1}, color},
		},
		Tris: []Tri{{0, 1, 2}, {0, 2, 3}},
	}
}

// NewClipTri 构造向左伸出视锥的三角形，用于跨眼不同裁剪验证：
// 非对称投影下左眼将其左顶点裁掉（产生 2 个子三角形），右眼完整保留。
func NewClipTri(name string) *Mesh {
	return &Mesh{
		Name: name,
		Verts: []Vertex{
			{Vec3{-1.5, 0.6, 0}, Vec3{0, 0, 1}, Vec3{1, 0.9, 0.3}},
			{Vec3{0.8, -0.8, 0.4}, Vec3{0, 0, 1}, Vec3{1, 0.6, 0.2}},
			{Vec3{0.8, 0.8, 0.4}, Vec3{0, 0, 1}, Vec3{0.9, 0.8, 0.2}},
		},
		Tris: []Tri{{0, 1, 2}},
	}
}

func eyeParams(eye Eye, ipd, n, f, fov float64, asym float64) EyeParams {
	shift := -ipd / 2
	if eye == Right {
		shift = ipd / 2
	}
	center := Vec3{shift, 0, 0}
	view := LookAt(Vec3{shift, 0, 0}, center.Add(Vec3{0, 0, -1}), Vec3{0, 1, 0})
	t := n * math.Tan(fov*math.Pi/360) // fov 为完整垂直视场角（度）
	// 非对称离轴投影：左右 frustum 偏移不同，且右眼额外加入 asym 剪切。
	skew := 0.0
	if eye == Right {
		skew = asym
	}
	l, r := -t-shift+skew, t-shift+skew
	proj := Frustum(l, r, -t, t, n, f)
	return EyeParams{
		Eye:      eye,
		View:     view,
		Proj:     proj,
		ViewProj: mul(proj, view),
		Frustum:  [6]float64{l, r, -t, t, n, f},
	}
}
