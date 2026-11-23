package scene

import (
	"image/color"
	"math"

	"timewarp/geom"
)

var lightDir = geom.Vec3{X: 0.35, Y: 0.85, Z: -0.62}.Normalize()
var fillDir = geom.Vec3{X: -0.45, Y: 0.25, Z: 0.55}.Normalize()

func shade(p, n geom.Vec3, region Region, cam geom.Vec3) ([3]float64, float64) {
	var base [3]float64
	switch region {
	case RegionPole:
		base = poleColor(p, n, cam)
	default:
		base = roomColor(p, n)
	}

	diffuse := math.Max(0, n.Dot(lightDir))
	fill := math.Max(0, n.Dot(fillDir))
	light := 0.50 + 0.46*diffuse + 0.10*fill
	base[0] *= light
	base[1] *= light
	base[2] *= light

	glintWeight := 0.0
	for _, g := range Glints {
		d2 := p.Sub(g.P).Dot(p.Sub(g.P))
		w := g.Gain * math.Exp(-0.5*d2/(g.Sigma*g.Sigma))
		if w < 0.01 {
			continue
		}
		base[0] += g.Color[0] * w
		base[1] += g.Color[1] * w
		base[2] += g.Color[2] * w
		glintWeight = math.Max(glintWeight, w)
	}
	return base, glintWeight
}

func roomColor(p, n geom.Vec3) [3]float64 {
	switch {
	case n.Z < -0.5:
		return backWallColor(p)
	case n.Y > 0.5:
		return floorColor(p)
	case n.Y < -0.5:
		return ceilingColor(p)
	default:
		return sideWallColor(p)
	}
}

func backWallColor(p geom.Vec3) [3]float64 {
	tx := (p.X + 3.2) / 0.52
	ty := (p.Y + 2.2) / 0.52
	ix, iy := math.Floor(tx), math.Floor(ty)
	odd := (int(ix) + int(iy)) & 1
	var c [3]float64
	if odd == 0 {
		c = [3]float64{0.24, 0.30, 0.37}
	} else {
		c = [3]float64{0.18, 0.25, 0.32}
	}
	if math.Abs(fract(tx)-0.5) > 0.485 || math.Abs(fract(ty)-0.5) > 0.485 {
		c = [3]float64{0.07, 0.09, 0.12}
	}

	// World-anchored ring and axis marks make lateral parallax obvious.
	r := math.Hypot(p.X, p.Y-0.10)
	if math.Abs(r-1.32) < 0.022 {
		c = mix(c, [3]float64{0.92, 0.58, 0.24}, 0.82)
	}
	if math.Abs(p.Y-0.10) < 0.012 && math.Abs(p.X) < 1.65 {
		c = mix(c, [3]float64{0.34, 0.70, 0.82}, 0.75)
	}
	if math.Abs(p.X) < 0.010 && math.Abs(p.Y-0.10) < 1.65 {
		c = mix(c, [3]float64{0.34, 0.70, 0.82}, 0.75)
	}
	return c
}

func sideWallColor(p geom.Vec3) [3]float64 {
	stripe := fract((p.Z + 2.0) / 0.58)
	band := int(math.Floor((p.Y + 2.2) / 0.55))
	var c [3]float64
	if band&1 == 0 {
		c = [3]float64{0.21, 0.27, 0.31}
	} else {
		c = [3]float64{0.17, 0.23, 0.28}
	}
	if stripe < 0.05 || math.Abs(fract((p.Y+2.2)/0.55)-0.5) > 0.485 {
		c = [3]float64{0.065, 0.085, 0.10}
	}
	if stripe > 0.47 && stripe < 0.53 {
		c[0] += 0.025
		c[1] += 0.035
		c[2] += 0.045
	}
	return c
}

func floorColor(p geom.Vec3) [3]float64 {
	ix := int(math.Floor((p.X + 4.0) / 0.55))
	iz := int(math.Floor((p.Z + 3.0) / 0.55))
	var c [3]float64
	if (ix+iz)&1 == 0 {
		c = [3]float64{0.13, 0.20, 0.23}
	} else {
		c = [3]float64{0.19, 0.27, 0.30}
	}
	fx := fract((p.X + 4.0) / 0.55)
	fz := fract((p.Z + 3.0) / 0.55)
	if fx < 0.035 || fz < 0.035 {
		c = [3]float64{0.055, 0.075, 0.09}
	}
	if math.Abs(math.Hypot(p.X, p.Z-3.4)-0.82) < 0.018 {
		c = mix(c, [3]float64{0.75, 0.44, 0.19}, 0.70)
	}
	return c
}

func ceilingColor(p geom.Vec3) [3]float64 {
	c := [3]float64{0.15, 0.17, 0.20}
	if math.Abs(fract((p.X+3.2)/0.8)-0.5) > 0.48 ||
		math.Abs(fract((p.Z+2.0)/0.8)-0.5) > 0.48 {
		c = [3]float64{0.07, 0.08, 0.10}
	}
	return c
}

func poleColor(p, n, cam geom.Vec3) [3]float64 {
	band := int(math.Floor((p.Y + 2.2) / 0.42))
	var c [3]float64
	switch {
	case band%5 == 0:
		c = [3]float64{0.94, 0.44, 0.22}
	case band%2 == 0:
		c = [3]float64{0.55, 0.72, 0.80}
	default:
		c = [3]float64{0.30, 0.47, 0.58}
	}
	view := cam.Sub(p).Normalize()
	half := lightDir.Add(view).Normalize()
	spec := math.Pow(math.Max(0, n.Dot(half)), 46)
	for i := range c {
		c[i] += 0.55 * spec
	}
	if math.Abs(p.Y-0.62) < 0.035 {
		c = [3]float64{0.98, 0.82, 0.30}
	}
	return c
}

func fract(v float64) float64 {
	return v - math.Floor(v)
}

func mix(a, b [3]float64, t float64) [3]float64 {
	return [3]float64{
		a[0] + (b[0]-a[0])*t,
		a[1] + (b[1]-a[1])*t,
		a[2] + (b[2]-a[2])*t,
	}
}

func toRGBA(r, g, b float64) color.RGBA {
	return color.RGBA{
		R: toByte(r), G: toByte(g), B: toByte(b), A: 255,
	}
}

func toByte(v float64) uint8 {
	v = math.Min(1, math.Max(0, v))
	v = math.Pow(v, 1/2.2)
	return uint8(math.Round(v * 255))
}
