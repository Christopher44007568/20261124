package lensdistortion

import "math"

// Point is a position in the shared display/source coordinate frame.
//
// Pixel raster helpers in this package use integer pixel indices as pixel
// centers. For an image of width W, valid pixel centers therefore have X in
// [0, W-1]. Calibration coordinates must use the same convention.
type Point struct {
	X float64
	Y float64
}

func (p Point) Add(q Point) Point   { return Point{p.X + q.X, p.Y + q.Y} }
func (p Point) Sub(q Point) Point   { return Point{p.X - q.X, p.Y - q.Y} }
func (p Point) Mul(s float64) Point { return Point{p.X * s, p.Y * s} }
func (p Point) Length() float64     { return math.Hypot(p.X, p.Y) }
func (p Point) IsFinite() bool      { return isFinite(p.X) && isFinite(p.Y) }
func distance(a, b Point) float64   { return a.Sub(b).Length() }
func isFinite(v float64) bool       { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Rect is an inclusive axis-aligned rectangle in the shared coordinate frame.
type Rect struct {
	Min Point
	Max Point
}

func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X &&
		p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// Region tests whether a display coordinate belongs to the visible area.
// Regions are only used to reject output pixels; source lookup remains the
// responsibility of the selected Mapper.
type Region interface {
	Contains(Point) bool
}

// RectRegion is useful for the conventional rectangular display case.
type RectRegion struct {
	Rect Rect
}

func (r RectRegion) Contains(p Point) bool { return r.Rect.Contains(p) }

// PolygonRegion describes a convex or concave non-rectangular display area.
// Vertices must be simple (non-self-intersecting), finite and listed in order.
// Edges are treated as part of the region.
type PolygonRegion struct {
	Vertices []Point
}

func NewPolygonRegion(vertices []Point) (PolygonRegion, error) {
	if len(vertices) < 3 {
		return PolygonRegion{}, ErrInvalidRegion
	}
	copied := make([]Point, len(vertices))
	for i, p := range vertices {
		if !p.IsFinite() {
			return PolygonRegion{}, ErrInvalidRegion
		}
		copied[i] = p
	}
	return PolygonRegion{Vertices: copied}, nil
}

func (p PolygonRegion) Contains(q Point) bool {
	if !q.IsFinite() || len(p.Vertices) < 3 {
		return false
	}
	n := len(p.Vertices)
	inside := false
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := p.Vertices[j], p.Vertices[i]
		if pointOnSegment(q, a, b) {
			return true
		}
		crosses := (a.Y > q.Y) != (b.Y > q.Y)
		if crosses {
			x := (b.X-a.X)*(q.Y-a.Y)/(b.Y-a.Y) + a.X
			if q.X < x {
				inside = !inside
			}
		}
	}
	return inside
}

func pointOnSegment(p, a, b Point) bool {
	cross := (p.X-a.X)*(b.Y-a.Y) - (p.Y-a.Y)*(b.X-a.X)
	scale := math.Max(1, math.Max(math.Abs(b.X-a.X), math.Abs(b.Y-a.Y)))
	if math.Abs(cross) > 1e-10*scale {
		return false
	}
	return p.X >= math.Min(a.X, b.X)-1e-10 &&
		p.X <= math.Max(a.X, b.X)+1e-10 &&
		p.Y >= math.Min(a.Y, b.Y)-1e-10 &&
		p.Y <= math.Max(a.Y, b.Y)+1e-10
}
