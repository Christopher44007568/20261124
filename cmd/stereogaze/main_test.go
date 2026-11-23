package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesAllScenarioArtifacts(t *testing.T) {
	output := filepath.Join(t.TempDir(), "out")
	if err := run(output, 12, 4); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	for _, name := range []string{"reverse_saccades", "brief_occlusion", "identical_gaze"} {
		if _, err := os.Stat(filepath.Join(output, name+".gif")); err != nil {
			t.Fatalf("missing %s GIF: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(output, name, "frame_003.png")); err != nil {
			t.Fatalf("missing %s final PNG: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "records.json")); err != nil {
		t.Fatalf("missing records: %v", err)
	}
}

func TestRunRejectsBadGrid(t *testing.T) {
	if err := run(filepath.Join(t.TempDir(), "out"), 0, 2); err == nil {
		t.Fatal("expected error for non-positive grid size")
	}
}
