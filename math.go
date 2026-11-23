package headpose

import (
	"errors"
	"fmt"
	"math"
)

const eps = 1e-12

// Vec3 is a three-component vector. Its unit is defined by the containing frame.
type Vec3 struct {
	X, Y, Z float64
}

// Add returns v+o.
func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z}
}

// Sub returns v-o.
func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z}
}

// Scale returns s*v.
func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

// Dot returns the dot product v·o.
func (v Vec3) Dot(o Vec3) float64 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

// Cross returns the right-handed cross product v×o.
func (v Vec3) Cross(o Vec3) Vec3 {
	return Vec3{
		X: v.Y*o.Z - v.Z*o.Y,
		Y: v.Z*o.X - v.X*o.Z,
		Z: v.X*o.Y - v.Y*o.X,
	}
}

// Length returns the Euclidean norm of v.
func (v Vec3) Length() float64 {
	return math.Sqrt(v.Dot(v))
}

// Normalize returns a unit vector in the direction of v.
func (v Vec3) Normalize() (Vec3, error) {
	length := v.Length()
	if !isFinite(length) || length < eps {
		return Vec3{}, errors.New("cannot normalize a zero or non-finite vector")
	}
	return v.Scale(1 / length), nil
}

// IsFinite reports whether every component is finite.
func (v Vec3) IsFinite() bool {
	return isFinite(v.X) && isFinite(v.Y) && isFinite(v.Z)
}

// AlmostEqual compares vectors with an absolute tolerance per component.
func (v Vec3) AlmostEqual(o Vec3, tolerance float64) bool {
	return math.Abs(v.X-o.X) <= tolerance &&
		math.Abs(v.Y-o.Y) <= tolerance &&
		math.Abs(v.Z-o.Z) <= tolerance
}

// MaxComponentDiff returns the largest absolute component difference.
func (v Vec3) MaxComponentDiff(o Vec3) float64 {
	return math.Max(math.Abs(v.X-o.X), math.Max(math.Abs(v.Y-o.Y), math.Abs(v.Z-o.Z)))
}

// Quat is a unit quaternion in scalar-first (w, x, y, z) order.
//
// It represents an active, right-handed rotation. Applying q to v rotates v by
// q*v*q^-1; q.Mul(r) applies r first and then q.
type Quat struct {
	W, X, Y, Z float64
}

// IdentityQuat returns the identity rotation.
func IdentityQuat() Quat {
	return Quat{W: 1}
}

// QuatFromComponents validates, normalizes, and returns (w, x, y, z).
func QuatFromComponents(w, x, y, z float64) (Quat, error) {
	q := Quat{W: w, X: x, Y: y, Z: z}
	normalized, err := q.Normalize()
	if err != nil {
		return Quat{}, fmt.Errorf("invalid quaternion: %w", err)
	}
	return normalized, nil
}

// QuatFromAxisAngle builds a right-handed rotation of angleRad around axis.
func QuatFromAxisAngle(axis Vec3, angleRad float64) (Quat, error) {
	if !isFinite(angleRad) {
		return Quat{}, errors.New("angle must be finite")
	}
	unitAxis, err := axis.Normalize()
	if err != nil {
		return Quat{}, fmt.Errorf("invalid rotation axis: %w", err)
	}
	half := angleRad / 2
	sine := math.Sin(half)
	return Quat{
		W: math.Cos(half),
		X: unitAxis.X * sine,
		Y: unitAxis.Y * sine,
		Z: unitAxis.Z * sine,
	}, nil
}

// QuatFromBasis builds the rotation whose local +X, +Y, +Z axes map to xAxis,
// yAxis, and zAxis expressed in the parent frame.
func QuatFromBasis(xAxis, yAxis, zAxis Vec3) (Quat, error) {
	if !xAxis.IsFinite() || !yAxis.IsFinite() || !zAxis.IsFinite() {
		return Quat{}, errors.New("basis axes must be finite")
	}
	if math.Abs(xAxis.Length()-1) > 1e-6 ||
		math.Abs(yAxis.Length()-1) > 1e-6 ||
		math.Abs(zAxis.Length()-1) > 1e-6 {
		return Quat{}, errors.New("basis axes must be unit length")
	}
	if math.Abs(xAxis.Dot(yAxis)) > 1e-6 ||
		math.Abs(xAxis.Dot(zAxis)) > 1e-6 ||
		math.Abs(yAxis.Dot(zAxis)) > 1e-6 {
		return Quat{}, errors.New("basis axes must be orthogonal")
	}
	if xAxis.Cross(yAxis).Sub(zAxis).Length() > 1e-6 {
		return Quat{}, errors.New("basis must be right-handed (X cross Y = Z)")
	}

	m00, m01, m02 := xAxis.X, yAxis.X, zAxis.X
	m10, m11, m12 := xAxis.Y, yAxis.Y, zAxis.Y
	m20, m21, m22 := xAxis.Z, yAxis.Z, zAxis.Z
	trace := m00 + m11 + m22

	var q Quat
	switch {
	case trace > 0:
		s := math.Sqrt(trace+1) * 2
		q = Quat{W: 0.25 * s, X: (m21 - m12) / s, Y: (m02 - m20) / s, Z: (m10 - m01) / s}
	case m00 > m11 && m00 > m22:
		s := math.Sqrt(1+m00-m11-m22) * 2
		q = Quat{W: (m21 - m12) / s, X: 0.25 * s, Y: (m01 + m10) / s, Z: (m02 + m20) / s}
	case m11 > m22:
		s := math.Sqrt(1+m11-m00-m22) * 2
		q = Quat{W: (m02 - m20) / s, X: (m01 + m10) / s, Y: 0.25 * s, Z: (m12 + m21) / s}
	default:
		s := math.Sqrt(1+m22-m00-m11) * 2
		q = Quat{W: (m10 - m01) / s, X: (m02 + m20) / s, Y: (m12 + m21) / s, Z: 0.25 * s}
	}
	return q.Normalize()
}

// Normalize returns q divided by its norm, preserving its sign.
func (q Quat) Normalize() (Quat, error) {
	norm := q.Norm()
	if !isFinite(norm) || norm < eps {
		return Quat{}, errors.New("quaternion norm must be finite and non-zero")
	}
	inv := 1 / norm
	return Quat{W: q.W * inv, X: q.X * inv, Y: q.Y * inv, Z: q.Z * inv}, nil
}

// Norm returns the Euclidean norm of the four components.
func (q Quat) Norm() float64 {
	return math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
}

// Conjugate returns the inverse rotation for a unit quaternion.
func (q Quat) Conjugate() Quat {
	return Quat{W: q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
}

// Negate returns -q, which represents the same rotation as q.
func (q Quat) Negate() Quat {
	return Quat{W: -q.W, X: -q.X, Y: -q.Y, Z: -q.Z}
}

// Dot returns the four-component dot product.
func (q Quat) Dot(o Quat) float64 {
	return q.W*o.W + q.X*o.X + q.Y*o.Y + q.Z*o.Z
}

// Mul returns q*r. The resulting rotation applies r first, then q.
func (q Quat) Mul(r Quat) Quat {
	return Quat{
		W: q.W*r.W - q.X*r.X - q.Y*r.Y - q.Z*r.Z,
		X: q.W*r.X + q.X*r.W + q.Y*r.Z - q.Z*r.Y,
		Y: q.W*r.Y - q.X*r.Z + q.Y*r.W + q.Z*r.X,
		Z: q.W*r.Z + q.X*r.Y - q.Y*r.X + q.Z*r.W,
	}
}

// Rotate applies the active rotation to v.
func (q Quat) Rotate(v Vec3) Vec3 {
	qv := Vec3{q.X, q.Y, q.Z}
	t := qv.Cross(v).Scale(2)
	return v.Add(t.Scale(q.W)).Add(qv.Cross(t))
}

// SameRotation reports whether q and o differ only by sign.
func (q Quat) SameRotation(o Quat, tolerance float64) bool {
	qNorm, oNorm := q.Norm(), o.Norm()
	if qNorm < eps || oNorm < eps {
		return false
	}
	dot := math.Abs(q.Dot(o) / (qNorm * oNorm))
	return 1-dot <= tolerance
}

// IsUnit reports whether q has unit norm within tolerance.
func (q Quat) IsUnit(tolerance float64) bool {
	return math.Abs(q.Norm()-1) <= tolerance
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
