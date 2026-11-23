package main

import (
	"fmt"
	"strings"

	"posebench/internal/analyze"
)

func formatTable(report analyze.Report, direction string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "posebench v1 | %dx%d camera | readout %.2f ms | scan %s | %d frames/period | %d segments\n",
		report.Camera.Width, report.Camera.Height, report.ReadoutMS, direction,
		report.FramesPerPeriod, report.Segments)
	fmt.Fprintf(&b, "boundary budget: %.2f deg / %.2f px\n", report.JumpBudgetDeg, report.JumpBudgetPx)
	for _, scenario := range report.Scenarios {
		fmt.Fprintf(&b, "\n[%s] %s\n", scenario.Name, scenario.Description)
		fmt.Fprintf(&b, "  peak angular speed %.0f deg/s\n", scenario.PeakAngularSpeedDegPerSec)
		fmt.Fprintf(&b, "  image shear: top-bottom %.2f px avg / %.2f px max, angle %.2f/%.2f deg, landmark RMS %.2f/%.2f px, affine |b|=%.4f |c|=%.4f\n",
			scenario.Shear.MeanShiftPx, scenario.Shear.MaxShiftPx,
			scenario.Shear.MeanAngleDeg, scenario.Shear.MaxAngleDeg,
			scenario.Shear.MeanLandmarkRMS, scenario.Shear.MaxLandmarkRMS,
			scenario.Shear.MeanAffineShearBX, scenario.Shear.MeanAffineShearCY)
		fmt.Fprintf(&b, "  %-20s %5s  %-25s %-15s %8s %8s %8s %13s %s\n",
			"strategy", "poses", "orientation err deg m/r/p95/max",
			"trans err mm m/max", "rowstep", "bnd raw deg", "bnd raw px", "bnd resid d/p", "status")
		for _, s := range scenario.Strategies {
			fmt.Fprintf(&b, "  %-20s %5d  %6.3f/%6.3f/%6.3f/%6.3f  %6.2f/%6.2f    %8.3f %8.3f %8.3f %6.3f/%-6.3f %s\n",
				s.Name,
				s.PosesPerFrame,
				s.ErrorDeg.Mean, s.ErrorDeg.RMS, s.ErrorDeg.P95, s.ErrorDeg.Max,
				s.ErrorMM.Mean, s.ErrorMM.Max,
				s.MaxRowStepDeg,
				s.BoundaryStepDeg.Max,
				s.BoundaryStepPx.Max,
				s.BoundaryResidualDeg,
				s.BoundaryResidualPx,
				status(s),
			)
		}
	}
	return b.String()
}

func status(s analyze.StrategySummary) string {
	switch {
	case !s.Deployable && s.Kind == "uniform-start":
		return "FORBIDDEN"
	case !s.Deployable && !s.BoundaryOK:
		return "JUMP/DIAG"
	case !s.Deployable:
		return "DIAGNOSTIC"
	case !s.BoundaryOK:
		return "JUMP"
	default:
		return "ok"
	}
}
