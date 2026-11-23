package xr

import "math"

// Vec3 is a 3-component vector in metres.
type Vec3 struct{ X, Y, Z float64 }

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
func (v Vec3) Norm() float64 { return math.Sqrt(v.Dot(v)) }

// Quat is a unit quaternion (W, X, Y, Z) describing a rotation.
type Quat struct{ W, X, Y, Z float64 }

// QuatFromAxisAngle builds the rotation of angle radians around the (unit) axis.
func QuatFromAxisAngle(axis Vec3, angle float64) Quat {
	h := angle * 0.5
	s := math.Sin(h)
	return Quat{math.Cos(h), axis.X * s, axis.Y * s, axis.Z * s}
}

func (q Quat) Rotate(v Vec3) Vec3 {
	// q v q^-1 = v + 2 w (u x v) + 2 (u x (u x v)) for unit quaternions.
	u := Vec3{q.X, q.Y, q.Z}
	uv := u.Cross(v)
	uuv := u.Cross(uv)
	return v.Add(uv.Scale(2 * q.W)).Add(uuv.Scale(2))
}

func (q Quat) Multiply(o Quat) Quat {
	return Quat{
		W: q.W*o.W - q.X*o.X - q.Y*o.Y - q.Z*o.Z,
		X: q.W*o.X + q.X*o.W + q.Y*o.Z - q.Z*o.Y,
		Y: q.W*o.Y - q.X*o.Z + q.Y*o.W + q.Z*o.X,
		Z: q.W*o.Z + q.X*o.Y - q.Y*o.X + q.Z*o.W,
	}
}

// AngularVelocity converts a finite yaw-delta (rotation about world Y) into
// an angular-velocity vector expressed in the device's local frame.
func AngularVelocity(yawDelta, dt float64, orientation Quat) Vec3 {
	if dt <= 0 {
		return Vec3{}
	}
	worldOmega := Vec3{0, yawDelta / dt, 0}
	// world -> local: use the inverse rotation; inverse of a unit quaternion
	// applied to a vector equals rotating by the conjugate.
	inv := Quat{W: orientation.W, X: -orientation.X, Y: -orientation.Y, Z: -orientation.Z}
	return inv.Rotate(worldOmega)
}

// Pose is a rigid transform: device position/orientation in a world frame.
type Pose struct {
	P Vec3
	Q Quat
}

// Transform re-expresses a pose through a rigid delta T (new = T * old).
func (p Pose) Transform(t RigidTransform) Pose {
	return Pose{P: t.Rotate(p.P).Add(t.T), Q: t.Q.Multiply(p.Q)}
}

// RigidTransform maps coordinates from one world origin to another:
// p_new = R * p_old + T.
type RigidTransform struct {
	Q Quat
	T Vec3
}

// IdentityTransform is the canonical rigid transform (no rotation/translation).
// A zero-value Quat is NOT a valid rotation, so prefer this constant.
var IdentityTransform = RigidTransform{Q: Quat{W: 1}}

func (t RigidTransform) Rotate(v Vec3) Vec3 { return t.Q.Rotate(v) }

// Invert returns the transform that maps the new frame back to the old one.
func (t RigidTransform) Invert() RigidTransform {
	invQ := Quat{W: t.Q.W, X: -t.Q.X, Y: -t.Q.Y, Z: -t.Q.Z}
	return RigidTransform{Q: invQ, T: invQ.Rotate(t.T).Scale(-1)}
}

// PositionError is the Euclidean distance between two poses in metres.
func PositionError(a, b Pose) float64 { return a.P.Sub(b.P).Norm() }

// AngleError returns the absolute yaw difference between two orientations.
func AngleError(a, b Quat) float64 {
	// Relative rotation quaternion: r = b^-1 * a; angle = 2*acos(|w|).
	inv := Quat{W: b.W, X: -b.X, Y: -b.Y, Z: -b.Z}
	r := inv.Multiply(a)
	w := r.W
	if w < 0 {
		w = -w
	}
	if w > 1 {
		w = 1
	}
	return 2 * math.Acos(w)
}

// Extrapolate advances a pose using last-known linear/angular velocity for dt.
// Linear motion integrates velocity; orientation integrates the local-frame
// angular velocity as an axis-angle delta on the left (world-frame update).
func Extrapolate(p Pose, linVel, angVel Vec3, dt float64) Pose {
	out := Pose{P: p.P.Add(linVel.Scale(dt))}
	angle := angVel.Norm() * dt
	if angle > 1e-12 {
		axis := Vec3{angVel.X, angVel.Y, angVel.Z}.Scale(1 / angVel.Norm())
		// angVel is local; bring the rotation axis into the world frame.
		worldAxis := p.Q.Rotate(axis)
		dq := QuatFromAxisAngle(worldAxis, angle)
		out.Q = dq.Multiply(p.Q)
	} else {
		out.Q = p.Q
	}
	return out
}
