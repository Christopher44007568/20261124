package analyze

import (
	"math"
	"sort"
)

type Distribution struct {
	Mean float64 `json:"mean"`
	RMS  float64 `json:"rms"`
	P95  float64 `json:"p95"`
	Max  float64 `json:"max"`
}

func Summarize(values []float64) Distribution {
	if len(values) == 0 {
		return Distribution{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	var sum, sumSq float64
	for _, v := range sorted {
		sum += v
		sumSq += v * v
	}
	n := float64(len(sorted))
	return Distribution{
		Mean: sum / n,
		RMS:  math.Sqrt(sumSq / n),
		P95:  percentile(sorted, 0.95),
		Max:  sorted[len(sorted)-1],
	}
}

func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if q <= 0 {
		return sorted[0]
	}
	if q >= 1 {
		return sorted[len(sorted)-1]
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	u := pos - float64(lo)
	return sorted[lo]*(1-u) + sorted[hi]*u
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
