package record

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"stereogaze/internal/gaze"
	"stereogaze/internal/scenario"
)

func TestRecorderWritesArtifactsAndHoldsInvariant(t *testing.T) {
	output := t.TempDir()
	recorder, err := NewRecorder(output)
	if err != nil {
		t.Fatalf("NewRecorder: %v", err)
	}

	for _, current := range scenario.All(6) {
		metrics := recorder.BeginScenario(current.Name, 16)
		for _, frame := range current.Frames {
			left, err := gaze.BuildMask(frame.Left, 16)
			if err != nil {
				t.Fatalf("left mask: %v", err)
			}
			right, err := gaze.BuildMask(frame.Right, 16)
			if err != nil {
				t.Fatalf("right mask: %v", err)
			}
			report, err := gaze.AnalyzeSharing(left, right)
			if err != nil {
				t.Fatalf("sharing: %v", err)
			}
			if err := recorder.AddFrame(metrics, current.Name, frame.Index, 16, left, right, report); err != nil {
				t.Fatalf("AddFrame: %v", err)
			}
		}
		recorder.CommitScenario(metrics)
	}
	if err := recorder.Finish(); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	for _, name := range []string{"reverse_saccades", "brief_occlusion", "identical_gaze"} {
		if _, err := os.Stat(filepath.Join(output, name+".gif")); err != nil {
			t.Fatalf("missing GIF for %s: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(output, name, "frame_000.png")); err != nil {
			t.Fatalf("missing PNG for %s: %v", name, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(output, "records.json"))
	if err != nil {
		t.Fatalf("read records: %v", err)
	}
	var records []ScenarioMetrics
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("scenario records = %d, want 3", len(records))
	}

	for _, current := range records {
		if len(current.Frames) != 6 {
			t.Fatalf("%s frame metrics = %d, want 6", current.Scenario, len(current.Frames))
		}
		for _, frame := range current.Frames {
			if !frame.InvariantHeld {
				t.Fatalf("%s frame %d violated sharing invariant", current.Scenario, frame.Frame)
			}
			if frame.WorkSaved < 0 || frame.WorkSaved > frame.StandaloneWork {
				t.Fatalf("%s frame %d has impossible saved work %d", current.Scenario, frame.Frame, frame.WorkSaved)
			}
		}
	}
}
