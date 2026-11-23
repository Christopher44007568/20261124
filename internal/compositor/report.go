package compositor

import (
	"fmt"
	"strings"
)

func formatRange(depths []float64) string {
	r := summarizeDepths(depths)
	if r == nil {
		return "none"
	}
	if r.Min == r.Max {
		return fmt.Sprintf("%.4f m", r.Min)
	}
	return fmt.Sprintf("%.4f-%.4f m", r.Min, r.Max)
}

func layerGlyph(kind SurfaceKind) byte {
	switch kind {
	case SurfaceHand:
		return 'H'
	case SurfacePanel:
		return 'P'
	case SurfaceNotification:
		return 'N'
	default:
		return '?'
	}
}

func resultGlyph(result Result, decision PixelDecision) byte {
	if decision.Winner == "" {
		return '.'
	}
	for _, layer := range resultSceneLayers(result) {
		if layer.ID == decision.Winner {
			glyph := layerGlyph(layer.Kind)
			if decision.Ambiguous {
				if glyph >= 'A' && glyph <= 'Z' {
					glyph += 'a' - 'A'
				}
			}
			return glyph
		}
	}
	return '?'
}

// resultSceneLayers is intentionally small: Result carries enough layout for
// reports, but renderers can also use the scene directly where available.
func resultSceneLayers(result Result) []Layer {
	return result.layers
}

func writeSummaryTable(b *strings.Builder, comparison Comparison) {
	b.WriteString("| submission | mode | pixels | correct | wrong occlusions | order fallbacks | incomplete-depth pixels | invalid samples | quantization ambiguities |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, result := range comparison.Submissions {
		s := result.Summary
		fmt.Fprintf(b, "| %s | %s | %d | %d | %d | %d | %d | %d | %d |\n",
			result.Submission, result.Mode, s.Pixels, s.CorrectPixels,
			s.WrongOcclusions, s.OrderFallbacks, s.IncompleteDepthPixels,
			s.InvalidDepthSamples, s.QuantizationAmbiguous)
	}
}

func writeSceneTable(b *strings.Builder, scene Scene) {
	b.WriteString("| layer | kind | system | fallback order | coverage | left depth | right depth |\n")
	b.WriteString("|---|---|:---:|---:|---:|---:|---:|\n")
	for _, layer := range scene.Layers {
		covered := 0
		for _, value := range layer.Coverage {
			if value {
				covered++
			}
		}
		fmt.Fprintf(b, "| %s | %s | %t | %d | %d | %s | %s |\n",
			layer.ID, layer.Kind, layer.System, layer.Order, covered,
			formatRange(layer.Left), formatRange(layer.Right))
	}
}

func writeCoverageGrids(b *strings.Builder, result Result) {
	fmt.Fprintf(b, "### %s (%s)\n\n", result.Submission, result.Mode)
	for _, eyeResult := range result.Eyes {
		fmt.Fprintf(b, "%s eye:\n\n```text\n", eyeResult.Eye)
		for y := 0; y < result.Height; y++ {
			for x := 0; x < result.Width; x++ {
				for _, decision := range eyeResult.Decisions {
					if decision.X == x && decision.Y == y {
						b.WriteByte(resultGlyph(result, decision))
						break
					}
				}
			}
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}
}

func writeErrors(b *strings.Builder, comparison Comparison) {
	errors := make([]OcclusionError, 0)
	names := make([]string, 0)
	for _, result := range comparison.Submissions {
		for _, eyeResult := range result.Eyes {
			for _, err := range eyeResult.Errors {
				errors = append(errors, err)
				names = append(names, result.Submission)
			}
		}
	}
	if len(errors) == 0 {
		b.WriteString("No occlusion errors were recorded for the compared submissions.\n")
		return
	}
	b.WriteString("| submission | eye | x | y | expected | actual | kind |\n")
	b.WriteString("|---|---|---:|---:|---|---|---|\n")
	limit := len(errors)
	if limit > 20 {
		limit = 20
	}
	for i := 0; i < limit; i++ {
		err := errors[i]
		fmt.Fprintf(b, "| %s | %s | %d | %d | %s | %s | %s |\n",
			names[i], err.Eye, err.X, err.Y, err.Expected, err.Actual, err.Kind)
	}
	if len(errors) > limit {
		fmt.Fprintf(b, "\n%d additional errors are available in JSON output.\n", len(errors)-limit)
	}
}

func writeDiagnostics(b *strings.Builder, diagnostics []Diagnostic) {
	b.WriteString("| case | safe errors | negative control | control errors | detail |\n")
	b.WriteString("|---|---:|---|---:|---|\n")
	for _, diag := range diagnostics {
		badName := "none"
		badErrors := "-"
		if diag.Bad != nil {
			badName = diag.Bad.Policy
			badErrors = fmt.Sprintf("%d", diag.Bad.Summary.WrongOcclusions)
		}
		fmt.Fprintf(b, "| %s | %d | %s | %s | %s |\n",
			diag.Name, diag.Safe.Summary.WrongOcclusions, badName, badErrors, diag.Detail)
	}
}

// RenderMarkdown produces a compact report. JSON output carries every pixel
// decision, including the depth observed for each covering candidate.
func RenderMarkdown(comparison Comparison) string {
	var b strings.Builder
	b.WriteString("# Compositor occlusion comparison\n\n")
	fmt.Fprintf(&b, "Scene `%s`: %d x %d, evaluated independently for left and right eyes.\n\n",
		comparison.Scene.Name, comparison.Scene.Width, comparison.Scene.Height)
	b.WriteString("The safe policy uses nearest valid depth for the current eye. If any covering layer has missing/invalid depth, it falls back to the explicit back-to-front order and marks the pixel ambiguous. A system flag never changes ordering.\n\n")

	b.WriteString("## Scene layers\n\n")
	writeSceneTable(&b, comparison.Scene)
	b.WriteString("\n## Submission summary\n\n")
	writeSummaryTable(&b, comparison)
	b.WriteString("\n`order fallbacks` includes no-depth submissions and incomplete-depth pixels. `quantization ambiguities` counts pixels where candidate depth intervals overlap after quantization.\n\n")

	b.WriteString("## Coverage decisions\n\n")
	b.WriteString("Uppercase means a decisive result; lowercase means the result used an ambiguous fallback. `H` hand, `P` panel, `N` system notification, `.` background.\n\n")
	for _, result := range comparison.Submissions {
		writeCoverageGrids(&b, result)
	}

	b.WriteString("## Occlusion errors\n\n")
	writeErrors(&b, comparison)

	b.WriteString("\n## Edge cases and negative controls\n\n")
	writeDiagnostics(&b, comparison.Diagnostics)
	b.WriteString("\nThe negative controls are not production policies. They demonstrate the errors caused by always promoting system layers, treating invalid samples as usable depth, or reusing left-eye depth for the right eye.\n")
	return b.String()
}
