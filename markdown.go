package reprojlab

import (
	"fmt"
	"strings"
)

func ReportMarkdown(rep ExperimentReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reprojection / distortion order experiment\n\n")
	fmt.Fprintf(&b, "Selected order: `%s`.\n\n", rep.SelectedOrder)
	fmt.Fprintf(&b, "Decision reason: %s.\n\n", rep.DecisionReason)
	fmt.Fprintf(&b, "The selection is based on color/depth error against a single-pass continuous composition, not on screenshot similarity or hole count alone. Coverage and resampling work are tie-breakers only.\n\n")
	fmt.Fprintf(&b, "## Scene and stress settings\n\n")
	fmt.Fprintf(&b, "- Scene: `%s`, %dx%d\n", rep.Scene, rep.Config.Width, rep.Config.Height)
	fmt.Fprintf(&b, "- Intrinsics: fx=%.3f fy=%.3f cx=%.3f cy=%.3f\n", rep.Config.Intrinsics.FX, rep.Config.Intrinsics.FY, rep.Config.Intrinsics.CX, rep.Config.Intrinsics.CY)
	fmt.Fprintf(&b, "- Distortion: k1=%.4f k2=%.4f k3=%.4f p1=%.4f p2=%.4f\n", rep.Config.Distortion.K1, rep.Config.Distortion.K2, rep.Config.Distortion.K3, rep.Config.Distortion.P1, rep.Config.Distortion.P2)
	fmt.Fprintf(&b, "- Translation: tx=%.4f ty=%.4f tz=%.4f\n", rep.Config.Translation.X, rep.Config.Translation.Y, rep.Config.Translation.Z)
	for _, o := range rep.Objects {
		fmt.Fprintf(&b, "- Object: %s\n", o)
	}
	fmt.Fprintf(&b, "\n## Order comparison\n\n")
	fmt.Fprintf(&b, "| order | selected | passes | source reads | writes | holes | hole %% | color RMSE | depth RMSE | probe failures |\n")
	fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, o := range rep.Orders {
		fmt.Fprintf(&b, "| `%s` | %v | %d | %d | %d | %d | %.3f | %.5f | %.5f | %d |\n",
			o.Order, o.Selected, o.Stats.Passes, o.Stats.SourceSamples, o.Stats.Writes,
			o.Stats.Holes, o.Stats.HolePercent, o.Reference.ColorRMSE, o.Reference.DepthRMSE, o.ProbeFailures)
	}
	for _, o := range rep.Orders {
		fmt.Fprintf(&b, "\n### `%s` stages\n\n", o.Order)
		fmt.Fprintf(&b, "| stage | source reads | writes | offscreen | occluded | input holes | output holes |\n")
		fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|\n")
		for _, s := range o.Stages {
			fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d |\n",
				s.Name, s.Stats.SourceSamples, s.Stats.Writes, s.Stats.Offscreen,
				s.Stats.Occluded, s.Stats.InputHoles, s.Stats.Holes)
		}
		fmt.Fprintf(&b, "\nDepth-coordinate probes:\n\n")
		fmt.Fprintf(&b, "| probe | x | y | source z | expected z | actual z | error |\n")
		fmt.Fprintf(&b, "|---|---:|---:|---:|---:|---:|---:|\n")
		for _, p := range o.Probes {
			fmt.Fprintf(&b, "| %s | %.2f | %.2f | %.4f | %.4f | %s | %s |\n",
				p.Name, p.OutputX, p.OutputY, p.SourceDepth, p.ExpectedDepth,
				depthString(p.ActualDepth), depthString(p.DepthError))
		}
	}
	writeVariantSummary(&b, rep)
	fmt.Fprintf(&b, "\n## Artifacts\n\n")
	fmt.Fprintf(&b, "`report.json` is the machine-readable record. PNG files are visual aids only: `source.png`, `reference.png`, both order outputs, low-resolution and stale-depth variants, `per-channel.png`, and depth maps.\n")
	return b.String()
}

func writeVariantSummary(b *strings.Builder, rep ExperimentReport) {
	fmt.Fprintf(b, "\n## Deliberate failure variants\n\n")
	fmt.Fprintf(b, "- Low-resolution distort then reproject: %d holes (%.3f%%), %d source reads, %d writes; depth RMSE against reference %.5f.\n",
		rep.LowResolution.Result.Stats.Holes, rep.LowResolution.Result.Stats.HolePercent,
		rep.LowResolution.Result.Stats.SourceSamples, rep.LowResolution.Result.Stats.Writes,
		rep.LowResolution.Reference.DepthRMSE)
	fmt.Fprintf(b, "- Per-channel distortion: holes R/G/B = %d/%d/%d, union = %d; right-edge blue shift %.4f px; center shift %.4f px.\n",
		rep.PerChannel.Holes[0], rep.PerChannel.Holes[1], rep.PerChannel.Holes[2],
		rep.PerChannel.UnionHoles, rep.PerChannel.EdgeShift, rep.PerChannel.CenterShift)
	fmt.Fprintf(b, "- Wrong depth reuse: color RMSE against correct B %.5f, depth RMSE %.5f, max depth error %.5f, probe failures %d.\n",
		rep.WrongDepth.CorrectB.ColorRMSE, rep.WrongDepth.CorrectB.DepthRMSE,
		rep.WrongDepth.CorrectB.MaxDepthError, rep.WrongDepth.ProbeFailures)
}

func depthString(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.5f", *v)
}
