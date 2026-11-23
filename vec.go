package reprojection

import "math"

// Vec2 is a screen-space point. X grows right and Y grows down.
type Vec2 struct {
	X float64
	Y float64
}

func (a Vec2) Add(b Vec2) Vec2      { return Vec2{a.X + b.X, a.Y + b.Y} }
func (a Vec2) Sub(b Vec2) Vec2      { return Vec2{a.X - b.X, a.Y - b.Y} }
func (a Vec2) Scale(s float64) Vec2 { return Vec2{a.X * s, a.Y * s} }
func (a Vec2) Dot(b Vec2) float64   { return a.X*b.X + a.Y*b.Y }
func (a Vec2) Len() float64         { return math.Hypot(a.X, a.Y) }
func (a Vec2) Distance(b Vec2) float64 {
	return a.Sub(b).Len()
}

// Vec3 is a world-space point. X grows right, Y grows up and Z grows forward.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Normalize() Vec3 {
	n := a.Len()
	if n == 0 {
		return Vec3{}
	}
	return a.Scale(1 / n)
}

// Pose is a head/camera transform. Yaw is positive to the right.
type Pose struct {
	Position Vec3
	Yaw      float64
	Pitch    float64
}

func rotateVec(v Vec3, yaw, pitch float64) Vec3 {
	sy, cy := math.Sincos(yaw)
	sp, cp := math.Sincos(pitch)

	y1 := v.Y*cp - v.Z*sp
	z1 := v.Y*sp + v.Z*cp
	return Vec3{
		X: v.X*cy + z1*sy,
		Y: y1,
		Z: -v.X*sy + z1*cy,
	}
}

func unrotateVec(v Vec3, yaw, pitch float64) Vec3 {
	sy, cy := math.Sincos(yaw)
	sp, cp := math.Sincos(pitch)

	x1 := v.X*cy - v.Z*sy
	z1 := v.X*sy + v.Z*cy
	return Vec3{
		X: x1,
		Y: v.Y*cp + z1*sp,
		Z: -v.Y*sp + z1*cp,
	}
}
