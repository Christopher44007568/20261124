package main

import "math"

// Vec3 is a 3D vector/point.
type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l < 1e-12 {
		return Vec3{}
	}
	return a.Scale(1 / l)
}

// Mat4 is a row-major 4x4 matrix used with column vectors: out = M * v.
type Mat4 [4][4]float64

func Identity() Mat4 {
	var m Mat4
	for i := 0; i < 4; i++ {
		m[i][i] = 1
	}
	return m
}

// Mul returns a*b (apply b first, then a).
func Mul(a, b Mat4) Mat4 {
	var r Mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[i][k] * b[k][j]
			}
			r[i][j] = s
		}
	}
	return r
}

// MulPoint transforms a point (w=1) and divides by w.
func (m Mat4) MulPoint(p Vec3) Vec3 {
	x := m[0][0]*p.X + m[0][1]*p.Y + m[0][2]*p.Z + m[0][3]
	y := m[1][0]*p.X + m[1][1]*p.Y + m[1][2]*p.Z + m[1][3]
	z := m[2][0]*p.X + m[2][1]*p.Y + m[2][2]*p.Z + m[2][3]
	w := m[3][0]*p.X + m[3][1]*p.Y + m[3][2]*p.Z + m[3][3]
	if math.Abs(w) < 1e-12 {
		return Vec3{x, y, z}
	}
	return Vec3{x / w, y / w, z / w}
}

// Basis returns column i of the upper 3x3 (world axis i, scale included).
func (m Mat4) Basis(i int) Vec3 { return Vec3{m[0][i], m[1][i], m[2][i]} }

func Translation(x, y, z float64) Mat4 {
	m := Identity()
	m[0][3], m[1][3], m[2][3] = x, y, z
	return m
}

func Scaling(x, y, z float64) Mat4 {
	var m Mat4
	m[0][0], m[1][1], m[2][2], m[3][3] = x, y, z, 1
	return m
}

// RotationAxis builds a rotation matrix around an axis (right-hand rule).
func RotationAxis(axis Vec3, angle float64) Mat4 {
	a := axis.Norm()
	c, s := math.Cos(angle), math.Sin(angle)
	t := 1 - c
	m := Identity()
	m[0][0] = c + a.X*a.X*t
	m[0][1] = a.X*a.Y*t - a.Z*s
	m[0][2] = a.X*a.Z*t + a.Y*s
	m[1][0] = a.Y*a.X*t + a.Z*s
	m[1][1] = c + a.Y*a.Y*t
	m[1][2] = a.Y*a.Z*t - a.X*s
	m[2][0] = a.Z*a.X*t - a.Y*s
	m[2][1] = a.Z*a.Y*t + a.X*s
	m[2][2] = c + a.Z*a.Z*t
	return m
}

// LookAt builds a right-handed view matrix (camera looks along -Z).
func LookAt(eye, target, up Vec3) Mat4 {
	f := target.Sub(eye).Norm()
	s := f.Cross(up).Norm()
	u := s.Cross(f)
	m := Identity()
	m[0][0], m[0][1], m[0][2], m[0][3] = s.X, s.Y, s.Z, -s.Dot(eye)
	m[1][0], m[1][1], m[1][2], m[1][3] = u.X, u.Y, u.Z, -u.Dot(eye)
	m[2][0], m[2][1], m[2][2], m[2][3] = -f.X, -f.Y, -f.Z, f.Dot(eye)
	return m
}

// Perspective builds an OpenGL-style perspective projection matrix.
func Perspective(fovY, aspect, near, far float64) Mat4 {
	f := 1 / math.Tan(fovY/2)
	var m Mat4
	m[0][0] = f / aspect
	m[1][1] = f
	m[2][2] = (far + near) / (near - far)
	m[2][3] = 2 * far * near / (near - far)
	m[3][2] = -1
	return m
}
