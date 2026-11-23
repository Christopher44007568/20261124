package pose

import "math"

// Timestamp is an absolute time in nanoseconds within one clock domain.
// Durations are also expressed in nanoseconds. Using int64 keeps ordering,
// negative clock corrections and arithmetic exact (no float time drift).
type Timestamp int64

// Vec3 is a translation sample component.
type Vec3 struct {
	X, Y, Z float64
}

// Quat is a unit quaternion in Hamilton convention: q = w + xi + yj + zk.
// A zero Quat is treated as the identity rotation.
type Quat struct {
	W, X, Y, Z float64
}

// Sample is one pose observation stamped by a single clock.
type Sample struct {
	Time Timestamp
	Pos  Vec3
	Rot  Quat
}

// Identity returns the unit quaternion.
func Identity() Quat { return Quat{W: 1} }

func (q Quat) norm() float64 {
	return math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
}

// Normalize returns the unit-length quaternion; the zero quaternion maps to identity.
func (q Quat) Normalize() Quat {
	n := q.norm()
	if n == 0 {
		return Identity()
	}
	return Quat{q.W / n, q.X / n, q.Y / n, q.Z / n}
}

// Mul returns the Hamilton product a*b, i.e. rotation b applied first, then a.
func (a Quat) Mul(b Quat) Quat {
	return Quat{
		W: a.W*b.W - a.X*b.X - a.Y*b.Y - a.Z*b.Z,
		X: a.W*b.X + a.X*b.W + a.Y*b.Z - a.Z*b.Y,
		Y: a.W*b.Y - a.X*b.Z + a.Y*b.W + a.Z*b.X,
		Z: a.W*b.Z + a.X*b.Y - a.Y*b.X + a.Z*b.W,
	}
}

// Conj returns the conjugate (inverse for unit quaternions).
func (q Quat) Conj() Quat { return Quat{q.W, -q.X, -q.Y, -q.Z} }

// Dot returns the 4-D inner product.
func (a Quat) Dot(b Quat) float64 {
	return a.W*b.W + a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// RotAngleZ returns a rotation of angle radians around the Z axis.
func RotAngleZ(angle float64) Quat {
	h := angle * 0.5
	return Quat{W: math.Cos(h), Z: math.Sin(h)}
}

// RotAngleY returns a rotation of angle radians around the Y axis.
func RotAngleY(angle float64) Quat {
	h := angle * 0.5
	return Quat{W: math.Cos(h), Y: math.Sin(h)}
}

// AngleBetween returns the geodesic rotation angle in radians between two
// unit quaternions, wrapped into [0, pi].
func AngleBetween(a, b Quat) float64 {
	d := math.Abs(a.Normalize().Dot(b.Normalize()))
	if d > 1 {
		d = 1
	}
	return 2 * math.Acos(d)
}

func lerp64(a, b, t float64) float64 { return a + (b-a)*t }

// Lerp interpolates a translation vector component-wise.
func Lerp(a, b Vec3, t float64) Vec3 {
	return Vec3{
		X: lerp64(a.X, b.X, t),
		Y: lerp64(a.Y, b.Y, t),
		Z: lerp64(a.Z, b.Z, t),
	}
}

// Slerp performs spherical linear interpolation between two unit quaternions.
// Translation and rotation are intentionally resampled by different rules:
// linear interpolation for position, constant-angular-velocity slerp for
// orientation. Degenerate (nearly identical or antipodal) cases fall back to
// normalized linear interpolation.
func Slerp(a, b Quat, t float64) Quat {
	a = a.Normalize()
	b = b.Normalize()
	d := a.Dot(b)
	if d < 0 { // take the shorter arc
		b = Quat{-b.W, -b.X, -b.Y, -b.Z}
		d = -d
	}
	if d > 0.9995 {
		return Quat{
			W: lerp64(a.W, b.W, t),
			X: lerp64(a.X, b.X, t),
			Y: lerp64(a.Y, b.Y, t),
			Z: lerp64(a.Z, b.Z, t),
		}.Normalize()
	}
	theta := math.Acos(d)
	sinTheta := math.Sin(theta)
	s0 := math.Sin((1-t)*theta) / sinTheta
	s1 := math.Sin(t*theta) / sinTheta
	return Quat{
		W: s0*a.W + s1*b.W,
		X: s0*a.X + s1*b.X,
		Y: s0*a.Y + s1*b.Y,
		Z: s0*a.Z + s1*b.Z,
	}.Normalize()
}

// Distance returns the Euclidean distance between two translations.
func Distance(a, b Vec3) float64 {
	dx, dy, dz := a.X-b.X, a.Y-b.Y, a.Z-b.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}
