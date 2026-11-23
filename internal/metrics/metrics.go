// Package metrics measures spatial and temporal reconstruction quality.
package metrics

import (
	"fmt"
	"math"
	"sort"

	"fidelitycheck/internal/imaging"
	"fidelitycheck/internal/scenes"
)

type Frame struct {
	GlobalMAE     float64 `json:"global_mae"`
	MaskedMAE     float64 `json:"masked_mae"`
	CenterMAE     float64 `json:"center_mae"`
	PeripheryMAE  float64 `json:"periphery_mae"`
	EdgeMAE       float64 `json:"edge_mae"`
	EdgeRetention float64 `json:"edge_retention"`
}

type Result struct {
	GlobalMAE     float64 `json:"global_mae"`
	GlobalP95     float64 `json:"global_p95"`
	GlobalMax     float64 `json:"global_max"`
	MaskedMAE     float64 `json:"masked_mae"`
	CenterMAE     float64 `json:"center_mae"`
	CenterP95     float64 `json:"center_p95"`
	CenterMax     float64 `json:"center_max"`
	PeripheryMAE  float64 `json:"periphery_mae"`
	EdgeMAE       float64 `json:"edge_mae"`
	EdgeRetention float64 `json:"edge_retention"`

	TemporalMAE   float64   `json:"temporal_mae"`
	FlickerMean   float64   `json:"flicker_mean"`
	FlickerP95    float64   `json:"flicker_p95"`
	FlickerMax    float64   `json:"flicker_max"`
	PerFrame      []Frame   `json:"per_frame"`
	TemporalError []float64 `json:"temporal_error"`
	Flicker       []float64 `json:"flicker"`
}

type Counts struct {
	MaskPixels      int `json:"mask_pixels"`
	CenterPixels    int `json:"center_pixels"`
	PeripheryPixels int `json:"periphery_pixels"`
	EdgePixels      int `json:"edge_pixels_first_frame"`
}

const edgeThreshold = 0.16

func Compute(seq *scenes.Sequence, outputs []*imaging.Frame) (Result, Counts, error) {
	if seq == nil || seq.Mask == nil {
		return Result{}, Counts{}, fmt.Errorf("metrics: missing sequence or mask")
	}
	if len(outputs) != len(seq.Frames) || len(outputs) == 0 {
		return Result{}, Counts{}, fmt.Errorf("metrics: got %d outputs for %d ground-truth frames", len(outputs), len(seq.Frames))
	}
	for i, out := range outputs {
		if out.W != seq.Frames[i].W || out.H != seq.Frames[i].H {
			return Result{}, Counts{}, fmt.Errorf("metrics: frame %d has wrong dimensions", i)
		}
	}

	counts := Counts{
		MaskPixels:      seq.Mask.Count(),
		CenterPixels:    seq.Center.Count(),
		PeripheryPixels: seq.Periphery.Count(),
	}
	res := Result{PerFrame: make([]Frame, len(outputs))}
	var edgeSamples int

	for i := range outputs {
		fm, edgeN := frameMetrics(seq.Frames[i], outputs[i], seq)
		res.PerFrame[i] = fm
		res.GlobalMAE += fm.GlobalMAE
		res.MaskedMAE += fm.MaskedMAE
		res.CenterMAE += fm.CenterMAE
		res.PeripheryMAE += fm.PeripheryMAE
		if edgeN > 0 {
			res.EdgeMAE += fm.EdgeMAE * float64(edgeN)
			res.EdgeRetention += fm.EdgeRetention * float64(edgeN)
			edgeSamples += edgeN
		}
		if i == 0 {
			counts.EdgePixels = edgeN
		}
	}
	n := float64(len(outputs))
	res.GlobalMAE /= n
	res.MaskedMAE /= n
	res.CenterMAE /= n
	res.PeripheryMAE /= n
	if edgeSamples > 0 {
		res.EdgeMAE /= float64(edgeSamples)
		res.EdgeRetention /= float64(edgeSamples)
	}

	var centers, globals []float64
	for _, fm := range res.PerFrame {
		centers = append(centers, fm.CenterMAE)
		globals = append(globals, fm.GlobalMAE)
	}
	res.CenterP95 = percentile(centers, 0.95)
	res.CenterMax = maxValue(centers)
	res.GlobalP95 = percentile(globals, 0.95)
	res.GlobalMax = maxValue(globals)

	res.TemporalError, res.Flicker = temporalErrors(seq, outputs)
	res.TemporalMAE = mean(res.TemporalError)
	res.FlickerMean = mean(res.Flicker)
	res.FlickerP95 = percentile(res.Flicker, 0.95)
	res.FlickerMax = maxValue(res.Flicker)
	return res, counts, nil
}

func frameMetrics(gt, out *imaging.Frame, seq *scenes.Sequence) (Frame, int) {
	global, _ := errorOver(gt, out, nil)
	masked, _ := errorOver(gt, out, seq.Mask)
	center, _ := errorOver(gt, out, seq.Center)
	periphery, _ := errorOver(gt, out, seq.Periphery)

	edgeMask := edgePixels(gt, seq.Mask)
	edgeN := edgeMask.Count()
	edgeErr, _ := errorOver(gt, out, edgeMask)
	retention := edgeRetention(gt, out, edgeMask)
	return Frame{
		GlobalMAE:     global,
		MaskedMAE:     masked,
		CenterMAE:     center,
		PeripheryMAE:  periphery,
		EdgeMAE:       edgeErr,
		EdgeRetention: retention,
	}, edgeN
}

func errorOver(gt, out *imaging.Frame, mask *imaging.Mask) (float64, int) {
	var sum float64
	var count int
	for i := range gt.Pix {
		if mask != nil && !mask.Bits[i] {
			continue
		}
		d := gt.Pix[i].Sub(out.Pix[i]).Abs()
		sum += (d.R + d.G + d.B) / 3
		count++
	}
	if count == 0 {
		return 0, 0
	}
	return sum / float64(count), count
}

func edgePixels(gt *imaging.Frame, mask *imaging.Mask) *imaging.Mask {
	edges := imaging.NewMask(gt.W, gt.H)
	if mask.W != gt.W || mask.H != gt.H {
		return edges
	}
	for y := 1; y < gt.H-1; y++ {
		for x := 1; x < gt.W-1; x++ {
			i := y*gt.W + x
			if !mask.Bits[i] {
				continue
			}
			if gradient(gt, x, y) >= edgeThreshold {
				edges.Set(x, y, true)
			}
		}
	}
	return edges
}

func gradient(f *imaging.Frame, x, y int) float64 {
	if x <= 0 || y <= 0 || x >= f.W-1 || y >= f.H-1 {
		return 0
	}
	dx := colorDistance(f.At(x+1, y), f.At(x-1, y)) / 2
	dy := colorDistance(f.At(x, y+1), f.At(x, y-1)) / 2
	return math.Hypot(dx, dy)
}

func colorDistance(a, b imaging.RGB) float64 {
	d := a.Sub(b).Abs()
	return (d.R + d.G + d.B) / 3
}

func edgeRetention(gt, out *imaging.Frame, edges *imaging.Mask) float64 {
	var sum float64
	var count int
	for y := 1; y < gt.H-1; y++ {
		for x := 1; x < gt.W-1; x++ {
			if !edges.At(x, y) {
				continue
			}
			g := gradient(gt, x, y)
			if g < 1e-9 {
				continue
			}
			o := gradient(out, x, y)
			sum += math.Min(1, o/g)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func temporalErrors(seq *scenes.Sequence, outputs []*imaging.Frame) ([]float64, []float64) {
	var temporal, flicker []float64
	for i := 1; i < len(outputs); i++ {
		temporal = append(temporal, temporalMAE(
			seq.Frames[i-1], seq.Frames[i],
			outputs[i-1], outputs[i],
			seq.Mask,
		))
		if i >= 2 {
			flicker = append(flicker, temporalAcceleration(
				seq.Frames[i-2], seq.Frames[i-1], seq.Frames[i],
				outputs[i-2], outputs[i-1], outputs[i],
				seq.Mask,
			))
		}
	}
	return temporal, flicker
}

func temporalMAE(gt0, gt1, out0, out1 *imaging.Frame, mask *imaging.Mask) float64 {
	var sum float64
	var count int
	for i, missing := range mask.Bits {
		if !missing {
			continue
		}
		gtDelta := gt1.Pix[i].Sub(gt0.Pix[i])
		outDelta := out1.Pix[i].Sub(out0.Pix[i])
		d := gtDelta.Sub(outDelta).Abs()
		sum += (d.R + d.G + d.B) / 3
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func temporalAcceleration(gt0, gt1, gt2, out0, out1, out2 *imaging.Frame, mask *imaging.Mask) float64 {
	var sum float64
	var count int
	for i, missing := range mask.Bits {
		if !missing {
			continue
		}
		gtAccel := gt2.Pix[i].Sub(gt1.Pix[i].Scale(2)).Add(gt0.Pix[i])
		outAccel := out2.Pix[i].Sub(out1.Pix[i].Scale(2)).Add(out0.Pix[i])
		d := gtAccel.Sub(outAccel).Abs()
		sum += (d.R + d.G + d.B) / 3
		count++
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := p * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	frac := pos - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func maxValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, v := range values[1:] {
		if v > best {
			best = v
		}
	}
	return best
}
