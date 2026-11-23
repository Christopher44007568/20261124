package calib

// MarkerSet groups the three probe families used by the tests:
// a crosshair at the view axis, boundary markers near the panel
// edges, and points at known depths along one ray.
type MarkerSet struct {
	Crosshair []Vec3
	Boundary  []Vec3
	Depth     []Vec3
}

// All flattens the marker families into one slice.
func (m MarkerSet) All() []Vec3 {
	out := make([]Vec3, 0, len(m.Crosshair)+len(m.Boundary)+len(m.Depth))
	out = append(out, m.Crosshair...)
	out = append(out, m.Boundary...)
	out = append(out, m.Depth...)
	return out
}

// DefaultMarkers builds probes for the rig. Boundary markers sit at
// 90% of the half-extents of the frustum at 2 m; depth probes share
// one direction at 1, 2 and 4 m so viewpoint (parallax) errors show
// up as a 1/z-dependent pixel shift.
func (r Rig) DefaultMarkers() MarkerSet {
	const z = 2.0
	mx := 0.9 * (r.PanelW / 2) / r.FocalPx * z
	my := 0.9 * (r.PanelH / 2) / r.FocalPx * z
	boundary := []Vec3{
		{-mx, -my, z}, {mx, -my, z}, {mx, my, z}, {-mx, my, z},
		{0, -my, z}, {mx, 0, z}, {0, my, z}, {-mx, 0, z},
	}
	depth := []Vec3{
		{0.3, -0.1, 1.0},
		{0.6, -0.2, 2.0},
		{1.2, -0.4, 4.0},
	}
	return MarkerSet{
		Crosshair: []Vec3{{0, 0, z}},
		Boundary:  boundary,
		Depth:     depth,
	}
}
