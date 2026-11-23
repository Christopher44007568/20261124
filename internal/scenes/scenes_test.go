package scenes

import (
	"testing"

	"fidelitycheck/internal/imaging"
)

func TestBuildAllKinds(t *testing.T) {
	for _, kind := range AllKinds {
		seq, err := Build(Spec{
			Kind:       kind,
			Width:      64,
			Height:     48,
			MaskRadius: 16,
			Frames:     5,
		})
		if err != nil {
			t.Fatalf("Build(%s): %v", kind, err)
		}
		if len(seq.Frames) != 5 {
			t.Fatalf("%s: got %d frames, want 5", kind, len(seq.Frames))
		}
		if seq.Mask.Count() == 0 || seq.Center.Count() == 0 || seq.Periphery.Count() == 0 {
			t.Fatalf("%s: empty mask regions", kind)
		}
		for i, frame := range seq.Frames {
			if frame.W != 64 || frame.H != 48 || len(frame.Pix) != 64*48 {
				t.Fatalf("%s frame %d has wrong shape", kind, i)
			}
		}
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	spec := Spec{Kind: SlowMotion, Width: 80, Height: 60, MaskRadius: 20, Frames: 5}
	a, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(spec)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Frames {
		for j := range a.Frames[i].Pix {
			if a.Frames[i].Pix[j] != b.Frames[i].Pix[j] {
				t.Fatalf("frame %d pixel %d differs between runs", i, j)
			}
		}
	}
}

func TestMotionActuallyMoves(t *testing.T) {
	seq, err := Build(Spec{Kind: SlowMotion, Width: 80, Height: 60, MaskRadius: 20, Frames: 5})
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	for i := range seq.Frames[0].Pix {
		if seq.Frames[0].Pix[i] != seq.Frames[len(seq.Frames)-1].Pix[i] {
			changed = true
			break
		}
	}
	if !changed {
		t.Fatal("slow-motion scene contains identical first and last frames")
	}
}

func TestCorruptedMasksOnlyMissingPixels(t *testing.T) {
	seq, err := Build(Spec{Kind: FrequencyGradient, Width: 48, Height: 40, MaskRadius: 10, Frames: 3})
	if err != nil {
		t.Fatal(err)
	}
	input := seq.Corrupted()
	for i := range seq.Frames[0].Pix {
		if seq.Mask.Bits[i] {
			if input[0].Pix[i] != (imaging.RGB{R: 0.5, G: 0.5, B: 0.5}) {
				t.Fatalf("masked pixel %d was not replaced", i)
			}
		} else if input[0].Pix[i] != seq.Frames[0].Pix[i] {
			t.Fatalf("visible pixel %d was modified", i)
		}
	}
}
