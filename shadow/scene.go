package shadow

import "math"

const eps = 1e-9

// Vec2 is a point or direction on the ground plane (X/Z).
type Vec2 struct {
	X, Z float64
}

func (v Vec2) Add(o Vec2) Vec2      { return Vec2{v.X + o.X, v.Z + o.Z} }
func (v Vec2) Sub(o Vec2) Vec2      { return Vec2{v.X - o.X, v.Z - o.Z} }
func (v Vec2) Scale(s float64) Vec2 { return Vec2{v.X * s, v.Z * s} }
func (v Vec2) Dot(o Vec2) float64   { return v.X*o.X + v.Z*o.Z }
func (v Vec2) Len() float64         { return math.Hypot(v.X, v.Z) }

// Vec3 is a world-space point or direction.
type Vec3 struct {
	X, Y, Z float64
}

func (v Vec3) Add(o Vec3) Vec3      { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v Vec3) Sub(o Vec3) Vec3      { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }
func (v Vec3) Scale(s float64) Vec3 { return Vec3{v.X * s, v.Y * s, v.Z * s} }
func (v Vec3) Dot(o Vec3) float64   { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }
func (v Vec3) Normalized() Vec3 {
	n := math.Sqrt(v.Dot(v))
	if n < eps {
		return Vec3{}
	}
	return v.Scale(1 / n)
}

// Bounds covers the ground area represented by the shadow maps.
type Bounds struct {
	MinX, MaxX, MinZ, MaxZ float64
}

func (b Bounds) Contains(p Vec2) bool {
	return p.X >= b.MinX && p.X <= b.MaxX && p.Z >= b.MinZ && p.Z <= b.MaxZ
}

// CasterKind identifies the footprint used by a vertical caster.
type CasterKind uint8

const (
	BoxCaster CasterKind = iota
	RodCaster
)

// Caster is a vertical prism. Boxes use Half; rods use Radius.
// BlocksSight enables camera-to-caster occlusion testing.
type Caster struct {
	Name        string
	Kind        CasterKind
	Center      Vec2
	Half        Vec2
	Radius      float64
	Height      float64
	BlocksSight bool
}

// HitsLight reports whether a ground point is shadowed by c and returns the
// entry distance along the direction toward the light.
func (c Caster) HitsLight(p Vec2, light Vec3) bool {
	if light.Y <= eps {
		return false
	}
	dir := Vec2{light.X, light.Z}
	tEnter, tExit, ok := c.rayInterval(p, dir)
	if !ok || tExit < 0 {
		return false
	}
	t := tEnter
	if t < 0 {
		t = 0
	}
	return t*light.Y <= c.Height+eps
}

func (c Caster) rayInterval(o, d Vec2) (float64, float64, bool) {
	switch c.Kind {
	case RodCaster:
		return rayCircle(o, d, c.Center, c.Radius)
	default:
		return rayBox(o, d, Vec2{c.Center.X - c.Half.X, c.Center.Z - c.Half.Z},
			Vec2{c.Center.X + c.Half.X, c.Center.Z + c.Half.Z})
	}
}

func rayBox(o, d, min, max Vec2) (float64, float64, bool) {
	tMin, tMax := math.Inf(-1), math.Inf(1)
	if math.Abs(d.X) < eps {
		if o.X < min.X || o.X > max.X {
			return 0, 0, false
		}
	} else {
		a := (min.X - o.X) / d.X
		b := (max.X - o.X) / d.X
		if a > b {
			a, b = b, a
		}
		tMin, tMax = math.Max(tMin, a), math.Min(tMax, b)
	}
	if math.Abs(d.Z) < eps {
		if o.Z < min.Z || o.Z > max.Z {
			return 0, 0, false
		}
	} else {
		a := (min.Z - o.Z) / d.Z
		b := (max.Z - o.Z) / d.Z
		if a > b {
			a, b = b, a
		}
		tMin, tMax = math.Max(tMin, a), math.Min(tMax, b)
	}
	return tMin, tMax, tMin <= tMax
}

func rayCircle(o, d Vec2, center Vec2, radius float64) (float64, float64, bool) {
	rel := o.Sub(center)
	a := d.Dot(d)
	if a < eps {
		return 0, 0, rel.Dot(rel) <= radius*radius
	}
	b := 2 * rel.Dot(d)
	c := rel.Dot(rel) - radius*radius
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, 0, false
	}
	root := math.Sqrt(disc)
	return (-b - root) / (2 * a), (-b + root) / (2 * a), true
}

// segmentHitsCaster tests a 3D segment against a caster's vertical prism.
func segmentHitsCaster(a, b Vec3, c Caster) bool {
	o := Vec2{a.X, a.Z}
	d := Vec2{b.X - a.X, b.Z - a.Z}
	t0, t1, ok := c.rayInterval(o, d)
	if !ok {
		return false
	}
	t0, t1 = math.Max(t0, 0), math.Min(t1, 1)
	if t0 > t1 {
		return false
	}
	y0 := a.Y + (b.Y-a.Y)*t0
	y1 := a.Y + (b.Y-a.Y)*t1
	return math.Min(y0, y1) <= c.Height+eps
}
