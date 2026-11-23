// Package metrics computes the four acceptance signals required by the
// stereo re-projection validator: per-eye reprojection error, disparity
// behavior, ghosting ratio and edge misalignment.
package metrics

import "math"

// EdgeMask returns a Sobel-based edge mask of a grayscale image. NaN holes
// are treated as black so hole boundaries count as edges too.
func EdgeMask(g []float64, w, h int) []bool {
	mask := make([]bool, w*h)
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 0
		}
		v := g[y*w+x]
		if math.IsNaN(v) {
			return 0
		}
		return v
	}
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			var gx, gy float64
			gy += at(x-1, y-1) + 2*at(x-1, y) + at(x-1, y+1)
			gy -= at(x+1, y-1) + 2*at(x+1, y) + at(x+1, y+1)
			gx += at(x-1, y-1) + 2*at(x, y-1) + at(x+1, y-1)
			gx -= at(x-1, y+1) + 2*at(x, y+1) + at(x+1, y+1)
			if gx*gx+gy*gy > 900 { // |gradient| > 30 gray levels
				mask[y*w+x] = true
			}
		}
	}
	return mask
}

// edgeDistance builds a chamfer distance field (in pixels) to the nearest
// edge pixel. Two-pass 3-4 chamfer, O(n) per image.
func edgeDistance(mask []bool, w, h int) []float64 {
	const big = 1e9
	d := make([]float64, w*h)
	for i, m := range mask {
		if m {
			d[i] = 0
		} else {
			d[i] = big
		}
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			if x > 0 {
				d[i] = math.Min(d[i], d[i-1]+1)
			}
			if y > 0 {
				d[i] = math.Min(d[i], d[i-w]+1)
			}
			if x > 0 && y > 0 {
				d[i] = math.Min(d[i], d[i-w-1]+1.4142)
			}
			if x < w-1 && y > 0 {
				d[i] = math.Min(d[i], d[i-w+1]+1.4142)
			}
		}
	}
	for y := h - 1; y >= 0; y-- {
		for x := w - 1; x >= 0; x-- {
			i := y*w + x
			if x < w-1 {
				d[i] = math.Min(d[i], d[i+1]+1)
			}
			if y < h-1 {
				d[i] = math.Min(d[i], d[i+w]+1)
			}
			if x < w-1 && y < h-1 {
				d[i] = math.Min(d[i], d[i+w+1]+1.4142)
			}
			if x > 0 && y < h-1 {
				d[i] = math.Min(d[i], d[i+w-1]+1.4142)
			}
		}
	}
	return d
}

// EdgeMisalignment is the mean distance from warped-image edges to
// ground-truth edges (directed metric), plus the symmetric chamfer distance
// averaged over both directions.
func EdgeMisalignment(warped, gt []float64, w, h int) (directed, symmetric float64) {
	wm := EdgeMask(warped, w, h)
	gm := EdgeMask(gt, w, h)
	wd := edgeDistance(wm, w, h)
	gd := edgeDistance(gm, w, h)
	var sumW, sumG float64
	var nW, nG int
	for i := range wm {
		if wm[i] {
			sumG += gd[i]
			nW++
		}
		if gm[i] {
			sumW += wd[i]
			nG++
		}
	}
	if nW == 0 || nG == 0 {
		return 0, 0
	}
	directed = sumG / float64(nW)
	symmetric = (sumG/float64(nW) + sumW/float64(nG)) / 2
	return
}
