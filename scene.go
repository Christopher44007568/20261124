package main

// OccluderKind classifies how an occluder participates in the two-layer query.
type OccluderKind int

const (
	// Wall is thick: valid for both eyes, goes to the shared occluder set.
	Wall OccluderKind = iota
	// Pillar is thin: usually covers only one eye's view, per-eye complement.
	Pillar
	// Sheet is a transparent panel: per-eye complement, partial coverage.
	Sheet
)

func (k OccluderKind) String() string {
	switch k {
	case Wall:
		return "wall"
	case Pillar:
		return "pillar"
	case Sheet:
		return "sheet"
	}
	return "unknown"
}

type Occluder struct {
	ID       int
	Kind     OccluderKind
	Min, Max Vec3
}

func (o Occluder) Center() Vec3 { return Mul(Add(o.Min, o.Max), 0.5) }

// Thickness is the smallest box extent; walls are thick in every axis.
func (o Occluder) Thickness() float64 {
	d := Sub(o.Max, o.Min)
	t := d.X
	if d.Y < t {
		t = d.Y
	}
	if d.Z < t {
		t = d.Z
	}
	return t
}

type Object struct {
	ID     int
	Center Vec3
	Radius float64
	Color  [3]float64
}

type Scene struct {
	Objects   []Object
	Occluders []Occluder
}

// SharedSet returns occluders valid for both eyes (thick walls).
func (s *Scene) SharedSet() []Occluder {
	var out []Occluder
	for _, o := range s.Occluders {
		if o.Kind == Wall {
			out = append(out, o)
		}
	}
	return out
}

// ComplementSet returns occluders that may only cover a single eye.
func (s *Scene) ComplementSet() []Occluder {
	var out []Occluder
	for _, o := range s.Occluders {
		if o.Kind != Wall {
			out = append(out, o)
		}
	}
	return out
}

// buildScene creates a deterministic demo scene: a street of objects behind
// thick walls, thin pillars and transparent sheets.
func buildScene() *Scene {
	s := &Scene{}
	// Thick walls: shared occluders.
	s.Occluders = append(s.Occluders,
		Occluder{ID: 0, Kind: Wall, Min: Vec3{-3.0, -1.5, 2.0}, Max: Vec3{-0.5, 1.5, 2.6}},
		Occluder{ID: 1, Kind: Wall, Min: Vec3{1.0, -1.5, 3.0}, Max: Vec3{3.5, 1.5, 3.7}},
	)
	// Thin pillars: typically only wide enough to cover one eye's frustum slice.
	s.Occluders = append(s.Occluders,
		Occluder{ID: 2, Kind: Pillar, Min: Vec3{-0.15, -1.5, 1.5}, Max: Vec3{0.15, 1.5, 1.8}},
		Occluder{ID: 3, Kind: Pillar, Min: Vec3{0.9, -1.5, 2.2}, Max: Vec3{1.2, 1.5, 2.5}},
	)
	// Transparent sheets: large but only partially opaque, per-eye coverage.
	s.Occluders = append(s.Occluders,
		Occluder{ID: 4, Kind: Sheet, Min: Vec3{-1.8, -1.0, 4.0}, Max: Vec3{0.6, 1.0, 4.1}},
	)
	// Objects scattered behind the occluders.
	positions := []Vec3{
		{-2.0, 0.3, 6.0}, {-1.2, -0.4, 7.0}, {-0.3, 0.6, 5.5},
		{0.4, -0.2, 6.5}, {1.1, 0.5, 5.8}, {1.9, -0.5, 7.2},
		{2.6, 0.2, 6.2}, {-2.6, -0.6, 8.0}, {0.8, 2.0, 8.5},
		{-0.9, 1.9, 9.0}, {3.2, 0.4, 7.6}, {-1.7, 2.2, 6.4},
	}
	for i, p := range positions {
		s.Objects = append(s.Objects, Object{
			ID:     i,
			Center: p,
			Radius: 0.35,
			Color: [3]float64{
				0.4 + 0.6*float64((i*37)%10)/10,
				0.4 + 0.6*float64((i*53)%10)/10,
				0.4 + 0.6*float64((i*91)%10)/10,
			},
		})
	}
	return s
}
