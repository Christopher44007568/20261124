package main

import (
	"fmt"
	"io"
	"strings"
)

func yesNo(b bool) string {
	if b {
		return "Y"
	}
	return "N"
}

func rectString(r Rect) string {
	if r.Empty() {
		return fmt.Sprintf("(%d,%d %dx%d EMPTY)", r.X, r.Y, r.W, r.H)
	}
	return fmt.Sprintf("(%d,%d %dx%d)", r.X, r.Y, r.W, r.H)
}

// WriteTraceTable 输出单个像素的完整状态序列。
func WriteTraceTable(w io.Writer, tr *PixelTrace) {
	fmt.Fprintln(w, "| step | op | note | clip logical | clip phys | scissor | stencil func/ref | stencil pass | s: before->after | color before->after |")
	fmt.Fprintln(w, "|---:|---|---|---|---|:---:|---|:---:|---|---|")
	for _, r := range tr.Recs {
		stencil := "-"
		if r.StencilEnabled {
			stencil = fmt.Sprintf("%s/%d", r.StencilFunc, r.StencilRef)
		}
		pass := "-"
		if r.StencilEnabled {
			pass = yesNo(r.StencilPass)
		}
		sChange := fmt.Sprintf("%d->%d", r.StencilBefore, r.StencilAfter)
		cChange := r.ColorBefore.Hex() + "->" + r.ColorAfter.Hex()
		fmt.Fprintf(w, "| %d | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			r.Step, r.OpKind, r.Comment,
			rectString(r.ClipLogical), rectString(r.ClipPhysical),
			yesNo(r.ScissorPass), stencil, pass, sChange, cChange)
	}
}

// WriteFrameCSV 逐像素转储最终颜色与模板，用于离线 diff。
func WriteFrameCSV(w io.Writer, fb *Framebuffer) {
	fmt.Fprintln(w, "x,y,stencil,color_hex")
	var sb strings.Builder
	for y := 0; y < fb.Height; y++ {
		for x := 0; x < fb.Width; x++ {
			c, s := fb.Pixel(x, y)
			sb.Reset()
			fmt.Fprintf(&sb, "%d,%d,%d,%s\n", x, y, s, c.Hex())
			io.WriteString(w, sb.String())
		}
	}
}
