package shadow

import "testing"

func TestDefaultSceneSplitsVisibilityByEye(t *testing.T) {
	s := DefaultScene()
	got := s.CasterMasks(0, s.BasePitch)
	want := map[string]EyeMask{
		"occluder-wall":  BothEyes,
		"left-eye-rod":   LeftEye,
		"right-eye-rod":  RightEye,
		"cross-eye-near": BothEyes,
		"cross-eye-far":  BothEyes,
	}
	for i, c := range s.Casters {
		if got[i] != want[c.Name] {
			t.Errorf("%s mask = %02b, want %02b", c.Name, got[i], want[c.Name])
		}
	}
}

func TestCasterMasksReactToHeadTurn(t *testing.T) {
	s := DefaultScene()
	base := s.CasterMasks(0, s.BasePitch)
	baseLeft := countMask(base, LeftEye)
	foundDifference := false
	for _, yaw := range []float64{-0.55, -0.35, 0.35, 0.55} {
		m := s.CasterMasks(yaw, s.BasePitch)
		for i := range m {
			if m[i] != base[i] {
				foundDifference = true
				break
			}
		}
		if foundDifference {
			break
		}
	}
	if !foundDifference {
		t.Fatal("head turn did not change any per-eye caster mask")
	}
	if baseLeft == 0 {
		t.Fatal("left eye sees no casters at rest")
	}
}
