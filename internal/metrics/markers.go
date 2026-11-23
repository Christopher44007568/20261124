package metrics

import (
	"math"
	"sort"
)

// MarkerObs is one detected marker: intensity-weighted centroid and whether
// its support touches a hole or a cross-eye fill region.
type MarkerObs struct {
	ID       int
	Found    bool
	U, V     float64
	OnFill   bool
	OnHole   bool
	NearHole bool
	Support  int
}

// LocateMarker measures the bright marker disk nearest the expected
// projection (eu, ev). fill/hole masks flag contaminated observations.
//
// The measurement is confined to a fixed capture radius (smaller than the
// inter-marker spacing, so a neighbor can never enter), and every pixel in it
// contributes with the brightness above its local background as weight. This
// soft, background-subtracted centroid stays unbiased when warping turns the
// disk rim anti-aliased.
func LocateMarker(gray []float64, w, h int, eu, ev float64, fill, hole []bool) MarkerObs {
	const captureR = 7.0
	const support = 180.0 // background level below which a pixel is not a marker
	cx, cy := int(math.Round(eu)), int(math.Round(ev))
	if !inBounds(w, h, cx, cy) {
		return MarkerObs{Found: false}
	}
	if math.IsNaN(gray[cy*w+cx]) {
		return MarkerObs{Found: false, OnHole: hole != nil && hole[cy*w+cx]}
	}

	r := int(captureR) + 1
	minG, maxG := 300.0, -1.0
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if !inBounds(w, h, x, y) {
				continue
			}
			dx := float64(x) + 0.5 - eu
			dy := float64(y) + 0.5 - ev
			if dx*dx+dy*dy > captureR*captureR {
				continue
			}
			g := gray[y*w+x]
			if math.IsNaN(g) {
				continue
			}
			if g < minG {
				minG = g
			}
			if g > maxG {
				maxG = g
			}
		}
	}
	if maxG < support || maxG-minG < 40 {
		return MarkerObs{Found: false, OnHole: holeNear(hole, w, h, eu, ev, captureR), NearHole: holeNear(hole, w, h, eu, ev, captureR+3)}
	}
	onFill, onHole := false, false
	var su, sv, sw float64
	n := 0
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if !inBounds(w, h, x, y) {
				continue
			}
			dx := float64(x) + 0.5 - eu
			dy := float64(y) + 0.5 - ev
			if dx*dx+dy*dy > captureR*captureR {
				continue
			}
			i := y*w + x
			g := gray[i]
			if math.IsNaN(g) {
				if hole != nil && hole[i] {
					onHole = true
				}
				continue
			}
			wt := g - minG
			if wt <= 0 {
				continue
			}
			su += (float64(x) + 0.5) * wt
			sv += (float64(y) + 0.5) * wt
			sw += wt
			n++
			if fill != nil && fill[i] {
				onFill = true
			}
		}
	}
	if n < 4 || sw <= 0 {
		return MarkerObs{Found: false, OnFill: onFill, OnHole: onHole, NearHole: holeNear(hole, w, h, eu, ev, captureR+3)}
	}
	return MarkerObs{Found: true, U: su / sw, V: sv / sw, OnFill: onFill, OnHole: onHole, NearHole: onHole || holeNear(hole, w, h, eu, ev, captureR+3), Support: n}
}

func inBounds(w, h, x, y int) bool {
	return x >= 0 && y >= 0 && x < w && y < h
}

func holeNear(hole []bool, w, h int, eu, ev, radius float64) bool {
	if hole == nil {
		return false
	}
	cx, cy := int(math.Round(eu)), int(math.Round(ev))
	r := int(radius) + 1
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if !inBounds(w, h, x, y) {
				continue
			}
			dx := float64(x) + 0.5 - eu
			dy := float64(y) + 0.5 - ev
			if dx*dx+dy*dy <= radius*radius && hole[y*w+x] {
				return true
			}
		}
	}
	return false
}

// ReprojError is the 2-D pixel displacement of an observation relative to the
// ground-truth projection of its known-depth world point.
func ReprojError(obs MarkerObs, eu, ev float64) (ex, ey, e float64) {
	if !obs.Found {
		return 0, 0, math.NaN()
	}
	ex = obs.U - eu
	ey = obs.V - ev
	return ex, ey, math.Hypot(ex, ey)
}

// Disparity is the horizontal/vertical stereo separation between the two
// eyes' observations of the same marker.
func Disparity(left, right MarkerObs) (dX, dY float64, ok bool) {
	if !left.Found || !right.Found {
		return 0, 0, false
	}
	return left.U - right.U, left.V - right.V, true
}

// RMS computes the root mean square (NaN ignored).
func RMS(vals []float64) float64 {
	var s float64
	n := 0
	for _, v := range vals {
		if math.IsNaN(v) {
			continue
		}
		s += v * v
		n++
	}
	if n == 0 {
		return math.NaN()
	}
	return math.Sqrt(s / float64(n))
}

// Mean computes the arithmetic mean (NaN ignored).
func Mean(vals []float64) float64 {
	var s float64
	n := 0
	for _, v := range vals {
		if math.IsNaN(v) {
			continue
		}
		s += v
		n++
	}
	if n == 0 {
		return math.NaN()
	}
	return s / float64(n)
}

// MaxAbs returns the largest absolute value (NaN ignored).
func MaxAbs(vals []float64) float64 {
	m := 0.0
	any := false
	for _, v := range vals {
		if math.IsNaN(v) {
			continue
		}
		any = true
		m = math.Max(m, math.Abs(v))
	}
	if !any {
		return math.NaN()
	}
	return m
}

// Percentile returns the p-quantile (p in [0,1]) of the absolute values,
// ignoring NaN. It is a robust alternative to MaxAbs for acceptance gates.
func Percentile(vals []float64, p float64) float64 {
	xs := make([]float64, 0, len(vals))
	for _, v := range vals {
		if !math.IsNaN(v) {
			xs = append(xs, math.Abs(v))
		}
	}
	if len(xs) == 0 {
		return math.NaN()
	}
	sort.Float64s(xs)
	idx := int(math.Round(p * float64(len(xs)-1)))
	if idx < 0 {
		idx = 0
	}
	if idx >= len(xs) {
		idx = len(xs) - 1
	}
	return xs[idx]
}
