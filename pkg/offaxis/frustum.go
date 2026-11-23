package offaxis

import (
	"errors"
	"math"
)

// Validation errors returned by Eye.Validate / BuildStereo.
var (
	// ErrInvalidDepth is returned when near or far is not strictly positive.
	ErrInvalidDepth = errors.New("near and far must be strictly positive")
	// ErrReversedDepth is returned when near >= far.
	ErrReversedDepth = errors.New("near plane must be in front of the far plane (near < far)")
	// ErrDegenerate is returned when left==right or bottom==top, which
	// collapses the frustum into a zero-area slice (degenerate corner).
	ErrDegenerate = errors.New("frustum aperture is degenerate: left==right or bottom==top")
	// ErrCenterRay is returned when the reconstructed projection matrix no
	// longer maps the frustum center ray onto the NDC origin, or when a
	// stereo pair does not share parallel center rays (toed-in setup).
	ErrCenterRay = errors.New("center ray inconsistent with projection matrix / stereo pair")
)

// Bounds describes the six signed frustum planes by their intersection with
// the near plane and the near/far distances. The camera looks down -Z, so
// near and far are positive eye-space depths.
//
// All four lateral bounds are arbitrary and independent: left may be
// positive, right may be negative, and top may sit below bottom until the
// strict ordering left < right / bottom < top holds. This is exactly the
// off-axis case that a symmetric viewport shift cannot represent.
type Bounds struct {
	Left, Right float64
	Bottom, Top float64
	Near, Far   float64
}

// Pose places an eye in head space. The zero value is the identity pose,
// which is used for all single-eye examples.
type Pose struct {
	Position Vec3
	// Rotation maps eye-space vectors into head space. Identity for
	// parallel-axis stereo; a yawed rotation models a toed-in camera.
	Rotation Mat4
}

// Eye is one frustum plus its head-space placement.
type Eye struct {
	Name   string
	Bounds Bounds
	Pose   Pose
}

const validationEps = 1e-10

// Validate checks the geometric preconditions that must hold before a
// projection matrix can be produced.
func (b Bounds) Validate() error {
	if !isFinite(b.Left) || !isFinite(b.Right) || !isFinite(b.Bottom) ||
		!isFinite(b.Top) || !isFinite(b.Near) || !isFinite(b.Far) {
		return ErrInvalidDepth
	}
	if b.Near <= 0 || b.Far <= 0 {
		return ErrInvalidDepth
	}
	if b.Near >= b.Far {
		return ErrReversedDepth
	}
	if b.Left == b.Right || b.Bottom == b.Top {
		return ErrDegenerate
	}
	return nil
}

// ProjectionMatrix builds the off-axis projection matrix for arbitrary
// (left, right, bottom, top, near, far) bounds. It matches glFrustum:
//
//	[ 2n/(r-l)   0        (r+l)/(r-l)   0                 ]
//	[ 0          2n/(t-b) (t+b)/(t-b)   0                 ]
//	[ 0          0        -(f+n)/(f-n) -2fn/(f-n)         ]
//	[ 0          0        -1            0                 ]
//
// The third column's (r+l)/(r-l) and (t+b)/(t-b) terms are the off-axis
// terms that a pure symmetric projection plus viewport translation cannot
// reproduce once left/right or bottom/top are asymmetric.
func (b Bounds) ProjectionMatrix() (Mat4, error) {
	if err := b.Validate(); err != nil {
		return Mat4{}, err
	}
	l, r := b.Left, b.Right
	bot, top := b.Bottom, b.Top
	n, f := b.Near, b.Far

	var m Mat4
	m[0][0] = 2 * n / (r - l)
	m[0][2] = (r + l) / (r - l)
	m[1][1] = 2 * n / (top - bot)
	m[1][2] = (top + bot) / (top - bot)
	m[2][2] = -(f + n) / (f - n)
	m[2][3] = -2 * f * n / (f - n)
	m[3][2] = -1

	if err := m.checkCenterRay(b); err != nil {
		return Mat4{}, err
	}
	return m, nil
}

// checkCenterRay projects the near and far center points and requires them
// to land on the NDC origin with the canonical z values (-1 and +1). This
// rejects matrices that have been tampered with and guards against floating
// point drift in the construction above.
func (m Mat4) checkCenterRay(b Bounds) error {
	cx := (b.Left + b.Right) / 2
	cy := (b.Bottom + b.Top) / 2
	nearCenter := Vec3{X: cx, Y: cy, Z: -b.Near}
	farCenter := Vec3{
		X: cx * b.Far / b.Near,
		Y: cy * b.Far / b.Near,
		Z: -b.Far,
	}
	clipNear := m.TransformPoint(nearCenter)
	clipFar := m.TransformPoint(farCenter)
	if clipNear.W <= 0 || clipFar.W <= 0 {
		return ErrCenterRay
	}
	ndcNear := clipNear.NDC()
	ndcFar := clipFar.NDC()
	if !approx(ndcNear.X, 0, validationEps) || !approx(ndcNear.Y, 0, validationEps) ||
		!approx(ndcNear.Z, -1, validationEps) {
		return ErrCenterRay
	}
	if !approx(ndcFar.X, 0, validationEps) || !approx(ndcFar.Y, 0, validationEps) ||
		!approx(ndcFar.Z, 1, validationEps) {
		return ErrCenterRay
	}
	return nil
}

// VerifyCenterRay is the exported form of the internal construction guard.
// It lets callers reject a projection matrix that no longer maps the
// frustum center ray onto the NDC origin, for example after a symmetric
// viewport-shift approximation has overwritten the off-axis column.
func (m Mat4) VerifyCenterRay(b Bounds) error {
	return m.checkCenterRay(b)
}

func approx(a, b, eps float64) bool {
	return math.Abs(a-b) <= eps
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
