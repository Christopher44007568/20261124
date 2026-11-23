package main

import "math"

// Distortion 把显示坐标 (x,y) ∈ [-1,1]² 映射到纹理坐标 (u,v) ∈ [0,1]²。
// 两部分组成：宽视野径向畸变（边缘曲率大）+ 局部高斯突变（局部不连续）。
type Distortion struct {
	K1, K2 float64
	BumpA  float64
	BumpX  float64
	BumpY  float64
	BumpS  float64
}

func DefaultDistortion() Distortion {
	return Distortion{
		K1: 0.28, K2: 0.42,
		BumpA: 0.22, BumpX: 0.48, BumpY: 0.30, BumpS: 0.10,
	}
}

// Scale 返回该点的径向缩放因子。
func (d Distortion) Scale(x, y float64) float64 {
	r2 := x*x + y*y
	s := 1.0 + d.K1*r2 + d.K2*r2*r2
	dx, dy := x-d.BumpX, y-d.BumpY
	s += d.BumpA * math.Exp(-(dx*dx+dy*dy)/(d.BumpS*d.BumpS))
	return s
}

// Map 显示坐标 -> 纹理坐标。
func (d Distortion) Map(x, y float64) (u, v float64) {
	s := d.Scale(x, y)
	return 0.5 + 0.5*x*s, 0.5 + 0.5*y*s
}
