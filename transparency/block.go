package transparency

import "math"

type blockHistory struct {
	rgb       []RGB
	alpha     []float64
	nearAlpha []float64
	nearTop   []string
}

func historyFromFrame(frame FrameResult) *blockHistory {
	return &blockHistory{
		rgb:       append([]RGB(nil), frame.RGB...),
		alpha:     append([]float64(nil), frame.Alpha...),
		nearAlpha: append([]float64(nil), frame.NearAlpha...),
		nearTop:   append([]string(nil), frame.NearTop...),
	}
}

// ResolveWholeBlock is intentionally naive: it reprojects the already
// composited image with one motion vector and one history weight. It cannot
// distinguish glass, filament, and fog depth or ordering.
func ResolveWholeBlock(
	scene *Scene,
	frame int,
	current FrameResult,
	prev *blockHistory,
	weight float64,
	config Config,
) FrameResult {
	config = config.withDefaults()
	w, h := config.Width, config.Height
	out := newFrameResult(w, h)
	out.NearSupport = append([]float64(nil), current.NearSupport...)
	motion := scene.BlockMotion(frame)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			out.RGB[idx] = current.RGB[idx]
			out.Alpha[idx] = current.Alpha[idx]
			out.NearAlpha[idx] = current.NearAlpha[idx]
			out.NearTop[idx] = current.NearTop[idx]
			if prev == nil {
				continue
			}
			p := Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			sample, ok := sampleBlockHistory(prev, p.Sub(motion), w, h)
			if !ok || sample.alpha <= 0.01 {
				continue
			}
			out.RGB[idx] = current.RGB[idx].Lerp(sample.rgb, weight)
			out.Alpha[idx] = clamp(current.Alpha[idx]*(1-weight)+sample.alpha*weight, 0, 1)
			out.NearAlpha[idx] = clamp(
				current.NearAlpha[idx]*(1-weight)+sample.nearAlpha*weight, 0, 1,
			)
			if current.NearTop[idx] == "none" &&
				sample.nearTop != "none" &&
				sample.nearAlpha*weight >= config.NearAlphaCutoff {
				out.NearTop[idx] = "stale-history"
			}
		}
	}
	return out
}

type blockSample struct {
	rgb       RGB
	alpha     float64
	nearAlpha float64
	nearTop   string
}

func sampleBlockHistory(prev *blockHistory, p Vec2, width, height int) (blockSample, bool) {
	sx, sy := p.X-0.5, p.Y-0.5
	x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
	if x0 < 0 || y0 < 0 || x0+1 >= width || y0+1 >= height {
		return blockSample{}, false
	}
	fx, fy := sx-float64(x0), sy-float64(y0)
	i00 := y0*width + x0
	i10, i01, i11 := i00+1, i00+width, i00+width+1
	bilerp := func(a, b, c, d float64) float64 {
		top := a + (b-a)*fx
		bottom := c + (d-c)*fx
		return top + (bottom-top)*fy
	}
	nearest := i00
	if fx >= 0.5 {
		nearest = i10
	}
	if fy >= 0.5 {
		nearest += width
	}
	return blockSample{
		rgb: RGB{
			R: bilerp(prev.rgb[i00].R, prev.rgb[i10].R, prev.rgb[i01].R, prev.rgb[i11].R),
			G: bilerp(prev.rgb[i00].G, prev.rgb[i10].G, prev.rgb[i01].G, prev.rgb[i11].G),
			B: bilerp(prev.rgb[i00].B, prev.rgb[i10].B, prev.rgb[i01].B, prev.rgb[i11].B),
		},
		alpha:     bilerp(prev.alpha[i00], prev.alpha[i10], prev.alpha[i01], prev.alpha[i11]),
		nearAlpha: bilerp(prev.nearAlpha[i00], prev.nearAlpha[i10], prev.nearAlpha[i01], prev.nearAlpha[i11]),
		nearTop:   prev.nearTop[nearest],
	}, true
}

func averageWeight(weights map[string]float64) float64 {
	if len(weights) == 0 {
		return 0
	}
	var sum float64
	for _, weight := range weights {
		sum += weight
	}
	return sum / float64(len(weights))
}
