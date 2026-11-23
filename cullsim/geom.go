package main

import "math"

// Vec2 is a 2D vector / point.
type Vec2 struct {
	X, Y float64
}

func (a Vec2) Add(b Vec2) Vec2     { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Sub(b Vec2) Vec2     { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Mul(s float64) Vec2  { return Vec2{a.X * s, a.Y * s} }
func (a Vec2) Dot(b Vec2) float64  { return a.X*b.X + a.Y*b.Y }
func (a Vec2) Len() float64        { return math.Hypot(a.X, a.Y) }
func (a Vec2) Dist(b Vec2) float64 { return a.Sub(b).Len() }

func (a Vec2) Norm() Vec2 {
	l := a.Len()
	if l < 1e-12 {
		return Vec2{}
	}
	return a.Mul(1 / l)
}

// Perp returns the vector rotated 90 degrees counter-clockwise.
func (a Vec2) Perp() Vec2 { return Vec2{-a.Y, a.X} }

// FromAngle returns a unit vector for the given angle (radians).
func FromAngle(rad float64) Vec2 { return Vec2{math.Cos(rad), math.Sin(rad)} }

// Seg is a line segment from A to B.
type Seg struct {
	A, B Vec2
}

func cross(o, a, b Vec2) float64 {
	return (a.X-o.X)*(b.Y-o.Y) - (a.Y-o.Y)*(b.X-o.X)
}

// SegmentsIntersect reports whether segments p and q properly intersect
// (touching endpoints counts as intersecting).
func SegmentsIntersect(p, q Seg) bool {
	d1 := cross(q.A, q.B, p.A)
	d2 := cross(q.A, q.B, p.B)
	d3 := cross(p.A, p.B, q.A)
	d4 := cross(p.A, p.B, q.B)
	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}
	const eps = 1e-9
	onSeg := func(a, b, c Vec2) bool {
		return math.Min(a.X, b.X)-eps <= c.X && c.X <= math.Max(a.X, b.X)+eps &&
			math.Min(a.Y, b.Y)-eps <= c.Y && c.Y <= math.Max(a.Y, b.Y)+eps
	}
	if math.Abs(d1) < eps && onSeg(q.A, q.B, p.A) {
		return true
	}
	if math.Abs(d2) < eps && onSeg(q.A, q.B, p.B) {
		return true
	}
	if math.Abs(d3) < eps && onSeg(p.A, p.B, q.A) {
		return true
	}
	if math.Abs(d4) < eps && onSeg(p.A, p.B, q.B) {
		return true
	}
	return false
}

// Blocked reports whether the segment a->b crosses any wall.
func Blocked(walls []Seg, a, b Vec2) bool {
	s := Seg{a, b}
	for _, w := range walls {
		if SegmentsIntersect(s, w) {
			return true
		}
	}
	return false
}

// InCone reports whether point p lies inside the cone rooted at origin,
// facing dirYaw with the given half-angle and within maxDist.
func InCone(origin Vec2, dirYaw, halfAngle, maxDist float64, p Vec2) bool {
	d := p.Sub(origin)
	dist := d.Len()
	if dist > maxDist {
		return false
	}
	if dist < 1e-9 {
		return true
	}
	ang := math.Atan2(d.Y, d.X)
	diff := math.Abs(NormAngle(ang - dirYaw))
	return diff <= halfAngle
}

// NormAngle wraps an angle to [-pi, pi].
func NormAngle(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
