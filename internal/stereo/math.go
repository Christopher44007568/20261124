package stereo

import (
	"image/color"
	"math"
)

// Vec3 uses millimetres for world-space coordinates. The camera looks down +Z,
// +X points right, and +Y points up.
type Vec3 struct {
	X, Y, Z float64
}

func (a Vec3) Add(b Vec3) Vec3 {
	return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z}
}

func (a Vec3) Sub(b Vec3) Vec3 {
	return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z}
}

func (a Vec3) Mul(s float64) Vec3 {
	return Vec3{a.X * s, a.Y * s, a.Z * s}
}

func (a Vec3) Dot(b Vec3) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func (a Vec3) Length() float64 {
	return math.Sqrt(a.Dot(a))
}

func (a Vec3) Normalize() Vec3 {
	n := a.Length()
	if n == 0 {
		return Vec3{}
	}
	return a.Mul(1 / n)
}

type RGB struct {
	R, G, B float64
}

func (c RGB) Scale(s float64) RGB {
	return RGB{c.R * s, c.G * s, c.B * s}
}

func (c RGB) Add(d RGB) RGB {
	return RGB{c.R + d.R, c.G + d.G, c.B + d.B}
}

func (c RGB) Mul(d RGB) RGB {
	return RGB{c.R * d.R, c.G * d.G, c.B * d.B}
}

func (c RGB) Clamp() color.RGBA {
	return color.RGBA{
		R: uint8(math.Round(clamp01(c.R) * 255)),
		G: uint8(math.Round(clamp01(c.G) * 255)),
		B: uint8(math.Round(clamp01(c.B) * 255)),
		A: 255,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func mix(a, b float64, t float64) float64 {
	return a + (b-a)*t
}

func smoothstep(t float64) float64 {
	t = clamp01(t)
	return t * t * (3 - 2*t)
}

func hash64(v uint64) uint64 {
	v ^= v >> 30
	v *= 0xbf58476d1ce4e5b9
	v ^= v >> 27
	v *= 0x94d049bb133111eb
	v ^= v >> 31
	return v
}

func latticeNoise(ix, iy, iz int64, seed uint64) float64 {
	h := hash64(uint64(ix)*0x9e3779b185ebca87 ^
		uint64(iy)*0xc2b2ae3d27d4eb4f ^
		uint64(iz)*0x165667b19e3779f9 ^ seed)
	return float64(h>>11) / float64(uint64(1)<<53)
}

// valueNoise is a deterministic, dependency-free 3D value noise. Texture
// values depend on the hit point in world space, so corresponding rays from
// the two eyes receive the same colour at the same physical surface point.
func valueNoise(p Vec3, scale float64, seed uint64) float64 {
	x := p.X / scale
	y := p.Y / scale
	z := p.Z / scale
	x0, y0, z0 := int64(math.Floor(x)), int64(math.Floor(y)), int64(math.Floor(z))
	fx, fy, fz := smoothstep(x-float64(x0)), smoothstep(y-float64(y0)), smoothstep(z-float64(z0))

	c000 := latticeNoise(x0, y0, z0, seed)
	c100 := latticeNoise(x0+1, y0, z0, seed)
	c010 := latticeNoise(x0, y0+1, z0, seed)
	c110 := latticeNoise(x0+1, y0+1, z0, seed)
	c001 := latticeNoise(x0, y0, z0+1, seed)
	c101 := latticeNoise(x0+1, y0, z0+1, seed)
	c011 := latticeNoise(x0, y0+1, z0+1, seed)
	c111 := latticeNoise(x0+1, y0+1, z0+1, seed)

	c00 := mix(c000, c100, fx)
	c10 := mix(c010, c110, fx)
	c01 := mix(c001, c101, fx)
	c11 := mix(c011, c111, fx)
	c0 := mix(c00, c10, fy)
	c1 := mix(c01, c11, fy)
	return mix(c0, c1, fz)
}
