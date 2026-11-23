package harness

import (
	"stereoval/internal/geo"
	"stereoval/internal/render"
)

// FramePair exposes one rendered stereo pair for external inspection such as
// the PGM debug dump.
type FramePair struct {
	Index       int
	Head        geo.HeadSample
	LeftGT      *render.Frame
	RightGT     *render.Frame
	LeftWarped  *render.WarpResult
	RightWarped *render.WarpResult
}

// Export converts internal frames into the debug view.
func Export(frames []stereoFrame) []FramePair {
	out := make([]FramePair, len(frames))
	for i, f := range frames {
		out[i] = FramePair{
			Index:       i,
			Head:        f.head,
			LeftGT:      f.left.gt,
			RightGT:     f.right.gt,
			LeftWarped:  f.left.warped,
			RightWarped: f.right.warped,
		}
	}
	return out
}
