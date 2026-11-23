package hzb

import "math"

// Vec3 is a point/vector in world or view space.
type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3      { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3      { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64   { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}
func (a Vec3) Len() float64 { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Norm() Vec3 {
	l := a.Len()
	if l == 0 {
		return a
	}
	return a.Scale(1 / l)
}

// AABB is an axis-aligned box used as the culling test proxy for an object.
type AABB struct{ Min, Max Vec3 }

// Corners returns the 8 box corners in world space.
func (b AABB) Corners() [8]Vec3 {
	return [8]Vec3{
		{b.Min.X, b.Min.Y, b.Min.Z}, {b.Max.X, b.Min.Y, b.Min.Z},
		{b.Min.X, b.Max.Y, b.Min.Z}, {b.Max.X, b.Max.Y, b.Min.Z},
		{b.Min.X, b.Min.Y, b.Max.Z}, {b.Max.X, b.Min.Y, b.Max.Z},
		{b.Min.X, b.Max.Y, b.Max.Z}, {b.Max.X, b.Max.Y, b.Max.Z},
	}
}

// Camera is a pinhole perspective camera. View space: +Z forward, +X right, +Y up.
type Camera struct {
	ID       string
	Position Vec3
	Forward  Vec3
	Up       Vec3
	FovYDeg  float64
	Near     float64
	Far      float64
	Width    int
	Height   int
}

func (c *Camera) basis() (right, up, fwd Vec3) {
	fwd = c.Forward.Norm()
	up = c.Up
	if up.Len() == 0 {
		up = Vec3{0, 1, 0}
	}
	right = up.Cross(fwd).Norm()
	up = fwd.Cross(right).Norm()
	return right, up, fwd
}

// ToView converts a world point to view space (Z forward, larger = farther).
func (c *Camera) ToView(p Vec3) Vec3 {
	right, up, fwd := c.basis()
	d := p.Sub(c.Position)
	return Vec3{d.Dot(right), d.Dot(up), d.Dot(fwd)}
}

// Project maps a world point to pixel coordinates. ok is false when the point
// lies outside the [Near, Far] range.
func (c *Camera) Project(p Vec3) (sx, sy, z float64, ok bool) {
	v := c.ToView(p)
	if v.Z < c.Near || v.Z > c.Far {
		return 0, 0, v.Z, false
	}
	sx, sy = c.projectView(v)
	return sx, sy, v.Z, true
}

func (c *Camera) projectView(v Vec3) (sx, sy float64) {
	f := 1 / math.Tan(c.FovYDeg*math.Pi/360)
	aspect := float64(c.Width) / float64(c.Height)
	ndcX := v.X * f / aspect / v.Z
	ndcY := v.Y * f / v.Z
	sx = (ndcX*0.5 + 0.5) * float64(c.Width)
	sy = (1 - (ndcY*0.5 + 0.5)) * float64(c.Height)
	return sx, sy
}
