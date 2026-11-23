package scene

import (
	"math"

	"timewarp/geom"
)

const planeEpsilon = 1e-6

// intersect returns the nearest room or pole hit among the six inward-facing
// room planes and the vertical near pole.
func intersect(origin, dir geom.Vec3) (t float64, normal geom.Vec3, region Region, ok bool) {
	t = math.Inf(1)
	if best, n, hit := intersectRoom(origin, dir); hit && best < t {
		t = best
		normal = n
		region = RegionRoom
		ok = true
	}
	if best, n, hit := intersectPole(origin, dir); hit && best < t {
		t = best
		normal = n
		region = RegionPole
		ok = true
	}
	return t, normal, region, ok
}

func intersectRoom(o, d geom.Vec3) (float64, geom.Vec3, bool) {
	type plane struct {
		axis   int
		value  float64
		normal geom.Vec3
	}
	planes := [...]plane{
		{0, roomMinX, geom.Vec3{X: 1}},
		{0, roomMaxX, geom.Vec3{X: -1}},
		{1, roomMinY, geom.Vec3{Y: 1}},
		{1, roomMaxY, geom.Vec3{Y: -1}},
		{2, roomMinZ, geom.Vec3{Z: 1}},
		{2, roomMaxZ, geom.Vec3{Z: -1}},
	}
	best := math.Inf(1)
	var bestN geom.Vec3
	for _, p := range planes {
		den := d.At(p.axis)
		if math.Abs(den) < 1e-12 {
			continue
		}
		t := (p.value - o.At(p.axis)) / den
		if t <= planeEpsilon || t >= best {
			continue
		}
		hit := o.Add(d.Mul(t))
		if hit.X < roomMinX-1e-7 || hit.X > roomMaxX+1e-7 ||
			hit.Y < roomMinY-1e-7 || hit.Y > roomMaxY+1e-7 ||
			hit.Z < roomMinZ-1e-7 || hit.Z > roomMaxZ+1e-7 {
			continue
		}
		best = t
		bestN = p.normal
	}
	return best, bestN, !math.IsInf(best, 1)
}

func intersectPole(o, d geom.Vec3) (float64, geom.Vec3, bool) {
	ox, oz := o.X-poleX, o.Z-poleZ
	a := d.X*d.X + d.Z*d.Z
	if a < 1e-12 {
		return 0, geom.Vec3{}, false
	}
	b := 2 * (ox*d.X + oz*d.Z)
	c := ox*ox + oz*oz - poleR*poleR
	disc := b*b - 4*a*c
	if disc < 0 {
		return 0, geom.Vec3{}, false
	}
	sq := math.Sqrt(disc)
	roots := [2]float64{(-b - sq) / (2 * a), (-b + sq) / (2 * a)}
	for _, t := range roots {
		if t <= planeEpsilon {
			continue
		}
		y := o.Y + d.Y*t
		if y < poleY0 || y > poleY1 {
			continue
		}
		hit := o.Add(d.Mul(t))
		n := geom.Vec3{X: (hit.X - poleX) / poleR, Z: (hit.Z - poleZ) / poleR}
		return t, n.Normalize(), true
	}
	return 0, geom.Vec3{}, false
}

func onMissingDepthPatch(hit, normal geom.Vec3) bool {
	if normal.Z > -0.5 {
		return false
	}
	return hit.X >= 1.28 && hit.X <= 2.42 && hit.Y >= 0.28 && hit.Y <= 1.18
}
