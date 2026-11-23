package stereo

import "math"

const (
	WallZ       = 1500.0
	RulerMinX   = -520.0
	RulerMaxX   = 520.0
	RulerTopY   = -100.0
	RulerBotY   = -230.0
	SphereCX    = -170.0
	SphereCY    = 10.0
	SphereCZ    = 990.0
	SphereR     = 115.0
	FrontPlateZ = 758.0
	BackPlateZ  = 1048.0
)

var (
	FrontPlateMin = Vec3{X: -125, Y: -95, Z: FrontPlateZ}
	FrontPlateMax = Vec3{X: 105, Y: 85, Z: FrontPlateZ + 4}
	BackPlateMin  = Vec3{X: -30, Y: -60, Z: BackPlateZ}
	BackPlateMax  = Vec3{X: 250, Y: 120, Z: BackPlateZ + 4}
	SphereCenter  = Vec3{X: SphereCX, Y: SphereCY, Z: SphereCZ}
)

type SurfaceID int

const (
	SurfaceWall SurfaceID = iota
	SurfaceRuler
	SurfaceSphere
	SurfaceFrontPlate
	SurfaceBackPlate
)

func (s SurfaceID) Name() string {
	switch s {
	case SurfaceRuler:
		return "ruler"
	case SurfaceSphere:
		return "sphere"
	case SurfaceFrontPlate:
		return "front_plate"
	case SurfaceBackPlate:
		return "back_plate"
	default:
		return "wall"
	}
}

type Hit struct {
	T       float64
	Point   Vec3
	Normal  Vec3
	Surface SurfaceID
}

func IntersectScene(ray Ray) (Hit, bool) {
	best := Hit{T: math.Inf(1)}
	found := false
	consider := func(h Hit, ok bool) {
		if ok && h.T > 1e-6 && h.T < best.T {
			best = h
			found = true
		}
	}

	if t, ok := intersectWall(ray); ok {
		p := ray.Origin.Add(ray.Direction.Mul(t))
		surface := SurfaceWall
		if p.X >= RulerMinX && p.X <= RulerMaxX && p.Y >= RulerBotY && p.Y <= RulerTopY {
			surface = SurfaceRuler
		}
		consider(Hit{T: t, Point: p, Normal: Vec3{Z: -1}, Surface: surface}, true)
	}
	consider(intersectSphere(ray))
	consider(intersectBox(ray, FrontPlateMin, FrontPlateMax, SurfaceFrontPlate))
	consider(intersectBox(ray, BackPlateMin, BackPlateMax, SurfaceBackPlate))
	return best, found
}

func intersectWall(ray Ray) (float64, bool) {
	if math.Abs(ray.Direction.Z) < 1e-9 {
		return 0, false
	}
	t := (WallZ - ray.Origin.Z) / ray.Direction.Z
	return t, t > 1e-6
}

func intersectSphere(ray Ray) (Hit, bool) {
	oc := ray.Origin.Sub(SphereCenter)
	a := ray.Direction.Dot(ray.Direction)
	b := 2 * oc.Dot(ray.Direction)
	c := oc.Dot(oc) - SphereR*SphereR
	disc := b*b - 4*a*c
	if disc < 0 {
		return Hit{}, false
	}
	root := math.Sqrt(disc)
	t := (-b - root) / (2 * a)
	if t <= 1e-6 {
		t = (-b + root) / (2 * a)
	}
	if t <= 1e-6 {
		return Hit{}, false
	}
	p := ray.Origin.Add(ray.Direction.Mul(t))
	n := p.Sub(SphereCenter).Mul(1 / SphereR).Normalize()
	return Hit{T: t, Point: p, Normal: n, Surface: SurfaceSphere}, true
}

func intersectBox(ray Ray, min, max Vec3, id SurfaceID) (Hit, bool) {
	tmin := math.Inf(-1)
	tmax := math.Inf(1)
	var normal Vec3

	axis := func(origin, direction, lo, hi float64, axisIndex int) bool {
		if math.Abs(direction) < 1e-12 {
			return origin >= lo && origin <= hi
		}
		t1 := (lo - origin) / direction
		t2 := (hi - origin) / direction
		sign := -1.0
		if t1 > t2 {
			t1, t2 = t2, t1
			sign = 1
		}
		if t1 > tmin {
			tmin = t1
			normal = Vec3{}
			switch axisIndex {
			case 0:
				normal.X = sign
			case 1:
				normal.Y = sign
			default:
				normal.Z = sign
			}
		}
		if t2 < tmax {
			tmax = t2
		}
		return tmin <= tmax
	}

	if !axis(ray.Origin.X, ray.Direction.X, min.X, max.X, 0) ||
		!axis(ray.Origin.Y, ray.Direction.Y, min.Y, max.Y, 1) ||
		!axis(ray.Origin.Z, ray.Direction.Z, min.Z, max.Z, 2) {
		return Hit{}, false
	}
	if tmax < 0 || tmin <= 1e-6 {
		return Hit{}, false
	}
	p := ray.Origin.Add(ray.Direction.Mul(tmin))
	return Hit{T: tmin, Point: p, Normal: normal, Surface: id}, true
}
