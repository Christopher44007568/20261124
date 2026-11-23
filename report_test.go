package gazedenoise

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunExperimentWritesComparableEvidence(t *testing.T) {
	out := t.TempDir()
	opts := DefaultExperimentOptions()
	opts.OutDir = filepath.Join(out, "report")
	opts.Width = 72
	opts.Height = 54
	opts.Noise = 0.025
	opts.Scenes = []SceneKind{SceneThinRod, SceneCombined}

	report, err := RunExperiment(opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 2 {
		t.Fatalf("got %d cases", len(report.Cases))
	}
	for _, c := range report.Cases {
		if len(c.Modes) != 3 {
			t.Fatalf("%s has %d modes", c.Kind, len(c.Modes))
		}
		var sawPartition bool
		for _, mode := range c.Modes {
			if mode.Mode == ModePartitioned.String() {
				sawPartition = true
				if mode.Metrics.CrossPartitionTaps != 0 {
					t.Fatalf("%s partition leaked %d taps", c.Kind, mode.Metrics.CrossPartitionTaps)
				}
				if !mode.Metrics.ProvenanceValid {
					t.Fatalf("%s provenance invalid: %s", c.Kind, mode.Metrics.ProvenanceError)
				}
			}
			for _, rel := range []string{mode.Image, mode.DifferenceImage} {
				if err := decodePNG(filepath.Join(opts.OutDir, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("%s: %v", rel, err)
				}
			}
			provData, err := os.ReadFile(filepath.Join(opts.OutDir, filepath.FromSlash(mode.ProvenanceJSON)))
			if err != nil {
				t.Fatalf("%s: %v", mode.ProvenanceJSON, err)
			}
			var provDoc ProvenanceDocument
			if err := json.Unmarshal(provData, &provDoc); err != nil {
				t.Fatalf("%s: %v", mode.ProvenanceJSON, err)
			}
			if provDoc.FilledHoles != mode.Metrics.FilledHoles || len(provDoc.Holes) != provDoc.FilledHoles {
				t.Fatalf("%s provenance count mismatch", mode.ProvenanceJSON)
			}
		}
		if !sawPartition {
			t.Fatalf("%s lacks partitioned result", c.Kind)
		}
		for _, rel := range []string{
			c.TruthImage, c.InputImage, c.MaskImage, c.NormalImage,
			c.DepthImage, c.LumaImage, c.ProvenanceImage,
		} {
			if err := decodePNG(filepath.Join(opts.OutDir, filepath.FromSlash(rel))); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
		}
	}

	jsonPath := filepath.Join(opts.OutDir, "report.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode report.json: %v", err)
	}
	if decoded.Format != "gazedenoise/v1" {
		t.Fatalf("unexpected format %q", decoded.Format)
	}
	md, err := os.ReadFile(filepath.Join(opts.OutDir, "report.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"分区审计：通过", "补洞来源审计：通过", "thin_rod"} {
		if !strings.Contains(string(md), text) {
			t.Fatalf("report.md lacks %q", text)
		}
	}
}

func TestModuleHasNoExternalRequirements(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "require ") {
		t.Fatalf("go.mod must not require external modules:\n%s", data)
	}
}

func decodePNG(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = png.Decode(file)
	return err
}
