package stereoholes

import (
	"math"
	"sort"
)

// Ray is an origin and a normalized direction.
type Ray struct {
	Origin Vec3
	Dir    Vec3
}

// Material describes a simple procedural surface.
type Material struct {
	Albedo   Vec3
	Emissive Vec3
	Specular float64
	Shine    float64
	Alpha    float64
	Checker  bool
	Category string
	Stable   bool
}

// Quad is a finite rectangle. U and V are half-edge vectors.
type Quad struct {
	ID         int
	Center     Vec3
	U, V       Vec3
	Normal     Vec3
	DoubleSide bool
	Material   Material
}

func NewQuad(id int, center, u, v Vec3, mat Material, doubleSide bool) *Quad {
	return &Quad{
		ID: id, Center: center, U: u, V: v,
		Normal:     u.Cross(v).Normalize(),
		DoubleSide: doubleSide, Material: mat,
	}
}

func (q *Quad) intersect(r Ray) (surfaceHit, bool) {
	den := r.Dir.Dot(q.Normal)
	if math.Abs(den) < 1e-9 {
		return surfaceHit{}, false
	}
	if !q.DoubleSide && den >= 0 {
		return surfaceHit{}, false
	}
	t := q.Center.Sub(r.Origin).Dot(q.Normal) / den
	if t <= 1e-6 {
		return surfaceHit{}, false
	}
	p := r.Origin.Add(r.Dir.Mul(t))
	d := p.Sub(q.Center)
	u := d.Dot(q.U) / q.U.Dot(q.U)
	v := d.Dot(q.V) / q.V.Dot(q.V)
	if math.Abs(u) > 1 || math.Abs(v) > 1 {
		return surfaceHit{}, false
	}
	n := q.Normal
	if q.DoubleSide && n.Dot(r.Dir) > 0 {
		n = n.Mul(-1)
	}
	return surfaceHit{t: t, p: p, n: n, quad: q}, true
}

type surfaceHit struct {
	t    float64
	p    Vec3
	n    Vec3
	quad *Quad
}

// Scene uses a fixed key light so the result is deterministic.
type Scene struct {
	Quads []*Quad
	Light Vec3 // direction from surface toward the light
}

func NewDemoScene() *Scene {
	s := &Scene{Light: Vec3{0.55, 0.72, -0.42}.Normalize()}
	id := 1

	wall := Material{
		Albedo: Vec3{0.55, 0.59, 0.65}, Alpha: 1,
		Checker: true, Category: "backdrop", Stable: true,
	}
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{0, 0, 5.2}, Vec3{3.2, 0, 0}, Vec3{0, 1.7, 0}, wall, true))
	id++

	corner := Material{
		Albedo: Vec3{0.78, 0.37, 0.28}, Specular: 0.12, Shine: 36,
		Alpha: 1, Category: "corner", Stable: true,
	}
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{-1.72, 0, 4.15}, Vec3{0, 0, 1.05}, Vec3{0, 1.2, 0}, corner, true))
	id++
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{-0.55, 0.05, 3.35}, Vec3{0.52, 0, 0}, Vec3{0, 1.22, 0},
			Material{
				Albedo: Vec3{0.30, 0.55, 0.48}, Specular: 0.22, Shine: 48,
				Alpha: 1, Category: "corner-return", Stable: true,
			}, true))
	id++

	// This return face lies between the two eye centers and is single-sided.
	// Only the left eye sees its front: a genuine one-eye wall corner sliver.
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{0.02, 0, 3.50},
			Vec3{0.02, 0, 0.60}, Vec3{0, 1.10, 0},
			Material{
				Albedo: Vec3{0.72, 0.46, 0.20}, Specular: 0.10, Shine: 32,
				Alpha: 1, Category: "wall-corner-one-eye", Stable: true,
			}, false))
	id++

	// A vertical silver fin is front-facing only to the target (left) eye.
	// It produces a narrow, view-dependent highlight at the corner.
	finMat := Material{
		Albedo: Vec3{0.95, 0.77, 0.32}, Emissive: Vec3{0.42, 0.30, 0.07},
		Specular: 2.8, Shine: 110, Alpha: 1,
		Category: "specular-edge", Stable: false,
	}
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{0.34, 0, 3.05},
			Vec3{0.05, 0, 0.50}, Vec3{0, 0.98, 0}, finMat, false))
	id++

	// The film is genuinely transparent, so its mixed color and depth are
	// ambiguous even when both eyes have a hit.
	filmMat := Material{
		Albedo: Vec3{0.16, 0.72, 0.70}, Specular: 0.65, Shine: 64,
		Alpha: 0.42, Category: "transparent-sheet", Stable: false,
	}
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{1.22, 0, 3.35},
			Vec3{0.62, 0, 0.24}, Vec3{0, 0.92, 0}, filmMat, true))
	id++

	// Opaque markers give the reprojection guard a stable surface to accept.
	markMat := Material{
		Albedo: Vec3{0.20, 0.33, 0.62}, Specular: 0.08, Shine: 28,
		Alpha: 1, Checker: true, Category: "common", Stable: true,
	}
	s.Quads = append(s.Quads,
		NewQuad(id, Vec3{1.10, -0.66, 4.86}, Vec3{0.74, 0, 0}, Vec3{0, 0.38, 0},
			markMat, true))
	id++

	return s
}

func (s *Scene) intersections(r Ray) []surfaceHit {
	hits := make([]surfaceHit, 0, len(s.Quads))
	for _, q := range s.Quads {
		if h, ok := q.intersect(r); ok {
			hits = append(hits, h)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].t < hits[j].t })
	return hits
}

func checker(u, v float64) float64 {
	iu := int(math.Floor(u*5)) & 1
	iv := int(math.Floor(v*5)) & 1
	if iu == iv {
		return 1.0
	}
	return 0.72
}

func (s *Scene) shade(h surfaceHit, r Ray) Vec3 {
	m := h.quad.Material
	base := m.Albedo
	if m.Checker {
		d := h.p.Sub(h.quad.Center)
		u := d.Dot(h.quad.U) / h.quad.U.Dot(h.quad.U)
		v := d.Dot(h.quad.V) / h.quad.V.Dot(h.quad.V)
		base = base.Mul(checker(u, v))
	}
	ndl := math.Max(0, h.n.Dot(s.Light))
	view := r.Dir.Mul(-1)
	half := s.Light.Add(view).Normalize()
	spec := math.Pow(math.Max(0, h.n.Dot(half)), m.Shine) * m.Specular
	ambient := 0.30
	c := base.Mul(ambient + 0.76*ndl)
	c = c.Add(Vec3{spec, spec, spec}).Add(m.Emissive)
	return c.Clamp01()
}

func background(r Ray) Vec3 {
	t := clamp(r.Dir.Y*0.5+0.5, 0, 1)
	return mix(Vec3{0.035, 0.055, 0.085}, Vec3{0.13, 0.16, 0.22}, t)
}
