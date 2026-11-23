package transparency

// LayerInfo documents the depth and per-layer history weight in a run.
type LayerInfo struct {
	ID            string    `json:"id"`
	Kind          LayerKind `json:"kind"`
	Depth         float64   `json:"depth"`
	HistoryWeight float64   `json:"history_weight"`
}

// StrategySummary aggregates metrics and weights over all frames.
type StrategySummary struct {
	Metrics      FrameMetrics       `json:"metrics"`
	LayerWeights map[string]float64 `json:"layer_weights"`
	SharedWeight float64            `json:"shared_weight,omitempty"`
}

// FrameReport is the JSON-friendly part of one frame's comparison.
type FrameReport struct {
	Frame   int                     `json:"frame"`
	Metrics map[string]FrameMetrics `json:"metrics"`
}

// FrameRun keeps the actual buffers for image export and inspection.
type FrameRun struct {
	Frame     int
	Reference FrameResult
	Results   map[string]FrameResult
}

// Evaluation is the complete deterministic comparison.
type Evaluation struct {
	Config      Config                     `json:"config"`
	DrawOrder   []string                   `json:"draw_order"`
	Layers      []LayerInfo                `json:"layers"`
	FrameReport []FrameReport              `json:"frames"`
	Summary     map[string]StrategySummary `json:"summary"`
	Runs        []FrameRun                 `json:"-"`
}

// Evaluate runs the same scene through all three paths. The reference is
// rebuilt from geometry every frame, so it never inherits temporal history.
func Evaluate(scene *Scene, config Config) *Evaluation {
	config = config.withDefaults()
	if scene.Width != config.Width || scene.Height != config.Height {
		scene.Width, scene.Height = config.Width, config.Height
	}
	evaluation := &Evaluation{
		Config:  config,
		Summary: make(map[string]StrategySummary, 3),
	}
	firstLayers := scene.LayersAt(0)
	sortFarToNear(firstLayers)
	for _, layer := range firstLayers {
		evaluation.DrawOrder = append(evaluation.DrawOrder, layer.ID)
		evaluation.Layers = append(evaluation.Layers, LayerInfo{
			ID: layer.ID, Kind: layer.Kind, Depth: layer.Depth,
			HistoryWeight: layer.HistoryWeight,
		})
	}

	layerHistory := make(map[string]*resolvedLayer)
	var blockPrev *blockHistory
	allMetrics := map[string][]FrameMetrics{
		StrategyPerLayer:   {},
		StrategyWholeBlock: {},
		StrategyNoHistory:  {},
	}
	weightSum := make(map[string]float64)
	weightCount := make(map[string]int)
	var blockWeightSum float64

	for frame := 0; frame < config.Frames; frame++ {
		images := RasterizeLayers(scene, frame, Jitter(frame))
		reference := RenderReference(scene, frame, config)
		currentLayers := currentResolved(images)
		current := Composite(scene, currentLayers, config)
		weights := layerWeights(images)

		resolved := ResolveLayers(images, layerHistory, config.Width, config.Height)
		perLayer := Composite(scene, resolved, config)
		layerHistory = historyMap(resolved)

		blockWeight := averageWeight(weights)
		wholeBlock := ResolveWholeBlock(scene, frame, current, blockPrev, blockWeight, config)
		blockPrev = historyFromFrame(wholeBlock)
		blockWeightSum += blockWeight

		results := map[string]FrameResult{
			StrategyPerLayer:   perLayer,
			StrategyWholeBlock: wholeBlock,
			StrategyNoHistory:  current,
		}
		metrics := make(map[string]FrameMetrics, len(results))
		for name, result := range results {
			m := ComputeMetrics(result, reference, config)
			metrics[name] = m
			allMetrics[name] = append(allMetrics[name], m)
		}
		evaluation.Runs = append(evaluation.Runs, FrameRun{
			Frame: frame, Reference: reference, Results: results,
		})
		evaluation.FrameReport = append(evaluation.FrameReport, FrameReport{
			Frame: frame, Metrics: metrics,
		})
		for id, weight := range weights {
			weightSum[id] += weight
			weightCount[id]++
		}
	}

	for _, name := range []string{StrategyPerLayer, StrategyWholeBlock, StrategyNoHistory} {
		layerWeights := make(map[string]float64, len(weightSum))
		for id, sum := range weightSum {
			layerWeights[id] = sum / float64(weightCount[id])
		}
		summary := StrategySummary{
			Metrics:      averageMetrics(allMetrics[name]),
			LayerWeights: layerWeights,
		}
		if name == StrategyWholeBlock {
			summary.SharedWeight = blockWeightSum / float64(maxInt(1, config.Frames))
		}
		evaluation.Summary[name] = summary
	}
	return evaluation
}

func historyMap(layers []*resolvedLayer) map[string]*resolvedLayer {
	out := make(map[string]*resolvedLayer, len(layers))
	for _, layer := range layers {
		out[layer.id] = layer
	}
	return out
}
