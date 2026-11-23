package main

// Rect 为窗口客户区逻辑坐标矩形，物理像素矩形同样使用该结构。
type Rect struct {
	X int
	Y int
	W int
	H int
}

// Empty 表示空矩形：宽度或高度非正，不包含任何像素。
func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// Contains 判定物理像素 (px, py) 是否落在矩形内。
func (r Rect) Contains(px, py int) bool {
	if r.Empty() {
		return false
	}
	return px >= r.X && px < r.X+r.W && py >= r.Y && py < r.Y+r.H
}

// Intersect 求两矩形交集，任一为空结果必为空。
func Intersect(a, b Rect) Rect {
	if a.Empty() || b.Empty() {
		return Rect{X: a.X, Y: a.Y, W: 0, H: 0}
	}
	x0 := max(a.X, b.X)
	y0 := max(a.Y, b.Y)
	x1 := min(a.X+a.W, b.X+b.W)
	y1 := min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return Rect{X: x0, Y: y0, W: 0, H: 0}
	}
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// Scale 按 DPI 比例把逻辑矩形映射为物理矩形，边界分别取整以保证邻接矩形无缝。
func (r Rect) Scale(s int) Rect {
	x0 := r.X * s
	y0 := r.Y * s
	x1 := (r.X + r.W) * s
	y1 := (r.Y + r.H) * s
	return Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// RGBA 为 8 位通道颜色。
type RGBA struct {
	R, G, B, A uint8
}

func (c RGBA) Hex() string {
	const hex = "0123456789abcdef"
	b := []byte{'#', 0, 0, 0, 0, 0, 0, 0, 0}
	v := [4]uint8{c.R, c.G, c.B, c.A}
	for i, n := range v {
		b[1+i*2] = hex[n>>4]
		b[2+i*2] = hex[n&0x0f]
	}
	return string(b)
}
