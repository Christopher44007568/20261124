// Package geom contains the small vector, matrix, and pinhole-camera toolkit
// used by the renderer and both warp implementations.
package geom

import "math"

// Vec3 is a three-dimensional vector.
type Vec3 struct {
	X, Y, Z float64
}

func (a Vec3) Add(b Vec3) Vec3 { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Mul(s float64) Vec3 {
	return Vec3{a.X * s, a.Y * s, a.Z * s}
}
func (a Vec3) Dot(b Vec3) float64 { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Len() float64       { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Normalize() Vec3 {
	l := a.Len()
	if l == 0 {
		return Vec3{}
	}
	return a.Mul(1 / l)
}
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}
func (a Vec3) At(i int) float64 {
	switch i {
	case 0:
		return a.X
	case 1:
		return a.Y
	default:
		return a.Z
	}
}

// Mat3 is a row-major 3x3 matrix.
type Mat3 [3][3]float64

func Identity3() Mat3 {
	return Mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
}

func (m Mat3) MulVec(v Vec3) Vec3 {
	return Vec3{
		m[0][0]*v.X + m[0][1]*v.Y + m[0][2]*v.Z,
		m[1][0]*v.X + m[1][1]*v.Y + m[1][2]*v.Z,
		m[2][0]*v.X + m[2][1]*v.Y + m[2][2]*v.Z,
	}
}

func (m Mat3) Mul(n Mat3) Mat3 {
	var out Mat3
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r][c] = m[r][0]*n[0][c] + m[r][1]*n[1][c] + m[r][2]*n[2][c]
		}
	}
	return out
}

func (m Mat3) Transpose() Mat3 {
	return Mat3{
		{m[0][0], m[1][0], m[2][0]},
		{m[0][1], m[1][1], m[2][1]},
		{m[0][2], m[1][2], m[2][2]},
	}
}

// Basis returns a world-to-camera rotation whose rows are right, up, and
// forward. Yaw is positive to the right; pitch is positive upward.
func Basis(yaw, pitch float64) Mat3 {
	cy, sy := math.Cos(yaw), math.Sin(yaw)
	cp, sp := math.Cos(pitch), math.Sin(pitch)
	forward := Vec3{sy * cp, sp, cy * cp}
	right := Vec3{cy, 0, -sy}
	up := Vec3{-sp * sy, cp, -sp * cy}
	return Mat3{
		{right.X, right.Y, right.Z},
		{up.X, up.Y, up.Z},
		{forward.X, forward.Y, forward.Z},
	}
}

// Pose maps world points into a camera. R is world-to-camera and P is the
// camera position in world space.
type Pose struct {
	R Mat3
	P Vec3
}

func NewPose(yaw, pitch float64, p Vec3) Pose {
	return Pose{R: Basis(yaw, pitch), P: p}
}

func (p Pose) WorldToCam(v Vec3) Vec3 {
	return p.R.MulVec(v.Sub(p.P))
}

func (p Pose) CamToWorld(v Vec3) Vec3 {
	return p.R.Transpose().MulVec(v).Add(p.P)
}

// Intrinsics describes a pinhole camera. Pixel indices are the sample
// coordinates; pixel (0,0) sits at u=0, v=0.
type Intrinsics struct {
	W, H   int
	FX, FY float64
	CX, CY float64
}

func NewIntrinsics(w, h int, fovYRadians float64) Intrinsics {
	fy := float64(h) / (2 * math.Tan(fovYRadians/2))
	return Intrinsics{
		W: w, H: h,
		FX: fy, FY: fy,
		CX: float64(w-1) / 2,
		CY: float64(h-1) / 2,
	}
}

// CamRay returns a camera-space ray with z=1 for pixel indices u,v.
func (in Intrinsics) CamRay(u, v float64) Vec3 {
	return Vec3{
		X: (u - in.CX) / in.FX,
		Y: (in.CY - v) / in.FY,
		Z: 1,
	}
}

// Project returns pixel coordinates and camera-space depth. ok is false when
// the point is not in front of the camera.
func (in Intrinsics) Project(p Vec3) (u, v, depth float64, ok bool) {
	if p.Z <= 1e-9 {
		return 0, 0, p.Z, false
	}
	u = in.FX*p.X/p.Z + in.CX
	v = in.CY - in.FY*p.Y/p.Z
	return u, v, p.Z, true
}

func (in Intrinsics) Unproject(u, v, depth float64) Vec3 {
	return Vec3{
		X: (u - in.CX) / in.FX * depth,
		Y: (in.CY - v) / in.FY * depth,
		Z: depth,
	}
}

// InImage reports whether u,v is a valid bilinear sample location. It is
// deliberately strict: nothing outside the stored frame is ever sampled.
func (in Intrinsics) InImage(u, v float64) bool {
	const eps = 1e-7
	return u >= -eps && v >= -eps && u <= float64(in.W-1)+eps && v <= float64(in.H-1)+eps
}
