package render

import (
	"image"
	"image/color"
	"math"

	"curvelayers/internal/geom"
	"curvelayers/internal/tex"
)

const gapPx = 10

// StereoCanvas lays two eye buffers side by side with a dark separator.
func StereoCanvas(left, right *EyeBuffer) *image.RGBA {
	w := left.Width*2 + gapPx
	h := left.Height
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	blitEye(img, left, 0)
	blitEye(img, right, left.Width+gapPx)
	for y := 0; y < h; y++ {
		for g := 0; g < gapPx; g++ {
			img.SetRGBA(left.Width+g, y, color.RGBA{18, 18, 22, 255})
		}
	}
	return img
}

func blitEye(dst *image.RGBA, b *EyeBuffer, offsetX int) {
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			idx := y*b.Width + x
			dst.SetRGBA(offsetX+x, y, color.RGBA{
				R: toByte(b.RGB[idx*3]),
				G: toByte(b.RGB[idx*3+1]),
				B: toByte(b.RGB[idx*3+2]),
				A: 255,
			})
		}
	}
}

// OrderingErrorCanvas shows where fixed painter order disagrees with true
// nearest-surface order: red pixels are compositing errors, grayscale is fine.
func OrderingErrorCanvas(left, right *EyeBuffer) *image.RGBA {
	w := left.Width*2 + gapPx
	h := left.Height
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	blitError(img, left, 0)
	blitError(img, right, left.Width+gapPx)
	for y := 0; y < h; y++ {
		for g := 0; g < gapPx; g++ {
			img.SetRGBA(left.Width+g, y, color.RGBA{18, 18, 22, 255})
		}
	}
	return img
}

func blitError(dst *image.RGBA, b *EyeBuffer, offsetX int) {
	for y := 0; y < b.Height; y++ {
		for x := 0; x < b.Width; x++ {
			idx := y*b.Width + x
			gray := toByte(b.RGB[idx*3]*0.6 + b.RGB[idx*3+1]*0.3 + b.RGB[idx*3+2]*0.1)
			c := color.RGBA{R: gray, G: gray, B: gray, A: 255}
			if b.Owner[idx] != b.NaiveOwner[idx] && b.Owner[idx] >= 0 {
				c = color.RGBA{R: 255, G: 48, B: 48, A: 255}
			}
			dst.SetRGBA(offsetX+x, y, c)
		}
	}
}

// DisparityCanvas encodes horizontal binocular disparity d = xL - xR as a
// false-color ramp; near surfaces shift more than far ones.
func DisparityCanvas(disparity []float64, w, h int, maxDisp float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w+60, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			d := disparity[y*w+x]
			if math.IsInf(d, 0) {
				img.SetRGBA(x, y, color.RGBA{10, 10, 14, 255})
				continue
			}
			t := math.Max(0, math.Min(1, d/maxDisp))
			r, g, b := ramp(t)
			img.SetRGBA(x, y, color.RGBA{R: toByte(r), G: toByte(g), B: toByte(b), A: 255})
		}
	}
	// Vertical color bar legend mapping high disparity (top) to zero (bottom).
	for y := 0; y < h; y++ {
		t := 1 - float64(y)/float64(h-1)
		r, g, b := ramp(t)
		for x := w + 12; x < w+40; x++ {
			img.SetRGBA(x, y, color.RGBA{R: toByte(r), G: toByte(g), B: toByte(b), A: 255})
		}
	}
	return img
}

func ramp(t float64) (float64, float64, float64) {
	stops := [][3]float64{
		{0.08, 0.12, 0.45}, // blue: far / zero disparity
		{0.1, 0.65, 0.95},  // cyan
		{0.2, 0.85, 0.35},  // green
		{0.98, 0.85, 0.2},  // yellow
		{0.95, 0.2, 0.15},  // red: near / max disparity
	}
	pos := t * float64(len(stops)-1)
	i := int(math.Min(math.Floor(pos), float64(len(stops)-2)))
	f := pos - float64(i)
	a, c := stops[i], stops[i+1]
	return a[0] + (c[0]-a[0])*f, a[1] + (c[1]-a[1])*f, a[2] + (c[2]-a[2])*f
}

// Disparity estimates per-pixel horizontal disparity from the two depth
// buffers using the pinhole relation d = f * baseline / depth.
func Disparity(left *EyeBuffer, focal, baseline float64) ([]float64, float64) {
	n := left.Width * left.Height
	out := make([]float64, n)
	maxDisp := 0.0
	for i, z := range left.Depth {
		if math.IsInf(z, 0) {
			out[i] = math.Inf(1)
			continue
		}
		d := focal * baseline / z
		out[i] = d
		if d > maxDisp {
			maxDisp = d
		}
	}
	return out, maxDisp
}

// QuadWarpEye is the intentionally wrong reference: a curved surface sampled
// through a planar four-corner bilinear UV warp using its screen-space
// silhouette corners. Curvature is flattened and the seam is duplicated.
func QuadWarpEye(cam geom.Camera, layer Layer, marker *tex.Marker) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, cam.WidthPx, cam.HeightPx))
	for y := 0; y < cam.HeightPx; y++ {
		for x := 0; x < cam.WidthPx; x++ {
			img.SetRGBA(x, y, color.RGBA{10, 10, 14, 255})
		}
	}
	r := layer.Surface.Radius
	bounds := silhouetteBounds(cam, layer)
	for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
		for x := bounds.Min.X; x <= bounds.Max.X; x++ {
			fx := float64(x-bounds.Min.X) / float64(bounds.Max.X-bounds.Min.X)
			fy := float64(y-bounds.Min.Y) / float64(bounds.Max.Y-bounds.Min.Y)
			u, v := bilinearQuad(fx, fy), fy
			rr, gg, bb, aa, ok := marker.SamplePlacement(u, v, layer.Placement)
			if !ok {
				base := layer.Base
				img.SetRGBA(x, y, color.RGBA{R: toByte(base[0]), G: toByte(base[1]), B: toByte(base[2]), A: 255})
				continue
			}
			img.SetRGBA(x, y, color.RGBA{R: toByte(rr * aa), G: toByte(gg * aa), B: toByte(bb * aa), A: 255})
		}
	}
	_ = r
	return img
}

type rect struct{ Min, Max image.Point }

func silhouetteBounds(cam geom.Camera, layer Layer) rect {
	r := layer.Surface.Radius
	z := layer.Surface.Center.Z - cam.Eye.Z
	spanX := int(math.Round(r * cam.Focal / z))
	if layer.Kind == geom.KindSphere {
		spanX = int(math.Round(r * cam.Focal / (z - r)))
	}
	cx := cam.WidthPx / 2
	cy := cam.HeightPx / 2
	halfH := int(math.Round(layer.Surface.HalfH * cam.Focal / z))
	if layer.Kind == geom.KindSphere {
		halfH = spanX
	}
	return rect{
		Min: image.Point{X: cx - spanX, Y: cy - halfH},
		Max: image.Point{X: cx + spanX, Y: cy + halfH},
	}
}

func bilinearQuad(fx, fy float64) float64 {
	// Four-corner interpolation of U=0,0,1,1 across a flat quad. It ignores
	// the wrapped seam entirely, which is exactly the documented failure.
	top := fx
	bottom := fx
	return top*(1-fy) + bottom*fy
}

func toByte(x float64) uint8 {
	v := int(math.Round(x * 255))
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
