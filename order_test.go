package reprojlab

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestOrderSensitiveUnderHighCurvatureLargeTranslationAndEdgeObjects(t *testing.T) {
	cfg := DefaultConfig()
	scene := NewScene(cfg)
	decision := DecideOrder(scene, cfg, 0.05)

	if decision.Selected != ReprojectThenDistort {
		t.Fatalf("selected %s, want %s; reason=%s", decision.Selected, ReprojectThenDistort, decision.Reason)
	}
	if !strings.Contains(decision.Reason, "reference color RMSE") {
		t.Fatalf("decision must be based on measurable reference error, got %q", decision.Reason)
	}
	if decision.A.ReferenceDiff.ColorRMSE >= decision.B.ReferenceDiff.ColorRMSE {
		t.Fatalf("A color RMSE %.6f must be below B %.6f",
			decision.A.ReferenceDiff.ColorRMSE, decision.B.ReferenceDiff.ColorRMSE)
	}
	if decision.A.ReferenceDiff.DepthRMSE >= decision.B.ReferenceDiff.DepthRMSE {
		t.Fatalf("A depth RMSE %.6f must be below B %.6f",
			decision.A.ReferenceDiff.DepthRMSE, decision.B.ReferenceDiff.DepthRMSE)
	}

	between := CompareRasters(decision.A.Result.Final, decision.B.Result.Final)
	if between.ColorRMSE < 1 || between.DepthRMSE < 1e-3 {
		t.Fatalf("orders are not measurably different: color=%g depth=%g", between.ColorRMSE, between.DepthRMSE)
	}
	if decision.A.Result.Stats.Passes != 2 || decision.B.Result.Stats.Passes != 2 {
		t.Fatalf("both orders must perform two resampling passes: A=%d B=%d",
			decision.A.Result.Stats.Passes, decision.B.Result.Stats.Passes)
	}
	if decision.A.Result.Stats.SourceSamples == decision.B.Result.Stats.SourceSamples {
		t.Fatalf("orders should not have identical resampling work")
	}
	if decision.A.Result.Final.HoleCount() == 0 || decision.B.Result.Final.HoleCount() == 0 {
		t.Fatalf("stress case must produce observable holes: A=%d B=%d",
			decision.A.Result.Final.HoleCount(), decision.B.Result.Final.HoleCount())
	}
	if len(decision.A.Result.Probes) == 0 || len(decision.B.Result.Probes) == 0 {
		t.Fatal("depth-coordinate probes were not recorded")
	}
	for _, name := range []string{"left-edge-center", "right-edge-center", "bottom-edge-center"} {
		if _, ok := scene.ProbeByName(name); !ok {
			t.Fatalf("missing screen-edge probe %s", name)
		}
	}
	var touchedLeft, touchedRight, touchedBottom bool
	for _, o := range scene.Objects {
		touchedLeft = touchedLeft || o.X0 == 0
		touchedRight = touchedRight || o.X1 == cfg.Width-1
		touchedBottom = touchedBottom || o.Y1 == cfg.Height-1
	}
	if !touchedLeft || !touchedRight || !touchedBottom {
		t.Fatalf("scene must contain left/right/bottom edge objects")
	}
}

func TestOrderDecisionRejectsFewerHolesWhenGeometryIsWrong(t *testing.T) {
	cfg := DefaultConfig()
	scene := NewScene(cfg)
	decision := DecideOrder(scene, cfg, 0.05)

	if decision.B.HolePercent >= decision.A.HolePercent {
		t.Fatalf("test precondition changed: B holes %.3f%% must be below A %.3f%%",
			decision.B.HolePercent, decision.A.HolePercent)
	}
	if decision.B.ProbeFailures >= decision.A.ProbeFailures {
		t.Fatalf("test precondition changed: B probe failures %d must be below A %d",
			decision.B.ProbeFailures, decision.A.ProbeFailures)
	}
	if decision.Selected != ReprojectThenDistort {
		t.Fatalf("a fuller B image must not override reference geometry error; selected %s", decision.Selected)
	}
}

func TestLowResolutionDistortionThenReprojectionDegrades(t *testing.T) {
	cfg := DefaultConfig()
	scene := NewScene(cfg)
	ref, _ := scene.DirectReference(cfg)
	full := RunOrder(scene, cfg, DistortThenReproject)
	low := RunLowResDistortThenReproject(scene, cfg)

	lowRef := CompareRasters(low.Final, ref)
	fullRef := CompareRasters(full.Final, ref)
	if low.Final.HoleCount() <= full.Final.HoleCount()+50000 {
		t.Fatalf("low-resolution distortion should create far more holes: low=%d full=%d",
			low.Final.HoleCount(), full.Final.HoleCount())
	}
	if low.Stats.SourceSamples >= full.Stats.SourceSamples {
		t.Fatalf("low-resolution path should read fewer samples: low=%d full=%d",
			low.Stats.SourceSamples, full.Stats.SourceSamples)
	}
	if low.Stats.Writes >= full.Stats.Writes {
		t.Fatalf("low-resolution path should write fewer samples: low=%d full=%d",
			low.Stats.Writes, full.Stats.Writes)
	}
	if lowRef.DepthRMSE <= fullRef.DepthRMSE {
		t.Fatalf("low-resolution depth must be worse: low=%.6f full=%.6f",
			lowRef.DepthRMSE, fullRef.DepthRMSE)
	}
	if low.Stats.Passes != 2 {
		t.Fatalf("low-resolution variant must still record both passes, got %d", low.Stats.Passes)
	}
}

func TestPerChannelDistortionSeparatesChannelsAtEdges(t *testing.T) {
	cfg := DefaultConfig()
	scene := NewScene(cfg)
	ch := RunPerChannelDistortion(scene, cfg)
	if ch.Holes[0] == ch.Holes[1] || ch.Holes[1] == ch.Holes[2] {
		t.Fatalf("per-channel holes should differ: %v", ch.Holes)
	}
	if ch.UnionHoles() <= ch.Holes[1] {
		t.Fatalf("channel union holes %d should exceed green holes %d", ch.UnionHoles(), ch.Holes[1])
	}
	edgeShift := ChannelDisplacement(Point{X: 470, Y: 121}, cfg, 1.04)
	centerShift := ChannelDisplacement(Point{X: cfg.Intrinsics.CX, Y: cfg.Intrinsics.CY}, cfg, 1.04)
	if edgeShift <= 1 {
		t.Fatalf("edge channel shift %.4f px is too small to test", edgeShift)
	}
	if centerShift > 1e-9 {
		t.Fatalf("center channel shift should vanish, got %.4f px", centerShift)
	}
	for c := range ch.Channels {
		if ch.Stats[c].Passes != 1 {
			t.Fatalf("channel %d expected one distort pass, got %d", c, ch.Stats[c].Passes)
		}
	}
}

func TestWrongDepthReuseIsRejected(t *testing.T) {
	cfg := DefaultConfig()
	scene := NewScene(cfg)
	correct := RunOrder(scene, cfg, DistortThenReproject)
	wrong := RunWrongDepthReuse(scene, cfg)
	diff := CompareRasters(wrong.Final, correct.Final)

	if diff.ColorRMSE < 1 {
		t.Fatalf("stale depth should visibly change color correspondence, RMSE=%.6f", diff.ColorRMSE)
	}
	if diff.DepthRMSE < 0.03 {
		t.Fatalf("stale depth should corrupt depth coordinates, RMSE=%.6f", diff.DepthRMSE)
	}
	if diff.MaxDepthError < 0.5 {
		t.Fatalf("stale depth should produce a large local depth error, max=%.6f", diff.MaxDepthError)
	}
	if ProbeFailures(wrong.Probes, 0.05) == 0 {
		t.Fatalf("probe checks must reject stale-depth reuse: %+v", wrong.Probes)
	}
	mean := MeanProbeDepthError(wrong.Probes)
	if math.IsInf(mean, 1) || math.IsNaN(mean) || mean < 0.05 {
		t.Fatalf("mean wrong-depth probe error too small: %g", mean)
	}
}

func TestReportRecordsCountsHolesAndDepthCoordinates(t *testing.T) {
	rep := BuildExperimentReport(DefaultConfig(), 0.05)
	if len(rep.Orders) != 2 {
		t.Fatalf("expected two order reports, got %d", len(rep.Orders))
	}
	for _, o := range rep.Orders {
		if o.Stats.Passes != 2 || o.Stats.SourceSamples == 0 || o.Stats.Holes == 0 {
			t.Fatalf("%s missing resampling/hole data: %+v", o.Order, o.Stats)
		}
		if len(o.Probes) == 0 {
			t.Fatalf("%s missing depth-coordinate probes", o.Order)
		}
		filled := 0
		for _, p := range o.Probes {
			if p.Filled && p.ActualDepth != nil {
				filled++
			}
		}
		if filled == 0 {
			t.Fatalf("%s has no valid probe depth coordinates", o.Order)
		}
		if rep.SelectedOrder == o.Order && !o.Selected {
			t.Fatalf("selected order %s is not marked selected", o.Order)
		}
	}
	data, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("report JSON: %v", err)
	}
	if strings.Contains(string(data), "NaN") || strings.Contains(string(data), "Inf") {
		t.Fatalf("report contains non-JSON floating point values")
	}
}
