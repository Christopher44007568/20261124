package timewarp_test

import (
	"errors"
	"testing"

	"github.com/Christopher44007568/20261124/timewarp"
)

func poseAt(version uint64, at timewarp.Timestamp, yaw float64) timewarp.Pose {
	return timewarp.Pose{Version: version, At: at, Yaw: yaw}
}

func TestRepeatedWarpsBuildProvenanceChain(t *testing.T) {
	p := timewarp.NewPipeline()
	first := poseAt(1, 0, 0)
	mustPose(t, p, first)
	mustFrame(t, p, timewarp.Frame{ID: 1, RenderTime: 0, Pose: first})

	initial, err := p.Scanout(0)
	if err != nil {
		t.Fatalf("initial scanout: %v", err)
	}
	if initial.Warped {
		t.Fatal("initial scanout should use the source image directly")
	}

	second := poseAt(2, timewarp.Millis(4), 1)
	mustPose(t, p, second)
	warpOne, err := p.Scanout(timewarp.Millis(8))
	if err != nil {
		t.Fatalf("first warp: %v", err)
	}
	if !warpOne.Warped || warpOne.Record == nil {
		t.Fatal("expected a warp record")
	}
	if warpOne.Record.HistorySourceID != "src/1" {
		t.Fatalf("first history source = %q, want src/1", warpOne.Record.HistorySourceID)
	}
	if warpOne.Record.HistoryPoseVersion != 1 || warpOne.Record.TargetPoseVersion != 2 {
		t.Fatalf("pose versions = %d->%d, want 1->2",
			warpOne.Record.HistoryPoseVersion, warpOne.Record.TargetPoseVersion)
	}

	third := poseAt(3, timewarp.Millis(12), 2)
	mustPose(t, p, third)
	warpTwo, err := p.Scanout(timewarp.Millis(16))
	if err != nil {
		t.Fatalf("second warp: %v", err)
	}
	if !warpTwo.Warped || warpTwo.Record == nil {
		t.Fatal("expected a second warp record")
	}
	if warpTwo.Record.HistorySourceID != warpOne.ImageID {
		t.Fatalf("second history source = %q, want %q",
			warpTwo.Record.HistorySourceID, warpOne.ImageID)
	}
	if !warpTwo.Record.ReusedFrame {
		t.Fatal("second warp should be marked as a reused source frame")
	}
	if warpTwo.Record.ChainDepth != 3 {
		t.Fatalf("chain depth = %d, want 3", warpTwo.Record.ChainDepth)
	}

	chain, err := p.SourceChain(warpTwo.ImageID)
	if err != nil {
		t.Fatalf("source chain: %v", err)
	}
	if len(chain) != len(warpTwo.Provenance) {
		t.Fatalf("scanout provenance length = %d, source chain length = %d",
			len(warpTwo.Provenance), len(chain))
	}
	wantIDs := []string{"src/1", warpOne.ImageID, warpTwo.ImageID}
	if len(chain) != len(wantIDs) {
		t.Fatalf("chain length = %d, want %d: %+v", len(chain), len(wantIDs), chain)
	}
	for i, want := range wantIDs {
		if chain[i].ID != want {
			t.Fatalf("chain[%d] = %q, want %q", i, chain[i].ID, want)
		}
	}
	if chain[len(chain)-1].Pose.Version != 3 {
		t.Fatalf("final pose version = %d, want 3", chain[len(chain)-1].Pose.Version)
	}

	stats := p.Stats()
	if stats.FramesAccepted != 1 || stats.Warps != 2 || stats.ReusedFrameWarps != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if got := p.WarpsForFrame(1); got != 2 {
		t.Fatalf("warps for frame 1 = %d, want 2", got)
	}
}

func TestDuplicateFrameIsNotAccountedTwice(t *testing.T) {
	p := timewarp.NewPipeline()
	first := poseAt(1, timewarp.Millis(10), 5)
	mustPose(t, p, first)
	frame := timewarp.Frame{ID: 7, RenderTime: timewarp.Millis(10), Pose: first}
	mustFrame(t, p, frame)

	accepted, err := p.SubmitFrame(frame)
	if err != nil {
		t.Fatalf("duplicate frame: %v", err)
	}
	if accepted {
		t.Fatal("duplicate frame reported as accepted")
	}
	second := poseAt(2, timewarp.Millis(12), 6)
	mustPose(t, p, second)
	scanout, err := p.Scanout(timewarp.Millis(16))
	if err != nil {
		t.Fatalf("scanout: %v", err)
	}
	if scanout.SourceFrameID != frame.ID {
		t.Fatalf("source frame = %d, want %d", scanout.SourceFrameID, frame.ID)
	}
	if !scanout.Warped {
		t.Fatal("expected the unique source frame to be warped")
	}

	stats := p.Stats()
	if stats.FramesAccepted != 1 {
		t.Fatalf("accepted frames = %d, want 1", stats.FramesAccepted)
	}
	if stats.DuplicateFrames != 1 {
		t.Fatalf("duplicate frames = %d, want 1", stats.DuplicateFrames)
	}
	if stats.Warps != 1 {
		t.Fatalf("warps = %d, want 1", stats.Warps)
	}
	if got := p.WarpsForFrame(frame.ID); got != 1 {
		t.Fatalf("warps for frame = %d, want 1", got)
	}
}

func TestClockRegressionStopsAllFutureWarps(t *testing.T) {
	p := timewarp.NewPipeline()
	first := poseAt(1, timewarp.Millis(10), 0)
	mustPose(t, p, first)
	mustFrame(t, p, timewarp.Frame{ID: 1, RenderTime: timewarp.Millis(10), Pose: first})
	if _, err := p.Scanout(timewarp.Millis(10)); err != nil {
		t.Fatalf("initial scanout: %v", err)
	}

	second := poseAt(2, timewarp.Millis(20), 4)
	mustPose(t, p, second)
	beforeStop, err := p.Scanout(timewarp.Millis(24))
	if err != nil {
		t.Fatalf("pre-stop warp: %v", err)
	}
	recordsBefore := len(p.Records())

	backwards := timewarp.Frame{ID: 2, RenderTime: timewarp.Millis(15), Pose: first}
	if _, err := p.SubmitFrame(backwards); !errors.Is(err, timewarp.ErrClockWentBackwards) {
		t.Fatalf("regression error = %v, want ErrClockWentBackwards", err)
	}
	if !p.Stopped() {
		t.Fatal("pipeline should be stopped after clock regression")
	}

	if _, err := p.Scanout(timewarp.Millis(32)); !errors.Is(err, timewarp.ErrWarpStopped) ||
		!errors.Is(err, timewarp.ErrClockWentBackwards) {
		t.Fatalf("future scanout error = %v, want stopped/backwards", err)
	}
	if _, err := p.SubmitPose(poseAt(3, timewarp.Millis(28), 8)); !errors.Is(err, timewarp.ErrWarpStopped) {
		t.Fatalf("future pose error = %v, want ErrWarpStopped", err)
	}
	if got := len(p.Records()); got != recordsBefore {
		t.Fatalf("records changed after stop: %d -> %d", recordsBefore, got)
	}
	if got := p.Stats().Warps; got != 1 {
		t.Fatalf("warps after stop = %d, want 1", got)
	}
	if beforeStop.ImageID == "" {
		t.Fatal("pre-stop scanout has no image")
	}
}

func TestNewPoseVersionWithoutMotionDoesNotWarp(t *testing.T) {
	p := timewarp.NewPipeline()
	first := poseAt(1, 0, 10)
	mustPose(t, p, first)
	mustFrame(t, p, timewarp.Frame{ID: 1, RenderTime: 0, Pose: first})
	if _, err := p.Scanout(0); err != nil {
		t.Fatalf("scanout: %v", err)
	}

	stopped := poseAt(2, timewarp.Millis(4), 10)
	mustPose(t, p, stopped)
	scanout, err := p.Scanout(timewarp.Millis(8))
	if err != nil {
		t.Fatalf("scanout after stop: %v", err)
	}
	if scanout.Warped {
		t.Fatal("identical orientation should reuse the image without a warp")
	}
	if p.Stats().Warps != 0 {
		t.Fatalf("identity warp count = %d, want 0", p.Stats().Warps)
	}
	if scanout.ImageID != "src/1" {
		t.Fatalf("final image = %q, want src/1", scanout.ImageID)
	}
}

func mustPose(t *testing.T, p *timewarp.Pipeline, pose timewarp.Pose) {
	t.Helper()
	accepted, err := p.SubmitPose(pose)
	if err != nil {
		t.Fatalf("submit pose v%d: %v", pose.Version, err)
	}
	if !accepted {
		t.Fatalf("pose v%d was not accepted", pose.Version)
	}
}

func mustFrame(t *testing.T, p *timewarp.Pipeline, frame timewarp.Frame) {
	t.Helper()
	accepted, err := p.SubmitFrame(frame)
	if err != nil {
		t.Fatalf("submit frame %d: %v", frame.ID, err)
	}
	if !accepted {
		t.Fatalf("frame %d was not accepted", frame.ID)
	}
}
