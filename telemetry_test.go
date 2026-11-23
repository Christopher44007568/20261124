package foveq

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestTelemetryRingBufferBounded(t *testing.T) {
	tel := NewTelemetry(4)
	for i := range int64(10) {
		tel.RecordDecision(Decision{Frame: i + 1})
		tel.RecordEvent(i+1, EventGpuPressure, map[string]any{"i": i})
		tel.RecordResolution(ResolutionPoint{Frame: i + 1, Tier: int(i % 3)})
	}
	if len(tel.Decisions()) != 4 || len(tel.Events()) != 4 || len(tel.Trajectory()) != 4 {
		t.Fatal("ring buffers must stay bounded at capacity")
	}
	// Oldest retained frame should be 7 (frames 7..10), proving FIFO overwrite.
	if tel.Decisions()[0].Frame != 7 || tel.Decisions()[3].Frame != 10 {
		t.Fatalf("ring overwrite order wrong: first=%d last=%d",
			tel.Decisions()[0].Frame, tel.Decisions()[3].Frame)
	}
}

func TestTelemetryDumpJSON(t *testing.T) {
	tel := NewTelemetry(8)
	tel.RecordDecision(Decision{Frame: 1, FromTier: 0, ToTier: 1, Reasons: []string{ReasonCenterError}})
	tel.RecordEvent(1, EventUpshift, map[string]any{"from": 0, "to": 1})
	tel.RecordResolution(ResolutionPoint{Frame: 1, Tier: 1, EyeWidth: 1536, EyeHeight: 1344})

	var buf bytes.Buffer
	if err := tel.Dump(&buf); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("dump must be valid JSON: %v", err)
	}
	for _, key := range []string{"decisions", "events", "trajectory"} {
		arr, ok := decoded[key].([]any)
		if !ok || len(arr) != 1 {
			t.Fatalf("dump key %q missing or wrong length", key)
		}
	}
}
