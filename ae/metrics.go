package ae

import "math"

const (
	DefaultConvergenceToleranceEV = 0.06
	defaultSettleWindow           = 5
)

// AnalyzePerformance measures convergence from the final scene transition. A
// trace is settled when every remaining sample lies within tolerance of the
// final EV. Overshoot is the farthest excursion beyond that final EV in the
// direction of the correction; PeakExcursionEV includes undershoot as well.
func AnalyzePerformance(samples []Sample, transition int) Performance {
	result := Performance{ConvergenceFrames: -1}
	if transition <= 0 || transition >= len(samples) {
		return result
	}
	result.StartEV = samples[transition-1].EV

	window := defaultSettleWindow
	if window > len(samples) {
		window = len(samples)
	}
	var sum float64
	for _, sample := range samples[len(samples)-window:] {
		sum += sample.EV
	}
	result.FinalEV = sum / float64(window)

	direction := result.FinalEV - result.StartEV
	if math.Abs(direction) < 0.02 {
		direction = samples[transition].EV - result.StartEV
	}
	maxEV, minEV := samples[transition].EV, samples[transition].EV
	for i := transition; i < len(samples); i++ {
		ev := samples[i].EV
		if ev > maxEV {
			maxEV = ev
		}
		if ev < minEV {
			minEV = ev
		}
		deviation := math.Abs(ev - result.FinalEV)
		if deviation > result.PeakExcursionEV {
			result.PeakExcursionEV = deviation
		}
	}
	firstStable := stableSuffixStart(samples, transition, result.FinalEV)
	if firstStable >= 0 {
		result.Settled = true
		result.ConvergenceFrames = firstStable - transition + 1
	}

	if direction > 0 {
		result.OvershootEV = math.Max(0, maxEV-result.FinalEV)
	} else if direction < 0 {
		result.OvershootEV = math.Max(0, result.FinalEV-minEV)
	}
	denominator := math.Abs(result.FinalEV - result.StartEV)
	if denominator > 1e-6 {
		result.OvershootPercent = 100 * result.OvershootEV / denominator
	}
	return result
}

func stableSuffixStart(samples []Sample, transition int, finalEV float64) int {
	start := -1
	for i := len(samples) - 1; i >= transition; i-- {
		if math.Abs(samples[i].EV-finalEV) > DefaultConvergenceToleranceEV {
			break
		}
		start = i
	}
	return start
}
