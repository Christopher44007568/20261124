package relight

import "math"

type Surface struct {
	ID         int
	Name       string
	Center     Vec3
	Normal     Vec3
	Right      Vec3
	Up         Vec3
	HalfWidth  float64
	HalfHeight float64
	Albedo     RGB
	Mirror     bool
}

type Hit struct {
	Surface *Surface
	Point   Vec3
	Normal  Vec3
	Dist    float64
}

func (s Surface) Intersect(origin, direction Vec3) (Hit, bool) {
	denominator := Dot(direction, s.Normal)
	if math.Abs(denominator) < 1e-10 {
		return Hit{}, false
	}
	distance := Dot(Sub(s.Center, origin), s.Normal) / denominator
	if distance <= 1e-6 {
		return Hit{}, false
	}
	point := Add(origin, Scale(direction, distance))
	offset := Sub(point, s.Center)
	u := Dot(offset, s.Right)
	v := Dot(offset, s.Up)
	if math.Abs(u) > s.HalfWidth || math.Abs(v) > s.HalfHeight {
		return Hit{}, false
	}
	return Hit{
		Surface: &s,
		Point:   point,
		Normal:  s.Normal,
		Dist:    distance,
	}, true
}

type Scene struct {
	Surfaces []Surface
}

func (s Scene) Trace(origin, direction Vec3, maxDistance float64) (Hit, bool) {
	var closest Hit
	found := false
	bestDistance := maxDistance
	for i := range s.Surfaces {
		hit, ok := s.Surfaces[i].Intersect(origin, direction)
		if ok && hit.Dist < bestDistance {
			closest = hit
			bestDistance = hit.Dist
			found = true
		}
	}
	return closest, found
}

func (s Scene) Visible(from, to Vec3) bool {
	direction := Normalize(Sub(to, from))
	distance := Length(Sub(to, from))
	_, hit := s.Trace(Add(from, Scale(direction, 1e-4)), direction, distance-2e-4)
	return !hit
}
