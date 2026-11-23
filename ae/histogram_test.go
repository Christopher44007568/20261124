package ae

import (
	"math"
	"testing"
)

func TestHistogramMeanAndHighFraction(t *testing.T) {
	var hist Histogram
	hist.Add(0.10)
	hist.Add(0.50)
	hist.Add(0.95)
	hist.Add(1.00)

	if got := hist.Mean(); math.Abs(got-0.6375) > 1e-9 {
		t.Fatalf("mean = %.9f, want 0.6375", got)
	}
	if got := hist.HighFraction(0.95); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("high fraction = %.9f, want 0.5", got)
	}
	if got := hist.Bin(HistogramBins - 1); got != 1 {
		t.Fatalf("last bin = %d, want 1", got)
	}

	hist.Reset()
	if hist.Count != 0 || hist.Sum != 0 || hist.Mean() != 0 {
		t.Fatalf("reset histogram is not empty: %+v", hist)
	}
}

func TestHistogramMergePreservesMean(t *testing.T) {
	var first, second, merged Histogram
	first.Add(0.2)
	second.Add(0.6)
	second.Add(0.8)
	merged.Merge(first)
	merged.Merge(second)

	want := (0.2 + 0.6 + 0.8) / 3
	if got := merged.Mean(); math.Abs(got-want) > 1e-9 {
		t.Fatalf("merged mean = %.9f, want %.9f", got, want)
	}
	if merged.Count != 3 {
		t.Fatalf("merged count = %d, want 3", merged.Count)
	}
}
