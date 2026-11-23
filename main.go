package main

import (
	"fmt"
	"os"
	"path/filepath"

	"stereocompare/stereo"
)

const (
	width  = 160
	height = 120
)

type runCase struct {
	dir   string
	fault stereo.FaultMode
	title string
}

func main() {
	out := "artifacts/out"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	sc := stereo.BuildScene(width, height)
	prev := stereo.StaleMatrices(sc)

	cases := []runCase{
		{filepath.Join(out, "healthy"), stereo.FaultNone, "健康基线：无故障"},
		{filepath.Join(out, "fault-single-eye-matrix"), stereo.FaultSingleEyeMatrix, "故障：仅更新单眼矩阵"},
	}
	var reports []string
	for _, rc := range cases {
		rep, err := runOne(sc, prev, rc)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		reports = append(reports, rep)
	}

	index := filepath.Join(out, "REPORT.md")
	if err := os.WriteFile(index, []byte(joinReports(reports)), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("报告已写入:", index)
}

func joinReports(parts []string) string {
	head := "# 单趟多视图 vs 双独立提交：渲染等价性对比报告\n\n"
	all := head
	for _, p := range parts {
		all += p + "\n---\n\n"
	}
	return all
}
