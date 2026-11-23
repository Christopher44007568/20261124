package ae

import "testing"

func TestScenariosHaveExpectedTransitions(t *testing.T) {
	scenarios := Scenarios()
	if len(scenarios) != 4 {
		t.Fatalf("scenario count = %d, want 4", len(scenarios))
	}
	expectedCuts := map[string]int{
		"black-field":     1,
		"white-field":     1,
		"flash":           2,
		"local-highlight": 1,
	}
	for _, scenario := range scenarios {
		if len(scenario.Frames) != scenarioFrames {
			t.Fatalf("%s has %d frames, want %d", scenario.Name, len(scenario.Frames), scenarioFrames)
		}
		cuts := 0
		for _, frame := range scenario.Frames {
			if frame.SceneCut {
				cuts++
			}
			if err := frame.Validate(); err != nil {
				t.Fatalf("%s: %v", scenario.Name, err)
			}
		}
		if cuts != expectedCuts[scenario.Name] {
			t.Fatalf("%s has %d cuts, want %d", scenario.Name, cuts, expectedCuts[scenario.Name])
		}
		if !scenario.Frames[stepFrame].SceneCut {
			t.Fatalf("%s does not mark its primary transition", scenario.Name)
		}
	}
}

func TestApplyExposureClipsAtSensorMaximum(t *testing.T) {
	frame := NewUniformFrame(2, 2, 0.6, false)
	exposed := ApplyExposure(frame, 1)
	for i, value := range exposed.Luma {
		if value != 1 {
			t.Fatalf("luma[%d] = %g, want clipped value 1", i, value)
		}
	}
	if frame.Luma[0] != 0.6 {
		t.Fatalf("ApplyExposure mutated its input: %g", frame.Luma[0])
	}
}
