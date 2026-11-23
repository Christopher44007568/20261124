package optics

import (
	"math"
	"testing"

	"pupilsim/internal/gaze"
)

func TestShiftArcMin(t *testing.T) {
	lens := DefaultLens()
	got := lens.ShiftArcMin(gaze.Vec2{X: 1}).X
	// 1 mm decentration = 0.1 cm * 50 D = 5 prism diopters.
	want := math.Atan(0.05) * (180 / math.Pi) * 60
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %v arcmin, want %v", got, want)
	}
}

func TestFixedAndPerEye(t *testing.T) {
	s := gaze.Sample{Measured: gaze.Vec2{X: 1, Y: 2}}
	if got := (Fixed{}).Update(s); got != (gaze.Vec2{}) {
		t.Fatalf("fixed must stay at origin, got %+v", got)
	}
	if got := (PerEye{}).Update(s); got != s.Measured {
		t.Fatalf("per-eye must pass measurement through, got %+v", got)
	}
	s.Lost = true
	if got := (PerEye{}).Update(s); got != (gaze.Vec2{}) {
		t.Fatalf("per-eye falls back to zero on loss, got %+v", got)
	}
}

func TestSmoothedNoJumpOnLoss(t *testing.T) {
	sm := NewSmoothed()
	// Build up a large offset.
	for i := 0; i < 200; i++ {
		sm.Update(gaze.Sample{Measured: gaze.Vec2{X: 4, Y: 3}})
	}
	prev := sm.Update(gaze.Sample{Measured: gaze.Vec2{X: 4, Y: 3}})
	// Tracking is lost: the offset must not snap to zero.
	for i := 0; i < 50; i++ {
		got := sm.Update(gaze.Sample{Lost: true})
		step := math.Hypot(got.X-prev.X, got.Y-prev.Y)
		if step > sm.MaxStep+1e-12 {
			t.Fatalf("frame %d: offset jumped %v mm during loss", i, step)
		}
		prev = got
	}
	if math.Hypot(prev.X, prev.Y) < 3 {
		t.Fatalf("offset collapsed during loss: %+v", prev)
	}
}

func TestSmoothedConverges(t *testing.T) {
	sm := NewSmoothed()
	var got gaze.Vec2
	for i := 0; i < 500; i++ {
		got = sm.Update(gaze.Sample{Measured: gaze.Vec2{X: 2, Y: -1}})
	}
	if math.Abs(got.X-2) > 0.05 || math.Abs(got.Y+1) > 0.05 {
		t.Fatalf("did not converge: %+v", got)
	}
}
