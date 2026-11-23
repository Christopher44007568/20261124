package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMarkdown(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run(nil, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	text := stdout.String()
	for _, want := range []string{
		"Compositor occlusion comparison",
		"submission-no-depth",
		"submission-sparse-depth",
		"submission-full-depth",
		"Occlusion errors",
		"near-ui-quantization",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestRunJSONToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-format", "json", "-out", path}, &stdout, &stderr); err != nil {
		t.Fatalf("run: %v", err)
	}
	if stdout.Len() != 0 {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode report: %v", err)
	}
	if _, ok := decoded["submissions"]; !ok {
		t.Fatal("JSON report has no submissions")
	}
}

func TestRunRejectsUnknownFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{"-format", "xml"}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("got %v, want unsupported format error", err)
	}
}
