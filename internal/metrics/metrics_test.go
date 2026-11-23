package metrics

import (
	"testing"

	"fidelitycheck/internal/imaging"
	"fidelitycheck/internal/scenes"
)

func TestComputeUsesEveryFrame(t *testing.T) {
	seq := mustSequence(t, scenes.FrequencyGradient, 40, 32, 10, 5)
	outputs := make([]*imaging.Frame, len(seq.Frames))
	for i := range seq.Frames {
		outputs[i] = seq.Frames[i].Clone()
	}
	bad := outputs[len(outputs)-1]
	for i, missing := range seq.Mask.Bits {
		if missing {
			bad.Pix[i] = imaging.RGB{R: 1, G: 0, B: 0}
		}
	}

	result, _, err := Compute(seq, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.PerFrame) != len(seq.Frames) {
		t.Fatalf("got %d per-frame results, want %d", len(result.PerFrame), len(seq.Frames))
	}
	if result.PerFrame[0].CenterMAE != 0 {
		t.Fatalf("first frame should have zero error, got %.6f", result.PerFrame[0].CenterMAE)
	}
	if result.PerFrame[len(result.PerFrame)-1].CenterMAE <= 0 {
		t.Fatal("last-frame center error was not measured")
	}
	if result.CenterP95 <= 0 || result.CenterMax <= 0 {
		t.Fatalf("tail stability was not aggregated: p95=%f max=%f", result.CenterP95, result.CenterMax)
	}
	if len(result.TemporalError) != len(seq.Frames)-1 {
		t.Fatalf("got %d temporal samples, want %d", len(result.TemporalError), len(seq.Frames)-1)
	}
	if result.TemporalError[len(result.TemporalError)-1] <= 0 {
		t.Fatal("temporal residual on the final transition was not measured")
	}
}

func TestComputeDetectsAlternatingFlicker(t *testing.T) {
	seq := mustSequence(t, scenes.SlowMotion, 48, 40, 12, 7)
	outputs := make([]*imaging.Frame, len(seq.Frames))
	for i := range seq.Frames {
		outputs[i] = seq.Frames[i].Clone()
		offset := 0.20
		if i%2 == 1 {
			offset = -0.20
		}
		for p, missing := range seq.Mask.Bits {
			if missing {
				c := outputs[i].Pix[p]
				c.R += offset
				c.G += offset
				c.B += offset
				outputs[i].Pix[p] = c.Clamp()
			}
		}
	}

	result, _, err := Compute(seq, outputs)
	if err != nil {
		t.Fatal(err)
	}
	if result.FlickerMean <= 0.01 || result.FlickerP95 <= 0.01 {
		t.Fatalf("alternating frames were not detected as flicker: mean=%f p95=%f", result.FlickerMean, result.FlickerP95)
	}
	if result.FlickerMax < result.FlickerP95 {
		t.Fatal("maximum flicker is below P95")
	}
}

func TestComputeRejectsShortOutputSequence(t *testing.T) {
	seq := mustSequence(t, scenes.ColoredEdges, 32, 24, 8, 3)
	if _, _, err := Compute(seq, seq.Frames[:2]); err == nil {
		t.Fatal("Compute accepted a missing output frame")
	}
}

func mustSequence(t *testing.T, kind scenes.Kind, w, h int, radius float64, frames int) *scenes.Sequence {
	t.Helper()
	seq, err := scenes.Build(scenes.Spec{
		Kind:       kind,
		Width:      w,
		Height:     h,
		MaskRadius: radius,
		Frames:     frames,
	})
	if err != nil {
		t.Fatal(err)
	}
	return seq
}
