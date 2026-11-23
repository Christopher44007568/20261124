package stereoholes

import (
	"math"
)

type ModeMetrics struct {
	Name             string         `json:"name"`
	Holes            int            `json:"holes"`
	Filled           int            `json:"filled"`
	FillRate         float64        `json:"fill_rate"`
	MeanError        float64        `json:"mean_error"`
	FilledMeanError  float64        `json:"filled_mean_error"`
	BadFilled        int            `json:"bad_filled"`
	GhostRate        float64        `json:"ghost_rate"`
	RejectedByReason map[string]int `json:"rejected_by_reason,omitempty"`
}

type GroupMetrics struct {
	Name  string        `json:"name"`
	Rect  [4]int        `json:"rect"`
	Holes int           `json:"holes"`
	Modes []ModeMetrics `json:"modes"`
}

type Report struct {
	Width          int            `json:"width"`
	Height         int            `json:"height"`
	Baseline       float64        `json:"baseline"`
	MeanDisparity  float64        `json:"mean_disparity_px"`
	GhostThreshold float64        `json:"ghost_threshold"`
	Global         []ModeMetrics  `json:"global"`
	Groups         []GroupMetrics `json:"groups"`
}

type NamedFill struct {
	Name   string
	Result FillResult
}

func colorError(a, b Vec3) float64 {
	dr := a.X - b.X
	dg := a.Y - b.Y
	db := a.Z - b.Z
	return math.Sqrt(dr*dr+dg*dg+db*db) / math.Sqrt(3)
}

func measure(ref *Frame, result FillResult, mask []bool, ghostThreshold float64) ModeMetrics {
	m := ModeMetrics{Name: "", RejectedByReason: make(map[string]int)}
	errSum, filledErrSum := 0.0, 0.0
	for i, selected := range mask {
		if !selected {
			continue
		}
		m.Holes++
		err := colorError(result.RGB[i], ref.RGB[i])
		errSum += err
		if result.Status[i] == StatusFilled {
			m.Filled++
			filledErrSum += err
			if err > ghostThreshold {
				m.BadFilled++
			}
		} else if result.Status[i] == StatusRejected {
			if result.Reason[i] != ReasonNone {
				m.RejectedByReason[string(result.Reason[i])]++
			}
		}
	}
	if m.Holes > 0 {
		m.FillRate = float64(m.Filled) / float64(m.Holes)
		m.MeanError = errSum / float64(m.Holes)
	}
	if m.Filled > 0 {
		m.FilledMeanError = filledErrSum / float64(m.Filled)
		m.GhostRate = float64(m.BadFilled) / float64(m.Filled)
	}
	return m
}

func BuildReport(ref *Frame, holes *HoleSet, baseline, meanDisparity float64, fills []NamedFill) Report {
	const ghostThreshold = 0.08
	rep := Report{
		Width: ref.Width, Height: ref.Height, Baseline: baseline,
		MeanDisparity: meanDisparity, GhostThreshold: ghostThreshold,
	}
	for _, f := range fills {
		gm := measure(ref, f.Result, holes.Mask, ghostThreshold)
		gm.Name = f.Name
		rep.Global = append(rep.Global, gm)
	}
	for _, g := range holes.Groups {
		gm := GroupMetrics{
			Name: g.Name,
			Rect: [4]int{g.Rect.Min.X, g.Rect.Min.Y, g.Rect.Dx(), g.Rect.Dy()},
		}
		for _, f := range fills {
			m := measure(ref, f.Result, g.Mask, ghostThreshold)
			m.Name = f.Name
			if f.Name == fills[0].Name {
				gm.Holes = m.Holes
			}
			gm.Modes = append(gm.Modes, m)
		}
		rep.Groups = append(rep.Groups, gm)
	}
	return rep
}

// ReferenceFill exposes the complete target-eye render in the same form as
// the candidate methods.
func ReferenceFill(ref *Frame, holes []bool) FillResult {
	res := newFillResult(ref)
	for i, isHole := range holes {
		if !isHole {
			continue
		}
		res.Stats.HolePixels++
		res.Stats.Filled++
		res.Status[i] = StatusFilled
	}
	return res
}
