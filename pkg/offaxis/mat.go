package offaxis

import "math"

// Vec3 is a point or direction in eye space.
type Vec3 struct {
	X, Y, Z float64
}

// Vec4 is a homogeneous coordinate.
type Vec4 struct {
	X, Y, Z, W float64
}

// Mat4 is a row-major 4x4 matrix: M[r][c].
//
// The projection matrices produced by this package match the OpenGL
// glFrustum convention: points are transformed as p' = M * p, the camera
// looks down -Z, and the NDC box is [-1,1]^3 with the near plane mapped to
// z_ndc = -1 and the far plane to z_ndc = +1.
type Mat4 [4][4]float64

// Identity returns the 4x4 identity matrix.
func Identity() Mat4 {
	var m Mat4
	m[0][0] = 1
	m[1][1] = 1
	m[2][2] = 1
	m[3][3] = 1
	return m
}

// Mul returns a * b (b is applied first to a column vector).
func Mul(a, b Mat4) Mat4 {
	var out Mat4
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[r][k] * b[k][c]
			}
			out[r][c] = s
		}
	}
	return out
}

// Transform applies m to the homogeneous vector v.
func (m Mat4) Transform(v Vec4) Vec4 {
	return Vec4{
		X: m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z + m[0][3]*v.W,
		Y: m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z + m[1][3]*v.W,
		Z: m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z + m[2][3]*v.W,
		W: m[3][0]*v.X + m[3][1]*v.Y + m[3][2]*v.Z + m[3][3]*v.W,
	}
}

// TransformPoint treats p as (x, y, z, 1).
func (m Mat4) TransformPoint(p Vec3) Vec4 {
	return m.Transform(Vec4{X: p.X, Y: p.Y, Z: p.Z, W: 1})
}

// Add returns a + b component-wise.
func (a Vec3) Add(b Vec3) Vec3 { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }

// Sub returns a - b component-wise.
func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }

// Scale returns v * s.
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }

// Dot returns the scalar product of a and b.
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

// Cross returns a x b.
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// Length returns the Euclidean norm of v.
func (v Vec3) Length() float64 { return math.Sqrt(v.Dot(v)) }

// Normalize returns the unit vector pointing along v.
func (v Vec3) Normalize() Vec3 {
	n := v.Length()
	if n == 0 {
		return Vec3{}
	}
	return v.Scale(1 / n)
}

// NDC converts a clip-space vector into normalized device coordinates.
// Callers must ensure v.W != 0.
func (v Vec4) NDC() Vec4 {
	return Vec4{v.X / v.W, v.Y / v.W, v.Z / v.W, 1}
}
