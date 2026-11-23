package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func main() {
	d := DefaultDistortion()
	outDir := "artifacts"

	var results []Metrics

	t0 := time.Now()
	uniform := BuildUniform(d, 16)
	results = append(results, Evaluate("uniform-16x16", uniform, d, 0,
		ms(t0), 400))

	t0 = time.Now()
	radius := BuildRadius(d, 16, 0.45)
	results = append(results, Evaluate("radius-warp-a0.45", radius, d, 0,
		ms(t0), 400))

	t0 = time.Now()
	adaptive, st := BuildAdaptive(d, 4, 1e-3, 7)
	results = append(results, Evaluate(
		fmt.Sprintf("adaptive-tol1e-3(depth<=%d)", st.MaxDepth), adaptive, d,
		st.TJunctions, ms(t0), 400))

	for i, m := range []*Mesh{uniform, radius, adaptive} {
		name := []string{"uniform", "radius", "adaptive"}[i]
		if err := writeVerts(filepath.Join(outDir, name+"_vertices.csv"), m); err != nil {
			fmt.Fprintln(os.Stderr, "write verts:", err)
		}
	}

	fmt.Println("畸变后处理网格策略对比（采样 400x400，误差单位：归一化 UV / texel@1024）")
	fmt.Println()
	hdr := []string{"strategy", "verts", "tris", "max_err", "max_err_texel",
		"holes", "t_junctions", "boundary_ok", "build_ms"}
	rows := [][]string{hdr}
	for _, r := range results {
		rows = append(rows, []string{
			r.Name,
			strconv.Itoa(r.Verts),
			strconv.Itoa(r.Tris),
			fmt.Sprintf("%.6f", r.MaxErr),
			fmt.Sprintf("%.3f", r.MaxErrTexel),
			strconv.Itoa(r.Holes),
			strconv.Itoa(r.TJunctions),
			strconv.FormatBool(r.BoundaryOK),
			fmt.Sprintf("%.3f", r.BuildMs),
		})
	}
	printTable(rows)

	f, err := os.Create(filepath.Join(outDir, "summary.csv"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "write summary:", err)
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.WriteAll(rows)
	fmt.Println("\n顶点位置与汇总已写入 artifacts/")
}

func ms(t time.Time) float64 {
	return float64(time.Since(t).Nanoseconds()) / 1e6
}

func writeVerts(path string, m *Mesh) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"x", "y", "u", "v"})
	for _, v := range m.Verts {
		w.Write([]string{
			strconv.FormatFloat(v.X, 'g', 10, 64),
			strconv.FormatFloat(v.Y, 'g', 10, 64),
			strconv.FormatFloat(v.U, 'g', 10, 64),
			strconv.FormatFloat(v.V, 'g', 10, 64),
		})
	}
	return w.Error()
}

func printTable(rows [][]string) {
	w := make([]int, len(rows[0]))
	for _, r := range rows {
		for i, c := range r {
			if len(c) > w[i] {
				w[i] = len(c)
			}
		}
	}
	for _, r := range rows {
		for i, c := range r {
			fmt.Printf("%-*s  ", w[i], c)
		}
		fmt.Println()
	}
}
