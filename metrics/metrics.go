// Package metrics compares a warped result with a ground-truth target render.
package metrics

import (
	"image/color"
	"math"

	"timewarp/scene"
	"timewarp/warp"
)

// Summary is one method's score for one motion case.
type Summary struct {
	Method          string          `json:"method"`
	Pixels          int             `json:"pixels"`
	Valid           int             `json:"valid"`
	Rejected        int             `json:"rejected"`
	Coverage        float64         `json:"coverage"`
	MAE             float64         `json:"mae"`
	RMSE            float64         `json:"rmse"`
	BadPixelRate    float64         `json:"bad_pixel_rate"`
	OutOfBounds     int             `json:"out_of_bounds"`
	NoCoverage      int             `json:"no_coverage"`
	BehindCamera    int             `json:"behind_camera"`
	SourceNoDepth   int             `json:"source_no_depth"`
	SourceOffscreen int             `json:"source_offscreen"`
	Regions         []RegionSummary `json:"regions"`
}

// RegionSummary isolates the room, near pole, and static specular glints.
type RegionSummary struct {
	Name         string  `json:"name"`
	Pixels       int     `json:"pixels"`
	Valid        int     `json:"valid"`
	Coverage     float64 `json:"coverage"`
	MAE          float64 `json:"mae"`
	RMSE         float64 `json:"rmse"`
	BadPixelRate float64 `json:"bad_pixel_rate"`
}

// Compute scores out against gt. Rejected pixels count against coverage but
// are excluded from color-error averages; the rejection mask exposes them.
func Compute(method string, out *warp.Result, gt *scene.Frame) Summary {
	s := Summary{
		Method:          method,
		Pixels:          out.W * out.H,
		OutOfBounds:     out.Stats.OutOfBounds,
		NoCoverage:      out.Stats.NoCoverage,
		BehindCamera:    out.Stats.BehindCamera,
		SourceNoDepth:   out.Stats.SourceNoDepth,
		SourceOffscreen: out.Stats.SourceOffscreen,
	}
	for i := range gt.Pix {
		if i >= len(out.Reject) || out.Reject[i] != warp.RejectNone {
			s.Rejected++
		} else {
			s.Valid++
		}
	}
	if s.Pixels > 0 {
		s.Coverage = float64(s.Valid) / float64(s.Pixels)
	}

	var absSum, sqSum float64
	bad := 0
	n := 0
	for i := range gt.Pix {
		if i >= len(out.Pix) || out.Reject[i] != warp.RejectNone {
			continue
		}
		d0, d1, d2 := diff(gt.Pix[i], out.Pix[i])
		mean := (d0 + d1 + d2) / 3
		absSum += mean
		sqSum += (d0*d0 + d1*d1 + d2*d2) / 3
		if max3(d0, d1, d2) > 12.0/255.0 {
			bad++
		}
		n++
	}
	if n > 0 {
		s.MAE = absSum / float64(n) * 255
		s.RMSE = math.Sqrt(sqSum/float64(n)) * 255
		s.BadPixelRate = float64(bad) / float64(n)
	}

	regions := []scene.Region{scene.RegionRoom, scene.RegionPole, scene.RegionSpecular}
	for _, region := range regions {
		rs := SummaryRegion(out, gt, region)
		if rs.Pixels > 0 {
			s.Regions = append(s.Regions, rs)
		}
	}
	return s
}

func SummaryRegion(out *warp.Result, gt *scene.Frame, region scene.Region) RegionSummary {
	rs := RegionSummary{Name: region.String()}
	var absSum, sqSum float64
	bad, n := 0, 0
	for i := range gt.Pix {
		if gt.Region[i] != region {
			continue
		}
		rs.Pixels++
		if i >= len(out.Reject) || out.Reject[i] != warp.RejectNone {
			continue
		}
		rs.Valid++
		d0, d1, d2 := diff(gt.Pix[i], out.Pix[i])
		mean := (d0 + d1 + d2) / 3
		absSum += mean
		sqSum += (d0*d0 + d1*d1 + d2*d2) / 3
		if max3(d0, d1, d2) > 12.0/255.0 {
			bad++
		}
		n++
	}
	if rs.Pixels > 0 {
		rs.Coverage = float64(rs.Valid) / float64(rs.Pixels)
	}
	if n > 0 {
		rs.MAE = absSum / float64(n) * 255
		rs.RMSE = math.Sqrt(sqSum/float64(n)) * 255
		rs.BadPixelRate = float64(bad) / float64(n)
	}
	return rs
}

func diff(a, b color.RGBA) (float64, float64, float64) {
	f := func(x, y uint8) float64 {
		return math.Abs(float64(x)-float64(y)) / 255
	}
	return f(a.R, b.R), f(a.G, b.G), f(a.B, b.B)
}

func max3(a, b, c float64) float64 {
	return math.Max(a, math.Max(b, c))
}
