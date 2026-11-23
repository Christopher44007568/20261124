package metrics

import "math"

// GhostStats measures double-images: strong-edge pixels in the warped frame
// that are valid (not holes) on both images yet remain intensity-inconsistent
// with the ground truth. Hole borders and ordinary sub-pixel resampling are
// excluded so the metric reflects genuine duplicated/shifted structure.
type GhostStats struct {
	GhostRatio     float64 // ghost edge pixels / all edge pixels
	FillGhostRatio float64 // same ratio restricted to cross-eye filled pixels
}

// Ghosting computes the ghost ratio. fill may be nil when no fill was used.
func Ghosting(warped, gt []float64, w, h int, fill []bool) GhostStats {
	wm := EdgeMask(warped, w, h)
	nEdges, nGhost := 0, 0
	nFillEdges, nFillGhost := 0, 0
	for i := range wm {
		if !wm[i] {
			continue
		}
		nEdges++
		if fill != nil && fill[i] {
			nFillEdges++
		}
		a, b := warped[i], gt[i]
		if math.IsNaN(a) || math.IsNaN(b) {
			continue
		}
		// A real ghost is a large, structured mismatch; small residuals are
		// expected from bilinear resampling along high-frequency checker edges.
		if math.Abs(a-b) > 70 {
			nGhost++
			if fill != nil && fill[i] {
				nFillGhost++
			}
		}
	}
	out := GhostStats{}
	if nEdges > 0 {
		out.GhostRatio = float64(nGhost) / float64(nEdges)
	}
	if nFillEdges > 0 {
		out.FillGhostRatio = float64(nFillGhost) / float64(nFillEdges)
	}
	return out
}

// EyeReport is one eye's per-frame measurement.
type EyeReport struct {
	Eye            string
	PoseTag        string
	ReprojMean     float64
	ReprojRMS      float64
	ReprojMax      float64
	MarkersFound   int
	MarkersTotal   int
	HoleRatio      float64
	FillRatio      float64
	EdgeDirected   float64
	EdgeSymmetric  float64
	GhostRatio     float64
	FillGhostRatio float64
}

// StereoReport is the frame-level cross-eye measurement.
type StereoReport struct {
	DispErrRMS       float64
	DispErrMax       float64
	DispVertRMS      float64
	DispChangeRMS    float64 // |dObs(t)-dObs(t-1) - (dExp(t)-dExp(t-1))|
	DispChangeMax    float64
	FillContaminated int // marker observations lying on cross-eye fill
	MatchedMarkers   int
}

// FrameReport bundles everything recorded for one timestamp.
type FrameReport struct {
	Frame  int
	Left   EyeReport
	Right  EyeReport
	Stereo StereoReport
}

// Aggregate is the run-level summary over all frames.
type Aggregate struct {
	Frames           int
	Left             EyeAgg
	Right            EyeAgg
	PooledReprojRMS  float64
	PooledReprojMean float64
	DispErrRMS       float64
	DispErrMax       float64
	DispErrP95       float64
	DispChangeRMS    float64
	DispChangeMax    float64
	DispChangeP95    float64
}

// EyeAgg aggregates per-frame eye reports.
type EyeAgg struct {
	ReprojMean    float64
	ReprojRMS     float64
	ReprojMax     float64
	ReprojP95     float64
	HoleMean      float64
	FillMean      float64
	EdgeMean      float64
	GhostMean     float64
	FillGhostMean float64
	FoundRate     float64
}
