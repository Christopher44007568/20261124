package scan

import (
	"math"

	"posebench/internal/geom"
)

// Camera is a pinhole camera. Focal lengths and principal point are in pixels.
type Camera struct {
	Width, Height int
	FX, FY        float64
	CX, CY        float64
}

type Projection struct {
	X, Y, Z float64
	Valid   bool
}

func DefaultCamera() Camera {
	return Camera{
		Width: 640, Height: 480,
		FX: 1000, FY: 1000,
		CX: 320, CY: 240,
	}
}

func (c Camera) Project(p geom.Pose, local geom.Vec3) Projection {
	v := p.Transform(local)
	if v.Z <= 1 {
		return Projection{Z: v.Z}
	}
	return Projection{
		X:     c.CX + c.FX*v.X/v.Z,
		Y:     c.CY + c.FY*v.Y/v.Z,
		Z:     v.Z,
		Valid: true,
	}
}

// FaceLandmarks is a compact 3D face model in head coordinates.
type Landmark struct {
	Name string
	P    geom.Vec3
}

func FaceLandmarks() []Landmark {
	return []Landmark{
		{Name: "brow-left", P: geom.Vec3{X: -38, Y: -45, Z: 12}},
		{Name: "brow-right", P: geom.Vec3{X: 38, Y: -45, Z: 12}},
		{Name: "eye-left", P: geom.Vec3{X: -32, Y: -20, Z: 18}},
		{Name: "eye-right", P: geom.Vec3{X: 32, Y: -20, Z: 18}},
		{Name: "nose-bridge", P: geom.Vec3{X: 0, Y: -5, Z: 38}},
		{Name: "nose-tip", P: geom.Vec3{X: 0, Y: 16, Z: 58}},
		{Name: "cheek-left", P: geom.Vec3{X: -55, Y: 20, Z: 0}},
		{Name: "cheek-right", P: geom.Vec3{X: 55, Y: 20, Z: 0}},
		{Name: "mouth-left", P: geom.Vec3{X: -25, Y: 46, Z: 22}},
		{Name: "mouth-right", P: geom.Vec3{X: 25, Y: 46, Z: 22}},
		{Name: "mouth-center", P: geom.Vec3{X: 0, Y: 50, Z: 30}},
		{Name: "jaw-left", P: geom.Vec3{X: -46, Y: 66, Z: 2}},
		{Name: "jaw-right", P: geom.Vec3{X: 46, Y: 66, Z: 2}},
		{Name: "chin", P: geom.Vec3{X: 0, Y: 86, Z: 12}},
	}
}

func FaceCenter() geom.Vec3 {
	return geom.Vec3{X: 0, Y: 20, Z: 24}
}

type Point2 struct {
	X, Y float64
}

type Affine struct {
	A, B, C, D float64
	TX, TY     float64
}

// FitAffine returns the least-squares affine map from src to dst.
func FitAffine(src, dst []Point2) (Affine, bool) {
	if len(src) != len(dst) || len(src) < 3 {
		return Affine{}, false
	}
	var mx, my, mu, mv float64
	for i := range src {
		mx += src[i].X
		my += src[i].Y
		mu += dst[i].X
		mv += dst[i].Y
	}
	n := float64(len(src))
	mx, my, mu, mv = mx/n, my/n, mu/n, mv/n

	var sxx, sxy, syy, sxu, syu, sxv, syv float64
	for i := range src {
		x := src[i].X - mx
		y := src[i].Y - my
		u := dst[i].X - mu
		v := dst[i].Y - mv
		sxx += x * x
		sxy += x * y
		syy += y * y
		sxu += x * u
		syu += y * u
		sxv += x * v
		syv += y * v
	}
	det := sxx*syy - sxy*sxy
	if math.Abs(det) < 1e-9 {
		return Affine{}, false
	}
	a := (sxu*syy - syu*sxy) / det
	b := (syu*sxx - sxu*sxy) / det
	c := (sxv*syy - syv*sxy) / det
	d := (syv*sxx - sxv*sxy) / det
	return Affine{
		A:  a,
		B:  b,
		C:  c,
		D:  d,
		TX: mu - a*mx - b*my,
		TY: mv - c*mx - d*my,
	}, true
}
