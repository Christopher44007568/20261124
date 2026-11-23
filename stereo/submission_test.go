package stereo

import "testing"

func TestSequentialAndBulkProduceSameResult(t *testing.T) {
	registry := NewDemoRegistry()
	cfg := EngineConfig{Width: FrameWidth, Height: FrameHeight}

	sequential := NewEngine(cfg, registry)
	seqRecords, err := sequential.SubmitSequential(CloneBatches(DemoBatches()))
	if err != nil {
		t.Fatalf("sequential submit: %v", err)
	}

	bulk := NewEngine(cfg, registry)
	bulkRecords, err := bulk.SubmitBulk(CloneBatches(DemoBatches()))
	if err != nil {
		t.Fatalf("bulk submit: %v", err)
	}

	if len(seqRecords) != len(bulkRecords) {
		t.Fatalf("record length mismatch: %d vs %d", len(seqRecords), len(bulkRecords))
	}
	for i := range seqRecords {
		if !seqRecords[i].Equal(bulkRecords[i]) {
			t.Fatalf("record %d differs:\nseq=%+v\nbulk=%+v", i, seqRecords[i], bulkRecords[i])
		}
	}
	for _, eye := range []Eye{LeftEye, RightEye} {
		if sequential.Framebuffer(eye).Hash() != bulk.Framebuffer(eye).Hash() {
			t.Fatalf("%s frame hash differs between submission modes", eye)
		}
	}
}

func TestBulkRejectsInvalidBatchAndKeepsPriorRecords(t *testing.T) {
	registry := NewDemoRegistry()
	engine := NewEngine(EngineConfig{Width: FrameWidth, Height: FrameHeight}, registry)
	batches := CloneBatches(DemoBatches())
	batches[3].GeometryName = "geom.missing"

	records, err := engine.SubmitBulk(batches)
	if err == nil {
		t.Fatal("expected validation error for unknown geometry")
	}
	if len(records) != 3 {
		t.Fatalf("prior records = %d, want 3 before the invalid batch", len(records))
	}
}

func TestStateKeyChangesOnAnyStateField(t *testing.T) {
	base := RenderState{
		Material: badgeMaterial(),
		Stencil: StencilState{
			Enabled: true, Ref: 1, Compare: CmpEqual,
			PassOp: StencilReplace, WriteMask: 0xff, ReadMask: 0xff,
		},
		Scissor: Scissor{Enabled: true, MaxX: 10, MaxY: 10},
	}
	baseKey := base.StateKey()

	materialVariant := base
	materialVariant.Material.Color[0]++
	if materialVariant.StateKey() == baseKey {
		t.Fatal("material change did not alter the state key")
	}

	stencilVariant := base
	stencilVariant.Stencil.Compare = CmpNotEqual
	if stencilVariant.StateKey() == baseKey {
		t.Fatal("stencil change did not alter the state key")
	}

	scissorVariant := base
	scissorVariant.Scissor.MaxX = 11
	if scissorVariant.StateKey() == baseKey {
		t.Fatal("scissor change did not alter the state key")
	}
}

func TestFrameHashChangesWhenPixelsChange(t *testing.T) {
	fb := NewFramebuffer(4, 4)
	before := fb.Hash()
	fb.Set(1, 2, [4]uint8{1, 2, 3, 4})
	if fb.Hash() == before {
		t.Fatal("frame hash did not change after a pixel write")
	}

	region, count, ok := PixelDiff(NewFramebuffer(4, 4), fb)
	if !ok || count != 1 || region != (Rect{MinX: 1, MinY: 2, MaxX: 2, MaxY: 3}) {
		t.Fatalf("unexpected pixel diff: ok=%v count=%d region=%+v", ok, count, region)
	}
}
