package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := filepath.Join("artifacts", "reports")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	scenarios := BuildScenarios()
	summary, err := os.Create(filepath.Join(outDir, "summary.txt"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer summary.Close()

	fmt.Printf("stencil/scissor 联动复现：%d 个场景\n", len(scenarios))
	for _, sc := range scenarios {
		runScenario(sc, outDir, summary)
	}
	fmt.Printf("报告目录: %s\n", outDir)
}

func runScenario(sc Scenario, outDir string, summary *os.File) {
	refCtx := ReferenceCtx(sc.Ctx.DPI)
	refFB, refTr := Render(Frame{Ctx: refCtx, Ops: sc.Ops}, Bugs{})
	bugFB, bugTr := Render(sc.Frame, sc.Bugs)

	mism := Compare(refFB, bugFB, refTr, bugTr)
	total := CountMismatches(refFB, bugFB)

	reportPath := filepath.Join(outDir, sc.Name+".md")
	f, err := os.Create(reportPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Fprintf(f, "# %s — %s\n\n", sc.Name, sc.Bugs.Label())
	fmt.Fprintf(f, "环境: %s, DPI=%d(prev=%d), 窗口=(%d,%d) prev=(%d,%d), 物理尺寸=%dx%d\n\n",
		sc.Ctx.Name, sc.Ctx.DPI, sc.Ctx.PrevDPI,
		sc.Ctx.WinX, sc.Ctx.WinY, sc.Ctx.PrevWinX, sc.Ctx.PrevWinY,
		sc.Ctx.PhysW(), sc.Ctx.PhysH())

	fmt.Fprintln(f, "## 参考实现帧缓冲")
	DumpASCII(f, refFB)
	fmt.Fprintln(f, "\n## 缺陷实现帧缓冲")
	DumpASCII(f, bugFB)

	if mism == nil {
		fmt.Fprintln(f, "\n未发现错误像素。")
	} else {
		fmt.Fprintf(f, "\n## 第一个错误像素\n\n")
		fmt.Fprintf(f, "物理坐标 (%d,%d)，错误像素总数 %d\n", mism.X, mism.Y, total)
		fmt.Fprintf(f, "参考: 颜色 %s 模板 %d；缺陷: 颜色 %s 模板 %d\n\n",
			mism.ColorRef.Hex(), mism.StencilRef, mism.ColorBug.Hex(), mism.StencilBug)
		fmt.Fprintln(f, "### 参考实现状态序列")
		WriteTraceTable(f, mism.RefTrace)
		fmt.Fprintln(f, "\n### 缺陷实现状态序列")
		WriteTraceTable(f, mism.BugTrace)
	}

	csvPath := filepath.Join(outDir, sc.Name+"_bug.csv")
	cf, err := os.Create(csvPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	WriteFrameCSV(cf, bugFB)
	cf.Close()

	head := "无错误像素"
	if mism != nil {
		head = fmt.Sprintf("首个错误像素 (%d,%d) ref=%s/s%d bug=%s/s%d，共 %d 个错误像素",
			mism.X, mism.Y, mism.ColorRef.Hex(), mism.StencilRef,
			mism.ColorBug.Hex(), mism.StencilBug, total)
	}
	line := fmt.Sprintf("%-26s %-28s %s\n", sc.Name, sc.Bugs.Label(), head)
	fmt.Print(line)
	fmt.Fprint(summary, line)
}
