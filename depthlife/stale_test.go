package depthlife

import (
	"errors"
	"testing"
)

// TestStaleEyeDataCaptured simulates the production bug: the right-eye slot
// ends up holding a parsed result tagged for the left eye. The lifecycle
// guard must refuse to serve it and must record a stale-eye audit event.
func TestStaleEyeDataCaptured(t *testing.T) {
	m, log := newTestManager(t)
	leftAtt := prepareParsed(t, m, Left, 1, []uint16{10, 20, 30, 40})
	rightAtt := prepareParsed(t, m, Right, 1, []uint16{1, 1, 1, 1})

	rightSlot := m.slots[Right-1]
	leftDepth := m.slots[Left-1].depth
	if leftDepth.Eye != Left {
		t.Fatalf("test setup: left depth not tagged")
	}
	rightSlot.depth = leftDepth

	if _, err := m.Parse(Right); !errors.Is(err, ErrStaleEye) {
		t.Fatalf("Parse with stale eye err=%v, want ErrStaleEye", err)
	}
	ev, ok := log.Last(EvStaleEye, Right)
	if !ok {
		t.Fatalf("stale-eye event not recorded; events=%+v", log.Events())
	}
	if ev.Detail == "" || ev.AttID != rightAtt.ID {
		t.Fatalf("stale-eye event missing context: %+v", ev)
	}

	if _, err := m.Reuse(Right, rightAtt.ID, 9); !errors.Is(err, ErrStaleEye) {
		t.Fatalf("Reuse with stale eye err=%v, want ErrStaleEye", err)
	}
	if log.CountByKind(EvStaleEye, Right) < 2 {
		t.Fatalf("stale events = %d, want >= 2", log.CountByKind(EvStaleEye, Right))
	}

	if err := m.Clear(Right); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := m.Parse(Right); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("after Clear err=%v, want ErrNotLoaded", err)
	}
	if m.slots[Left-1].depth == nil || m.slots[Left-1].depth.AttID != leftAtt.ID {
		t.Fatalf("clearing right slot must not touch left slot")
	}
}

func TestRestoreRejectsForeignEyeAndMachine(t *testing.T) {
	hostLog := NewMemoryLogger()
	host := New(Host, hostLog)
	prepareParsed(t, host, Left, 1, []uint16{100, 100, 100, 100})
	mask, err := host.ApplyMask(Left, 50, 0.99)
	if err != nil || !mask.Passed {
		t.Fatalf("host mask setup failed: %+v err=%v", mask, err)
	}
	_, blob, err := host.Save(Left)
	if err != nil {
		t.Fatalf("host Save: %v", err)
	}

	rightLog := NewMemoryLogger()
	rightMgr := New(RightMachine, rightLog)
	if _, err := rightMgr.Restore(Right, blob); !errors.Is(err, ErrStaleEye) {
		t.Fatalf("foreign machine restore err=%v, want ErrStaleEye", err)
	}
	if _, err := host.Restore(Right, blob); !errors.Is(err, ErrStaleEye) {
		t.Fatalf("foreign eye restore err=%v, want ErrStaleEye", err)
	}
	if rightLog.CountByKind(EvStaleEye, 0) != 1 || hostLog.CountByKind(EvStaleEye, Right) != 1 {
		t.Fatalf("stale-eye restore events missing: host=%+v right=%+v",
			hostLog.Events(), rightLog.Events())
	}

	tpl, err := host.Restore(Left, blob)
	if err != nil || tpl.Eye != Left || tpl.Machine != Host {
		t.Fatalf("same-machine same-eye restore failed: %v", err)
	}
}

func TestStaleAttachmentGuardOnParse(t *testing.T) {
	m, log := newTestManager(t)
	att := prepareParsed(t, m, Left, 1, []uint16{10, 20, 30, 40})

	stale := parseDepth(Attachment{ID: "stale-attachment-id", Version: 1,
		Width: 2, Height: 2, Raw: att.Raw}, Left)
	m.slots[Left-1].depth = stale

	if _, err := m.Parse(Left); !errors.Is(err, ErrStaleAttachment) {
		t.Fatalf("Parse err=%v, want ErrStaleAttachment", err)
	}
	if log.CountByKind(EvStaleAttachment, Left) != 1 {
		t.Fatalf("stale-attachment event count = %d, want 1",
			log.CountByKind(EvStaleAttachment, Left))
	}
}
