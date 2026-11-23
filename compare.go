package main

import (
	"fmt"
	"math"
)

// Mismatch kinds between the lightweight draw list and raster coverage.
const (
	AgreeVisible     = "一致-可见"
	AgreeHidden      = "一致-不可见"
	MismatchOccluded = "失配-遮挡/亚像素"
	MismatchMiss     = "失配-漏画(BUG)"
)

// EyeCompare is the per-instance, per-eye comparison verdict.
type EyeCompare struct {
	InstName string
	Eye      string
	Kind     string
	Coverage int
	Note     string
}

// screenBBox estimates the NDC-space bbox of an instance for one eye,
// using only corners in front of the near plane. ok=false if none qualify.
// onScreen reports whether the bbox intersects the NDC cube [-1,1]^2.
func screenBBox(cam Camera, b BoundInstance) (area float64, onScreen, ok bool) {
	f := 1 / math.Tan(cam.FovY/2)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range b.Corners() {
		cp := cam.View.MulPoint(p)
		if cp.Z > -cam.Near {
			continue
		}
		nx := f / cam.Aspect * cp.X / -cp.Z
		ny := f * cp.Y / -cp.Z
		minX, maxX = minf(minX, nx), maxf(maxX, nx)
		minY, maxY = minf(minY, ny), maxf(maxY, ny)
		ok = true
	}
	if !ok {
		return 0, false, false
	}
	intersects := maxX >= -1 && minX <= 1 && maxY >= -1 && minY <= 1
	return (maxX - minX) * (maxY - minY), intersects, true
}

// CompareEye matches the lightweight draw list against raster coverage
// for one eye and returns per-instance verdicts.
func CompareEye(cam Camera, eye int, records []Record, bound []BoundInstance, coverage map[int]int) []EyeCompare {
	byID := map[int]BoundInstance{}
	for _, b := range bound {
		byID[b.Inst.ID] = b
	}
	var out []EyeCompare
	for _, r := range records {
		culled := r.Evidence[eye].Visible
		cov := coverage[r.Inst.ID]
		vc := EyeCompare{InstName: r.Inst.Name, Eye: eyeNames[eye], Coverage: cov}
		switch {
		case culled && cov > 0:
			vc.Kind = AgreeVisible
		case !culled && cov == 0:
			vc.Kind = AgreeHidden
		case culled && cov == 0:
			vc.Kind = MismatchOccluded
			area, onScreen, ok := screenBBox(cam, byID[r.Inst.ID])
			switch {
			case !ok:
				vc.Note = "整体位于近平面前,光栅无覆盖"
			case !onScreen:
				vc.Note = "投影在屏幕外,保守剔除假阳性"
			case area < 0.002:
				vc.Note = "投影面积极小,疑似擦边/亚像素"
			default:
				vc.Note = "投影可见但被其他实例遮挡"
			}
		default:
			vc.Kind = MismatchMiss
			vc.Note = "剔除判定不可见但光栅有覆盖,剔除器过于激进"
		}
		out = append(out, vc)
	}
	return out
}

// Summarize counts verdicts by kind.
func Summarize(vcs []EyeCompare) map[string]int {
	m := map[string]int{}
	for _, v := range vcs {
		m[v.Kind]++
	}
	return m
}

func (v EyeCompare) String() string {
	s := fmt.Sprintf("    [%s] %-22s %s 覆盖=%d", v.Eye, v.InstName, v.Kind, v.Coverage)
	if v.Note != "" {
		s += " — " + v.Note
	}
	return s
}
