package headpose

import (
	"errors"
	"fmt"
	"math"
)

// Transform is a uniform-scale, right-handed similarity transform:
//
//	p_target = Rotation * (Scale * p_source) + Translation
//
// Rotation is a unit quaternion. Scale converts one source unit into target
// units. Translation is expressed in target units.
type Transform struct {
	rotation    Quat
	translation Vec3
	scale       float64
}

// NewSimilarity validates and constructs a similarity transform.
func NewSimilarity(rotation Quat, translation Vec3, scale float64) (Transform, error) {
	normalized, err := rotation.Normalize()
	if err != nil {
		return Transform{}, fmt.Errorf("rotation: %w", err)
	}
	if !translation.IsFinite() {
		return Transform{}, errors.New("translation must be finite")
	}
	if !isFinite(scale) || scale <= 0 {
		return Transform{}, errors.New("scale must be finite and greater than zero")
	}
	return Transform{rotation: normalized, translation: translation, scale: scale}, nil
}

// NewRigid constructs a rotation and translation with a scale of one.
func NewRigid(rotation Quat, translation Vec3) (Transform, error) {
	return NewSimilarity(rotation, translation, 1)
}

// IdentityTransform returns the identity transform with unit scale.
func IdentityTransform() Transform {
	return Transform{rotation: IdentityQuat(), scale: 1}
}

// Rotation returns the target-from-source rotation.
func (t Transform) Rotation() Quat {
	return t.rotation
}

// Translation returns the target-frame translation.
func (t Transform) Translation() Vec3 {
	return t.translation
}

// Scale returns target units per source unit.
func (t Transform) Scale() float64 {
	return t.scale
}

// ApplyPoint maps a point from the source frame to the target frame.
func (t Transform) ApplyPoint(point Vec3) Vec3 {
	return t.rotation.Rotate(point.Scale(t.scale)).Add(t.translation)
}

// ApplyDirection rotates a direction vector without applying translation or scale.
func (t Transform) ApplyDirection(direction Vec3) Vec3 {
	return t.rotation.Rotate(direction)
}

// Compose returns the transform from t's source to next's target.
//
// If t maps A to B and next maps B to C, t.Compose(next) maps A to C.
func (t Transform) Compose(next Transform) Transform {
	rotation := normalizedQuat(next.rotation.Mul(t.rotation))
	translation := next.rotation.Rotate(t.translation.Scale(next.scale)).Add(next.translation)
	return Transform{
		rotation:    rotation,
		translation: translation,
		scale:       t.scale * next.scale,
	}
}

// Inverse returns the exact inverse transform.
func (t Transform) Inverse() Transform {
	rotation := normalizedQuat(t.rotation.Conjugate())
	translation := rotation.Rotate(t.translation.Scale(-1 / t.scale))
	return Transform{
		rotation:    rotation,
		translation: translation,
		scale:       1 / t.scale,
	}
}

// Determinant returns the determinant of the linear part. It is Scale cubed
// for a similarity and is always positive for a valid right-handed transform.
func (t Transform) Determinant() float64 {
	return t.scale * t.scale * t.scale
}

// AlmostEqual compares two similarities.
func (t Transform) AlmostEqual(o Transform, tolerance float64) bool {
	if !t.translation.AlmostEqual(o.translation, tolerance) {
		return false
	}
	if math.Abs(t.scale-o.scale) > tolerance*math.Max(1, math.Abs(t.scale)) {
		return false
	}
	return t.rotation.SameRotation(o.rotation, tolerance)
}

// IsRigid reports whether the scale is one within tolerance.
func (t Transform) IsRigid(tolerance float64) bool {
	return math.Abs(t.scale-1) <= tolerance
}

// Validate checks that the transform is finite, right-handed, and normalized.
func (t Transform) Validate() error {
	if !t.translation.IsFinite() {
		return errors.New("translation must be finite")
	}
	if !isFinite(t.scale) || t.scale <= 0 {
		return errors.New("scale must be finite and greater than zero")
	}
	if !isFinite(t.rotation.W) || !isFinite(t.rotation.X) ||
		!isFinite(t.rotation.Y) || !isFinite(t.rotation.Z) {
		return errors.New("rotation components must be finite")
	}
	if !t.rotation.IsUnit(1e-9) {
		return errors.New("rotation must be a unit quaternion")
	}
	return nil
}

func normalizedQuat(q Quat) Quat {
	normalized, err := q.Normalize()
	if err != nil {
		return IdentityQuat()
	}
	return normalized
}
