package reprojlab

import "math"

// Intrinsics use pixel-center coordinates.
type Intrinsics struct {
	FX float64
	FY float64
	CX float64
	CY float64
}

// Distortion describes an OpenCV-style radial-tangential model. It maps an
// undistorted pinhole coordinate to the coordinate seen through the lens.
type Distortion struct {
	K1 float64
	K2 float64
	K3 float64
	P1 float64
	P2 float64
}

type Point struct {
	X float64
	Y float64
}

// Translation is the source-to-target relative camera translation in world
// units. A negative Z moves the target camera toward the scene.
type Translation struct {
	X float64
	Y float64
	Z float64
}

func (d Distortion) Apply(nx, ny float64) Point {
	r2 := nx*nx + ny*ny
	r4 := r2 * r2
	r6 := r4 * r2
	radial := 1 + d.K1*r2 + d.K2*r4 + d.K3*r6
	xy := 2 * nx * ny
	tangentX := d.P1*xy + d.P2*(r2+2*nx*nx)
	tangentY := d.P1*(r2+2*ny*ny) + d.P2*xy
	return Point{
		X: nx*radial + tangentX,
		Y: ny*radial + tangentY,
	}
}

func (d Distortion) Scale(factor float64) Distortion {
	return Distortion{
		K1: d.K1 * factor,
		K2: d.K2 * factor,
		K3: d.K3 * factor,
		P1: d.P1 * factor,
		P2: d.P2 * factor,
	}
}

func DistortPixel(p Point, k Intrinsics, d Distortion) Point {
	nx := (p.X - k.CX) / k.FX
	ny := (p.Y - k.CY) / k.FY
	q := d.Apply(nx, ny)
	return Point{X: k.FX*q.X + k.CX, Y: k.FY*q.Y + k.CY}
}

// Reproject maps a pixel and its camera-space depth into the target camera.
// The returned Z is the target-camera depth, not the source depth.
func Reproject(p Point, z float64, k Intrinsics, t Translation) (Point, float64, bool) {
	xn := (p.X - k.CX) / k.FX
	yn := (p.Y - k.CY) / k.FY
	x := xn * z
	y := yn * z
	zt := z + t.Z
	if zt <= 1e-9 || math.IsNaN(zt) {
		return Point{}, 0, false
	}
	xt := x + t.X
	yt := y + t.Y
	return Point{
		X: k.FX*xt/zt + k.CX,
		Y: k.FY*yt/zt + k.CY,
	}, zt, true
}

func (k Intrinsics) Pixel(nx, ny, z float64) (Point, float64) {
	return Point{X: k.FX*nx + k.CX, Y: k.FY*ny + k.CY}, z
}

func (k Intrinsics) Ray(p Point, z float64) (float64, float64) {
	return (p.X - k.CX) / k.FX, (p.Y - k.CY) / k.FY
}
