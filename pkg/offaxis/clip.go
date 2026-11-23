package offaxis

// ClipVertex is a vertex traveling through the clipping and rasterization
// pipeline. EdgeMask records every clip plane that rejected a vertex.
type ClipVertex struct {
	Pos      Vec4 // clip space
	EdgeMask uint8
}

// clipPlane describes one homogeneous half-space: sign*Pos.Axis + Pos.W >= 0.
type clipPlane struct {
	axis int
	sign float64
	bit  uint8
}

// sidePlanes are the four lateral half-spaces. They are only evaluated
// after the near-plane pass, because a vertex behind the camera has w < 0
// and would trivially fail every homogeneous inequality at once.
var sidePlanes = []clipPlane{
	{axis: 0, sign: 1, bit: 1 << PlaneLeft},   // w + x >= 0
	{axis: 0, sign: -1, bit: 1 << PlaneRight}, // w - x >= 0
	{axis: 1, sign: 1, bit: 1 << PlaneBottom}, // w + y >= 0 fails below the bottom bound
	{axis: 1, sign: -1, bit: 1 << PlaneTop},   // w - y >= 0 fails above the top bound
}

var (
	nearPlane = clipPlane{axis: 2, sign: 1, bit: 1 << PlaneNear} // w + z >= 0
	farPlane  = clipPlane{axis: 2, sign: -1, bit: 1 << PlaneFar} // w - z >= 0
)

// behindGuard is the w > 0 half-space used only to reject geometry that is
// entirely behind the camera. Clipping against w >= 0 itself would place
// intersection vertices on the camera plane (w = 0) with infinite NDC, so
// the real near-plane pass (w + z >= 0) performs the actual slicing.
var behindGuard = clipPlane{axis: 3, sign: 1, bit: 1 << PlaneNear}

func clipValue(v Vec4, p clipPlane) float64 {
	if p.axis == 3 {
		return v.W
	}
	axis := []float64{v.X, v.Y, v.Z}[p.axis]
	return v.W + p.sign*axis
}

// MarkEdges fills EdgeMask for a single vertex from its clip coordinates.
func MarkEdges(v Vec4) uint8 {
	var mask uint8
	// Tolerance matches the analytic Classify epsilon so points that land
	// on a plane through floating point round-trip are not flagged.
	const edgeEps = 1e-7
	for _, p := range allClipPlanes() {
		if clipValue(v, p) < -edgeEps {
			mask |= p.bit
		}
	}
	return mask
}

func allClipPlanes() []clipPlane {
	all := make([]clipPlane, 0, 7)
	all = append(all, behindGuard, nearPlane)
	all = append(all, sidePlanes...)
	all = append(all, farPlane)
	return all
}

func interpolate(a, b ClipVertex, t float64) ClipVertex {
	return ClipVertex{
		Pos: Vec4{
			X: a.Pos.X + (b.Pos.X-a.Pos.X)*t,
			Y: a.Pos.Y + (b.Pos.Y-a.Pos.Y)*t,
			Z: a.Pos.Z + (b.Pos.Z-a.Pos.Z)*t,
			W: a.Pos.W + (b.Pos.W-a.Pos.W)*t,
		},
		EdgeMask: 0, // intersection points sit on a plane, not outside it
	}
}

// ClipPolygon runs Sutherland-Hodgman against all six homogeneous clip
// planes. The returned slice is empty when the whole polygon is outside.
// Every surviving vertex carries the edge mask for the planes it violates;
// inside vertices have mask zero.
func ClipPolygon(verts []ClipVertex) []ClipVertex {
	out := make([]ClipVertex, len(verts))
	copy(out, verts)
	for i := range out {
		out[i].EdgeMask = MarkEdges(out[i].Pos)
	}
	// Reject geometry that is wholly at or behind the camera plane first.
	if allBehind(out) {
		return nil
	}
	// Slice against the true near plane, then the four lateral planes and
	// finally the far plane. Every surviving vertex has strictly positive w.
	out = clipAgainst(out, nearPlane)
	for _, plane := range sidePlanes {
		if len(out) == 0 {
			return nil
		}
		out = clipAgainst(out, plane)
	}
	if len(out) > 0 {
		out = clipAgainst(out, farPlane)
	}
	if len(out) == 0 {
		return nil
	}
	for i := range out {
		out[i].EdgeMask = MarkEdges(out[i].Pos)
	}
	return out
}

func allBehind(verts []ClipVertex) bool {
	for _, v := range verts {
		if clipValue(v.Pos, behindGuard) > 0 {
			return false
		}
	}
	return true
}

func clipAgainst(in []ClipVertex, plane clipPlane) []ClipVertex {
	out := make([]ClipVertex, 0, len(in)+1)
	for i := range in {
		cur := in[i]
		prev := in[(i-1+len(in))%len(in)]
		curInside := clipValue(cur.Pos, plane) >= 0
		prevInside := clipValue(prev.Pos, plane) >= 0
		switch {
		case curInside && prevInside:
			out = append(out, cur)
		case curInside && !prevInside:
			t := intersect(prev, cur, plane)
			out = append(out, interpolate(prev, cur, t), cur)
		case !curInside && prevInside:
			t := intersect(prev, cur, plane)
			out = append(out, interpolate(prev, cur, t))
		}
	}
	return out
}

func intersect(a, b ClipVertex, p clipPlane) float64 {
	da := clipValue(a.Pos, p)
	db := clipValue(b.Pos, p)
	denom := da - db
	if denom == 0 {
		return 0
	}
	return da / denom
}
