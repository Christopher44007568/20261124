package transparency

import "math"

const pixelFootprintRadius = 0.75

// FullScreenShape is used by fog. Fog has coverage everywhere, but its
// opacity and color still move independently of the glass and filament.
type FullScreenShape struct{}

func (FullScreenShape) Inside(Vec2) bool  { return true }
func (FullScreenShape) Support(Vec2) bool { return true }

// RotatedRect is a rectangular glass pane.
type RotatedRect struct {
	Center Vec2
	Half   Vec2
	Angle  float64
}

func (r RotatedRect) local(p Vec2) Vec2 {
	d := p.Sub(r.Center)
	c, s := math.Cos(-r.Angle), math.Sin(-r.Angle)
	return Vec2{d.X*c - d.Y*s, d.X*s + d.Y*c}
}

func (r RotatedRect) Inside(p Vec2) bool {
	q := r.local(p)
	return math.Abs(q.X) <= r.Half.X && math.Abs(q.Y) <= r.Half.Y
}

func (r RotatedRect) Support(p Vec2) bool {
	q := r.local(p)
	dx := math.Max(math.Abs(q.X)-r.Half.X, 0)
	dy := math.Max(math.Abs(q.Y)-r.Half.Y, 0)
	return math.Hypot(dx, dy) <= pixelFootprintRadius
}

// LocalPoint exposes the rectangle's local coordinates for depth-varying
// glass. It is intentionally small and CPU-only.
func (r RotatedRect) LocalPoint(p Vec2) Vec2 { return r.local(p) }

// Segment is a thin filament represented by a capsule. Subpixel samples use
// Inside; pixel footprints use Support so antialiased edges can still receive
// reprojected history.
type Segment struct {
	A      Vec2
	B      Vec2
	Radius float64
}

func (s Segment) distance(p Vec2) float64 {
	ab := s.B.Sub(s.A)
	denom := ab.X*ab.X + ab.Y*ab.Y
	if denom == 0 {
		return p.Sub(s.A).Length()
	}
	ap := p.Sub(s.A)
	t := clamp((ap.X*ab.X+ap.Y*ab.Y)/denom, 0, 1)
	return p.Sub(s.A.Add(ab.Mul(t))).Length()
}

func (s Segment) Inside(p Vec2) bool  { return s.distance(p) <= s.Radius }
func (s Segment) Support(p Vec2) bool { return s.distance(p) <= s.Radius+pixelFootprintRadius }
