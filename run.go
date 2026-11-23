package main

import (
	"fmt"
	"path/filepath"

	"stereocompare/stereo"
)

func runOne(sc *stereo.Scene, prev [2]stereo.Mat4, rc runCase) (string, error) {
	ref := stereo.RenderReference(sc)
	mv := stereo.RenderMultiview(sc, rc.fault, prev)
	tp := stereo.RenderTwoPass(sc, rc.fault, prev)
	dg := stereo.Diagnose(ref, mv, tp, rc.fault)
	cov := stereo.CollectCoverage(sc, ref)

	if err := saveImages(rc.dir, ref, mv, tp); err != nil {
		return "", err
	}
	return buildReport(rc, ref, mv, tp, dg, cov), nil
}

func saveImages(dir string, ref, mv, tp stereo.PathResult) error {
	for _, e := range []stereo.Eye{stereo.Left, stereo.Right} {
		eye := e.String()
		if err := stereo.SaveColorPNG(ref.Frames[e], filepath.Join(dir, "ref_"+eye+".png")); err != nil {
			return err
		}
		if err := stereo.SaveColorPNG(mv.Frames[e], filepath.Join(dir, "multiview_"+eye+".png")); err != nil {
			return err
		}
		if err := stereo.SaveColorPNG(tp.Frames[e], filepath.Join(dir, "twopass_"+eye+".png")); err != nil {
			return err
		}
		if err := stereo.SaveDepthPNG(ref.Frames[e], filepath.Join(dir, "depth_ref_"+eye+".png")); err != nil {
			return err
		}
		if err := stereo.SaveDiffPNG(mv.Frames[e], tp.Frames[e], 8,
			filepath.Join(dir, "diff_mv_vs_tp_"+eye+".png")); err != nil {
			return err
		}
		if err := stereo.SaveDiffPNG(ref.Frames[e], tp.Frames[e], 8,
			filepath.Join(dir, "diff_ref_vs_tp_"+eye+".png")); err != nil {
			return err
		}
	}
	return nil
}

func pct(n, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%.2f%%", 100*float64(n)/float64(total))
}
