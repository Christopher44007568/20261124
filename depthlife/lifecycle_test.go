package depthlife

import (
	"errors"
	"testing"
)

func makeAttachment(t *testing.T, version uint32, w, h int, samples []uint16) []byte {
	t.Helper()
	return EncodeAttachment(version, uint16(w), uint16(h), samples)
}

func newTestManager(t *testing.T) (*Manager, *MemoryLogger) {
	t.Helper()
	log := NewMemoryLogger()
	return New(Host, log), log
}

func TestClearLoadParseHappyPath(t *testing.T) {
	m, log := newTestManager(t)
	raw := makeAttachment(t, 1, 2, 2, []uint16{10, 20, 30, 40})

	if states := m.States(); states[Left] != Empty || states[Right] != Empty {
		t.Fatalf("initial states = %v, want both empty", states)
	}
	if _, err := m.Parse(Left); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("Parse before Load err=%v, want ErrNotLoaded", err)
	}
	att, err := m.Load(Left, raw, 1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := m.States()[Left]; got != Loaded {
		t.Fatalf("state after Load = %s, want loaded", got)
	}
	d, err := m.Parse(Left)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Eye != Left || d.AttID != att.ID || d.MeanZ != 25 {
		t.Fatalf("unexpected depth result: %+v", d)
	}
	if m.States()[Left] != Parsed {
		t.Fatalf("state after Parse = %s, want parsed", m.States()[Left])
	}

	if err := m.Clear(Left); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	snap, _ := m.Snapshot(Left)
	if snap.State != Empty || snap.HasDepth || snap.AttID != "" {
		t.Fatalf("slot not cleared: %+v", snap)
	}
	if log.CountByKind(EvCleared, Left) != 1 {
		t.Fatalf("cleared events = %d, want 1", log.CountByKind(EvCleared, Left))
	}
}

func TestCrossFrameReuseSameAttachment(t *testing.T) {
	m, log := newTestManager(t)
	raw := makeAttachment(t, 1, 2, 2, []uint16{5, 5, 5, 5})
	att, _ := m.Load(Left, raw, 1)
	first, err := m.Parse(Left)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if _, err := m.Load(Left, raw, 2); err != nil {
		t.Fatalf("re-Load: %v", err)
	}
	res, err := m.Reuse(Left, att.ID, 2)
	if err != nil {
		t.Fatalf("Reuse: %v", err)
	}
	if !res.Reused || res.Depth != first {
		t.Fatalf("expected cached depth reuse, got %+v", res)
	}
	second, err := m.Parse(Left)
	if err != nil || second != first {
		t.Fatalf("second Parse must return same object, err=%v", err)
	}
	if log.CountByKind(EvParseReused, Left) != 1 {
		t.Fatalf("parse-reused events = %d, want 1", log.CountByKind(EvParseReused, Left))
	}

	if _, err := m.Reuse(Left, "deadbeef", 3); !errors.Is(err, ErrAttachmentMoved) {
		t.Fatalf("foreign attachment reuse err=%v, want ErrAttachmentMoved", err)
	}
	if log.CountByKind(EvReuseRejected, Left) != 1 {
		t.Fatalf("reuse-rejected events = %d, want 1", log.CountByKind(EvReuseRejected, Left))
	}
}
