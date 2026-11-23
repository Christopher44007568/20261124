package foveal

import "math"

// invalidateEye marks the saccade corridor between the previous and current
// gaze points, plus the landing disk. Slow pursuit only moves the disk
// slightly; its newly covered crescent is handled by ReasonZone ownership
// changes instead of a broad reset.
func (d *Denoiser) invalidateEye(f *Frame) {
	if !d.hasPrevGaze {
		return
	}
	g0, g1 := d.prevGaze, f.Gaze
	speed := vecLen(vecSub(g1, g0))
	if speed <= d.params.SaccadeSpeed {
		return
	}

	landingR := d.params.FoveaRadius + d.params.GuardWidth
	landingR2 := landingR * landingR
	corr := d.params.SaccadeCorridor
	corr2 := corr * corr

	w, h := f.Width, f.Height
	for y := 0; y < h; y++ {
		py := float32(y) + 0.5
		for x := 0; x < w; x++ {
			px := float32(x) + 0.5
			p := Vec2{px, py}
			inCorridor := pointSegmentDist2(p, g0, g1) <= corr2
			dx, dy := px-g1.X, py-g1.Y
			inLanding := dx*dx+dy*dy <= landingR2
			if inCorridor || inLanding {
				d.reset[y*w+x] |= ReasonEye
			}
		}
	}
}

// invalidateMotionAndDisocclusion flags only locally inconsistent motion.
// Uniform global motion (a head turn) has near-zero divergence and therefore
// does not invalidate any history; object boundaries and revealed regions do.
func (d *Denoiser) invalidateMotionAndDisocclusion(f *Frame) {
	w, h := f.Width, f.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := y*w + x
			mv := f.MV[p]
			if f.Invalid != nil && f.Invalid[p] {
				d.reset[p] |= ReasonDisocclusion
				continue
			}
			if vecLen(mv) <= d.params.MotionThreshold {
				continue
			}
			if d.motionDivergence(f, x, y) > d.params.DivergenceThreshold {
				d.reset[p] |= ReasonMotion
			}
		}
	}
}

// motionDivergence returns the largest component-wise MV difference against
// the four connected neighbors (edge pixels reuse the border neighbor).
func (d *Denoiser) motionDivergence(f *Frame, x, y int) float32 {
	w, h := f.Width, f.Height
	p := y*w + x
	mv := f.MV[p]

	xl := max(x-1, 0)
	xr := min(x+1, w-1)
	yt := max(y-1, 0)
	yb := min(y+1, h-1)

	neighbors := [4]int{
		y*w + xl,
		y*w + xr,
		yt*w + x,
		yb*w + x,
	}
	var worst float32
	for _, q := range neighbors {
		diff := vecSub(mv, f.MV[q])
		worst = max(worst, max(abs32(diff.X), abs32(diff.Y)))
	}
	return worst
}

// invalidateHighlight is applied in processPixel after reprojection, because
// the comparison needs the reprojected history color. This helper only
// classifies whether the current pixel is a dynamic highlight.
func highlightActive(current, history [3]float32, params *Params) bool {
	curLum := luminance(current)
	if curLum < params.HighlightThreshold {
		return false
	}
	histLum := luminance(history)
	denom := max(histLum, 1e-4)
	return abs32(curLum-histLum)/denom > params.HighlightTolerance
}

func vecSub(a, b Vec2) Vec2 { return Vec2{a.X - b.X, a.Y - b.Y} }

func vecLen(v Vec2) float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y)))
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// pointSegmentDist2 is the squared pixel distance from p to segment ab.
func pointSegmentDist2(p, a, b Vec2) float32 {
	ab := vecSub(b, a)
	ap := vecSub(p, a)
	len2 := ab.X*ab.X + ab.Y*ab.Y
	if len2 == 0 {
		return ap.X*ap.X + ap.Y*ap.Y
	}
	t := (ap.X*ab.X + ap.Y*ab.Y) / len2
	t = max(0, min(1, t))
	proj := Vec2{a.X + ab.X*t, a.Y + ab.Y*t}
	d := vecSub(p, proj)
	return d.X*d.X + d.Y*d.Y
}

func luminance(rgb [3]float32) float32 {
	return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
}

// log2Abs returns |log2(scale)|. Non-positive scales are treated as an
// infinitely large exposure change, forcing a reset.
func log2Abs(scale float32) float32 {
	if scale <= 0 {
		return float32(math.Inf(1))
	}
	return float32(math.Abs(math.Log2(float64(scale))))
}
