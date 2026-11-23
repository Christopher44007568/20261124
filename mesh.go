package main

// Vert 网格顶点：显示坐标 (X,Y) + 畸变后的纹理坐标 (U,V)。
type Vert struct {
	X, Y float64
	U, V float64
}

// Mesh 三角网格，三角形索引指向 Verts。
type Mesh struct {
	Verts []Vert
	Tris  [][3]int
}

func (m *Mesh) addVert(d Distortion, x, y float64) int {
	u, v := d.Map(x, y)
	m.Verts = append(m.Verts, Vert{X: x, Y: y, U: u, V: v})
	return len(m.Verts) - 1
}

// BuildUniform 均匀 n×n 四边形网格，每个四边形拆两个三角形。
// 边界固定在 [-1,1]²，不参与任何加密。
func BuildUniform(d Distortion, n int) *Mesh {
	return buildTensor(d, n, func(t float64) float64 { return t })
}

// BuildRadius 按半径加密的张量网格：坐标经 p(t)=t*(1+a-a*t²)  Warp，
// p(±1)=±1 保证显示边界不变；p'(±1)=1-2a < p'(0)=1+a，
// 即边缘（大半径、宽视野区）单元更密，中心更稀。
func BuildRadius(d Distortion, n int, a float64) *Mesh {
	return buildTensor(d, n, func(t float64) float64 {
		return t * (1 + a - a*t*t)
	})
}

// buildTensor 用一维 warp 函数生成张量积网格。warp 必须满足
// warp(-1)=-1, warp(1)=1，否则显示边界会被改变。
func buildTensor(d Distortion, n int, warp func(float64) float64) *Mesh {
	m := &Mesh{}
	idx := make([][]int, n+1)
	for j := 0; j <= n; j++ {
		idx[j] = make([]int, n+1)
		y := warp(-1 + 2*float64(j)/float64(n))
		for i := 0; i <= n; i++ {
			x := warp(-1 + 2*float64(i)/float64(n))
			idx[j][i] = m.addVert(d, x, y)
		}
	}
	for j := 0; j < n; j++ {
		for i := 0; i < n; i++ {
			a, b := idx[j][i], idx[j][i+1]
			c, e := idx[j+1][i], idx[j+1][i+1]
			m.Tris = append(m.Tris, [3]int{a, b, e}, [3]int{a, e, c})
		}
	}
	return m
}
