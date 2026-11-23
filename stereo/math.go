package stereo

import "math"

// Vec3 是三维向量 / RGB 颜色共用类型。
type Vec3 struct {
	X, Y, Z float64
}

func (v Vec3) Add(o Vec3) Vec3      { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v Vec3) Sub(o Vec3) Vec3      { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }
func (v Vec3) Dot(o Vec3) float64   { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }
func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		v.Y*o.Z - v.Z*o.Y,
		v.Z*o.X - v.X*o.Z,
		v.X*o.Y - v.Y*o.X,
	}
}

func norm(v Vec3) Vec3 {
	l := math.Sqrt(v.Dot(v))
	if l == 0 {
		return v
	}
	return v.Scale(1 / l)
}

// Vec4 为齐次坐标 / 裁剪空间点。
type Vec4 struct {
	X, Y, Z, W float64
}

// Mat4 按列主序存储，与 OpenGL 约定一致。
type Mat4 [16]float64

func identity() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}

// mul 计算 a*b（先应用 b 再应用 a）。
func mul(a, b Mat4) Mat4 {
	var r Mat4
	for c := 0; c < 4; c++ {
		for row := 0; row < 4; row++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[k*4+row] * b[c*4+k]
			}
			r[c*4+row] = s
		}
	}
	return r
}

func (m Mat4) transform(v Vec4) Vec4 {
	return Vec4{
		m[0]*v.X + m[4]*v.Y + m[8]*v.Z + m[12]*v.W,
		m[1]*v.X + m[5]*v.Y + m[9]*v.Z + m[13]*v.W,
		m[2]*v.X + m[6]*v.Y + m[10]*v.Z + m[14]*v.W,
		m[3]*v.X + m[7]*v.Y + m[11]*v.Z + m[15]*v.W,
	}
}

// Frustum 构造 OpenGL 风格的非对称透视投影矩阵，
// l/r/b/t/n/f 均为近裁剪平面上的有符号距离。
func Frustum(l, r, b, t, n, f float64) Mat4 {
	return Mat4{
		2 * n / (r - l), 0, 0, 0,
		0, 2 * n / (t - b), 0, 0,
		(r + l) / (r - l), (t + b) / (t - b), -(f + n) / (f - n), -1,
		0, 0, -2 * f * n / (f - n), 0,
	}
}

// LookAt 构造视图矩阵。
func LookAt(eye, center, up Vec3) Mat4 {
	f := norm(center.Sub(eye))
	s := norm(f.Cross(up))
	u := s.Cross(f)
	return Mat4{
		s.X, u.X, -f.X, 0,
		s.Y, u.Y, -f.Y, 0,
		s.Z, u.Z, -f.Z, 0,
		-s.Dot(eye), -u.Dot(eye), f.Dot(eye), 1,
	}
}

// Fingerprint 返回矩阵的稳定哈希（FNV-1a），用于定位“哪只眼的矩阵出了错”。
func (m Mat4) Fingerprint() uint64 {
	const off, prime = uint64(14695981039346656037), uint64(1099511628211)
	h := off
	for _, v := range m {
		b := math.Float64bits(v)
		for i := 0; i < 8; i++ {
			h ^= b & 0xff
			h *= prime
			b >>= 8
		}
	}
	return h
}
