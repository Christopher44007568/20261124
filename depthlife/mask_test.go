package depthlife

import (
	"errors"
	"testing"
)

func prepareParsed(t *testing.T, m *Manager, eye Eye, version uint32, samples []uint16) *Attachment {
	t.Helper()
	raw := makeAttachment(t, version, 2, 2, samples)
	att, err := m.Load(eye, raw, 1)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := m.Parse(eye); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return att
}

func TestMaskSingleEyePassAndSave(t *testing.T) {
	m, log := newTestManager(t)
	prepareParsed(t, m, Left, 1, []uint16{100, 100, 1, 1})

	mask, err := m.ApplyMask(Left, 50, 0.5)
	if err != nil {
		t.Fatalf("ApplyMask: %v", err)
	}
	if !mask.Passed || mask.Coverage != 0.5 {
		t.Fatalf("mask = %+v, want passed coverage 0.5", mask)
	}
	tpl, blob, err := m.Save(Left)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if tpl.Eye != Left || tpl.Machine != Host || len(blob) == 0 {
		t.Fatalf("bad template: %+v blob=%d", tpl, len(blob))
	}
	round, err := DecodeTemplate(blob)
	if err != nil || round.AttID != tpl.AttID || round.Eye != Left {
		t.Fatalf("template round-trip failed: %+v err=%v", round, err)
	}

	rightSnap, _ := m.Snapshot(Right)
	if rightSnap.State != Empty || rightSnap.HasDepth || rightSnap.HasMask {
		t.Fatalf("right slot must stay empty: %+v", rightSnap)
	}
	if log.CountByKind(EvMaskApplied, Left) != 1 || log.CountByKind(EvSaved, Left) != 1 {
		t.Fatalf("missing mask/save events: %+v", log.Events())
	}
	if log.CountByKind(EvMaskApplied, Right) != 0 {
		t.Fatalf("right eye must not receive left events")
	}
}

func TestMaskRejectedBlocksSave(t *testing.T) {
	m, log := newTestManager(t)
	prepareParsed(t, m, Right, 1, []uint16{1, 2, 3, 4})

	mask, err := m.ApplyMask(Right, 100, 0.9)
	if err != nil {
		t.Fatalf("ApplyMask: %v", err)
	}
	if mask.Passed {
		t.Fatalf("mask should be rejected: %+v", mask)
	}
	snap, _ := m.Snapshot(Right)
	if !snap.HasMask || snap.MaskPassed || snap.State != Masked {
		t.Fatalf("rejected mask snapshot wrong: %+v", snap)
	}
	if _, _, err := m.Save(Right); !errors.Is(err, ErrTemplateRejected) {
		t.Fatalf("Save err=%v, want ErrTemplateRejected", err)
	}
	if log.CountByKind(EvMaskRejected, Right) < 1 {
		t.Fatalf("mask-rejected event not logged")
	}
	if log.CountByKind(EvSaved, Right) != 0 {
		t.Fatalf("rejected template must not be saved")
	}
}

func TestLeftParsedRightIncomplete(t *testing.T) {
	m, _ := newTestManager(t)
	att := prepareParsed(t, m, Left, 1, []uint16{10, 20, 30, 40})

	if _, err := m.Reuse(Right, att.ID, 1); !errors.Is(err, ErrEmptySlot) {
		t.Fatalf("right Reuse err=%v, want ErrEmptySlot", err)
	}
	if _, err := m.ApplyMask(Right, 1, 0.1); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("right ApplyMask err=%v, want ErrNotLoaded", err)
	}
	if _, _, err := m.Save(Right); !errors.Is(err, ErrNotLoaded) {
		t.Fatalf("right Save err=%v, want ErrNotLoaded", err)
	}

	rightRaw := makeAttachment(t, 1, 2, 2, []uint16{9, 9, 9, 9})
	rAtt, err := m.Load(Right, rightRaw, 1)
	if err != nil {
		t.Fatalf("right Load: %v", err)
	}
	if rAtt.ID == att.ID {
		t.Fatalf("left/right attachments must differ")
	}
	if _, err := m.Reuse(Right, att.ID, 2); !errors.Is(err, ErrAttachmentMoved) {
		t.Fatalf("right slot must reject left attachment id, err=%v", err)
	}

	leftSnap, _ := m.Snapshot(Left)
	rightSnap, _ := m.Snapshot(Right)
	if leftSnap.State != Parsed || rightSnap.State != Loaded {
		t.Fatalf("states = left:%s right:%s", leftSnap.State, rightSnap.State)
	}
}
