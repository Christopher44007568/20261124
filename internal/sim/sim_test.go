package sim

import (
	"testing"

	"pupilsim/internal/optics"
)

func TestSmoothedBeatsPerEyeOnLossJumps(t *testing.T) {
	lens := optics.DefaultLens()
	sc := Scenario{Name: "loss", Frames: 1200, LossProb: 0.02}
	_, perEye := Run(sc, lens, func() optics.Compensator { return optics.PerEye{} })
	_, smooth := Run(sc, lens, func() optics.Compensator { return optics.NewSmoothed() })
	if smooth.MaxOffsetJumpLoss >= perEye.MaxOffsetJumpLoss {
		t.Fatalf("smoothed loss jump %.2f not below per-eye %.2f",
			smooth.MaxOffsetJumpLoss, perEye.MaxOffsetJumpLoss)
	}
	if smooth.RMSDrift >= perEye.RMSDrift*1.5 {
		t.Fatalf("smoothed RMS drift %.2f too high vs per-eye %.2f",
			smooth.RMSDrift, perEye.RMSDrift)
	}
}

func TestCompensationReducesDrift(t *testing.T) {
	lens := optics.DefaultLens()
	sc := Scenario{Name: "saccade", Frames: 1200, Events: Scenarios()[1].Events}
	_, fixed := Run(sc, lens, func() optics.Compensator { return optics.Fixed{} })
	_, perEye := Run(sc, lens, func() optics.Compensator { return optics.PerEye{} })
	if perEye.RMSDrift >= fixed.RMSDrift {
		t.Fatalf("per-eye RMS %.2f not below fixed %.2f",
			perEye.RMSDrift, fixed.RMSDrift)
	}
}
