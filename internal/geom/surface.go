package geom

import "math"

const (
	KindPlane = iota
	KindCylinder
	KindSphere
)

// Surface describes one layer. All layers are centered on the optical axis Z.
type Surface struct {
	ID     int
	Kind   int
	Center Vec3
	// Radius is used by the cylinder and the equirectangular sphere.
	Radius float64
	// HalfW/HalfH are the plane half extents in world units.
	HalfW float64
	HalfH float64
}

// Hit is the nearest intersection of a ray with a layer.
type Hit struct {
	T       float64 // distance along the normalized view ray
	U, V    float64 // parameter-space coordinates, U in [0,1), V in [0,1]
	Surface *Surface
}

const epsilon = 1e-9

// Intersect solves the ray/surface equation analytically and returns the
// inverse-mapped parameter coordinates. Nothing here projects a curved
// surface through a planar four-corner bilinear warp.
func (s *Surface) Intersect(origin, dir Vec3) (Hit, bool) {
	switch s.Kind {
	case KindPlane:
		return s.intersectPlane(origin, dir)
	case KindCylinder:
		return s.intersectCylinder(origin, dir)
	default:
		return s.intersectSphere(origin, dir)
	}
}

func (s *Surface) intersectPlane(origin, dir Vec3) (Hit, bool) {
	if math.Abs(dir.Z) < epsilon {
		return Hit{}, false
	}
	t := (s.Center.Z - origin.Z) / dir.Z
	if t <= epsilon {
		return Hit{}, false
	}
	p := Add(origin, Scale(dir, t))
	dx := p.X - s.Center.X
	dy := p.Y - s.Center.Y
	if math.Abs(dx) > s.HalfW || math.Abs(dy) > s.HalfH {
		return Hit{}, false
	}
	u := (dx + s.HalfW) / (2 * s.HalfW)
	v := (dy + s.HalfH) / (2 * s.HalfH)
	return Hit{T: t, U: u, V: v, Surface: s}, true
}

func (s *Surface) intersectCylinder(origin, dir Vec3) (Hit, bool) {
	// Infinite cylinder x^2 + z^2 = r^2 around the vertical Y axis, clipped in y.
	a := dir.X*dir.X + dir.Z*dir.Z
	if a < epsilon {
		return Hit{}, false
	}
	ox := origin.X - s.Center.X
	oz := origin.Z - s.Center.Z
	b := 2 * (ox*dir.X + oz*dir.Z)
	c := ox*ox + oz*oz - s.Radius*s.Radius
	disc := b*b - 4*a*c
	if disc < 0 {
		return Hit{}, false
	}
	sqrt := math.Sqrt(disc)
	tFar := (-b + sqrt) / (2 * a)
	// The visible shell is always the positive far root: when the eye is
	// outside the cross-section the near root is the hidden back interior
	// wall, and when inside it the near root is behind the eye.
	t := tFar
	if t <= epsilon {
		return Hit{}, false
	}
	p := Add(origin, Scale(dir, t))
	y := p.Y - s.Center.Y
	if math.Abs(y) > s.HalfH {
		return Hit{}, false
	}
	// U=0/1 seam lies on the +Z meridian facing the camera, so a marker
	// straddling the seam is split across the screen-center wrap correctly.
	theta := math.Atan2(p.X-s.Center.X, p.Z-s.Center.Z)
	u := Wrap01(theta / (2 * math.Pi))
	v := (y + s.HalfH) / (2 * s.HalfH)
	return Hit{T: t, U: u, V: v, Surface: s}, true
}

func (s *Surface) intersectSphere(origin, dir Vec3) (Hit, bool) {
	oc := Sub(origin, s.Center)
	a := Dot(dir, dir)
	b := 2 * Dot(oc, dir)
	c := Dot(oc, oc) - s.Radius*s.Radius
	disc := b*b - 4*a*c
	if disc < 0 {
		return Hit{}, false
	}
	sqrt := math.Sqrt(disc)
	t := (-b - sqrt) / (2 * a)
	if t <= epsilon {
		t = (-b + sqrt) / (2 * a)
		if t <= epsilon {
			return Hit{}, false
		}
	}
	p := Sub(Add(origin, Scale(dir, t)), s.Center)
	// Equirectangular mapping with poles on the vertical Y axis: longitude
	// wraps around Y, latitude clamps at the two poles.
	// Zero longitude faces the camera (+Z); the U=0/1 seam therefore sits on
	// the back meridian and wrap sampling hides it in front.
	lon := math.Atan2(p.X, p.Z)
	lat := math.Asin(clamp(p.Y/s.Radius, -1, 1))
	u := Wrap01(lon / (2 * math.Pi))
	v := lat/math.Pi + 0.5
	return Hit{T: t, U: u, V: v, Surface: s}, true
}

func clamp(x, lo, hi float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}
