package strategy

import (
	"math"
	"testing"

	"posebench/internal/geom"
	"posebench/internal/motion"
	"posebench/internal/scan"
)

func TestPerRowMatchesRowTimestamps(t *testing.T) {
	sh := scan.DefaultShutter()
	profile := motion.Catalog()[0]
	start := 0.123
	sampling, err := Build(PerRow, DefaultConfig(), sh, start, profile)
	if err != nil {
		t.Fatal(err)
	}
	for row := 0; row < sh.Camera.Height; row++ {
		want := profile.PoseAt(sh.RowTime(start, row))
		if got := geom.AngularErrorDeg(sampling.Poses[row], want); got > 1e-12 {
			t.Fatalf("row %d error = %g", row, got)
		}
	}
}

func TestUniformStartIsNotDeployable(t *testing.T) {
	sh := scan.DefaultShutter()
	profile := motion.Catalog()[0]
	sampling, err := Build(UniformStart, DefaultConfig(), sh, 0.25, profile)
	if err != nil {
		t.Fatal(err)
	}
	if sampling.Deployable {
		t.Fatal("frame-start uniform sampling must be marked non-deployable")
	}
	if sampling.PoseCount != 1 {
		t.Fatalf("pose count = %d, want 1", sampling.PoseCount)
	}
}

func TestUniformMidRejectsFrameStartPhase(t *testing.T) {
	sh := scan.DefaultShutter()
	profile := motion.Catalog()[0]
	cfg := DefaultConfig()
	cfg.UnifiedPhase = 0
	if _, err := Build(UniformMid, cfg, sh, 0.25, profile); err == nil {
		t.Fatal("uniform-mid must reject a frame-start timestamp")
	}
}

func TestSmoothSegmentsAreContinuousAtBoundaries(t *testing.T) {
	sh := scan.DefaultShutter()
	profile := motion.Catalog()[2]
	sampling, err := Build(Segmented, DefaultConfig(), sh, 0.1, profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(sampling.SegmentBoundaries) != DefaultConfig().Segments-1 {
		t.Fatalf("boundary count = %d", len(sampling.SegmentBoundaries))
	}
	for _, b := range sampling.SegmentBoundaries {
		step := geom.AngularErrorDeg(sampling.Poses[b-1], sampling.Poses[b])
		if step > 0.05 {
			t.Fatalf("smooth boundary %d step = %.6f deg", b, step)
		}
	}
}

func TestHardSegmentsJumpAtBoundaries(t *testing.T) {
	sh := scan.DefaultShutter()
	profile := motion.Catalog()[0]
	sampling, err := Build(HardSegments, DefaultConfig(), sh, 0.1, profile)
	if err != nil {
		t.Fatal(err)
	}
	b := sampling.SegmentBoundaries[0]
	if step := geom.AngularErrorDeg(sampling.Poses[b-1], sampling.Poses[b]); step < 0.1 {
		t.Fatalf("hard boundary step = %.6f deg, expected a visible jump", step)
	}
	if math.Abs(geom.AngularErrorDeg(sampling.Poses[0], sampling.Poses[b-2])) > 1e-12 {
		t.Fatal("hard segment poses should be constant inside a segment")
	}
}
