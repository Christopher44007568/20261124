package main

import "math"

type Metrics struct {
	Scene            string
	Panel            string
	PanelPeakLuma    float64
	PanelPeakChannel [3]float64
	PanelActiveCount int
	EyePeakLuma      float64
	EyeMeanLuma      float64
	LeftMaxRGB       [3]float64
	RightMaxRGB      [3]float64
	Crosstalk        float64
	BinocularMAD     float64
	BinocularMaxDiff float64
	EdgeMidMAD       float64
	Mono             bool
}

func maxChannel(img EyeImage) [3]float64 {
	var out [3]float64
	for _, c := range img.px {
		for k := 0; k < 3; k++ {
			if c[k] > out[k] {
				out[k] = c[k]
			}
		}
	}
	return out
}

func eyeLumaStats(img EyeImage) (peak, mean float64) {
	var sum float64
	for _, c := range img.px {
		v := luma(c)
		sum += v
		if v > peak {
			peak = v
		}
	}
	return peak, sum / float64(len(img.px))
}

// chromaticCrosstalk measures unintended color-channel energy. For a red-only
// scene it is non-red over red. For nominally white content it is the chromatic
// imbalance between channels. A mono panel has no color filters, so its
// chromatic crosstalk is structurally zero.
func chromaticCrosstalk(sceneName string, mono bool, left, right EyeImage) float64 {
	if mono {
		return 0
	}
	if sceneName == "redline" {
		var red, other float64
		for _, c := range left.px {
			red += c[0]
			other += c[1] + c[2]
		}
		if red == 0 {
			return 0
		}
		return other / red
	}
	var total, imbalance float64
	for i := range left.px {
		a := left.px[i]
		b := right.px[i]
		r, g, bl := a[0]+b[0], a[1]+b[1], a[2]+b[2]
		total += r + g + bl
		imbalance += math.Abs(r-g) + math.Abs(g-bl) + math.Abs(r-bl)
	}
	if total == 0 {
		return 0
	}
	return imbalance / total
}

func binocularDiff(left, right EyeImage) (mad, maxDiff float64) {
	var sum float64
	for i := range left.px {
		for k := 0; k < 3; k++ {
			d := math.Abs(left.px[i][k] - right.px[i][k])
			sum += d
			if d > maxDiff {
				maxDiff = d
			}
		}
	}
	return sum / float64(3*len(left.px)), maxDiff
}

// edgeMidMAD compares the two eyes along the transition column only,
// exposing phase-dependent ringing at the high-contrast boundary.
func edgeMidMAD(p *Panel, left, right EyeImage) float64 {
	col := p.w / 2
	var sum float64
	for y := 0; y < p.h; y++ {
		a := left.px[y*p.w+col]
		b := right.px[y*p.w+col]
		for k := 0; k < 3; k++ {
			sum += math.Abs(a[k] - b[k])
		}
	}
	return sum / float64(3*p.h)
}

func measure(p *Panel, sim *Simulation, sceneName string, src *Source) Metrics {
	m := Metrics{Scene: sceneName, Panel: p.name, Mono: isMono(p)}
	for _, v := range sim.panelLuma {
		if v > m.PanelPeakLuma {
			m.PanelPeakLuma = v
		}
		if v > 1e-9 {
			m.PanelActiveCount++
		}
	}
	for i, e := range p.emitters {
		for k := 0; k < 3; k++ {
			if v := sim.drive[i] * e.filter[k]; v > m.PanelPeakChannel[k] {
				m.PanelPeakChannel[k] = v
			}
		}
	}
	m.EyePeakLuma, m.EyeMeanLuma = eyeLumaStats(sim.left)
	m.LeftMaxRGB = maxChannel(sim.left)
	m.RightMaxRGB = maxChannel(sim.right)
	m.Crosstalk = chromaticCrosstalk(sceneName, m.Mono, sim.left, sim.right)
	m.BinocularMAD, m.BinocularMaxDiff = binocularDiff(sim.left, sim.right)
	if sceneName == "edge" {
		m.EdgeMidMAD = edgeMidMAD(p, sim.left, sim.right)
	}
	return m
}

// countPartialEmitters reports driven emitters whose coverage is neither off
// nor fully on, indicating sub-pixel phase rather than a blurred average.
func countPartialEmitters(sim *Simulation) int {
	var n int
	for _, d := range sim.drive {
		if d > 1e-6 && d < 1-1e-6 {
			n++
		}
	}
	return n
}

// emitterMix reports how many logical pixels see 0, 1, 2, or 3 distinct
// channels among overlapping emitters, proving no panel is treated as full
// RGB per pixel.
func emitterMix(p *Panel) [4]int {
	seen := make([]uint8, p.w*p.h)
	for _, e := range p.emitters {
		cx := int((e.ap.min.x + e.ap.max.x) / 2)
		cy := int((e.ap.min.y + e.ap.max.y) / 2)
		if cx < 0 || cy < 0 || cx >= p.w || cy >= p.h {
			continue
		}
		var ch uint8 = 3
		for k := 0; k < 3; k++ {
			if e.filter[k] > 0 && e.filter != whiteFilter {
				ch = uint8(k)
				break
			}
		}
		seen[cy*p.w+cx] |= 1 << ch
	}
	var hist [4]int
	for _, mask := range seen {
		hist[popcount8(mask)]++
	}
	return hist
}

// filterDensity returns the fraction of emitters carrying each filter.
// Mono panels report everything on the white slot (index 3).
func filterDensity(p *Panel) [4]float64 {
	var counts [4]int
	for _, e := range p.emitters {
		if e.filter == whiteFilter {
			counts[3]++
			continue
		}
		for k := 0; k < 3; k++ {
			if e.filter[k] > 0 {
				counts[k]++
				break
			}
		}
	}
	var out [4]float64
	n := float64(len(p.emitters))
	for i := range counts {
		out[i] = float64(counts[i]) / n
	}
	return out
}

func popcount8(v uint8) int {
	n := 0
	for v != 0 {
		n += int(v & 1)
		v >>= 1
	}
	return n
}
