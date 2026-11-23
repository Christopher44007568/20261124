package stereo

import (
	"fmt"
	"math"
	"sort"
)

type localSample struct {
	depth float64
	x     float64
	y     float64
}

func probeResults(camera Camera, depth StereoDepth) []ProbeResult {
	results := make([]ProbeResult, 0, len(probeSpecs))
	for _, spec := range probeSpecs {
		u, v, ok := camera.Project(spec.Point)
		if !ok {
			results = append(results, ProbeResult{Name: spec.Name, AuthoredPoint: spec.Point})
			continue
		}
		px := clampInt(int(math.Round(u-0.5)), 0, ImageWidth-1)
		py := clampInt(int(math.Round(v-0.5)), 0, ImageHeight-1)
		ray, _, _ := camera.RayAt(float64(px)+0.5, float64(py)+0.5)
		hit, hitOK := IntersectScene(ray)
		probe := ProbeResult{
			Name:          spec.Name,
			AuthoredPoint: spec.Point,
			LeftPixel:     Pixel{X: px, Y: py},
		}
		if !hitOK {
			results = append(results, probe)
			continue
		}
		probe.AnalyticHitPoint = hit.Point
		probe.ExpectedDisparity = FocalPixels * camera.IPD / hit.Point.Z
		sample, valid := localDepth(depth, px, py, 5)
		if !valid {
			sample, valid = localDepth(depth, px, py, 10)
		}
		if !valid {
			results = append(results, probe)
			continue
		}
		recovered := Vec3{X: sample.x, Y: sample.y, Z: sample.depth}
		probe.RecoveredPoint = recovered
		probe.ObservedDisparity = FocalPixels * camera.IPD / recovered.Z
		probe.DepthErrorMM = recovered.Z - hit.Point.Z
		probe.PositionErrorMM = recovered.Sub(hit.Point).Length()
		probe.Valid = true
		results = append(results, probe)
	}
	return results
}

func localDepth(depth StereoDepth, px, py, radius int) (localSample, bool) {
	var samples []localSample
	for y := py - radius; y <= py+radius; y++ {
		if y < 0 || y >= ImageHeight {
			continue
		}
		for x := px - radius; x <= px+radius; x++ {
			if x < 0 || x >= ImageWidth {
				continue
			}
			idx := y*ImageWidth + x
			if !depth.Valid[idx] || math.IsInf(depth.Depth[idx], 0) {
				continue
			}
			samples = append(samples, localSample{
				depth: depth.Depth[idx],
				x:     depth.WorldX[idx],
				y:     depth.WorldY[idx],
			})
		}
	}
	if len(samples) == 0 {
		return localSample{}, false
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].depth < samples[j].depth })
	median := samples[len(samples)/2]
	best := samples[0]
	bestDistance := math.Abs(best.depth - median.depth)
	for _, sample := range samples[1:] {
		if d := math.Abs(sample.depth - median.depth); d < bestDistance {
			best = sample
			bestDistance = d
		}
	}
	best.depth = median.depth
	return best, true
}

func buildFailureChecks(correct, broken RunReport) []FailureCheck {
	correctFront := findProbe(correct, "front_plate")
	correctBack := findProbe(correct, "back_plate")
	brokenFront := findProbe(broken, "front_plate")
	brokenBack := findProbe(broken, "back_plate")
	correctOrder := depthOrderOK(correctFront, correctBack)
	brokenOrder := depthOrderOK(brokenFront, brokenBack)

	correctCenter := correct.CenterRayErrorMM < 0.01
	brokenCenterDetected := broken.CenterRayErrorMM >= 1
	correctOOB := correct.OutOfBoundsPixels == 0
	brokenOOBDetected := broken.OutOfBoundsPixels > 0
	correctOcclusion := correct.Occlusion.BackVisibleDelta > 0
	brokenOcclusionDetected := broken.Occlusion.BackVisibleDelta == 0

	return []FailureCheck{
		{
			Name:           "depth_order",
			CorrectPass:    correctOrder,
			BrokenDetected: !brokenOrder,
			CorrectDetail: fmt.Sprintf(
				"front %.1f mm < back %.1f mm",
				probeDepth(correctFront), probeDepth(correctBack),
			),
			BrokenDetail: fmt.Sprintf(
				"front %.1f mm, back %.1f mm (depth order collapsed)",
				probeDepth(brokenFront), probeDepth(brokenBack),
			),
		},
		{
			Name:           "center_ray",
			CorrectPass:    correctCenter,
			BrokenDetected: brokenCenterDetected,
			CorrectDetail:  fmt.Sprintf("principal-ray origin error %.3f mm", correct.CenterRayErrorMM),
			BrokenDetail:   fmt.Sprintf("principal-ray origin error %.3f mm", broken.CenterRayErrorMM),
		},
		{
			Name:           "out_of_bounds_pixels",
			CorrectPass:    correctOOB,
			BrokenDetected: brokenOOBDetected,
			CorrectDetail:  fmt.Sprintf("%d out-of-view samples", correct.OutOfBoundsPixels),
			BrokenDetail:   fmt.Sprintf("%d magenta out-of-view samples", broken.OutOfBoundsPixels),
		},
		{
			Name:           "occlusion_change",
			CorrectPass:    correctOcclusion,
			BrokenDetected: brokenOcclusionDetected,
			CorrectDetail: fmt.Sprintf(
				"back-plate visibility delta %d px",
				correct.Occlusion.BackVisibleDelta,
			),
			BrokenDetail: fmt.Sprintf(
				"back-plate visibility delta %d px (no viewpoint-dependent occlusion)",
				broken.Occlusion.BackVisibleDelta,
			),
		},
	}
}

func findProbe(run RunReport, name string) ProbeResult {
	for _, probe := range run.Probes {
		if probe.Name == name {
			return probe
		}
	}
	return ProbeResult{Name: name}
}

func probeDepth(probe ProbeResult) float64 {
	if !probe.Valid {
		return math.NaN()
	}
	return probe.RecoveredPoint.Z
}

func depthOrderOK(front, back ProbeResult) bool {
	return front.Valid && back.Valid &&
		front.RecoveredPoint.Z+20 < back.RecoveredPoint.Z
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
