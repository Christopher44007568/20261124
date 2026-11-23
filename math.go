package stereocull

import "math"

// Vec3 is a world/head-local 3D vector.
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
func (v Vec3) Length() float64 { return math.Sqrt(v.Dot(v)) }
func (v Vec3) Normalized() Vec3 {
	l := v.Length()
	if l == 0 {
		return Vec3{}
	}
	return v.Scale(1 / l)
}

// Quat is a unit quaternion W + Xi + Yj + Zk.
type Quat struct{ W, X, Y, Z float64 }

// QuatEuler builds a quaternion from intrinsic yaw (Y), pitch (X), roll (Z)
// radians applied in roll-pitch-yaw composition order.
func QuatEuler(pitch, yaw, roll float64) Quat {
	hp, cp := math.Sincos(pitch / 2)
	hy, cy := math.Sincos(yaw / 2)
	hr, cr := math.Sincos(roll / 2)
	qr := Quat{W: cr, Z: hr}
	qp := Quat{W: cp, X: hp}
	qy := Quat{W: cy, Y: hy}
	return qy.Mul(qp).Mul(qr).normalized()
}

// Mul returns the Hamilton product q*o.
func (q Quat) Mul(o Quat) Quat {
	return Quat{
		W: q.W*o.W - q.X*o.X - q.Y*o.Y - q.Z*o.Z,
		X: q.W*o.X + q.X*o.W + q.Y*o.Z - q.Z*o.Y,
		Y: q.W*o.Y - q.X*o.Z + q.Y*o.W + q.Z*o.X,
		Z: q.W*o.Z + q.X*o.Y - q.Y*o.X + q.Z*o.W,
	}
}

func (q Quat) normalized() Quat {
	l := math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
	return Quat{q.W / l, q.X / l, q.Y / l, q.Z / l}
}

// Rotate rotates a vector by the quaternion.
func (q Quat) Rotate(v Vec3) Vec3 {
	// v' = q * v * q^-1, expanded per component.
	uv := Vec3{q.X, q.Y, q.Z}
	u := uv.Cross(v).Scale(2)
	return v.Add(u.Scale(q.W)).Add(uv.Cross(u))
}

// Plane is a half-space: points with normal.Dot(p) + d >= 0 are inside.
type Plane struct {
	Normal Vec3
	D      float64
}

// PlaneFromPoint builds a plane with the given outward unit normal passing
// through point p: normal.Dot(x) - normal.Dot(p) >= 0 inside.
func PlaneFromPoint(normal, p Vec3) Plane {
	return Plane{Normal: normal, D: -normal.Dot(p)}
}

// Transform moves a local-space plane into world space under rotation r and
// translation t, keeping its inside/outside orientation.
func (p Plane) Transform(r Quat, t Vec3) Plane {
	n := r.Rotate(p.Normal)
	return Plane{Normal: n, D: p.D - n.Dot(t)}
}

// Distance is the signed distance of point p (positive inside).
func (p Plane) Distance(pnt Vec3) float64 { return p.Normal.Dot(pnt) + p.D }
