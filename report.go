package main

import (
	"fmt"
	"strings"

	"stereocompare/stereo"
)

func buildReport(rc runCase, ref, mv, tp stereo.PathResult,
	dg stereo.Diagnosis, cov stereo.Coverage) string {
	var b strings.Builder
	totalPx := sc2px(ref)
	fmt.Fprintf(&b, "## %s\n\n", rc.title)

	b.WriteString("### 颜色与深度差异（多视图 vs 双趟）\n\n")
	b.WriteString("| 眼 | 颜色最大差 | 颜色均值 | 差异像素 | 深度最大差 | 深度均值 | 透明顺序一致 | 逐位一致 |\n")
	b.WriteString("|---|---|---|---|---|---|---|---|\n")
	for _, e := range []stereo.Eye{stereo.Left, stereo.Right} {
		d := stereo.CompareFrames(mv.Frames[e], tp.Frames[e], e)
		fmt.Fprintf(&b, "| %s | %.6f | %.2e | %d (%s) | %.6f | %.2e | %v | %v |\n",
			e.String(), d.ColorMaxAbs, d.ColorMeanAbs, d.DiffPixels, pct(d.DiffPixels, totalPx),
			d.DepthMaxAbs, d.DepthMeanAbs, d.TransOrderSame, d.BitIdentical)
	}

	b.WriteString("\n### 提交批次与内存\n\n")
	b.WriteString("| 指标 | 单趟多视图 | 双独立提交 |\n|---|---|---|\n")
	fmt.Fprintf(&b, "| 提交次数 submissions | %d | %d |\n", mv.Counters.Submissions, tp.Counters.Submissions)
	fmt.Fprintf(&b, "| draw calls | %d | %d |\n", mv.Counters.DrawCalls, tp.Counters.DrawCalls)
	fmt.Fprintf(&b, "| 视口/视图绑定 | %d | %d |\n", mv.Counters.ViewBindings, tp.Counters.ViewBindings)
	fmt.Fprintf(&b, "| uniform 写入次数 | %d | %d |\n", mv.Counters.UniformUpdates, tp.Counters.UniformUpdates)
	fmt.Fprintf(&b, "| 透明队列排序次数 | %d | %d |\n", mv.Counters.TransparentSorts, tp.Counters.TransparentSorts)
	fmt.Fprintf(&b, "| 裁剪后三角形 L/R | %d/%d | %d/%d |\n",
		mv.Counters.ClippedTris[0], mv.Counters.ClippedTris[1],
		tp.Counters.ClippedTris[0], tp.Counters.ClippedTris[1])
	fmt.Fprintf(&b, "| 总内存估算 | %s | %s |\n", kb(mv.Counters.MemoryBytes), kb(tp.Counters.MemoryBytes))
	b.WriteString("\n内存明细（字节）：\n\n| 资源 | 多视图 | 双趟 |\n|---|---|---|\n")
	for _, k := range []string{"shared-geometry", "view-framebuffers", "command-buffer", "ubo-view-matrices", "view-state"} {
		fmt.Fprintf(&b, "| %s | %d | %d |\n", k,
			mv.Counters.MemoryBreakdown[k], tp.Counters.MemoryBreakdown[k])
	}

	b.WriteString("\n### 矩阵指纹（定位单眼矩阵故障）\n\n")
	b.WriteString("| 眼 | 参考 VP | 多视图 VP | 双趟 VP | 双趟是否偏离 |\n|---|---|---|---|---|\n")
	for _, e := range []stereo.Eye{stereo.Left, stereo.Right} {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %v |\n", e.String(),
			fp(ref.MatrixFP[e]), fp(mv.MatrixFP[e]), fp(tp.MatrixFP[e]),
			tp.MatrixFP[e] != ref.MatrixFP[e])
	}

	fmt.Fprintf(&b, "\n### 错误归属\n\n- 健康：%v\n- 故障路径：`%s`\n", dg.Healthy, dg.FaultPath)
	for _, ev := range dg.Evidence {
		fmt.Fprintf(&b, "- %s\n", ev)
	}

	b.WriteString("\n### 需求覆盖证据\n\n")
	fmt.Fprintf(&b, "- 跨眼不同裁剪：左眼裁剪后三角形 %d 个，右眼 %d 个（非对称视锥导致侧面裁剪差异）\n",
		cov.ClipCounts[0], cov.ClipCounts[1])
	fmt.Fprintf(&b, "- 非对称投影 frustum (l,r,b,t,n,f)：L=%v\n  R=%v\n",
		frustumFmt(cov.AsymmetricFrustum[0]), frustumFmt(cov.AsymmetricFrustum[1]))
	fmt.Fprintf(&b, "- 共享实例：%d 个 *Mesh 在两条路径间零复制共享，几何内存两条路径相等\n", cov.SharedMeshCount)
	fmt.Fprintf(&b, "- 透明混合顺序（画家算法，远→近）：\n  - L: %v\n  - R: %v\n",
		cov.TransOrderL, cov.TransOrderR)

	b.WriteString("\n### 图像产物\n\n")
	for _, e := range []string{"L", "R"} {
		fmt.Fprintf(&b, "- 眼 %s：`ref_%s.png` `multiview_%s.png` `twopass_%s.png` `depth_ref_%s.png` `diff_mv_vs_tp_%s.png` `diff_ref_vs_tp_%s.png`\n",
			e, e, e, e, e, e, e)
	}
	return b.String()
}

func sc2px(r stereo.PathResult) int {
	return r.Frames[0].Width * r.Frames[0].Height
}

func kb(n int64) string { return fmt.Sprintf("%d B (%.2f KiB)", n, float64(n)/1024) }

func fp(v uint64) string { return fmt.Sprintf("0x%016x", v) }

func frustumFmt(f [6]float64) string {
	return fmt.Sprintf("[%.3f %.3f %.3f %.3f %.1f %.1f]", f[0], f[1], f[2], f[3], f[4], f[5])
}
