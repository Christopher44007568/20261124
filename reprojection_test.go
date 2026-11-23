package reprojection

import (
	"math"
	"testing"
	"time"
)

func TestCameraProjectUnprojectRoundTrip(t *testing.T) {
	cam := NewCamera(480, 270, 60*math.Pi/180)
	poses := []Pose{
		{},
		{Yaw: 0.31, Pitch: -0.12},
		{Yaw: -0.22, Pitch: 0.08, Position: Vec3{0.1, -0.2, 0.3}},
	}
	for _, pose := range poses {
		for _, p := range []Vec3{
			{X: 0.2, Y: -0.1, Z: 1.2},
			{X: -1.1, Y: 0.4, Z: 3.0},
			{X: 0.0, Y: 0.0, Z: 0.7},
		} {
			screen, depth, ok := cam.Project(p, pose)
			if !ok {
				t.Fatalf("point %+v was not projectable", p)
			}
			got := cam.UnprojectScreen(screen, depth, pose)
			if got.Sub(p).Len() > 1e-9 {
				t.Fatalf("round trip mismatch: got %+v want %+v", got, p)
			}
		}
	}
}

func TestRayAndProjectAreConsistent(t *testing.T) {
	cam := NewCamera(320, 180, 60*math.Pi/180)
	pose := Pose{Yaw: 0.18, Pitch: -0.07, Position: Vec3{0.05, 0.02, -0.04}}
	for _, screen := range []Vec2{{10.5, 20.5}, {160.5, 90.5}, {300.5, 150.5}} {
		dir := cam.Ray(screen.X, screen.Y, pose)
		depth := (2.5 - pose.Position.Z) / dir.Z
		hit := pose.Position.Add(dir.Scale(depth))
		got, _, ok := cam.Project(hit, pose)
		if !ok || got.Distance(screen) > 1e-8 {
			t.Fatalf("ray/project mismatch at %+v: got %+v", screen, got)
		}
	}
}

func TestWorldHasNearOcclusion(t *testing.T) {
	cam := NewCamera(320, 180, 60*math.Pi/180)
	world := RenderWorld(cam, DefaultWorld(), Pose{})
	if world.Coverage() < 0.99 {
		t.Fatalf("world coverage too low: %.3f", world.Coverage())
	}
	if world.NearCoverage() <= 0.02 {
		t.Fatalf("near occluder is missing: %.3f", world.NearCoverage())
	}
}

func testScenario() Scenario {
	s := DefaultScenario()
	s.Camera = NewCamera(320, 180, s.Camera.VFOV)
	return s
}

func compareMap(t *testing.T, s Scenario, scanout time.Duration) map[Mode]FrameResult {
	t.Helper()
	results, err := s.CompareAt(scanout)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[Mode]FrameResult, len(results))
	for _, r := range results {
		out[r.Mode] = r
	}
	return out
}

func TestReprojectionUnderFastTurnKeepsNearOcclusion(t *testing.T) {
	cam := NewCamera(320, 180, 60*math.Pi/180)
	scanout := 300 * time.Millisecond
	source := defaultHead(scanout - 11*time.Millisecond)
	target := defaultHead(scanout)
	dst := ReprojectWorld(RenderWorld(cam, DefaultWorld(), source), cam, target)
	if dst.Coverage() < 0.90 {
		t.Fatalf("too many reprojection holes: %.3f", 1-dst.Coverage())
	}
	if dst.NearCoverage() < 0.20 {
		t.Fatalf("near occlusion disappeared after reprojection: %.3f", dst.NearCoverage())
	}
}

func TestLayeredUIHasNoWorldDepthOcclusion(t *testing.T) {
	s := testScenario()
	scanout := 300 * time.Millisecond
	r := compareMap(t, s, scanout)[Layered]

	if r.Metrics.UIRequiredPixels == 0 {
		t.Fatal("UI layer produced no pixels")
	}
	if r.Metrics.DepthRejectedRate != 0 || r.Metrics.UIPresenceError != 0 {
		t.Fatalf("layered UI was occluded: rejected %.4f presence error %.4f",
			r.Metrics.DepthRejectedRate, r.Metrics.UIPresenceError)
	}
	if r.Metrics.ReferenceMismatch != 0 || r.Metrics.FixedUIDriftPx != 0 {
		t.Fatalf("layered UI differs from reference: mismatch %.4f drift %.3f",
			r.Metrics.ReferenceMismatch, r.Metrics.FixedUIDriftPx)
	}
	if r.UIScanout != r.Timing.Scanout || r.Metrics.ScanoutSkewMs != 0 {
		t.Fatalf("layered UI used a different scanout clock: %s vs %s",
			r.UIScanout, r.Timing.Scanout)
	}

	reticle, ok := findAnchor(r.Anchors, "reticle")
	if !ok {
		t.Fatal("reticle anchor missing")
	}
	want := Vec2{float64(s.Camera.Width) / 2, float64(s.Camera.Height) / 2}
	if reticle.Screen.Distance(want) > 1e-9 {
		t.Fatalf("fixed reticle moved: got %+v want %+v", reticle.Screen, want)
	}
	if r.Metrics.WorldHoleRate > 0.15 {
		t.Fatalf("world reprojection hole rate too high: %.3f", r.Metrics.WorldHoleRate)
	}
}

func TestNaiveMergedDepthLetsNearWorldOccludeSystemUI(t *testing.T) {
	s := testScenario()
	scanout := 300 * time.Millisecond
	results := compareMap(t, s, scanout)
	layered := results[Layered]
	merged := results[MergedDepth]

	if merged.Metrics.DepthRejectedRate <= 0.10 {
		t.Fatalf("expected near-world depth rejection, got %.4f", merged.Metrics.DepthRejectedRate)
	}
	if merged.Metrics.UIPresenceError <= 0.10 {
		t.Fatalf("expected missing UI pixels, got %.4f", merged.Metrics.UIPresenceError)
	}
	if merged.Metrics.ReferenceMismatch <= 0 {
		t.Fatal("merged-depth output unexpectedly matches the correct reference")
	}
	if merged.Metrics.FixedUIDriftPx != 0 || merged.UIScanout != scanout {
		t.Fatal("merged-depth test should isolate depth occlusion from camera/time errors")
	}
	if layered.Metrics.DepthRejectedRate != 0 {
		t.Fatal("layered reference was depth-tested")
	}
}

func TestInheritedWorldCameraBreaksFixedUIAndScanoutClock(t *testing.T) {
	s := testScenario()
	scanout := 300 * time.Millisecond
	results := compareMap(t, s, scanout)
	layered := results[Layered]
	inherited := results[InheritedWorldCamera]

	if inherited.Metrics.FixedUIDriftPx < 2 {
		t.Fatalf("fixed UI did not inherit world camera drift: %.3f px", inherited.Metrics.FixedUIDriftPx)
	}
	if inherited.Metrics.CursorErrorPx <= layered.Metrics.CursorErrorPx+1 {
		t.Fatalf("cursor error did not grow: inherited %.3f px, layered %.3f px",
			inherited.Metrics.CursorErrorPx, layered.Metrics.CursorErrorPx)
	}
	if inherited.UIScanout >= inherited.Timing.Scanout {
		t.Fatal("inherited camera path did not reuse the world render timestamp")
	}
	wantSkew := float64(s.WorldLatency) / float64(time.Millisecond)
	if math.Abs(inherited.Metrics.ScanoutSkewMs-wantSkew) > 0.01 {
		t.Fatalf("scanout skew %.3f ms, want %.3f ms", inherited.Metrics.ScanoutSkewMs, wantSkew)
	}
}

func TestRapidTurnAndFollowLatencySeries(t *testing.T) {
	s := testScenario()
	start := 250 * time.Millisecond
	step := time.Second / 90
	const frames = 45
	sums, err := s.Compare(start, step, frames)
	if err != nil {
		t.Fatal(err)
	}
	byMode := make(map[Mode]Summary, len(sums))
	for _, sum := range sums {
		byMode[sum.Mode] = sum
	}

	if rate := maxYawRate(s, start, step, frames); rate < 250 {
		t.Fatalf("head turn is not fast enough for the stress test: %.1f deg/s", rate)
	}
	layered := byMode[Layered]
	if layered.Max.DepthRejectedRate != 0 ||
		layered.Max.FixedUIDriftPx != 0 ||
		layered.Max.ScanoutSkewMs != 0 ||
		layered.Max.ReferenceMismatch != 0 {
		t.Fatalf("layered path violated an invariant: %+v", layered.Max)
	}
	if layered.Max.WorldHoleRate > 0.15 {
		t.Fatalf("late world reprojection left too many holes: %.3f", layered.Max.WorldHoleRate)
	}

	merged := byMode[MergedDepth]
	if merged.Max.DepthRejectedRate < 0.5 || merged.Max.ReferenceMismatch < 0.005 {
		t.Fatalf("naive merge did not fail strongly enough: %+v", merged.Max)
	}

	inherited := byMode[InheritedWorldCamera]
	if inherited.Max.FixedUIDriftPx < 5 ||
		inherited.Max.FollowLatencyMs <= layered.Max.FollowLatencyMs ||
		inherited.Max.ScanoutSkewMs < 10.9 {
		t.Fatalf("inherited camera stress metrics too weak: %+v", inherited.Max)
	}
}

func TestUIFollowLatencyIsMeasuredAgainstLatestInput(t *testing.T) {
	s := testScenario()
	s.HeadMotion = func(time.Duration) Pose { return Pose{} }
	s.HandMotion = func(t time.Duration) Vec3 {
		return Vec3{X: 0.08 + 0.35*t.Seconds(), Y: 0, Z: 1.05}
	}
	s.WorldLatency = 20 * time.Millisecond
	s.UIInputLatency = 6 * time.Millisecond

	results := compareMap(t, s, 400*time.Millisecond)
	layered := results[Layered]
	merged := results[MergedDepth]
	inherited := results[InheritedWorldCamera]

	if math.Abs(layered.Metrics.FollowLatencyMs-6) > 0.3 {
		t.Fatalf("layered follow latency %.3f ms, want 6 ms", layered.Metrics.FollowLatencyMs)
	}
	if math.Abs(merged.Metrics.FollowLatencyMs-6) > 0.3 {
		t.Fatalf("merged follow latency %.3f ms, want 6 ms", merged.Metrics.FollowLatencyMs)
	}
	if math.Abs(inherited.Metrics.FollowLatencyMs-20) > 0.3 {
		t.Fatalf("inherited follow latency %.3f ms, want 20 ms", inherited.Metrics.FollowLatencyMs)
	}
	if inherited.Metrics.CursorErrorPx <= layered.Metrics.CursorErrorPx+0.5 {
		t.Fatalf("cursor error should expose inherited latency: layered %.3f, inherited %.3f",
			layered.Metrics.CursorErrorPx, inherited.Metrics.CursorErrorPx)
	}
	if inherited.Metrics.ScanoutSkewMs != 20 {
		t.Fatalf("inherited scanout skew %.3f ms, want 20 ms", inherited.Metrics.ScanoutSkewMs)
	}
}

func maxYawRate(s Scenario, start, step time.Duration, frames int) float64 {
	end := start + time.Duration(frames-1)*step
	const dt = 250 * time.Microsecond
	maxRate := 0.0
	prev := s.HeadAt(start).Yaw
	for t := start + dt; t <= end; t += dt {
		yaw := s.HeadAt(t).Yaw
		rate := math.Abs(yaw-prev) / dt.Seconds()
		if rate > maxRate {
			maxRate = rate
		}
		prev = yaw
	}
	return maxRate * 180 / math.Pi
}
