// Package motion 提供把"相机(头)运动"与"物体自身运动"拆成独立来源的
// 屏幕空间速度 / 深度差 / 历史权重计算。仅使用 Go 标准库。
package motion

import "math"

// Vec2 是屏幕空间(NDC)向量。
type Vec2 struct {
	X, Y float64
}

func (a Vec2) Sub(b Vec2) Vec2 { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Add(b Vec2) Vec2 { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Len() float64    { return math.Hypot(a.X, a.Y) }

// Vec3 是世界/视图空间向量。
type Vec3 struct {
	X, Y, Z float64
}

func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}

// Mat4 是行主序 4x4 矩阵。
type Mat4 [16]float64

// Identity 返回单位矩阵。
func Identity() Mat4 {
	return Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
}

// Mul 返回 a*b(先应用 b,再应用 a)。
func (a Mat4) Mul(b Mat4) Mat4 {
	var r Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				r[i*4+j] += a[i*4+k] * b[k*4+j]
			}
		}
	}
	return r
}

// TransformPoint 对点做齐次变换并做 w 除法。
func (m Mat4) TransformPoint(p Vec3) Vec3 {
	x := m[0]*p.X + m[1]*p.Y + m[2]*p.Z + m[3]
	y := m[4]*p.X + m[5]*p.Y + m[6]*p.Z + m[7]
	z := m[8]*p.X + m[9]*p.Y + m[10]*p.Z + m[11]
	w := m[12]*p.X + m[13]*p.Y + m[14]*p.Z + m[15]
	if w != 0 && w != 1 {
		return Vec3{x / w, y / w, z / w}
	}
	return Vec3{x, y, z}
}

// Translate 构造平移矩阵。
func Translate(d Vec3) Mat4 {
	return Mat4{1, 0, 0, d.X, 0, 1, 0, d.Y, 0, 0, 1, d.Z, 0, 0, 0, 1}
}

// RotateY 构造绕 Y 轴旋转矩阵(弧度)。
func RotateY(a float64) Mat4 {
	c, s := math.Cos(a), math.Sin(a)
	return Mat4{c, 0, s, 0, 0, 1, 0, 0, -s, 0, c, 0, 0, 0, 0, 1}
}

// LookAt 构造视图矩阵(右手系,相机看向 -Z)。
func LookAt(eye, target, up Vec3) Mat4 {
	f := target.Sub(eye).Norm()
	s := f.Cross(up).Norm()
	u := s.Cross(f)
	return Mat4{
		s.X, s.Y, s.Z, -s.Dot(eye),
		u.X, u.Y, u.Z, -u.Dot(eye),
		-f.X, -f.Y, -f.Z, f.Dot(eye),
		0, 0, 0, 1,
	}
}

// Perspective 构造透视投影矩阵,深度映射到 [0,1]。
func Perspective(fovY, aspect, near, far float64) Mat4 {
	t := 1 / math.Tan(fovY/2)
	return Mat4{
		t / aspect, 0, 0, 0,
		0, t, 0, 0,
		0, 0, far / (near - far), near * far / (near - far),
		0, 0, -1, 0,
	}
}
