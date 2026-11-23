package geom

import "math"

// Vec3 is a three-dimensional vector in millimetres unless stated otherwise.
type Vec3 struct {
	X, Y, Z float64
}

func (v Vec3) Add(o Vec3) Vec3 {
	return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z}
}

func (v Vec3) Sub(o Vec3) Vec3 {
	return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z}
}

func (v Vec3) Scale(s float64) Vec3 {
	return Vec3{v.X * s, v.Y * s, v.Z * s}
}

func (v Vec3) Dot(o Vec3) float64 {
	return v.X*o.X + v.Y*o.Y + v.Z*o.Z
}

func (v Vec3) Len() float64 {
	return math.Sqrt(v.Dot(v))
}

func LerpVec3(a, b Vec3, u float64) Vec3 {
	return Vec3{
		X: a.X + (b.X-a.X)*u,
		Y: a.Y + (b.Y-a.Y)*u,
		Z: a.Z + (b.Z-a.Z)*u,
	}
}
