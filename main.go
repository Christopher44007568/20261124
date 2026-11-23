package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	width  = 72
	height = 40
)

type check struct {
	name   string
	ok     bool
	detail string
}

func main() {
	outDir := "artifacts"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	panels := []*Panel{
		stripePanel(width, height),
		pentilePanel(width, height),
		monoPanel(width, height),
	}

	var report strings.Builder
	report.WriteString("# Subpixel coverage simulation\n\n")
	report.WriteString(fmt.Sprintf("Canvas: %dx%d logical pixels. Eye shifts: L=-0.5px, R=+0.5px; triangle PSF half-width X=%.2fpx Y=%.2fpx.\n\n",
		width, height, panels[0].psfX, panels[0].psfY))

	report.WriteString("## Emitters per logical pixel (distinct channels)\n\n")
	report.WriteString("panel,total_emitters,pixels_with_0ch,pixels_with_1ch,pixels_with_2ch,pixels_with_3ch\n")
	for _, p := range panels {
		hist := emitterMix(p)
		report.WriteString(fmt.Sprintf("%s,%d,%d,%d,%d,%d\n",
			p.name, len(p.emitters), hist[0], hist[1], hist[2], hist[3]))
	}

	report.WriteString("\n## Filter density (fraction of single-channel emitters)\n\n")
	report.WriteString("panel,red,green,blue,white,emitters_per_pixel\n")
	for _, p := range panels {
		d := filterDensity(p)
		report.WriteString(fmt.Sprintf("%s,%.3f,%.3f,%.3f,%.3f,%.3f\n",
			p.name, d[0], d[1], d[2], d[3],
			float64(len(p.emitters))/float64(width*height)))
	}

	var allMetrics []Metrics
	var checks []check

	for _, sc := range scenes {
		src := sc.make(width, height)
		for _, p := range panels {
			sim := runSimulation(p, src)
			m := measure(p, sim, sc.name, src)
			allMetrics = append(allMetrics, m)

			base := filepath.Join(outDir, sc.name+"__"+p.name)
			if err := renderPanel(p, sim).write(base + "__panel.ppm"); err != nil {
				fatal(err)
			}
			if err := renderEye(p, sim.left).write(base + "__eyeL.ppm"); err != nil {
				fatal(err)
			}
			if err := renderEye(p, sim.right).write(base + "__eyeR.ppm"); err != nil {
				fatal(err)
			}

			partial := countPartialEmitters(sim)
			report.WriteString(fmt.Sprintf(
				"\n## %s / %s\npartial_emitters=%d panel_peak_luma=%.4f panel_peak_RGB=(%.3f,%.3f,%.3f) active_emitters=%d\n",
				sc.name, p.name, partial, m.PanelPeakLuma,
				m.PanelPeakChannel[0], m.PanelPeakChannel[1], m.PanelPeakChannel[2],
				m.PanelActiveCount))
			report.WriteString(fmt.Sprintf(
				"eye_peak_luma=%.4f eye_mean_luma=%.4f crosstalk=%.4f binocular_MAD=%.5f binocular_max=%.4f edge_mid_MAD=%.5f\n",
				m.EyePeakLuma, m.EyeMeanLuma, m.Crosstalk, m.BinocularMAD, m.BinocularMaxDiff, m.EdgeMidMAD))
			report.WriteString(fmt.Sprintf("L_maxRGB=(%.3f,%.3f,%.3f) R_maxRGB=(%.3f,%.3f,%.3f)\n",
				m.LeftMaxRGB[0], m.LeftMaxRGB[1], m.LeftMaxRGB[2],
				m.RightMaxRGB[0], m.RightMaxRGB[1], m.RightMaxRGB[2]))

			if sc.name == "diagonal45" {
				checks = append(checks, check{
					name:   sc.name + "/" + p.name + " keeps sub-pixel partial coverage",
					ok:     partial > 0,
					detail: fmt.Sprintf("partial=%d", partial),
				})
			}
			if sc.name == "redline" && strings.HasPrefix(p.name, "rgb_stripe") {
				checks = append(checks, check{
					name:   "redline stripe panel drive reaches full red peak",
					ok:     m.PanelPeakChannel[0] > 0.99,
					detail: fmt.Sprintf("red_peak=%.4f", m.PanelPeakChannel[0]),
				})
			}
			if sc.name == "redline" && strings.HasPrefix(p.name, "pentile") {
				checks = append(checks, check{
					name: "redline pentile red channel is driven",
					ok:   m.PanelPeakChannel[0] > 0.99 && m.PanelPeakChannel[1] == 0,
					detail: fmt.Sprintf("R=%.3f G=%.3f B=%.3f",
						m.PanelPeakChannel[0], m.PanelPeakChannel[1], m.PanelPeakChannel[2]),
				})
			}
		}
	}

	// Structural guarantees independent of scene.
	for _, p := range panels {
		hist := emitterMix(p)
		if strings.HasPrefix(p.name, "pentile") {
			d := filterDensity(p)
			perPixel := float64(len(p.emitters)) / float64(width*height)
			checks = append(checks, check{
				name: "pentile uses single-channel emitters, 2 per pixel, half green",
				ok: approx(d[1], 0.5) && approx(perPixel, 2.0) &&
					approx(d[0], 0.25) && approx(d[2], 0.25),
				detail: fmt.Sprintf("R=%.3f G=%.3f B=%.3f per_pixel=%.3f",
					d[0], d[1], d[2], perPixel),
			})
		}
		if strings.HasPrefix(p.name, "mono") {
			checks = append(checks, check{
				name:   "mono panel has exactly one emitter per pixel",
				ok:     hist[1] == width*height && len(p.emitters) == width*height,
				detail: fmt.Sprintf("1ch=%d emitters=%d", hist[1], len(p.emitters)),
			})
		}
		if strings.HasPrefix(p.name, "rgb_stripe") {
			checks = append(checks, check{
				name:   "stripe panel provides exactly three channels per pixel",
				ok:     hist[3] == width*height,
				detail: fmt.Sprintf("3ch=%d", hist[3]),
			})
		}
	}

	// Binocular disparity must be phase-driven and nonzero on the sharp edge.
	for _, m := range allMetrics {
		if m.Scene == "edge" {
			checks = append(checks, check{
				name:   "edge binocular phase difference exists: " + m.Panel,
				ok:     m.EdgeMidMAD > 0,
				detail: fmt.Sprintf("edge_mid_MAD=%.5f", m.EdgeMidMAD),
			})
		}
		if m.Scene == "redline" {
			checks = append(checks, check{
				name:   "redline chromatic crosstalk stays sub-unity: " + m.Panel,
				ok:     m.Mono || m.Crosstalk < 1,
				detail: fmt.Sprintf("crosstalk=%.4f", m.Crosstalk),
			})
		}
	}

	report.WriteString("\n## Self checks\n")
	failed := 0
	for _, c := range checks {
		status := "PASS"
		if !c.ok {
			status = "FAIL"
			failed++
		}
		report.WriteString(fmt.Sprintf("\n- [%s] %s (%s)", status, c.name, c.detail))
	}
	report.WriteString("\n")

	reportPath := filepath.Join(outDir, "report.txt")
	if err := os.WriteFile(reportPath, []byte(report.String()), 0o644); err != nil {
		fatal(err)
	}
	fmt.Print(report.String())
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "\n%d check(s) failed\n", failed)
		os.Exit(1)
	}
	fmt.Printf("\nWrote images and %s\n", reportPath)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func approx(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}
