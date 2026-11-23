package stereo

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

var IPDValues = []float64{55, 64, 72}

var probeSpecs = []struct {
	Name  string
	Point Vec3
}{
	{Name: "front_plate", Point: Vec3{X: -10, Y: -5, Z: FrontPlateZ}},
	{Name: "back_plate", Point: Vec3{X: 200, Y: 40, Z: BackPlateZ}},
	{Name: "sphere_front", Point: Vec3{X: SphereCX, Y: SphereCY, Z: SphereCZ - SphereR}},
	{Name: "ruler_400mm", Point: Vec3{X: 400, Y: -195, Z: WallZ}},
}

type runData struct {
	report RunReport
	left   Frame
	right  Frame
	depth  StereoDepth
}

func Run(outDir string) (Report, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Report{}, err
	}
	report := Report{
		Config: ReportConfig{
			ImageWidth:      ImageWidth,
			ImageHeight:     ImageHeight,
			FocalPixels:     FocalPixels,
			PrincipalX:      PrincipalX,
			PrincipalY:      PrincipalY,
			IPDValuesMM:     append([]float64(nil), IPDValues...),
			BrokenReference: BrokenRefZ,
			MatchingWindow:  5,
		},
	}
	for _, ipd := range IPDValues {
		data, err := renderPair(outDir, ipd, Synchronized, fmt.Sprintf("ipd%.0f", ipd))
		if err != nil {
			return Report{}, err
		}
		report.Runs = append(report.Runs, data.report)
	}
	broken, err := renderPair(outDir, 72, ViewportOnly, "broken")
	if err != nil {
		return Report{}, err
	}
	report.Broken = broken.report
	report.FailureChecks = buildFailureChecks(report.Runs[len(report.Runs)-1], broken.report)
	if err := SaveJSON(filepath.Join(outDir, "report.json"), report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func renderPair(outDir string, ipd float64, variant Variant, prefix string) (runData, error) {
	leftCam := NewCamera(ipd, LeftEye, variant)
	rightCam := NewCamera(ipd, RightEye, variant)
	left := Render(leftCam)
	right := Render(rightCam)
	depth := RecoverDepth(left.Image, right.Image, ipd)

	leftName := prefix + "_L.png"
	rightName := prefix + "_R.png"
	depthName := prefix + "_depth.png"
	if err := SavePNG(filepath.Join(outDir, leftName), left.Image); err != nil {
		return runData{}, err
	}
	if err := SavePNG(filepath.Join(outDir, rightName), right.Image); err != nil {
		return runData{}, err
	}
	if err := SaveDepthPNG(filepath.Join(outDir, depthName), depth.Depth); err != nil {
		return runData{}, err
	}

	run := RunReport{
		Label:      prefix,
		Variant:    variant.Name(),
		IPDMM:      ipd,
		LeftEyeMM:  leftCam.EyeCenter().X,
		RightEyeMM: rightCam.EyeCenter().X,
		Images: ImageFiles{
			Left:  leftName,
			Right: rightName,
			Depth: depthName,
		},
		Match:             matchStats(left, right, depth),
		Probes:            probeResults(leftCam, depth),
		Occlusion:         occlusionStats(left, right),
		OutOfBoundsPixels: left.OOBCount + right.OOBCount,
		CenterRayErrorMM: math.Max(
			leftCam.CenterRayOriginError(),
			rightCam.CenterRayOriginError(),
		),
		ProjectionOffsets: ProjectionOffsets{
			LeftAtFrontPlatePx:  leftCam.ProjectionOffsetAt(FrontPlateZ),
			RightAtFrontPlatePx: rightCam.ProjectionOffsetAt(FrontPlateZ),
			LeftAtWallPx:        leftCam.ProjectionOffsetAt(WallZ),
			RightAtWallPx:       rightCam.ProjectionOffsetAt(WallZ),
		},
	}
	return runData{report: run, left: left, right: right, depth: depth}, nil
}

func matchStats(left, right Frame, depth StereoDepth) MatchStats {
	stats := MatchStats{
		TotalPixels:   ImageWidth * ImageHeight,
		ValidPixels:   depth.ValidCount,
		ValidRatio:    float64(depth.ValidCount) / float64(ImageWidth*ImageHeight),
		MeanBlockCost: depth.MeanMatchCost,
	}
	var errors []float64
	var squared float64
	var within int
	for y := 0; y < ImageHeight; y++ {
		for x := 0; x < ImageWidth; x++ {
			idx := y*ImageWidth + x
			if !depth.Valid[idx] {
				continue
			}
			xr := int(math.Round(float64(x) - depth.Disparity[idx]))
			if xr < 0 || xr >= ImageWidth {
				continue
			}
			ridx := y*ImageWidth + xr
			if left.Surface[idx] != right.Surface[ridx] {
				continue
			}
			analytic := left.Depth[idx]
			if math.IsInf(analytic, 0) || analytic <= 0 {
				continue
			}
			err := math.Abs(depth.Depth[idx] - analytic)
			errors = append(errors, err)
			squared += err * err
			if err/analytic <= 0.05 {
				within++
			}
		}
	}
	stats.ComparedPixels = len(errors)
	if len(errors) == 0 {
		return stats
	}
	sort.Float64s(errors)
	var sum float64
	for _, v := range errors {
		sum += v
	}
	stats.MeanAbsDepthErrorMM = sum / float64(len(errors))
	stats.RMSEMM = math.Sqrt(squared / float64(len(errors)))
	stats.MedianAbsDepthErrorMM = quantile(errors, 0.5)
	stats.P95AbsDepthErrorMM = quantile(errors, 0.95)
	stats.WithinFivePercent = float64(within) / float64(len(errors))
	return stats
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	pos := q * float64(len(sorted)-1)
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return sorted[lo]
	}
	return mix(sorted[lo], sorted[hi], pos-float64(lo))
}

func occlusionStats(left, right Frame) OcclusionStats {
	stats := OcclusionStats{}
	for idx := range left.Surface {
		if left.Surface[idx] == SurfaceBackPlate {
			stats.BackPixelsLeft++
		}
		if right.Surface[idx] == SurfaceBackPlate {
			stats.BackPixelsRight++
		}
	}
	stats.BackVisibleDelta = absInt(stats.BackPixelsLeft - stats.BackPixelsRight)
	return stats
}
