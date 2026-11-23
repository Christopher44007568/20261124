package scenario

import "testing"

func TestReverseSaccadesMirrorHorizontalGaze(t *testing.T) {
	current := ReverseSaccades(12)
	if len(current.Frames) != 12 {
		t.Fatalf("frame count = %d, want 12", len(current.Frames))
	}
	for _, frame := range current.Frames {
		sum := frame.Left.Gaze.X + frame.Right.Gaze.X
		if sum < 0.999 || sum > 1.001 {
			t.Fatalf("frame %d gazes are not mirrored: %.3f + %.3f", frame.Index,
				frame.Left.Gaze.X, frame.Right.Gaze.X)
		}
		if frame.Left.Occluded || frame.Right.Occluded {
			t.Fatalf("frame %d should have no occlusion", frame.Index)
		}
	}
}

func TestBriefOcclusionContainsVisibleAndHiddenFrames(t *testing.T) {
	current := BriefOcclusion(20)
	occludedCount := 0
	for _, frame := range current.Frames {
		if frame.Right.Occluded {
			t.Fatalf("frame %d: right eye must keep tracking", frame.Index)
		}
		if frame.Left.Occluded {
			occludedCount++
		}
	}
	if occludedCount == 0 || occludedCount == len(current.Frames) {
		t.Fatalf("occluded frames = %d, want a strict subset", occludedCount)
	}
}

func TestIdenticalGazeUsesOneViewpoint(t *testing.T) {
	current := IdenticalGaze(10)
	for _, frame := range current.Frames {
		if frame.Left.Gaze != frame.Right.Gaze {
			t.Fatalf("frame %d gaze mismatch: %+v vs %+v", frame.Index,
				frame.Left.Gaze, frame.Right.Gaze)
		}
	}
}

func TestAllContainsMandatoryScenarios(t *testing.T) {
	all := All(8)
	names := map[string]bool{}
	for _, current := range all {
		names[current.Name] = true
		if len(current.Frames) != 8 {
			t.Fatalf("%s frame count = %d, want 8", current.Name, len(current.Frames))
		}
	}
	for _, mandatory := range []string{"reverse_saccades", "brief_occlusion", "identical_gaze"} {
		if !names[mandatory] {
			t.Fatalf("mandatory scenario %q missing", mandatory)
		}
	}
}
