package stereoholes

import "math"

// Camera is a pinhole camera. Depth is measured along Forward, in world units.
type Camera struct {
	Pos     Vec3
	Right   Vec3
	Up      Vec3
	Forward Vec3
	Focal   float64
	Width   int
	Height  int
	CX, CY  float64
}

// NewCamera builds a camera looking at target with a vertical field of view.
func NewCamera(pos, target Vec3, width, height int, vfovDeg float64) Camera {
	forward := target.Sub(pos).Normalize()
	right := Vec3{0, 1, 0}.Cross(forward).Normalize()
	up := forward.Cross(right).Normalize()
	focal := (float64(height) * 0.5) / math.Tan(vfovDeg*math.Pi/360)
	return Camera{
		Pos: pos, Right: right, Up: up, Forward: forward,
		Focal: focal, Width: width, Height: height,
		CX: float64(width) * 0.5, CY: float64(height) * 0.5,
	}
}

// Ray returns a normalized ray through the center of pixel (x, y).
func (c Camera) Ray(x, y int) Ray {
	sx := float64(x) + 0.5 - c.CX
	sy := c.CY - (float64(y) + 0.5)
	d := c.Forward.Add(c.Right.Mul(sx / c.Focal)).Add(c.Up.Mul(sy / c.Focal))
	return Ray{Origin: c.Pos, Dir: d.Normalize()}
}

// Project maps a world point to a target pixel and camera-space depth.
func (c Camera) Project(p Vec3) (x, y float64, depth float64, ok bool) {
	d := p.Sub(c.Pos)
	depth = d.Dot(c.Forward)
	if depth <= 1e-6 {
		return 0, 0, 0, false
	}
	x = c.CX + d.Dot(c.Right)/depth*c.Focal
	y = c.CY - d.Dot(c.Up)/depth*c.Focal
	return x, y, depth, true
}

// Unproject reconstructs a world point from pixel coordinates and z-depth.
func (c Camera) Unproject(x, y float64, depth float64) Vec3 {
	sx := x - c.CX
	sy := c.CY - y
	return c.Pos.
		Add(c.Right.Mul(sx / c.Focal * depth)).
		Add(c.Up.Mul(sy / c.Focal * depth)).
		Add(c.Forward.Mul(depth))
}

func (c Camera) Contains(x, y int) bool {
	return x >= 0 && y >= 0 && x < c.Width && y < c.Height
}

// StereoRig is a parallel stereo pair. Eyes are separated along world X.
type StereoRig struct {
	Left, Right Camera
	Baseline    float64
}

func NewStereoRig(width, height int, baseline, vfovDeg float64) StereoRig {
	base := NewCamera(Vec3{0, 0, 0}, Vec3{0, 0, 4}, width, height, vfovDeg)
	left, right := base, base
	left.Pos = Vec3{-baseline / 2, 0, 0}
	right.Pos = Vec3{baseline / 2, 0, 0}
	return StereoRig{
		Left:     left,
		Right:    right,
		Baseline: baseline,
	}
}
