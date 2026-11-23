package stereo

import "math"

// ClipVertex carries both world and view positions so clipping can be
// measured in world space and projected from the same generated vertex.
type ClipVertex struct {
	World Vec3
	View  Vec3
	Near  bool
}

// ClipTriangleNear clips one world-space triangle against a camera's near
// plane. The result is a convex polygon with zero to four vertices.
func ClipTriangleNear(cam Camera, tri [3]Vec3) []ClipVertex {
	input := make([]ClipVertex, 3)
	for i, p := range tri {
		v := cam.ToView(p)
		input[i] = ClipVertex{
			World: p,
			View:  v,
			Near:  math.Abs(v.Z-cam.Near) <= 1e-9,
		}
	}

	out := make([]ClipVertex, 0, 4)
	prev := input[2]
	prevInside := prev.View.Z >= cam.Near
	for _, cur := range input {
		curInside := cur.View.Z >= cam.Near
		switch {
		case !prevInside && curInside:
			out = appendClipVertex(out, intersectNear(cam, prev, cur))
			out = appendClipVertex(out, cur)
		case prevInside && curInside:
			out = appendClipVertex(out, cur)
		case prevInside && !curInside:
			out = appendClipVertex(out, intersectNear(cam, prev, cur))
		}
		prev, prevInside = cur, curInside
	}

	if len(out) < 3 {
		return nil
	}
	return out
}

func intersectNear(cam Camera, a, b ClipVertex) ClipVertex {
	denom := b.View.Z - a.View.Z
	t := 0.0
	if !nearZero(denom) {
		t = (cam.Near - a.View.Z) / denom
	}
	t = math.Max(0, math.Min(1, t))
	return ClipVertex{
		World: a.World.Lerp(b.World, t),
		View:  a.View.Lerp(b.View, t),
		Near:  true,
	}
}

func appendClipVertex(dst []ClipVertex, v ClipVertex) []ClipVertex {
	if len(dst) > 0 {
		last := dst[len(dst)-1]
		if v.World.Sub(last.World).Len() < 1e-9 && v.View.Sub(last.View).Len() < 1e-9 {
			return dst
		}
	}
	return append(dst, v)
}

// ScreenVertex keeps the near-plane marker while clipping against the
// viewport. Border intersections inherit Near only when both parents lie on
// the same near-plane edge.
type ScreenVertex struct {
	P    Vec2
	Near bool
}

// ProjectPolygon converts a near-clipped polygon to NDC and clips it to
// [-1,1]^2. All inputs must have view z > 0, which ClipTriangleNear ensures.
func ProjectPolygon(cam Camera, poly []ClipVertex) []ScreenVertex {
	ndc := make([]ScreenVertex, len(poly))
	for i, v := range poly {
		ndc[i] = ScreenVertex{P: cam.ProjectView(v.View), Near: v.Near}
	}
	return clipScreenRect(ndc)
}

func clipScreenRect(poly []ScreenVertex) []ScreenVertex {
	edges := []struct {
		axis  int
		value float64
		keep  func(float64) bool
	}{
		{0, -1, func(v float64) bool { return v >= -1 }},
		{0, 1, func(v float64) bool { return v <= 1 }},
		{1, -1, func(v float64) bool { return v >= -1 }},
		{1, 1, func(v float64) bool { return v <= 1 }},
	}
	for _, edge := range edges {
		if len(poly) == 0 {
			return nil
		}
		out := make([]ScreenVertex, 0, len(poly)+2)
		prev := poly[len(poly)-1]
		for _, cur := range poly {
			prevValue := valueOnAxis(prev.P, edge.axis)
			curValue := valueOnAxis(cur.P, edge.axis)
			prevInside := edge.keep(prevValue)
			curInside := edge.keep(curValue)
			switch {
			case !prevInside && curInside:
				out = append(out, screenIntersection(prev, cur, edge.axis, edge.value))
				out = append(out, cur)
			case prevInside && curInside:
				out = append(out, cur)
			case prevInside && !curInside:
				out = append(out, screenIntersection(prev, cur, edge.axis, edge.value))
			}
			prev = cur
		}
		poly = out
	}
	return poly
}

func valueOnAxis(p Vec2, axis int) float64 {
	if axis == 0 {
		return p.X
	}
	return p.Y
}

func screenIntersection(a, b ScreenVertex, axis int, value float64) ScreenVertex {
	av := valueOnAxis(a.P, axis)
	bv := valueOnAxis(b.P, axis)
	denom := bv - av
	t := 0.0
	if !nearZero(denom) {
		t = (value - av) / denom
	}
	t = math.Max(0, math.Min(1, t))
	return ScreenVertex{
		P:    a.P.Add(b.P.Sub(a.P).Mul(t)),
		Near: a.Near && b.Near,
	}
}

// PixelPolygon maps a screen-clipped NDC polygon to viewport pixels.
func PixelPolygon(cam Camera, poly []ScreenVertex) []Vec2 {
	out := make([]Vec2, len(poly))
	for i, v := range poly {
		out[i] = cam.NDCToPixel(v.P)
	}
	return out
}

// NearEdges returns viewport-space segments of the near-plane boundary.
func NearEdges(cam Camera, poly []ScreenVertex) [][2]Vec2 {
	var out [][2]Vec2
	for i := range poly {
		j := (i + 1) % len(poly)
		if !poly[i].Near || !poly[j].Near {
			continue
		}
		a := cam.NDCToPixel(poly[i].P)
		b := cam.NDCToPixel(poly[j].P)
		if a.Dist(b) > 1e-7 {
			out = append(out, [2]Vec2{a, b})
		}
	}
	return out
}

// Area2D is the absolute shoelace area of a polygon.
func Area2D(poly []Vec2) float64 {
	if len(poly) < 3 {
		return 0
	}
	sum := 0.0
	for i := range poly {
		j := (i + 1) % len(poly)
		sum += poly[i].Cross(poly[j])
	}
	return math.Abs(sum) * 0.5
}

// Area3D triangulates a convex polygon with a fan and sums 3D triangle areas.
func Area3D(poly []Vec3) float64 {
	if len(poly) < 3 {
		return 0
	}
	sum := 0.0
	for i := 1; i+1 < len(poly); i++ {
		a := poly[i].Sub(poly[0])
		b := poly[i+1].Sub(poly[0])
		sum += a.Cross(b).Len() * 0.5
	}
	return sum
}
