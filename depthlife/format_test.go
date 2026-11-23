package depthlife

import (
	"errors"
	"testing"
)

func TestAttachmentFormatChangeInvalidatesArtifacts(t *testing.T) {
	m, log := newTestManager(t)
	v1 := makeAttachment(t, 1, 2, 2, []uint16{100, 100, 100, 100})
	att1, err := m.Load(Left, v1, 1)
	if err != nil {
		t.Fatalf("Load v1: %v", err)
	}
	if _, err := m.Parse(Left); err != nil {
		t.Fatalf("Parse v1: %v", err)
	}
	mask, err := m.ApplyMask(Left, 50, 0.99)
	if err != nil || !mask.Passed {
		t.Fatalf("v1 mask: %+v err=%v", mask, err)
	}
	tpl, _, err := m.Save(Left)
	if err != nil {
		t.Fatalf("v1 Save: %v", err)
	}
	if tpl.Version != 1 {
		t.Fatalf("template version = %d, want 1", tpl.Version)
	}

	v2 := makeAttachment(t, 2, 2, 2, []uint16{200, 200, 200, 200})
	att2, err := m.Load(Left, v2, 2)
	if err != nil {
		t.Fatalf("Load v2: %v", err)
	}
	if att1.ID == att2.ID {
		t.Fatalf("format change must change attachment identity")
	}

	ev, ok := log.Last(EvFormatChanged, Left)
	if !ok || ev.FormatVer != 2 {
		t.Fatalf("format-changed event wrong: %+v ok=%v", ev, ok)
	}
	snap, _ := m.Snapshot(Left)
	if snap.State != Loaded || snap.HasDepth || snap.HasMask || snap.HasTemplate {
		t.Fatalf("format change did not invalidate artifacts: %+v", snap)
	}
	if _, _, err := m.Save(Left); !errors.Is(err, ErrNotParsed) {
		t.Fatalf("Save after format change err=%v, want ErrNotParsed", err)
	}

	d2, err := m.Parse(Left)
	if err != nil {
		t.Fatalf("Parse v2: %v", err)
	}
	if d2.Version != 2 || d2.MeanZ != 200 || d2.AttID != att2.ID {
		t.Fatalf("v2 depth result stale: %+v", d2)
	}
	mask2, err := m.ApplyMask(Left, 150, 0.99)
	if err != nil || !mask2.Passed || mask2.AttID != att2.ID {
		t.Fatalf("v2 mask stale: %+v err=%v", mask2, err)
	}
	tpl2, _, err := m.Save(Left)
	if err != nil || tpl2.Version != 2 {
		t.Fatalf("v2 template: %+v err=%v", tpl2, err)
	}
}

func TestSameVersionNewPayloadNoStaleReuse(t *testing.T) {
	m, log := newTestManager(t)
	raw1 := makeAttachment(t, 1, 2, 2, []uint16{10, 10, 10, 10})
	att1, _ := m.Load(Left, raw1, 1)
	if _, err := m.Parse(Left); err != nil {
		t.Fatalf("Parse: %v", err)
	}

	raw2 := makeAttachment(t, 1, 2, 2, []uint16{90, 90, 90, 90})
	att2, _ := m.Load(Left, raw2, 2)
	if att1.ID == att2.ID {
		t.Fatalf("different payload must differ in identity")
	}
	snap, _ := m.Snapshot(Left)
	if snap.State != Loaded || snap.HasDepth {
		t.Fatalf("new payload must drop parsed depth: %+v", snap)
	}
	if log.CountByKind(EvFormatChanged, Left) != 0 {
		t.Fatalf("same version must not emit format-changed")
	}
	if _, err := m.Reuse(Left, att1.ID, 2); !errors.Is(err, ErrAttachmentMoved) {
		t.Fatalf("old attachment reuse err=%v", err)
	}
	d, err := m.Parse(Left)
	if err != nil || d.AttID != att2.ID || d.MeanZ != 90 {
		t.Fatalf("re-parse produced stale data: %+v err=%v", d, err)
	}
}
