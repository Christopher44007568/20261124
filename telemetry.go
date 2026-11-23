package foveq

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// EventKind categorizes quality events recorded on the telemetry sink.
type EventKind string

const (
	EventUpshift       EventKind = "upshift"
	EventDownshift     EventKind = "downshift"
	EventBlockedUp     EventKind = "upshift_blocked"
	EventSpikeFreeze   EventKind = "load_spike_freeze"
	EventGazeFreeze    EventKind = "fast_gaze_freeze"
	EventCooldown      EventKind = "cooldown_hold"
	EventClamp         EventKind = "boundary_clamp"
	EventFrameFailure  EventKind = "frame_failure"
	EventTrialRollback EventKind = "trial_rollback"
	EventTrialPromote  EventKind = "trial_promoted"
	EventGpuPressure   EventKind = "gpu_pressure"
)

// Event is one quality-related occurrence.
type Event struct {
	Frame  int64          `json:"frame"`
	At     time.Time      `json:"at"`
	Kind   EventKind      `json:"kind"`
	Detail map[string]any `json:"detail,omitempty"`
}

// ResolutionPoint is one entry in the resolution trajectory: the active tier,
// its eye-buffer size and foveal quality at a frame.
type ResolutionPoint struct {
	Frame        int64   `json:"frame"`
	Tier         int     `json:"tier"`
	EyeWidth     int     `json:"eye_width"`
	EyeHeight    int     `json:"eye_height"`
	FoveaScale   float64 `json:"fovea_scale"`
	BudgetMicros float64 `json:"budget_micros"`
}

// Telemetry is a thread-safe, bounded record of controller activity. Bounded
// ring buffers keep long-running sessions from growing without limit while
// preserving the most recent history.
type Telemetry struct {
	mu         sync.Mutex
	decisions  []Decision
	dHead      int
	dCount     int
	events     []Event
	eHead      int
	eCount     int
	trajectory []ResolutionPoint
	tHead      int
	tCount     int
}

// NewTelemetry builds a sink with per-stream capacity entries.
func NewTelemetry(capacity int) *Telemetry {
	if capacity < 1 {
		capacity = 1
	}
	return &Telemetry{
		decisions:  make([]Decision, capacity),
		events:     make([]Event, capacity),
		trajectory: make([]ResolutionPoint, capacity),
	}
}

func ringPut[T any](buf []T, head, count *int, v T) {
	if *count < len(buf) {
		buf[*count] = v
		*count++
		return
	}
	buf[*head] = v
	*head = (*head + 1) % len(buf)
}

func ringGet[T any](buf []T, head, count int) []T {
	out := make([]T, count)
	for i := range count {
		out[i] = buf[(head+i)%len(buf)]
	}
	return out
}

// RecordDecision stores a full controller evaluation.
func (t *Telemetry) RecordDecision(d Decision) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ringPut(t.decisions, &t.dHead, &t.dCount, d)
}

// RecordEvent stores a quality event.
func (t *Telemetry) RecordEvent(frame int64, kind EventKind, detail map[string]any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ringPut(t.events, &t.eHead, &t.eCount, Event{
		Frame: frame, At: time.Now(), Kind: kind, Detail: detail,
	})
}

// RecordResolution appends one point to the resolution trajectory.
func (t *Telemetry) RecordResolution(p ResolutionPoint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ringPut(t.trajectory, &t.tHead, &t.tCount, p)
}

// Decisions returns a chronological copy of stored evaluations.
func (t *Telemetry) Decisions() []Decision {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ringGet(t.decisions, t.dHead, t.dCount)
}

// Events returns a chronological copy of stored events.
func (t *Telemetry) Events() []Event {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ringGet(t.events, t.eHead, t.eCount)
}

// Trajectory returns a chronological copy of the resolution trajectory.
func (t *Telemetry) Trajectory() []ResolutionPoint {
	t.mu.Lock()
	defer t.mu.Unlock()
	return ringGet(t.trajectory, t.tHead, t.tCount)
}

// Dump writes the complete telemetry history as one JSON object to w.
func (t *Telemetry) Dump(w io.Writer) error {
	t.mu.Lock()
	payload := struct {
		Decisions  []Decision        `json:"decisions"`
		Events     []Event           `json:"events"`
		Trajectory []ResolutionPoint `json:"trajectory"`
	}{
		Decisions:  ringGet(t.decisions, t.dHead, t.dCount),
		Events:     ringGet(t.events, t.eHead, t.eCount),
		Trajectory: ringGet(t.trajectory, t.tHead, t.tCount),
	}
	t.mu.Unlock()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}
