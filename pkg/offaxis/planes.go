package offaxis

// Plane indices and the corresponding bit flags. The same bit layout is
// reused by the rasterizer's clip-space edge markers so the two outputs can
// be compared bit for bit.
const (
	PlaneLeft = iota
	PlaneRight
	PlaneBottom
	PlaneTop
	PlaneNear
	PlaneFar
)

// Plane is a half-space. A point p is on the interior side when
// Normal.Dot(p) + D >= 0.
type Plane struct {
	Normal Vec3
	D      float64
}

// SignedDistance returns Normal.Dot(p) + D.
func (p Plane) SignedDistance(v Vec3) float64 {
	return p.Normal.Dot(v) + p.D
}

// AnalyticPlanes builds the six inward-facing planes directly from the
// frustum bounds. All lateral planes pass through the apex at the origin.
func (b Bounds) AnalyticPlanes() ([]Plane, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	l, r := b.Left, b.Right
	bot, top := b.Bottom, b.Top
	n, f := b.Near, b.Far
	return []Plane{
		{Normal: Vec3{n, 0, l}, D: 0},     // left:  n*x + l*z >= 0
		{Normal: Vec3{-n, 0, -r}, D: 0},   // right: n*(r-x) >= 0
		{Normal: Vec3{0, n, bot}, D: 0},   // bottom: n*y + bot*z >= 0
		{Normal: Vec3{0, -n, -top}, D: 0}, // top:    -n*y - top*z >= 0
		{Normal: Vec3{0, 0, -1}, D: -n},   // near: -z - n >= 0
		{Normal: Vec3{0, 0, 1}, D: f},     // far:  z + f >= 0
	}, nil
}

// ExtractPlanes recovers the six clip planes from a projection matrix using
// the Gribb-Hartmann row combination. It is an independent derivation that
// cross-checks AnalyticPlanes. Plane order matches the Plane* constants.
func (m Mat4) ExtractPlanes() []Plane {
	row := func(i int) Vec4 {
		return Vec4{X: m[i][0], Y: m[i][1], Z: m[i][2], W: m[i][3]}
	}
	r0, r1, r2, r3 := row(0), row(1), row(2), row(3)
	combine := func(a, b Vec4) Plane {
		p := Plane{Normal: Vec3{X: a.X + b.X, Y: a.Y + b.Y, Z: a.Z + b.Z}, D: a.W + b.W}
		norm := p.Normal.Length()
		if norm != 0 {
			p.Normal = p.Normal.Scale(1 / norm)
			p.D /= norm
		}
		return p
	}
	return []Plane{
		combine(r3, r0), // left:   w + x >= 0
		combine(r3, r0.ScaleV(-1)),
		combine(r3, r1),            // bottom: w + y >= 0
		combine(r3, r1.ScaleV(-1)), // top:    w - y >= 0
		combine(r3, r2),            // near:   w + z >= 0
		combine(r3, r2.ScaleV(-1)),
	}
}

// ScaleV scales every component of the homogeneous row, including W.
func (v Vec4) ScaleV(s float64) Vec4 {
	return Vec4{X: v.X * s, Y: v.Y * s, Z: v.Z * s, W: v.W * s}
}

// Classify evaluates a point against the six planes. violated is a bitmask
// of planes whose interior inequality fails (Plane* bit flags); boundary is
// the subset of planes the point lies on within eps.
func Classify(planes []Plane, p Vec3, eps float64) (violated uint8, boundary uint8) {
	for i, plane := range planes {
		d := plane.SignedDistance(p)
		if d < -eps {
			violated |= 1 << uint(i)
		} else if d < eps {
			boundary |= 1 << uint(i)
		}
	}
	return violated, boundary
}

// Inside reports whether the point is within the closed frustum.
func Inside(planes []Plane, p Vec3, eps float64) bool {
	v, _ := Classify(planes, p, eps)
	return v == 0
}
