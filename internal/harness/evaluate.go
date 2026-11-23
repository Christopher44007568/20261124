package harness

import (
	"math"

	"stereoval/internal/geo"
	"stereoval/internal/metrics"
	"stereoval/internal/scene"
)

// markerSample stores one marker's observations at one frame.
type markerSample struct {
	id                   int
	expectedL, expectedR [2]float64
	obsL, obsR           metrics.MarkerObs
	dExpX, dExpY         float64
	dObsX, dObsY         float64
	matched              bool
}

// Evaluate walks all frames and records per-eye error, disparity behavior,
// ghosting and edge misalignment against exact ground truth.
func Evaluate(frames []stereoFrame, k geo.Intrinsics, rig geo.Rig, sc *scene.Scene) []metrics.FrameReport {
	reports := make([]metrics.FrameReport, len(frames))
	var prevSamples []markerSample

	for fi, sf := range frames {
		samples := make([]markerSample, 0, len(sc.Markers))
		report := metrics.FrameReport{Frame: fi}

		report.Left = evalEye(sf.left, k, rig, sf, sc, geo.Left)
		report.Right = evalEye(sf.right, k, rig, sf, sc, geo.Right)

		var dErr, dVert, dChgX, dChgY []float64
		fillContam, matched := 0, 0

		for _, m := range sc.Markers {
			s := sampleMarker(m, sf, k, rig)
			if s.matched {
				matched++
				dErr = append(dErr, math.Hypot(s.dObsX-s.dExpX, s.dObsY-s.dExpY))
				dVert = append(dVert, math.Abs(s.dObsY-s.dExpY))
				if s.obsL.OnFill || s.obsR.OnFill {
					fillContam++
				}
				if fi > 0 {
					if ps := findMarker(prevSamples, m.ID); ps != nil && ps.matched {
						dChgX = append(dChgX, math.Abs((s.dObsX-ps.dObsX)-(s.dExpX-ps.dExpX)))
						dChgY = append(dChgY, math.Abs((s.dObsY-ps.dObsY)-(s.dExpY-ps.dExpY)))
					}
				}
			}
			samples = append(samples, s)
		}

		report.Stereo = metrics.StereoReport{
			DispErrRMS:       metrics.RMS(dErr),
			DispErrMax:       metrics.MaxAbs(dErr),
			DispVertRMS:      metrics.RMS(dVert),
			DispChangeRMS:    metrics.RMS(append(append([]float64{}, dChgX...), dChgY...)),
			DispChangeMax:    metrics.MaxAbs(append(append([]float64{}, dChgX...), dChgY...)),
			FillContaminated: fillContam,
			MatchedMarkers:   matched,
		}
		reports[fi] = report
		prevSamples = samples
	}
	return reports
}

func evalEye(ef eyeFrame, k geo.Intrinsics, rig geo.Rig, sf stereoFrame, sc *scene.Scene, eye geo.Eye) metrics.EyeReport {
	var errs []float64
	found, total := 0, 0
	for _, m := range sc.Markers {
		eu, ev := projectMarker(m, k, rig, sf, eye)
		if eu < 0 {
			continue
		}
		total++
		// Reference is the marker's measured centroid in the rendered ground
		// truth, so centroid quantization cancels and only re-projection
		// displacement remains.
		gtObs := metrics.LocateMarker(ef.gt.Gray, k.W, k.H, eu, ev, nil, nil)
		obs := metrics.LocateMarker(ef.warped.Gray, k.W, k.H, eu, ev, ef.warped.Filled, ef.warped.Hole)
		// Observations touching a hole are unreliable (truncated disk); the
		// hole/fill metrics already account for them.
		if obs.Found && gtObs.Found && !obs.NearHole {
			found++
			_, _, e := metrics.ReprojError(obs, gtObs.U, gtObs.V)
			errs = append(errs, e)
		}
	}
	directed, symmetric := metrics.EdgeMisalignment(ef.warped.Gray, ef.gt.Gray, k.W, k.H)
	ghost := metrics.Ghosting(ef.warped.Gray, ef.gt.Gray, k.W, k.H, ef.warped.Filled)

	return metrics.EyeReport{
		Eye:            eye.String(),
		PoseTag:        ef.tag,
		ReprojMean:     metrics.Mean(errs),
		ReprojRMS:      metrics.RMS(errs),
		ReprojMax:      metrics.MaxAbs(errs),
		MarkersFound:   found,
		MarkersTotal:   total,
		HoleRatio:      ef.warped.HoleRatio(),
		FillRatio:      ef.warped.FillRatio(),
		EdgeDirected:   directed,
		EdgeSymmetric:  symmetric,
		GhostRatio:     ghost.GhostRatio,
		FillGhostRatio: ghost.FillGhostRatio,
	}
}

func sampleMarker(m scene.Marker, sf stereoFrame, k geo.Intrinsics, rig geo.Rig) markerSample {
	lu, lv := projectMarker(m, k, rig, sf, geo.Left)
	ru, rv := projectMarker(m, k, rig, sf, geo.Right)
	s := markerSample{id: m.ID, matched: false}
	if lu < 0 || ru < 0 {
		return s
	}
	s.expectedL = [2]float64{lu, lv}
	s.expectedR = [2]float64{ru, rv}
	s.dExpX, s.dExpY = lu-ru, lv-rv

	s.obsL = metrics.LocateMarker(sf.left.warped.Gray, k.W, k.H, lu, lv, sf.left.warped.Filled, sf.left.warped.Hole)
	s.obsR = metrics.LocateMarker(sf.right.warped.Gray, k.W, k.H, ru, rv, sf.right.warped.Filled, sf.right.warped.Hole)
	if dx, dy, ok := metrics.Disparity(s.obsL, s.obsR); ok {
		s.dObsX, s.dObsY, s.matched = dx, dy, true
	}
	return s
}

func projectMarker(m scene.Marker, k geo.Intrinsics, rig geo.Rig, sf stereoFrame, eye geo.Eye) (u, v float64) {
	pose := sf.head.EyePose(rig, eye)
	cam := pose.ToCamera(m.Center)
	uu, vv, _, ok := k.Project(cam)
	if !ok {
		return -1, -1
	}
	return uu, vv
}

func findMarker(xs []markerSample, id int) *markerSample {
	for i := range xs {
		if xs[i].id == id {
			return &xs[i]
		}
	}
	return nil
}
