// Package report writes the side-by-side PNG diagnostics and the standalone
// HTML report. It uses only image/png and the standard library.
package report

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"timewarp/scene"
	"timewarp/warp"
)

func FrameImage(f *scene.Frame) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	for i, c := range f.Pix {
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func WarpImage(out *warp.Result) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, out.W, out.H))
	for i, c := range out.Pix {
		var r, g, b uint8
		if out.Reject[i] == warp.RejectNone {
			r, g, b = c.R, c.G, c.B
		} else {
			r, g, b = 18, 22, 30
			if ((i/out.W)/6+(i%out.W)/6)%2 == 0 {
				r, g, b = 26, 31, 41
			}
		}
		img.Pix[i*4+0] = r
		img.Pix[i*4+1] = g
		img.Pix[i*4+2] = b
		img.Pix[i*4+3] = 255
	}
	return img
}

func MaskImage(out *warp.Result) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, out.W, out.H))
	for i, why := range out.Reject {
		c := color.RGBA{R: 11, G: 16, B: 24, A: 255}
		switch why {
		case warp.RejectOutOfBounds:
			c = color.RGBA{R: 237, G: 64, B: 52, A: 255}
		case warp.RejectNoCoverage:
			c = color.RGBA{R: 221, G: 64, B: 198, A: 255}
		case warp.RejectBehindCamera:
			c = color.RGBA{R: 255, G: 157, B: 28, A: 255}
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func HeatImage(gt *scene.Frame, out *warp.Result) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, out.W, out.H))
	for i := range gt.Pix {
		var c color.RGBA
		if out.Reject[i] != warp.RejectNone {
			c = color.RGBA{R: 25, G: 29, B: 38, A: 255}
			if ((i/out.W)/5+(i%out.W)/5)%2 == 0 {
				c = color.RGBA{R: 43, G: 48, B: 59, A: 255}
			}
		} else {
			e := absDiffMean(gt.Pix[i], out.Pix[i])
			c = heatColor(e)
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func RegionImage(f *scene.Frame) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	for i, reg := range f.Region {
		c := color.RGBA{R: 20, G: 25, B: 34, A: 255}
		switch reg {
		case scene.RegionPole:
			c = color.RGBA{R: 25, G: 205, B: 225, A: 255}
		case scene.RegionSpecular:
			c = color.RGBA{R: 255, G: 215, B: 54, A: 255}
		}
		if !f.DepthValid[i] {
			c = color.RGBA{R: 226, G: 47, B: 203, A: 255}
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func DepthImage(f *scene.Frame) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, f.W, f.H))
	for i := range f.Pix {
		var c color.RGBA
		if !f.DepthValid[i] {
			c = color.RGBA{R: 226, G: 47, B: 203, A: 255}
		} else {
			t := math.Max(0, math.Min(1, (f.Depth[i]-0.35)/(8.0-0.35)))
			c = depthColor(t)
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = 255
	}
	return img
}

func WritePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func absDiffMean(a, b color.RGBA) float64 {
	dr := math.Abs(float64(a.R) - float64(b.R))
	dg := math.Abs(float64(a.G) - float64(b.G))
	db := math.Abs(float64(a.B) - float64(b.B))
	return (dr + dg + db) / 3 / 255
}

func heatColor(e float64) color.RGBA {
	type stop struct {
		t float64
		c [3]float64
	}
	stops := [...]stop{
		{0.000, [3]float64{10, 13, 22}},
		{0.012, [3]float64{24, 72, 147}},
		{0.040, [3]float64{23, 180, 189}},
		{0.090, [3]float64{248, 211, 50}},
		{0.220, [3]float64{244, 76, 44}},
		{0.550, [3]float64{255, 246, 236}},
	}
	if e <= stops[0].t {
		return color.RGBA{uint8(stops[0].c[0]), uint8(stops[0].c[1]), uint8(stops[0].c[2]), 255}
	}
	for i := 1; i < len(stops); i++ {
		if e <= stops[i].t {
			t := (e - stops[i-1].t) / (stops[i].t - stops[i-1].t)
			return color.RGBA{
				R: uint8(stops[i-1].c[0] + (stops[i].c[0]-stops[i-1].c[0])*t),
				G: uint8(stops[i-1].c[1] + (stops[i].c[1]-stops[i-1].c[1])*t),
				B: uint8(stops[i-1].c[2] + (stops[i].c[2]-stops[i-1].c[2])*t),
				A: 255,
			}
		}
	}
	return color.RGBA{R: 255, G: 246, B: 236, A: 255}
}

func depthColor(t float64) color.RGBA {
	// Near is warm, far is deep blue; the midpoint adds a green shelf for
	// readable depth edges.
	type stop struct {
		t float64
		c [3]float64
	}
	stops := [...]stop{
		{0.00, [3]float64{250, 204, 92}},
		{0.25, [3]float64{244, 100, 64}},
		{0.50, [3]float64{95, 190, 130}},
		{0.75, [3]float64{34, 128, 180}},
		{1.00, [3]float64{18, 27, 70}},
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].t {
			u := (t - stops[i-1].t) / (stops[i].t - stops[i-1].t)
			return color.RGBA{
				R: uint8(stops[i-1].c[0] + (stops[i].c[0]-stops[i-1].c[0])*u),
				G: uint8(stops[i-1].c[1] + (stops[i].c[1]-stops[i-1].c[1])*u),
				B: uint8(stops[i-1].c[2] + (stops[i].c[2]-stops[i-1].c[2])*u),
				A: 255,
			}
		}
	}
	return color.RGBA{R: 18, G: 27, B: 70, A: 255}
}
