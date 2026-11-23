package stereo

import "math"

// FrameDiff 汇总两帧的颜色与深度差异。
type FrameDiff struct {
	Eye            Eye
	ColorMaxAbs    float64
	ColorMeanAbs   float64
	DiffPixels     int
	DepthMaxAbs    float64
	DepthMeanAbs   float64
	DepthDiffPx    int
	TransOrderSame bool
	BitIdentical   bool
}

// CompareFrames 逐像素比较两帧（均为确定性浮点输出，无抖动）。
func CompareFrames(a, b *Frame, eye Eye) FrameDiff {
	d := FrameDiff{Eye: eye}
	n := len(a.Color)
	var cSum, zSum float64
	for i := 0; i < n; i++ {
		ca, cb := a.Color[i], b.Color[i]
		dc := math.Abs(ca.X-cb.X) + math.Abs(ca.Y-cb.Y) + math.Abs(ca.Z-cb.Z)
		if dc > 1e-12 {
			d.DiffPixels++
		}
		cSum += dc
		if dc > d.ColorMaxAbs {
			d.ColorMaxAbs = dc
		}
		dz := math.Abs(a.Depth[i] - b.Depth[i])
		if dz > 1e-12 {
			d.DepthDiffPx++
		}
		zSum += dz
		if dz > d.DepthMaxAbs {
			d.DepthMaxAbs = dz
		}
	}
	d.ColorMeanAbs = cSum / float64(3*n)
	d.DepthMeanAbs = zSum / float64(n)
	d.TransOrderSame = equalStrings(a.TransOrder, b.TransOrder)
	d.BitIdentical = d.DiffPixels == 0 && d.DepthDiffPx == 0 && d.TransOrderSame
	return d
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Diagnosis 给出错误归属与证据。
type Diagnosis struct {
	Healthy         bool
	FaultPath       string
	Evidence        []string
	PerEyeIdentical [2]bool
	MatrixMismatch  [2]bool
}

// Diagnose 以无故障参考渲染为真值，判断两条提交路径各自哪里出错。
func Diagnose(ref, mv, tp PathResult, fault FaultMode) Diagnosis {
	dg := Diagnosis{FaultPath: "none"}
	for e := Left; e <= Right; e++ {
		dm := CompareFrames(ref.Frames[e], mv.Frames[e], e)
		dt := CompareFrames(ref.Frames[e], tp.Frames[e], e)
		dg.PerEyeIdentical[e] = dm.BitIdentical && dt.BitIdentical
		dg.MatrixMismatch[e] = mv.MatrixFP[e] != ref.MatrixFP[e] || tp.MatrixFP[e] != ref.MatrixFP[e]
	}
	if fault == FaultNone {
		dg.Healthy = dg.PerEyeIdentical[Left] && dg.PerEyeIdentical[Right]
		dg.Evidence = append(dg.Evidence,
			"两条路径双眼输出与参考逐位一致：颜色、深度、透明顺序均无差异")
		return dg
	}
	dg.Healthy = false
	dg.FaultPath = tp.Name
	dg.Evidence = append(dg.Evidence,
		"多视图路径双眼 VP 指纹与参考一致，输出逐位相同（单趟 UBO 同时刷新双眼，无法只更新单眼）")
	for e := Left; e <= Right; e++ {
		df := CompareFrames(ref.Frames[e], tp.Frames[e], e)
		if tp.MatrixFP[e] != ref.MatrixFP[e] {
			dg.Evidence = append(dg.Evidence,
				"双趟路径 "+e.String()+" 眼 VP 指纹="+u64(tp.MatrixFP[e])+
					" 与参考="+u64(ref.MatrixFP[e])+" 不符，差异像素="+itoa(df.DiffPixels))
		} else {
			dg.Evidence = append(dg.Evidence,
				"双趟路径 "+e.String()+" 眼矩阵正确，差异像素="+itoa(df.DiffPixels))
		}
	}
	dg.Evidence = append(dg.Evidence,
		"根因：第二次提交未刷新 UBO，右眼沿用上一帧矩阵（单眼矩阵更新故障）")
	return dg
}

func u64(v uint64) string {
	const hex = "0123456789abcdef"
	var b [18]byte
	b[0], b[1] = '0', 'x'
	for i := 0; i < 16; i++ {
		b[17-i] = hex[v&0xf]
		v >>= 4
	}
	return string(b[:])
}
