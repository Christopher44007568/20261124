package ae

import "math"

const HistogramBins = 64

// Histogram is a fixed-size luminance histogram. Count and Sum are kept so
// merged histograms retain an exact mean instead of a bin-center approximation.
type Histogram struct {
	bins  [HistogramBins]uint64
	Count uint64
	Sum   float64
}

func (h *Histogram) Reset() {
	*h = Histogram{}
}

func (h *Histogram) Add(value float64) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	index := int(value * HistogramBins)
	if index >= HistogramBins {
		index = HistogramBins - 1
	}
	h.bins[index]++
	h.Count++
	h.Sum += value
}

func (h *Histogram) AddFrame(frame Frame) {
	for _, value := range frame.Luma {
		h.Add(value)
	}
}

func (h *Histogram) Merge(other Histogram) {
	for i := range h.bins {
		h.bins[i] += other.bins[i]
	}
	h.Count += other.Count
	h.Sum += other.Sum
}

func (h Histogram) Mean() float64 {
	if h.Count == 0 {
		return 0
	}
	return h.Sum / float64(h.Count)
}

// HighFraction returns the fraction of samples in bins at or above threshold.
func (h Histogram) HighFraction(threshold float64) float64 {
	if h.Count == 0 {
		return 0
	}
	start := int(math.Floor(threshold * HistogramBins))
	if start < 0 {
		start = 0
	}
	if start >= HistogramBins {
		return 0
	}
	var count uint64
	for i := start; i < HistogramBins; i++ {
		count += h.bins[i]
	}
	return float64(count) / float64(h.Count)
}

func (h Histogram) Bin(index int) uint64 {
	if index < 0 || index >= HistogramBins {
		return 0
	}
	return h.bins[index]
}
