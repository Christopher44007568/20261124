package main

import (
	"fmt"
	"strings"

	"offaxis/pkg/offaxis"
)

const planeLegend = "L=left R=right B=bottom T=top N=near F=far"

func printMatrix(title string, m offaxis.Mat4) {
	fmt.Printf("%s\n", title)
	for r := 0; r < 4; r++ {
		fmt.Printf("  | %9.4f %9.4f %9.4f %11.4f |\n",
			m[r][0], m[r][1], m[r][2], m[r][3])
	}
}

func printBounds(b offaxis.Bounds) {
	fmt.Printf("bounds: L=%g R=%g B=%g T=%g near=%g far=%g\n",
		b.Left, b.Right, b.Bottom, b.Top, b.Near, b.Far)
	fmt.Printf("off-axis tx=%.4f ty=%.4f (zero only when symmetric)\n",
		(b.Right+b.Left)/(b.Right-b.Left),
		(b.Top+b.Bottom)/(b.Top-b.Bottom))
}

func maskName(mask uint8) string {
	if mask == 0 {
		return "-"
	}
	names := []string{"L", "R", "B", "T", "N", "F"}
	var parts []string
	for i, n := range names {
		if mask&(1<<uint(i)) != 0 {
			parts = append(parts, n)
		}
	}
	return strings.Join(parts, "+")
}

// printProbeGrid renders the three-way comparison as one ASCII cell per
// probe: glyph shows analytic inside/outside/boundary, and the annotation
// line reports the edge-mask equality and depth reconciliation.
func printProbeGrid(probes []offaxis.Probe, cols int) {
	rows := len(probes) / cols
	fmt.Println("analytic classification (.: inside, o: outside, =: boundary):")
	for r := 0; r < rows; r++ {
		var line strings.Builder
		line.WriteString("  ")
		for c := 0; c < cols; c++ {
			p := probes[r*cols+c]
			switch {
			case p.Boundary != 0:
				line.WriteByte('=')
			case p.Inside:
				line.WriteByte('.')
			default:
				line.WriteByte('o')
			}
			line.WriteByte(' ')
		}
		fmt.Println(line.String())
	}
	fmt.Println("edge markers (same glyph set, sourced from clip-space masks):")
	for r := 0; r < rows; r++ {
		var line strings.Builder
		line.WriteString("  ")
		for c := 0; c < cols; c++ {
			p := probes[r*cols+c]
			switch {
			case p.EdgeMask != 0:
				line.WriteByte('o')
			case p.Boundary != 0:
				line.WriteByte('=')
			default:
				line.WriteByte('.')
			}
			line.WriteByte(' ')
		}
		fmt.Println(line.String())
	}
}

func printDepthGrid(probes []offaxis.Probe, cols int) {
	rows := len(probes) / cols
	fmt.Println("window depth (digit = tenths of [0,1], space = outside):")
	for r := 0; r < rows; r++ {
		var line strings.Builder
		line.WriteString("  ")
		for c := 0; c < cols; c++ {
			p := probes[r*cols+c]
			if !p.Inside {
				line.WriteByte(' ')
			} else {
				d := int(p.WindowDepth*10) % 10
				line.WriteByte(byte('0' + d))
			}
			line.WriteByte(' ')
		}
		fmt.Println(line.String())
	}
}
