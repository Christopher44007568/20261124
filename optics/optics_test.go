package optics

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func validEye() *EyeParams {
	return &EyeParams{
		Distortion:          &Distortion{Coefficients: []float64{0.12, -0.03}},
		ChromaticAberration: &ChromaticAberration{Scale: [3]float64{1.001, 1.0, 0.999}},
		PanelCenter:         &PanelCenter{X: 0.5, Y: 0.5},
		SafetyBoundary:      &SafetyBoundary{MinX: 0.05, MinY: 0.05, MaxX: 0.95, MaxY: 0.95},
	}
}

func validConfig() *Config {
	return &Config{Version: CurrentVersion, Left: validEye(), Right: validEye()}
}

func problems(err error) []FieldError {
	var verr *ValidationError
	if errors.As(err, &verr) {
		return verr.Problems
	}
	return nil
}

func hasField(err error, field string) bool {
	for _, p := range problems(err) {
		if p.Field == field {
			return true
		}
	}
	return false
}

func TestValidConfigPasses(t *testing.T) {
	if err := Validate(validConfig()); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
}

func TestMissingFieldReported(t *testing.T) {
	cfg := validConfig()
	cfg.Left.Distortion = nil
	cfg.Right.SafetyBoundary = nil
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !hasField(err, "left.distortion") {
		t.Errorf("missing left.distortion not reported: %v", err)
	}
	if !hasField(err, "right.safety_boundary") {
		t.Errorf("missing right.safety_boundary not reported: %v", err)
	}
}

func TestVersionIncompatible(t *testing.T) {
	cfg := validConfig()
	cfg.Version = CurrentVersion + 1
	err := Validate(cfg)
	if !hasField(err, "version") {
		t.Fatalf("version incompatibility not reported: %v", err)
	}
}

func TestEyeFormatMismatch(t *testing.T) {
	cfg := validConfig()
	cfg.Right.Distortion.Coefficients = []float64{0.1}
	err := Validate(cfg)
	if !hasField(err, "distortion.coefficients") {
		t.Fatalf("left/right format mismatch not reported: %v", err)
	}
}

func TestRoundtripAccuracy(t *testing.T) {
	report, err := CheckRoundtrip(validConfig(), 17)
	if err != nil {
		t.Fatalf("roundtrip failed: %v", err)
	}
	if report.MaxError > RoundtripTolerance {
		t.Fatalf("max error %g exceeds tolerance", report.MaxError)
	}
	t.Logf("samples=%d maxErr=%g", report.Samples, report.MaxError)
}

func TestManagerKeepsLastGood(t *testing.T) {
	m := NewManager(filepath.Join(t.TempDir(), "cfg.json"))
	good := validConfig()
	if err := m.Apply(good); err != nil {
		t.Fatalf("good config rejected: %v", err)
	}

	bad := validConfig()
	bad.Left.PanelCenter = nil
	if err := m.Apply(bad); err == nil {
		t.Fatal("bad config accepted")
	} else if !strings.Contains(err.Error(), "left.panel_center") {
		t.Fatalf("error does not name the field: %v", err)
	}
	if m.Active() != good {
		t.Fatal("active config was displaced by a rejected candidate")
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	m := NewManager(path)
	if err := m.Apply(validConfig()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	m2 := NewManager(path)
	if err := m2.Reload(); err != nil {
		t.Fatalf("reload: %v", err)
	}
	if m2.Active() == nil || m2.Active().Version != CurrentVersion {
		t.Fatal("reloaded config mismatch")
	}
}

func TestReloadCorruptKeepsActive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg.json")
	m := NewManager(path)
	good := validConfig()
	if err := m.Apply(good); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Corrupt the file: downgrade nothing, break JSON.
	if err := os.WriteFile(path, []byte(`{"version": 99,`), 0o644); err != nil {
		t.Fatalf("corrupt: %v", err)
	}
	if err := m.Reload(); err == nil {
		t.Fatal("corrupt reload accepted")
	}
	if m.Active() != good {
		t.Fatal("active config lost after corrupt reload")
	}
}
