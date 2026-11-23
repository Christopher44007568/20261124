package geom

import "math"

// Vec3 is a Euclidean vector used for scene points and directions.
type Vec3 struct {
	X, Y, Z float64
}

func Add(a, b Vec3) Vec3           { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func Sub(a, b Vec3) Vec3           { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func Scale(a Vec3, s float64) Vec3 { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func Dot(a, b Vec3) float64        { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func Cross(a, b Vec3) Vec3 {
	return Vec3{
		a.Y*b.Z - a.Z*b.Y,
		a.Z*b.X - a.X*b.Z,
		a.X*b.Y - a.Y*b.X,
	}
}
func Norm(a Vec3) float64   { return math.Sqrt(Dot(a, a)) }
func Normalize(a Vec3) Vec3 { return Scale(a, 1/Norm(a)) }
func Wrap01(x float64) float64 {
	x = math.Mod(x, 1)
	if x < 0 {
		x++
	}
	return x
}

// Camera is a pinhole camera looking down +Z with eyes offset along X.
type Camera struct {
	Eye      Vec3
	Focal    float64 // focal length in pixel units
	WidthPx  int
	HeightPx int
}

// RayFor returns the view ray through pixel (px, py); py grows downward.
func (c Camera) RayFor(px, py int) (origin, dir Vec3) {
	ndcX := (float64(px) - (float64(c.WidthPx)-1)/2) / c.Focal
	ndcY := ((float64(c.HeightPx)-1)/2 - float64(py)) / c.Focal
	return c.Eye, Normalize(Vec3{ndcX, ndcY, 1})
}

// EyeX shifts the eye along the camera X axis by the given baseline offset.
func (c Camera) EyeX(offset float64) Camera {
	c2 := c
	c2.Eye = Add(c.Eye, Vec3{X: offset})
	return c2
}
