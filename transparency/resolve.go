package transparency

import "math"

type resolvedLayer struct {
	id      string
	depth   float64
	alpha   []float64
	premul  []RGB
	depthM  []float64
	support []float64
}

func currentResolved(images []*LayerImage) []*resolvedLayer {
	out := make([]*resolvedLayer, 0, len(images))
	for _, img := range images {
		layer := &resolvedLayer{
			id:      img.ID,
			depth:   img.Depth,
			alpha:   make([]float64, len(img.Pixels)),
			premul:  make([]RGB, len(img.Pixels)),
			depthM:  make([]float64, len(img.Pixels)),
			support: make([]float64, len(img.Pixels)),
		}
		for i, s := range img.Pixels {
			layer.alpha[i] = s.Alpha
			layer.premul[i] = s.Color.Mul(s.Alpha)
			layer.depthM[i] = s.Depth
			layer.support[i] = s.Support
		}
		out = append(out, layer)
	}
	return out
}

// ResolveLayers reprojects every layer independently. A history sample is
// accepted only when all of these hold:
//   - the current pixel is in that layer's footprint;
//   - the reprojected sample was covered in the previous frame;
//   - its depth agrees with the current layer's depth.
//
// The current footprint test is what prevents old coverage from becoming a
// fake near surface after an object moves.
func ResolveLayers(images []*LayerImage, history map[string]*resolvedLayer, width, height int) []*resolvedLayer {
	out := make([]*resolvedLayer, 0, len(images))
	for _, img := range images {
		prev := history[img.ID]
		layer := &resolvedLayer{
			id:      img.ID,
			depth:   img.Depth,
			alpha:   make([]float64, len(img.Pixels)),
			premul:  make([]RGB, len(img.Pixels)),
			depthM:  make([]float64, len(img.Pixels)),
			support: make([]float64, len(img.Pixels)),
		}
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idx := y*width + x
				cur := img.Pixels[idx]
				layer.depthM[idx] = cur.Depth
				layer.support[idx] = cur.Support
				curAlpha := cur.Alpha
				curPremul := cur.Color.Mul(curAlpha)
				if cur.Support == 0 {
					continue
				}
				if prev == nil {
					layer.alpha[idx], layer.premul[idx] = curAlpha, curPremul
					continue
				}
				p := Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
				prevAlpha, prevPremul, prevDepth, ok := sampleHistory(
					prev, p.Sub(img.Motion), width, height,
				)
				if !ok || prevAlpha <= 0.01 {
					layer.alpha[idx], layer.premul[idx] = curAlpha, curPremul
					continue
				}
				tolerance := img.DepthTolerance
				if tolerance <= 0 {
					tolerance = 1e-6
				}
				diff := math.Abs(prevDepth - cur.Depth)
				if diff > tolerance {
					layer.alpha[idx], layer.premul[idx] = curAlpha, curPremul
					continue
				}
				weight := clamp(img.HistoryWeight, 0, 0.95) *
					(1 - 0.45*clamp(diff/tolerance, 0, 1))
				layer.alpha[idx] = clamp(curAlpha*(1-weight)+prevAlpha*weight, 0, 1)
				layer.premul[idx] = curPremul.Mul(1 - weight).Add(prevPremul.Mul(weight))
			}
		}
		out = append(out, layer)
	}
	return out
}

func sampleHistory(prev *resolvedLayer, p Vec2, width, height int) (float64, RGB, float64, bool) {
	sx, sy := p.X-0.5, p.Y-0.5
	x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
	if x0 < 0 || y0 < 0 || x0+1 >= width || y0+1 >= height {
		return 0, RGB{}, 0, false
	}
	fx, fy := sx-float64(x0), sy-float64(y0)
	i00 := y0*width + x0
	i10 := i00 + 1
	i01 := i00 + width
	i11 := i01 + 1

	bilerp := func(a, b, c, d float64) float64 {
		top := a + (b-a)*fx
		bottom := c + (d-c)*fx
		return top + (bottom-top)*fy
	}
	alpha := bilerp(prev.alpha[i00], prev.alpha[i10], prev.alpha[i01], prev.alpha[i11])
	depth := bilerp(prev.depthM[i00], prev.depthM[i10], prev.depthM[i01], prev.depthM[i11])
	premul := RGB{
		R: bilerp(prev.premul[i00].R, prev.premul[i10].R, prev.premul[i01].R, prev.premul[i11].R),
		G: bilerp(prev.premul[i00].G, prev.premul[i10].G, prev.premul[i01].G, prev.premul[i11].G),
		B: bilerp(prev.premul[i00].B, prev.premul[i10].B, prev.premul[i01].B, prev.premul[i11].B),
	}
	return alpha, premul, depth, true
}

// Composite resolves the final opaque image in strict far-to-near order. It
// also records near-layer coverage and the nearest contributing layer, which
// are compared against the reference's draw-order oracle.
func Composite(scene *Scene, layers []*resolvedLayer, config Config) FrameResult {
	config = config.withDefaults()
	w, h := config.Width, config.Height
	result := newFrameResult(w, h)
	ordered := append([]*resolvedLayer(nil), layers...)
	sortResolvedFarToNear(ordered)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			rgb := scene.BackgroundAt(x, y)
			transmittance, nearTransmittance := 1.0, 1.0
			top := "none"
			for _, layer := range ordered {
				alpha := layer.alpha[idx]
				if layer.depth <= config.NearDepthCutoff && layer.support[idx] > 0 {
					result.NearSupport[idx] = 1
				}
				if alpha <= 0 {
					continue
				}
				premul := layer.premul[idx]
				rgb = premul.Add(rgb.Mul(1 - alpha))
				transmittance *= 1 - alpha
				if layer.depth <= config.NearDepthCutoff {
					nearTransmittance *= 1 - alpha
					if alpha >= config.NearAlphaCutoff {
						top = layer.id
					}
				}
			}
			result.RGB[idx] = rgb
			result.Alpha[idx] = 1 - transmittance
			result.NearAlpha[idx] = 1 - nearTransmittance
			result.NearTop[idx] = top
		}
	}
	return result
}

func sortResolvedFarToNear(layers []*resolvedLayer) {
	for i := 1; i < len(layers); i++ {
		for j := i; j > 0 && layers[j-1].depth < layers[j].depth; j-- {
			layers[j-1], layers[j] = layers[j], layers[j-1]
		}
	}
}

func layerWeights(images []*LayerImage) map[string]float64 {
	out := make(map[string]float64, len(images))
	for _, img := range images {
		out[img.ID] = img.HistoryWeight
	}
	return out
}
