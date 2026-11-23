// Package geo contains the minimal, dependency-free 3-D geometry used by the
// stereo re-projection validator. No graphics SDK is used anywhere.
package geo

import "math"

// Vec3 is a 3-D column vector in world or camera coordinates.
type Vec3 struct{ X, Y, Z float64 }

func Add(a, b Vec3) Vec3           { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func Sub(a, b Vec3) Vec3           { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func Scale(a Vec3, s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func Dot(a, b Vec3) float64        { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }

func Cross(a, b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}

func Norm(a Vec3) float64 { return math.Sqrt(Dot(a, a)) }

// Mat3 is a row-major 3x3 rotation matrix.
type Mat3 struct{ R [3][3]float64 }

func (m Mat3) Mul(v Vec3) Vec3 {
	return Vec3{
		m.R[0][0]*v.X + m.R[0][1]*v.Y + m.R[0][2]*v.Z,
		m.R[1][0]*v.X + m.R[1][1]*v.Y + m.R[1][2]*v.Z,
		m.R[2][0]*v.X + m.R[2][1]*v.Y + m.R[2][2]*v.Z,
	}
}

// RotYX builds the rotation R = RotY(yaw) * RotX(pitch), matching the
// frame orientation produced by Path.Sample.
func RotYX(yaw, pitch float64) Mat3 {
	cy, sy := math.Cos(yaw), math.Sin(yaw)
	cp, sp := math.Cos(pitch), math.Sin(pitch)
	ry := Mat3{R: [3][3]float64{
		{cy, 0, sy},
		{0, 1, 0},
		{-sy, 0, cy},
	}}
	rx := Mat3{R: [3][3]float64{
		{1, 0, 0},
		{0, cp, -sp},
		{0, sp, cp},
	}}
	var out Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out.R[i][j] = ry.R[i][0]*rx.R[0][j] + ry.R[i][1]*rx.R[1][j] + ry.R[i][2]*rx.R[2][j]
		}
	}
	return out
}

// Pose is the world-to-camera transform of one frame: p_cam = R*(p_w - C).
type Pose struct {
	C   Vec3
	Rot Mat3
}

func (p Pose) ToCamera(v Vec3) Vec3 { return p.Rot.Mul(Sub(v, p.C)) }

// Intrinsics is a simple pinhole camera (no distortion, square pixels).
type Intrinsics struct {
	W, H   int
	Fx, Fy float64
	Cx, Cy float64
}

// Project maps a camera-space point to pixel coordinates and depth.
func (k Intrinsics) Project(p Vec3) (u, v, z float64, ok bool) {
	if p.Z <= 1e-6 {
		return 0, 0, 0, false
	}
	u = k.Fx*p.X/p.Z + k.Cx
	v = k.Fy*p.Y/p.Z + k.Cy
	return u, v, p.Z, true
}

// Unproject maps a pixel and camera-space depth back to a world point.
func (k Intrinsics) Unproject(u, v, z float64, p Pose) Vec3 {
	cam := Vec3{X: (u - k.Cx) * z / k.Fx, Y: (v - k.Cy) * z / k.Fy, Z: z}
	inv := transpose(p.Rot) // pure rotation matrix: inverse = transpose
	return Add(p.C, inv.Mul(cam))
}

func transpose(m Mat3) Mat3 {
	var out Mat3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			out.R[i][j] = m.R[j][i]
		}
	}
	return out
}

// Rig describes a parallel-axis stereo rig expressed in the head frame.
type Rig struct {
	Baseline float64 // left center - right center separation (world units)
}

// EyeCenter returns the world position of an eye, given the head center and
// the head's rotation. Right eye sits at -X of the left eye.
func (r Rig) EyeCenter(headC Vec3, headRot Mat3, eye Eye) Vec3 {
	if eye == Left {
		return Add(headC, headRot.Mul(Vec3{X: r.Baseline / 2}))
	}
	return Add(headC, headRot.Mul(Vec3{X: -r.Baseline / 2}))
}

// Eye selects a stereo channel.
type Eye int

const (
	Left Eye = iota
	Right
)

func (e Eye) String() string {
	if e == Left {
		return "left"
	}
	return "right"
}

// HeadSample is the head pose at one instant of the continuous trajectory.
type HeadSample struct {
	T     float64
	C     Vec3
	Yaw   float64
	Pitch float64
	Rot   Mat3
}

// EyePose builds the per-eye world-to-camera pose. Both eyes share the head
// rotation (parallel-axis rig), only the optical center shifts.
func (h HeadSample) EyePose(rig Rig, eye Eye) Pose {
	return Pose{C: rig.EyeCenter(h.C, h.Rot, eye), Rot: h.Rot}
}

// Path is a C1-continuous analytic camera trajectory. All derivatives are
// smooth sinusoids so adjacent frames always form a continuous camera path.
type Path struct {
	Duration     float64
	PosAmplitude Vec3
	YawAmp       float64
	PitchAmp     float64
}

// Sample evaluates the trajectory at time t in [0, Duration].
func (p Path) Sample(t float64) HeadSample {
	w := 2 * math.Pi / p.Duration
	c := Vec3{
		X: p.PosAmplitude.X * math.Sin(w*t),
		Y: p.PosAmplitude.Y * math.Sin(0.5*w*t),
		Z: p.PosAmplitude.Z * math.Sin(w*t),
	}
	yaw := p.YawAmp * math.Sin(0.5*w*t)
	pitch := p.PitchAmp * math.Sin(w*t)
	return HeadSample{T: t, C: c, Yaw: yaw, Pitch: pitch, Rot: RotYX(yaw, pitch)}
}
