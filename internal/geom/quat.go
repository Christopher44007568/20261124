package geom

import "math"

// Quat is a unit quaternion in (w, x, y, z) order.
type Quat struct {
	W, X, Y, Z float64
}

func QuatIdentity() Quat {
	return Quat{W: 1}
}

// QuatFromEuler builds a rotation from yaw (Y), pitch (X), and roll (Z)
// in radians. The order is yaw * pitch * roll in active, right-handed form.
func QuatFromEuler(yaw, pitch, roll float64) Quat {
	qy := Quat{W: math.Cos(yaw / 2), Y: math.Sin(yaw / 2)}
	qx := Quat{W: math.Cos(pitch / 2), X: math.Sin(pitch / 2)}
	qz := Quat{W: math.Cos(roll / 2), Z: math.Sin(roll / 2)}
	return qy.Mul(qx).Mul(qz).Normalized()
}

func (q Quat) Mul(o Quat) Quat {
	return Quat{
		W: q.W*o.W - q.X*o.X - q.Y*o.Y - q.Z*o.Z,
		X: q.W*o.X + q.X*o.W + q.Y*o.Z - q.Z*o.Y,
		Y: q.W*o.Y - q.X*o.Z + q.Y*o.W + q.Z*o.X,
		Z: q.W*o.Z + q.X*o.Y - q.Y*o.X + q.Z*o.W,
	}
}

func (q Quat) Dot(o Quat) float64 {
	return q.W*o.W + q.X*o.X + q.Y*o.Y + q.Z*o.Z
}

func (q Quat) Norm() float64 {
	return math.Sqrt(q.Dot(q))
}

func (q Quat) Normalized() Quat {
	n := q.Norm()
	if n == 0 {
		return QuatIdentity()
	}
	return Quat{W: q.W / n, X: q.X / n, Y: q.Y / n, Z: q.Z / n}
}

// Rotate applies the quaternion to v.
func (q Quat) Rotate(v Vec3) Vec3 {
	u := Vec3{X: q.X, Y: q.Y, Z: q.Z}
	uv := u.Dot(v)
	uuv := u.Dot(u)
	cross := Vec3{
		X: u.Y*v.Z - u.Z*v.Y,
		Y: u.Z*v.X - u.X*v.Z,
		Z: u.X*v.Y - u.Y*v.X,
	}
	return v.Scale(q.W*q.W - uuv).
		Add(u.Scale(2 * uv)).
		Add(cross.Scale(2 * q.W))
}

// Slerp interpolates along the shortest arc. u is clamped to [0, 1].
func Slerp(a, b Quat, u float64) Quat {
	a = a.Normalized()
	b = b.Normalized()
	if u <= 0 {
		return a
	}
	if u >= 1 {
		return b
	}
	dot := a.Dot(b)
	if dot < 0 {
		b = Quat{W: -b.W, X: -b.X, Y: -b.Y, Z: -b.Z}
		dot = -dot
	}
	if dot > 0.9995 {
		return Quat{
			W: a.W + (b.W-a.W)*u,
			X: a.X + (b.X-a.X)*u,
			Y: a.Y + (b.Y-a.Y)*u,
			Z: a.Z + (b.Z-a.Z)*u,
		}.Normalized()
	}
	theta := math.Acos(clamp(dot, -1, 1))
	sinTheta := math.Sin(theta)
	wa := math.Sin((1-u)*theta) / sinTheta
	wb := math.Sin(u*theta) / sinTheta
	return Quat{
		W: wa*a.W + wb*b.W,
		X: wa*a.X + wb*b.X,
		Y: wa*a.Y + wb*b.Y,
		Z: wa*a.Z + wb*b.Z,
	}.Normalized()
}

// AngleDeg returns the shortest angular distance between two orientations.
func AngleDeg(a, b Quat) float64 {
	dot := math.Abs(a.Normalized().Dot(b.Normalized()))
	return 2 * math.Acos(clamp(dot, -1, 1)) * 180 / math.Pi
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
