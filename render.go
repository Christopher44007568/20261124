package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func sceneColor(x, y float64) color.RGBA {
	r := uint8(30 + 40*math.Sin(x*0.01))
	g := uint8(40 + 30*math.Cos(y*0.02))
	b := uint8(70)
	if (int(x/32)+int(y/32))%2 == 0 {
		r += 20
		g += 20
		b += 10
	}
	type circle struct {
		cx, cy, rad float64
		col         color.RGBA
	}
	for _, c := range []circle{
		{160, 120, 40, color.RGBA{220, 80, 60, 255}},
		{480, 240, 55, color.RGBA{60, 160, 220, 255}},
		{320, 90, 30, color.RGBA{240, 200, 70, 255}},
		{520, 90, 25, color.RGBA{120, 220, 120, 255}},
		{140, 280, 35, color.RGBA{200, 120, 220, 255}},
	} {
		if math.Hypot(x-c.cx, y-c.cy) < c.rad {
			return c.col
		}
	}
	return color.RGBA{r, g, b, 255}
}

func RenderFrame(cfg SimConfig, f Frame, d Decision) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, cfg.Width, cfg.Height))
	periph := 4
	switch {
	case d.Rate >= RateHigh:
		periph = 2
	case d.Rate >= RateBase:
		periph = 4
	default:
		periph = 8
	}
	const foveaR = 90.0
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			sx, sy := x, y
			if math.Hypot(float64(x)-d.CenterX, float64(y)-d.CenterY) > foveaR {
				sx = x / periph * periph
				sy = y / periph * periph
			}
			img.SetRGBA(x, y, sceneColor(float64(sx), float64(sy)))
		}
	}
	white := color.RGBA{255, 255, 255, 255}
	red := color.RGBA{255, 60, 60, 255}
	for a := 0.0; a < 2*math.Pi; a += 0.002 {
		px := int(d.CenterX + foveaR*math.Cos(a))
		py := int(d.CenterY + foveaR*math.Sin(a))
		if px >= 0 && px < cfg.Width && py >= 0 && py < cfg.Height {
			img.SetRGBA(px, py, white)
		}
	}
	for k := -8; k <= 8; k++ {
		gx, gy := int(f.GazeX), int(f.GazeY)
		if gx+k >= 0 && gx+k < cfg.Width && gy >= 0 && gy < cfg.Height {
			img.SetRGBA(gx+k, gy, red)
		}
		if gx >= 0 && gx < cfg.Width && gy+k >= 0 && gy+k < cfg.Height {
			img.SetRGBA(gx, gy+k, red)
		}
	}
	return img
}

func SavePNG(path string, img *image.RGBA) {
	fh, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer fh.Close()
	if err := png.Encode(fh, img); err != nil {
		panic(err)
	}
}
