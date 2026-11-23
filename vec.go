package main

import "math"

type Vec3 struct {
	X, Y, Z float64
}

func Add(a, b Vec3) Vec3         { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func Sub(a, b Vec3) Vec3         { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func Mul(a Vec3, s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func Dot(a, b Vec3) float64      { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func Cross(a, b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func Len(a Vec3) float64 { return math.Sqrt(Dot(a, a)) }

func Norm(a Vec3) Vec3 {
	l := Len(a)
	if l < 1e-12 {
		return Vec3{}
	}
	return Mul(a, 1/l)
}

type Ray struct {
	Orig, Dir Vec3
}

// raySphere returns the nearest positive hit distance, or -1.
func raySphere(r Ray, center Vec3, radius float64) float64 {
	oc := Sub(r.Orig, center)
	b := Dot(oc, r.Dir)
	c := Dot(oc, oc) - radius*radius
	disc := b*b - c
	if disc < 0 {
		return -1
	}
	sq := math.Sqrt(disc)
	t := -b - sq
	if t > 1e-6 {
		return t
	}
	t = -b + sq
	if t > 1e-6 {
		return t
	}
	return -1
}

// rayBox returns the nearest positive hit distance against an AABB, or -1.
func rayBox(r Ray, min, max Vec3) float64 {
	tmin, tmax := -math.MaxFloat64, math.MaxFloat64
	ro := [3]float64{r.Orig.X, r.Orig.Y, r.Orig.Z}
	rd := [3]float64{r.Dir.X, r.Dir.Y, r.Dir.Z}
	mn := [3]float64{min.X, min.Y, min.Z}
	mx := [3]float64{max.X, max.Y, max.Z}
	for i := 0; i < 3; i++ {
		if math.Abs(rd[i]) < 1e-12 {
			if ro[i] < mn[i] || ro[i] > mx[i] {
				return -1
			}
			continue
		}
		inv := 1 / rd[i]
		t0 := (mn[i] - ro[i]) * inv
		t1 := (mx[i] - ro[i]) * inv
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		if t0 > tmin {
			tmin = t0
		}
		if t1 < tmax {
			tmax = t1
		}
		if tmin > tmax {
			return -1
		}
	}
	if tmax < 1e-6 {
		return -1
	}
	if tmin > 1e-6 {
		return tmin
	}
	return tmax
}
