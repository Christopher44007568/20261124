package compositor

import (
	"encoding/json"
	"math"
)

func invalidDepth(d float64) bool {
	return math.IsNaN(d) || math.IsInf(d, 0) || d <= 0
}

func validDepth(d float64) bool {
	return !invalidDepth(d)
}

func missingDepth() float64 {
	return math.NaN()
}

// QuantizeDepth rounds a valid depth to the submitted quantization lattice.
// Invalid values stay invalid and are never coerced to zero or infinity.
func QuantizeDepth(d, step float64) float64 {
	if !validDepth(d) || step <= 0 {
		return d
	}
	return math.Round(d/step) * step
}

func quantizedTie(a, b, step float64) bool {
	if step <= 0 {
		return a == b
	}
	return math.Abs(a-b) <= step+step*1e-9
}

func rectMask(width, height, x0, y0, x1, y1 int) []bool {
	mask := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x >= x0 && x <= x1 && y >= y0 && y <= y1 {
				mask[y*width+x] = true
			}
		}
	}
	return mask
}

func depthField(width, height int, coverage []bool, base, dx, dy float64) []float64 {
	depth := make([]float64, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := y*width + x
			if coverage[i] {
				depth[i] = base + dx*float64(x) + dy*float64(y)
				continue
			}
			depth[i] = missingDepth()
		}
	}
	return depth
}

func cloneDepths(src []float64) []float64 {
	if src == nil {
		return nil
	}
	dst := make([]float64, len(src))
	copy(dst, src)
	return dst
}

func cloneLayer(l Layer) Layer {
	return Layer{
		ID:       l.ID,
		Kind:     l.Kind,
		System:   l.System,
		Order:    l.Order,
		Coverage: append([]bool(nil), l.Coverage...),
		Left:     cloneDepths(l.Left),
		Right:    cloneDepths(l.Right),
	}
}

func cloneScene(s Scene) Scene {
	out := Scene{Name: s.Name, Width: s.Width, Height: s.Height}
	out.Layers = make([]Layer, len(s.Layers))
	for i, layer := range s.Layers {
		out.Layers[i] = cloneLayer(layer)
	}
	return out
}

type depthRange struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Samples int     `json:"samples"`
}

type sceneLayerSummary struct {
	ID            string      `json:"id"`
	Kind          SurfaceKind `json:"kind"`
	System        bool        `json:"system"`
	Order         int         `json:"order"`
	CoveredPixels int         `json:"coveredPixels"`
	LeftDepth     *depthRange `json:"leftDepth,omitempty"`
	RightDepth    *depthRange `json:"rightDepth,omitempty"`
}

func summarizeDepths(depths []float64) *depthRange {
	summary := &depthRange{Min: math.Inf(1), Max: math.Inf(-1)}
	for _, depth := range depths {
		if !validDepth(depth) {
			continue
		}
		if depth < summary.Min {
			summary.Min = depth
		}
		if depth > summary.Max {
			summary.Max = depth
		}
		summary.Samples++
	}
	if summary.Samples == 0 {
		return nil
	}
	return summary
}

// MarshalJSON emits a JSON-safe scene summary. Invalid depth samples are
// omitted rather than encoded as NaN, which encoding/json rejects.
func (s Scene) MarshalJSON() ([]byte, error) {
	layers := make([]sceneLayerSummary, 0, len(s.Layers))
	for _, layer := range s.Layers {
		covered := 0
		for _, coveredPixel := range layer.Coverage {
			if coveredPixel {
				covered++
			}
		}
		layers = append(layers, sceneLayerSummary{
			ID:            layer.ID,
			Kind:          layer.Kind,
			System:        layer.System,
			Order:         layer.Order,
			CoveredPixels: covered,
			LeftDepth:     summarizeDepths(layer.Left),
			RightDepth:    summarizeDepths(layer.Right),
		})
	}
	payload := struct {
		Name   string              `json:"name"`
		Width  int                 `json:"width"`
		Height int                 `json:"height"`
		Layers []sceneLayerSummary `json:"layers"`
	}{
		Name: s.Name, Width: s.Width, Height: s.Height, Layers: layers,
	}
	return json.Marshal(payload)
}
