package transparency

import "sort"

// RasterizeLayers samples each transparent object into its own image. There is
// no compositing here: sharing one buffer at this point is what causes layers
// at different depths to inherit one history weight.
func RasterizeLayers(scene *Scene, frame int, jitter Vec2) []*LayerImage {
	layers := scene.LayersAt(frame)
	sortFarToNear(layers)
	out := make([]*LayerImage, 0, len(layers))
	for _, layer := range layers {
		out = append(out, rasterizeLayer(scene.Width, scene.Height, layer, jitter))
	}
	return out
}

func rasterizeLayer(width, height int, layer *Layer, jitter Vec2) *LayerImage {
	img := &LayerImage{
		ID:             layer.ID,
		Kind:           layer.Kind,
		Depth:          layer.Depth,
		DepthTolerance: layer.DepthTolerance,
		Motion:         layer.Motion,
		HistoryWeight:  layer.HistoryWeight,
		Pixels:         make([]Surface, width*height),
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			center := Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			p := center.Add(jitter)
			s := Surface{Depth: layer.Depth}
			if layer.DepthAt != nil {
				s.Depth = layer.DepthAt(p)
			}
			if layer.Shape.Support(center) {
				s.Support = 1
			}
			if layer.Shape.Inside(p) {
				shade := 1.0
				if layer.Shade != nil {
					shade = layer.Shade(p)
				}
				s.Alpha = clamp(layer.Opacity*shade, 0, 1)
				s.Color = layer.Color
			}
			img.Pixels[y*width+x] = s
		}
	}
	return img
}

// RenderReference renders each pixel with a regular subpixel grid, composites
// transparent layers far-to-near, and averages the resulting linear color. It
// is the oracle for the three temporal strategies.
func RenderReference(scene *Scene, frame int, config Config) FrameResult {
	config = config.withDefaults()
	w, h, spp := config.Width, config.Height, config.ReferenceSamples
	layers := scene.LayersAt(frame)
	sortFarToNear(layers)
	result := newFrameResult(w, h)
	counts := make(map[string]int, len(layers)+1)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum RGB
			var sumAlpha, sumNearAlpha float64
			for id := range counts {
				delete(counts, id)
			}
			center := Vec2{X: float64(x) + 0.5, Y: float64(y) + 0.5}
			bg := scene.BackgroundAt(x, y)
			for sy := 0; sy < spp; sy++ {
				for sx := 0; sx < spp; sx++ {
					p := center.Add(Vec2{
						X: (float64(sx)+0.5)/float64(spp) - 0.5,
						Y: (float64(sy)+0.5)/float64(spp) - 0.5,
					})
					rgb, alpha, nearAlpha, top := referenceSample(layers, config, bg, p)
					sum = sum.Add(rgb)
					sumAlpha += alpha
					sumNearAlpha += nearAlpha
					counts[top]++
				}
			}
			samples := float64(spp * spp)
			idx := y*w + x
			result.RGB[idx] = sum.Mul(1 / samples)
			result.Alpha[idx] = sumAlpha / samples
			result.NearAlpha[idx] = sumNearAlpha / samples
			result.NearSupport[idx] = nearSupportAt(layers, config.NearDepthCutoff, center)
			result.NearTop[idx] = modeTop(counts)
		}
	}
	return result
}

func referenceSample(layers []*Layer, config Config, bg RGB, p Vec2) (RGB, float64, float64, string) {
	rgb := bg
	transmittance, nearTransmittance := 1.0, 1.0
	top := "none"
	for _, layer := range layers {
		if !layer.Shape.Inside(p) {
			continue
		}
		shade := 1.0
		if layer.Shade != nil {
			shade = layer.Shade(p)
		}
		alpha := clamp(layer.Opacity*shade, 0, 1)
		rgb = layer.Color.Mul(alpha).Add(rgb.Mul(1 - alpha))
		transmittance *= 1 - alpha
		if layer.Depth <= config.NearDepthCutoff {
			nearTransmittance *= 1 - alpha
			if alpha >= config.NearAlphaCutoff {
				top = layer.ID
			}
		}
	}
	return rgb, 1 - transmittance, 1 - nearTransmittance, top
}

func nearSupportAt(layers []*Layer, cutoff float64, p Vec2) float64 {
	for _, layer := range layers {
		if layer.Depth <= cutoff && layer.Shape.Support(p) {
			return 1
		}
	}
	return 0
}

func modeTop(counts map[string]int) string {
	best := "none"
	bestCount := -1
	for id, count := range counts {
		if count > bestCount || (count == bestCount && id < best) {
			best, bestCount = id, count
		}
	}
	return best
}

func sortFarToNear(layers []*Layer) {
	sort.SliceStable(layers, func(i, j int) bool {
		if layers[i].Depth == layers[j].Depth {
			return layers[i].ID < layers[j].ID
		}
		return layers[i].Depth > layers[j].Depth
	})
}

func newFrameResult(w, h int) FrameResult {
	n := w * h
	return FrameResult{
		RGB:         make([]RGB, n),
		Alpha:       make([]float64, n),
		NearAlpha:   make([]float64, n),
		NearSupport: make([]float64, n),
		NearTop:     make([]string, n),
	}
}
