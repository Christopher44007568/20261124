package hzb

import (
	"math"
	"testing"
)

var stereoPolicy = SharePolicy{SharedFromLevel: 2, RecheckShared: true, RecheckCoarse: true}

// Thin rod: a thin rod occluder is dilated by max-pooling in coarse layers
// and wrongly culls a wide box behind it. The conservative replay against
// the eye's own near layers must catch every false cull.
func TestThinRodFalseCullAndRecheck(t *testing.T) {
	camL := testCam("left", Vec3{-0.05, 0, 0}, Vec3{0, 0, 1})
	camR := testCam("right", Vec3{0.05, 0, 0}, Vec3{0, 0, 1})
	rod := BoxTriangles(Vec3{-0.03, -2, 5.0}, Vec3{0.03, 2, 5.06})
	targets := []Object{{ID: "wide-box", Box: boxAt(0, 0, 6.2, 0.6, 0.6, 0.15)}}
	dbL := RenderDepth(camL, rod)
	dbR := RenderDepth(camR, rod)

	// Sanity: the box is wider than the rod, so it is visible to both eyes.
	if !GroundTruthVisible(camL, dbL, targets[0].Box) || !GroundTruthVisible(camR, dbR, targets[0].Box) {
		t.Fatal("wide box behind thin rod must be ground-truth visible")
	}

	// Negative control: no recheck -> dilated rod produces false culls.
	pol := SharePolicy{SharedFromLevel: 2}
	s := BuildStereo(camL, camR, dbL, dbR, 1, pol)
	s.ValidateEye(camL, 0, dbL, targets)
	s.ValidateEye(camR, 1, dbR, targets)
	if s.Stats.FalseCulls == 0 {
		t.Fatal("expected false culls from thin-rod dilation in coarse layers")
	}
	var rightEyeEvent *FalseCullEvent
	for i := range s.Stats.FalseCullEvents {
		ev := &s.Stats.FalseCullEvents[i]
		if ev.LayerCamera == "" || ev.LayerFrame != 1 {
			t.Errorf("event not traceable to layer+camera: %+v", ev)
		}
		if ev.Level < pol.SharedFromLevel {
			t.Errorf("false cull decided on near level %d, want far layer", ev.Level)
		}
		if ev.VisiblePixels == 0 {
			t.Errorf("event must count wrongly culled pixels: %+v", ev)
		}
		if ev.Eye == "right" {
			rightEyeEvent = ev
		}
	}
	if rightEyeEvent == nil {
		t.Fatal("expected a false cull for the right eye")
	}
	if rightEyeEvent.LayerCamera != "left" || !rightEyeEvent.LayerShared {
		t.Errorf("right-eye false cull must trace to shared left layer: %+v", rightEyeEvent)
	}

	// With recheck: every false cull is caught by per-eye near-layer replay.
	s2 := BuildStereo(camL, camR, dbL, dbR, 1, stereoPolicy)
	s2.ValidateEye(camL, 0, dbL, targets)
	s2.ValidateEye(camR, 1, dbR, targets)
	if s2.Stats.FalseCulls != 0 {
		t.Fatalf("recheck must eliminate false culls: %v", s2.Stats.FalseCullEvents)
	}
	if s2.Stats.ReplaySaves == 0 {
		t.Error("expected at least one conservative replay save")
	}
	if !s2.Sharing || s2.Stats.ReuseRatio() <= 0 {
		t.Error("far layers should be shared between compatible eyes")
	}
}

// Occlusion reveal: the camera translates between frames and reveals a
// target that was hidden. Far layers reused from the previous frame still
// cull it; the false cull must be traceable to the old frame's layer, and a
// replay against rebuilt near layers must fix it.
func TestOcclusionRevealWithStaleLayers(t *testing.T) {
	camOld := testCam("camA", Vec3{0, 0, 0}, Vec3{0, 0, 1})
	camNew := testCam("camA", Vec3{0.6, 0, 0}, Vec3{0, 0, 1})
	wall := BoxTriangles(Vec3{-0.5, -1, 4.9}, Vec3{0.5, 1, 5.1})
	target := Object{ID: "revealed", Box: AABB{Vec3{0.1, -0.3, 6.1}, Vec3{0.7, 0.3, 6.5}}}
	dbOld := RenderDepth(camOld, wall)
	dbNew := RenderDepth(camNew, wall)

	if GroundTruthVisible(camOld, dbOld, target.Box) {
		t.Fatal("target should be hidden behind the wall in frame 1")
	}
	if !GroundTruthVisible(camNew, dbNew, target.Box) {
		t.Fatal("target should be revealed after the camera move")
	}

	build := func() *Pyramid {
		near := NewPyramid(dbNew, "camA", 2, 2) // near levels rebuilt now
		far := NewPyramid(dbOld, "camA", 1, 0)  // far levels reused from frame 1
		return ReuseFarLevels(near, far, 2)
	}

	// Without recheck the stale far layer culls the revealed target.
	polOff := SharePolicy{SharedFromLevel: 2}
	var st Stats
	ValidateTargets(build(), camNew, dbNew, []Object{target}, 2, polOff, &st)
	if st.FalseCulls != 1 {
		t.Fatalf("expected exactly 1 false cull from stale layers, got %d", st.FalseCulls)
	}
	ev := st.FalseCullEvents[0]
	if ev.LayerFrame != 1 || ev.Frame != 2 {
		t.Errorf("event must point at the old frame's layer: %+v", ev)
	}
	if ev.LayerCamera != "camA" || !ev.LayerShared {
		t.Errorf("event must identify the reused layer's camera: %+v", ev)
	}
	if ev.VisiblePixels == 0 {
		t.Errorf("event must count wrongly culled pixels: %+v", ev)
	}

	// With recheck the replay against current near layers saves it.
	polOn := SharePolicy{SharedFromLevel: 2, RecheckShared: true}
	var st2 Stats
	ValidateTargets(build(), camNew, dbNew, []Object{target}, 2, polOn, &st2)
	if st2.FalseCulls != 0 {
		t.Fatalf("recheck must fix stale-layer false culls: %v", st2.FalseCullEvents)
	}
	if st2.ReplaySaves != 1 {
		t.Errorf("expected 1 replay save, got %d", st2.ReplaySaves)
	}
}

// Different near planes: eyes with mismatched clip planes must not share
// layers; the system falls back to a full per-eye pyramid and stays exact.
func TestDifferentNearPlanesDisableSharing(t *testing.T) {
	camL := testCam("left", Vec3{-0.05, 0, 0}, Vec3{0, 0, 1})
	camR := testCam("right", Vec3{0.05, 0, 0}, Vec3{0, 0, 1})
	camR.Near = 0.9

	if Compatible(camL, camR) {
		t.Error("different near planes must be reported incompatible")
	}
	same := *camR
	same.Near = camL.Near
	if !Compatible(camL, &same) {
		t.Error("identical clip planes should be compatible")
	}

	wall := BoxTriangles(Vec3{-1, -1, 4.9}, Vec3{1, 1, 5.1})
	targets := []Object{
		{ID: "hidden", Box: boxAt(0, 0, 6.5, 0.3, 0.3, 0.3)},
		{ID: "shown", Box: boxAt(3.0, 0, 6.5, 0.3, 0.3, 0.3)},
	}
	dbL := RenderDepth(camL, wall)
	dbR := RenderDepth(camR, wall)

	s := BuildStereo(camL, camR, dbL, dbR, 1, stereoPolicy)
	if s.Sharing {
		t.Error("sharing must be disabled when near planes differ")
	}
	if s.Stats.ReuseRatio() != 0 {
		t.Errorf("reuse ratio = %v, want 0 without sharing", s.Stats.ReuseRatio())
	}
	s.ValidateEye(camL, 0, dbL, targets)
	s.ValidateEye(camR, 1, dbR, targets)
	if s.Stats.FalseCulls != 0 {
		t.Fatalf("per-eye pyramids must not false-cull: %v", s.Stats.FalseCullEvents)
	}
	if s.Stats.Occluded != 2 {
		t.Errorf("the hidden target should be culled for both eyes, occluded=%d", s.Stats.Occluded)
	}
}

// History mismatch: the camera rotates between frames and far layers from
// the old frame no longer match. Every false cull must be traceable to the
// old layer and camera, and recheck must eliminate them.
func TestHistoryFrameMismatchTraceable(t *testing.T) {
	camOld := testCam("camA", Vec3{0, 0, 0}, Vec3{0, 0, 1})
	fwd := Vec3{math.Sin(10 * math.Pi / 180), 0, math.Cos(10 * math.Pi / 180)}
	camNew := testCam("camA", Vec3{0, 0, 0}, fwd)
	wall := BoxTriangles(Vec3{-0.4, -1, 4.9}, Vec3{0.4, 1, 5.1})
	target := Object{ID: "rotated-in", Box: AABB{Vec3{0.4, -0.25, 6.1}, Vec3{1.0, 0.25, 6.3}}}
	dbOld := RenderDepth(camOld, wall)
	dbNew := RenderDepth(camNew, wall)

	if !GroundTruthVisible(camNew, dbNew, target.Box) {
		t.Fatal("target must be visible to the rotated camera")
	}

	build := func() *Pyramid {
		near := NewPyramid(dbNew, "camA", 2, 2)
		far := NewPyramid(dbOld, "camA", 1, 0)
		return ReuseFarLevels(near, far, 2)
	}

	polOff := SharePolicy{SharedFromLevel: 2}
	var st Stats
	ValidateTargets(build(), camNew, dbNew, []Object{target}, 2, polOff, &st)
	if st.FalseCulls == 0 {
		t.Fatal("expected false culls from history mismatch after rotation")
	}
	for _, ev := range st.FalseCullEvents {
		if ev.LayerFrame != 1 || ev.LayerCamera != "camA" || ev.Frame != 2 {
			t.Errorf("false cull not traceable to old layer/camera: %+v", ev)
		}
		if ev.Eye != "camA" {
			t.Errorf("event eye = %q, want camA", ev.Eye)
		}
	}

	polOn := SharePolicy{SharedFromLevel: 2, RecheckShared: true}
	var st2 Stats
	ValidateTargets(build(), camNew, dbNew, []Object{target}, 2, polOn, &st2)
	if st2.FalseCulls != 0 {
		t.Fatalf("recheck must fix history-mismatch false culls: %v", st2.FalseCullEvents)
	}
	if st2.ReplaySaves == 0 {
		t.Error("expected replay saves for the rotated camera")
	}
}

// Small rotation: with a 2-degree inter-eye rotation, far (coarse) layers of
// the two eyes should align much better than near (fine) layers, which is
// the quantitative justification for sharing far layers only.
func TestSmallRotationFarLayersAlign(t *testing.T) {
	camL := testCam("left", Vec3{-0.05, 0, 0}, Vec3{0, 0, 1})
	fwd := Vec3{math.Sin(2 * math.Pi / 180), 0, math.Cos(2 * math.Pi / 180)}
	camR := testCam("right", Vec3{0.05, 0, 0}, fwd)

	var tris []Triangle
	tris = append(tris, BoxTriangles(Vec3{0.2, -0.5, 1.4}, Vec3{0.6, 0.5, 1.8})...)
	tris = append(tris, BoxTriangles(Vec3{-1.5, 0.3, 4.0}, Vec3{-1.0, 1.0, 4.5})...)
	tris = append(tris, BoxTriangles(Vec3{-2, -1, 8.0}, Vec3{2, 1, 9.0})...)
	pl := NewPyramid(RenderDepth(camL, tris), "left", 0, 0)
	pr := NewPyramid(RenderDepth(camR, tris), "right", 0, 0)

	// Fraction of cells whose depth differs by more than 1% of the depth
	// range; empty-vs-filled counts as a mismatch.
	mismatch := func(level int) float64 {
		l1, l2 := pl.Levels[level], pr.Levels[level]
		bad, total := 0, len(l1.Data)
		for i := range l1.Data {
			a, b := l1.Data[i], l2.Data[i]
			if math.IsInf(a, 1) || math.IsInf(b, 1) {
				if a != b {
					bad++
				}
				continue
			}
			if math.Abs(a-b) > 0.09 {
				bad++
			}
		}
		return float64(bad) / float64(total)
	}

	near := mismatch(0)
	far := mismatch(len(pl.Levels) - 1)
	t.Logf("cell mismatch ratio: mip0=%.3f far level=%.3f", near, far)
	if near == 0 {
		t.Error("near layers should differ between eyes (parallax)")
	}
	if far >= near {
		t.Errorf("far layers should align better than near layers: far=%.3f near=%.3f", far, near)
	}
}
