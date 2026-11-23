package mvquant

// Result 是一组“向量场 × 编码器”实验的结果。
type Result struct {
	Kind    FieldKind
	Encoder string
	Metrics Metrics
}

func newEncoders() []Encoder {
	return []Encoder{
		TruncEncoder{},
		RoundEncoder{},
		NewDiffusionEncoder(GridRows, GridCols),
	}
}

// RunExperiment 对指定向量场依次运行三种编码器并返回指标。
func RunExperiment(kind FieldKind, frames int) ([]Result, error) {
	truth := BuildSequence(kind, frames)
	results := make([]Result, 0, 3)
	for _, encoder := range newEncoders() {
		codes := make([][][][2]int32, frames)
		for frame := range truth {
			encoded, err := encoder.Encode(truth[frame])
			if err != nil {
				return nil, err
			}
			codes[frame] = encoded
		}
		decoded := make([][][]Vec2, frames)
		for frame := range codes {
			decoded[frame] = Decode(codes[frame])
		}
		results = append(results, Result{
			Kind:    kind,
			Encoder: encoder.Name(),
			Metrics: Evaluate(truth, decoded, codes),
		})
	}
	return results, nil
}

// AllKinds 返回全部四类向量场。
func AllKinds() []FieldKind {
	return []FieldKind{KindTranslation, KindRotation, KindScale, KindStatic}
}
