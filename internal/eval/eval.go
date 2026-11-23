// Package eval runs the synthetic reconstruction matrix and applies quality
// guardrails that cannot be satisfied by improving the global average alone.
package eval

import (
	"fmt"
	"math"
	"sort"

	"fidelitycheck/internal/imaging"
	"fidelitycheck/internal/inpaint"
	"fidelitycheck/internal/metrics"
	"fidelitycheck/internal/scenes"
)

type Config struct {
	Width          int
	Height         int
	Frames         int
	RadiusPercents []float64
	Algorithms     []inpaint.Algorithm
	Baseline       string
	GlobalEpsilon  float64
	CenterEpsilon  float64
	FlickerEpsilon float64
	EdgeEpsilon    float64
}

func DefaultConfig() Config {
	return Config{
		Width:          192,
		Height:         144,
		Frames:         9,
		RadiusPercents: []float64{30, 45, 60},
		Algorithms:     inpaint.Defaults(),
		Baseline:       "mean",
		GlobalEpsilon:  0.0005,
		CenterEpsilon:  0.0005,
		FlickerEpsilon: 0.0005,
		EdgeEpsilon:    0.01,
	}
}

type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Guard struct {
	Baseline           string      `json:"baseline"`
	Compared           bool        `json:"compared"`
	GlobalGain         float64     `json:"global_gain"`
	CenterDelta        float64     `json:"center_delta"`
	CenterP95Delta     float64     `json:"center_p95_delta"`
	EdgeRetentionDelta float64     `json:"edge_retention_delta"`
	FlickerP95Delta    float64     `json:"flicker_p95_delta"`
	Passed             bool        `json:"passed"`
	Violations         []Violation `json:"violations,omitempty"`
}

type AlgoResult struct {
	Algorithm   string         `json:"algorithm"`
	Description string         `json:"description"`
	Metrics     metrics.Result `json:"metrics"`
	Guard       Guard          `json:"guard"`
}

type Case struct {
	Scene         string         `json:"scene"`
	RadiusPercent float64        `json:"radius_percent_of_half_min_side"`
	RadiusPixels  float64        `json:"radius_pixels"`
	Counts        metrics.Counts `json:"counts"`
	Results       []AlgoResult   `json:"results"`
	Passed        bool           `json:"passed"`
}

type Summary struct {
	Cases         int `json:"cases"`
	AlgorithmRuns int `json:"algorithm_runs"`
	FailedRuns    int `json:"failed_runs"`
	FramesPerCase int `json:"frames_per_case"`
}

type Report struct {
	Config  ConfigSummary `json:"config"`
	Summary Summary       `json:"summary"`
	Cases   []Case        `json:"cases"`
	Passed  bool          `json:"passed"`
}

type ConfigSummary struct {
	Width          int       `json:"width"`
	Height         int       `json:"height"`
	Frames         int       `json:"frames"`
	RadiusPercents []float64 `json:"radius_percents"`
	Baseline       string    `json:"baseline"`
	Algorithms     []string  `json:"algorithms"`
}

func Run(cfg Config) (*Report, error) {
	if err := validate(cfg); err != nil {
		return nil, err
	}
	cfg.RadiusPercents = sortedUnique(cfg.RadiusPercents)
	report := &Report{
		Config: ConfigSummary{
			Width:          cfg.Width,
			Height:         cfg.Height,
			Frames:         cfg.Frames,
			RadiusPercents: append([]float64(nil), cfg.RadiusPercents...),
			Baseline:       cfg.Baseline,
			Algorithms:     inpaint.Names(cfg.Algorithms),
		},
		Passed: true,
	}
	minDim := math.Min(float64(cfg.Width), float64(cfg.Height))

	for _, kind := range scenes.AllKinds {
		for _, percent := range cfg.RadiusPercents {
			radius := minDim * percent / 200
			seq, err := scenes.Build(scenes.Spec{
				Kind:       kind,
				Width:      cfg.Width,
				Height:     cfg.Height,
				MaskRadius: radius,
				Frames:     cfg.Frames,
			})
			if err != nil {
				return nil, fmt.Errorf("build %s at radius %.2f: %w", kind, radius, err)
			}
			caseResult, err := runCase(cfg, seq, percent, radius)
			if err != nil {
				return nil, err
			}
			if !caseResult.Passed {
				report.Passed = false
			}
			for _, result := range caseResult.Results {
				report.Summary.AlgorithmRuns++
				if !result.Guard.Passed {
					report.Summary.FailedRuns++
				}
			}
			report.Cases = append(report.Cases, caseResult)
		}
	}
	report.Summary.Cases = len(report.Cases)
	report.Summary.FramesPerCase = cfg.Frames
	return report, nil
}

func runCase(cfg Config, seq *scenes.Sequence, percent, radius float64) (Case, error) {
	result := Case{
		Scene:         string(seq.Kind),
		RadiusPercent: percent,
		RadiusPixels:  radius,
		Passed:        true,
	}
	corrupted := seq.Corrupted()

	for _, algorithm := range cfg.Algorithms {
		outputs := make([]*imaging.Frame, len(corrupted))
		for i, input := range corrupted {
			out, err := reconstruct(algorithm, input, seq.Mask)
			if err != nil {
				return Case{}, fmt.Errorf("%s / %s / frame %d: %w", seq.Kind, algorithm.Name, i, err)
			}
			outputs[i] = out
		}
		measured, counts, err := metrics.Compute(seq, outputs)
		if err != nil {
			return Case{}, fmt.Errorf("%s / %s: %w", seq.Kind, algorithm.Name, err)
		}
		if len(result.Results) == 0 {
			result.Counts = counts
		}
		result.Results = append(result.Results, AlgoResult{
			Algorithm:   algorithm.Name,
			Description: algorithm.Description,
			Metrics:     measured,
		})
	}

	baseline, ok := findResult(result.Results, cfg.Baseline)
	if !ok {
		return Case{}, fmt.Errorf("baseline %q has no result", cfg.Baseline)
	}
	for i := range result.Results {
		candidate := &result.Results[i]
		if candidate.Algorithm == cfg.Baseline {
			candidate.Guard = Guard{Baseline: cfg.Baseline, Passed: true}
		} else {
			candidate.Guard = guardAgainst(baseline, candidate.Metrics, cfg, seq.Kind)
		}
		if !candidate.Guard.Passed {
			result.Passed = false
		}
	}
	return result, nil
}

func reconstruct(algorithm inpaint.Algorithm, input *imaging.Frame, mask *imaging.Mask) (out *imaging.Frame, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("reconstructor %q panicked: %v", algorithm.Name, recovered)
		}
	}()
	out = algorithm.Reconstruct(input, mask)
	if out == nil {
		return nil, fmt.Errorf("reconstructor %q returned nil", algorithm.Name)
	}
	if out.W != input.W || out.H != input.H {
		return nil, fmt.Errorf("reconstructor %q returned %dx%d, want %dx%d", algorithm.Name, out.W, out.H, input.W, input.H)
	}
	return out, nil
}

func findResult(results []AlgoResult, name string) (AlgoResult, bool) {
	for _, result := range results {
		if result.Algorithm == name {
			return result, true
		}
	}
	return AlgoResult{}, false
}

func guardAgainst(baseline AlgoResult, candidate metrics.Result, cfg Config, kind scenes.Kind) Guard {
	guard := Guard{
		Baseline:           cfg.Baseline,
		Compared:           true,
		GlobalGain:         baseline.Metrics.GlobalMAE - candidate.GlobalMAE,
		CenterDelta:        candidate.CenterMAE - baseline.Metrics.CenterMAE,
		CenterP95Delta:     candidate.CenterP95 - baseline.Metrics.CenterP95,
		EdgeRetentionDelta: candidate.EdgeRetention - baseline.Metrics.EdgeRetention,
		FlickerP95Delta:    candidate.FlickerP95 - baseline.Metrics.FlickerP95,
		Passed:             true,
	}
	if guard.GlobalGain <= cfg.GlobalEpsilon {
		return guard
	}
	if guard.CenterDelta > cfg.CenterEpsilon || guard.CenterP95Delta > cfg.CenterEpsilon {
		guard.Violations = append(guard.Violations, Violation{
			Code: "center_regression",
			Message: fmt.Sprintf(
				"global MAE improves by %.6f but center mean/P95 deltas are %+.6f/%+.6f (positive is worse)",
				guard.GlobalGain, guard.CenterDelta, guard.CenterP95Delta,
			),
		})
	}
	if kind == scenes.ColoredEdges && guard.EdgeRetentionDelta < -cfg.EdgeEpsilon {
		guard.Violations = append(guard.Violations, Violation{
			Code: "edge_retention_regression",
			Message: fmt.Sprintf(
				"global MAE improves by %.6f but color-edge retention drops by %.4f",
				guard.GlobalGain, -guard.EdgeRetentionDelta,
			),
		})
	}
	if kind == scenes.SlowMotion && guard.FlickerP95Delta > cfg.FlickerEpsilon {
		guard.Violations = append(guard.Violations, Violation{
			Code: "temporal_flicker_regression",
			Message: fmt.Sprintf(
				"global MAE improves by %.6f but sequence flicker P95 worsens by %.6f",
				guard.GlobalGain, guard.FlickerP95Delta,
			),
		})
	}
	guard.Passed = len(guard.Violations) == 0
	return guard
}

func validate(cfg Config) error {
	if cfg.Width < 16 || cfg.Height < 16 {
		return fmt.Errorf("resolution must be at least 16x16")
	}
	if cfg.Frames < 3 {
		return fmt.Errorf("at least 3 frames are required to measure temporal stability")
	}
	if len(cfg.RadiusPercents) == 0 {
		return fmt.Errorf("at least one mask radius is required")
	}
	for _, percent := range cfg.RadiusPercents {
		if math.IsNaN(percent) || math.IsInf(percent, 0) || percent < 2 || percent > 95 {
			return fmt.Errorf("radius percent %.2f is outside [2, 95]", percent)
		}
	}
	if len(cfg.Algorithms) == 0 {
		return fmt.Errorf("at least one reconstructor is required")
	}
	seen := make(map[string]bool)
	for _, alg := range cfg.Algorithms {
		if alg.Name == "" || alg.Reconstruct == nil {
			return fmt.Errorf("reconstructor has an empty name or nil function")
		}
		if seen[alg.Name] {
			return fmt.Errorf("duplicate reconstructor %q", alg.Name)
		}
		seen[alg.Name] = true
	}
	if !seen[cfg.Baseline] {
		return fmt.Errorf("baseline %q is not in the reconstructor set", cfg.Baseline)
	}
	tolerances := []float64{cfg.GlobalEpsilon, cfg.CenterEpsilon, cfg.FlickerEpsilon, cfg.EdgeEpsilon}
	for _, tolerance := range tolerances {
		if math.IsNaN(tolerance) || math.IsInf(tolerance, 0) {
			return fmt.Errorf("tolerances must be finite")
		}
	}
	if cfg.GlobalEpsilon < 0 || cfg.CenterEpsilon < 0 || cfg.FlickerEpsilon < 0 || cfg.EdgeEpsilon < 0 {
		return fmt.Errorf("tolerances cannot be negative")
	}
	return nil
}

func sortedUnique(values []float64) []float64 {
	out := append([]float64(nil), values...)
	sort.Float64s(out)
	j := 0
	for _, v := range out {
		if j == 0 || v != out[j-1] {
			out[j] = v
			j++
		}
	}
	return out[:j]
}
