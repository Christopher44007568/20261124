package stereo

import "testing"

func testConfig() EngineConfig {
	return EngineConfig{Width: FrameWidth, Height: FrameHeight, KeepSnapshots: true}
}

func TestThreeBatchKindsAndRecords(t *testing.T) {
	registry := NewDemoRegistry()
	engine := NewEngine(testConfig(), registry)
	records, err := engine.SubmitSequential(CloneBatches(DemoBatches()))
	if err != nil {
		t.Fatalf("submit sequential: %v", err)
	}
	if len(records) != len(DemoBatches()) {
		t.Fatalf("record count = %d, want %d", len(records), len(DemoBatches()))
	}

	kinds := map[BatchKind]bool{}
	for _, rec := range records {
		kinds[rec.Kind] = true
		if rec.StateKey == 0 || rec.InputsDigest == 0 || rec.LeftHash == 0 || rec.RightHash == 0 {
			t.Fatalf("record %s has zero hash field: %+v", rec.BatchID, rec)
		}
	}
	for _, kind := range []BatchKind{SharedBatch, MonoBatch, StereoBatch} {
		if !kinds[kind] {
			t.Fatalf("missing batch kind %s", kind)
		}
	}

	sky := records[0]
	if sky.Kind != SharedBatch || sky.LeftHash != sky.RightHash {
		t.Fatalf("shared sky batch must keep both eyes identical: %+v", sky)
	}

	monoLeft := records[2]
	if len(monoLeft.Eyes) != 1 || monoLeft.Eyes[0] != LeftEye {
		t.Fatalf("mono batch eyes = %v, want [left]", monoLeft.Eyes)
	}
	if monoLeft.LeftHash == monoLeft.RightHash {
		t.Fatal("mono left batch unexpectedly changed the right eye hash")
	}
}

func TestSharedBatchRasterizesOnce(t *testing.T) {
	registry := NewDemoRegistry()
	engine := NewEngine(testConfig(), registry)
	batch := Batch{
		ID: "shared-only", Kind: SharedBatch,
		State:        RenderState{Material: skyMaterial()},
		GeometryName: GeomSky,
	}
	if _, err := engine.Run([]Batch{batch}); err != nil {
		t.Fatal(err)
	}
	left := engine.Framebuffer(LeftEye)
	right := engine.Framebuffer(RightEye)
	if left.Hash() != right.Hash() {
		t.Fatal("shared geometry produced different eye frames")
	}
	if !equalBytes(engine.StencilBuffer(LeftEye).Pix, engine.StencilBuffer(RightEye).Pix) {
		t.Fatal("shared geometry produced different stencil buffers")
	}
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScissorClipsDraw(t *testing.T) {
	registry := NewDemoRegistry()
	engine := NewEngine(testConfig(), registry)
	batch := Batch{
		ID: "clipped", Kind: StereoBatch,
		State: RenderState{
			Material: markerMaterial(),
			Scissor:  Scissor{Enabled: true, MinX: 0, MinY: 0, MaxX: 5, MaxY: 5},
		},
		GeometryName: GeomSky,
	}
	if _, err := engine.Run([]Batch{batch}); err != nil {
		t.Fatal(err)
	}
	fb := engine.Framebuffer(LeftEye)
	if fb.At(4, 4) == ClearColor {
		t.Fatal("pixels inside scissor were not drawn")
	}
	if fb.At(5, 5) != ClearColor {
		t.Fatal("pixels outside scissor were drawn")
	}
}

func TestStateIsolationPreventsPollution(t *testing.T) {
	registry := NewDemoRegistry()
	cfg := testConfig()

	isolated := NewEngine(cfg, registry)
	if _, err := isolated.Run(CloneBatches(DemoBatches())); err != nil {
		t.Fatal(err)
	}

	polluted := CloneBatches(DemoBatches())
	for i := range polluted {
		if polluted[i].ID == "b09-mono-glow-r" {
			// 只改状态、不改绘制结果：后续 Inherit 批次若被污染，
			// 会在标记区域写出与基线不同的颜色。
			polluted[i].State.Material.Name = "mat.renamed-only"
		}
	}
	replay := NewEngine(cfg, registry)
	if _, err := replay.Run(polluted); err != nil {
		t.Fatal(err)
	}

	// 隔离开启时，b09 的状态改动在批次结束后还原，b10 不受影响。
	if isolated.Framebuffer(RightEye).Hash() != replay.Framebuffer(RightEye).Hash() {
		region, count, _ := PixelDiff(isolated.Framebuffer(RightEye), replay.Framebuffer(RightEye))
		t.Fatalf("isolation leaked state: count=%d region=%+v", count, region)
	}
}
