package calib

import "math"

// Vec2 is a 2D point or offset in panel pixel coordinates.
type Vec2 struct {
	X, Y float64
}

func (a Vec2) Add(b Vec2) Vec2 { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Sub(b Vec2) Vec2 { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Scale(s float64) Vec2 {
	return Vec2{a.X * s, a.Y * s}
}
func (a Vec2) Len() float64 { return math.Hypot(a.X, a.Y) }

// Vec3 is a 3D point in eye space, meters, z forward.
type Vec3 struct {
	X, Y, Z float64
}

func (a Vec3) Sub(b Vec3) Vec3 { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }

// Rot2 rotates v by angle radians counterclockwise.
func Rot2(v Vec2, angle float64) Vec2 {
	c, s := math.Cos(angle), math.Sin(angle)
	return Vec2{c*v.X - s*v.Y, s*v.X + c*v.Y}
}

// RotateAbout rotates p around pivot by angle radians.
func RotateAbout(p, pivot Vec2, angle float64) Vec2 {
	return pivot.Add(Rot2(p.Sub(pivot), angle))
}
