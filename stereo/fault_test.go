package stereo

import "testing"

func TestInjectFaultRejectsWrongTarget(t *testing.T) {
	batches := DemoBatches()

	if _, err := InjectFault(batches, Fault{
		Kind: FaultMaterial, BatchID: "b05-highlight-l", Eye: RightEye,
		BadMaterial: PollutedMaterial(),
	}); err == nil {
		t.Fatal("expected error when fault eye does not match mono batch eye")
	}

	if _, err := InjectFault(batches, Fault{
		Kind: FaultMaterial, BatchID: "b01-sky", Eye: LeftEye,
		BadMaterial: PollutedMaterial(),
	}); err == nil {
		t.Fatal("expected error when fault targets a non-mono batch")
	}

	original := batches[4].State.Material.Name
	out, err := InjectFault(batches, Fault{
		Kind: FaultMaterial, BatchID: "b05-highlight-l", Eye: LeftEye,
		BadMaterial: PollutedMaterial(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if batches[4].State.Material.Name != original {
		t.Fatal("InjectFault mutated the original batch slice")
	}
	if out[4].State.Material.Name != PollutedMaterial().Name {
		t.Fatal("fault material was not applied to the copy")
	}
}

func TestMaterialFaultLocatesFirstEyeAndRegion(t *testing.T) {
	report, err := RunAudit(testConfig(), NewDemoRegistry(), DemoBatches(), Fault{
		Kind: FaultMaterial, BatchID: "b05-highlight-l", Eye: LeftEye,
		BadMaterial: Material{Name: "mat.corrupt", Color: [4]uint8{255, 0, 255, 255}, Blend: BlendOverwrite},
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FirstStateKeyDiff == nil || report.FirstStateKeyDiff.Eye != LeftEye ||
		report.FirstStateKeyDiff.BatchID != "b05-highlight-l" {
		t.Fatalf("unexpected first state key diff: %+v", report.FirstStateKeyDiff)
	}
	if len(report.Impacts) != 1 || report.Impacts[0].Eye != LeftEye {
		t.Fatalf("material fault must affect only the left eye: %+v", report.Impacts)
	}
	impact := report.Impacts[0]
	if impact.Seq != 4 || impact.BatchID != "b05-highlight-l" {
		t.Fatalf("first impact = %+v, want seq 4 b05", impact)
	}
	wantRegion := Rect{MinX: 6, MinY: 8, MaxX: 12, MaxY: 11}
	if impact.Region != wantRegion || impact.PixelCount != 18 {
		t.Fatalf("impact region=%+v count=%d, want %+v 18", impact.Region, impact.PixelCount, wantRegion)
	}
	if _, ok := report.FinalRegions[RightEye]; ok {
		t.Fatal("right eye must stay clean under state isolation")
	}
}

func TestStencilFaultLocatesFirstEyeAndRegion(t *testing.T) {
	bad := StencilState{
		Enabled: true, Ref: 1, ReadMask: 0xff, Compare: CmpNotEqual,
		FailOp: StencilZero, PassOp: StencilKeep, WriteMask: 0xff,
	}
	report, err := RunAudit(testConfig(), NewDemoRegistry(), DemoBatches(), Fault{
		Kind: FaultStencil, BatchID: "b06-highlight-r", Eye: RightEye, BadStencil: bad,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.FirstStateKeyDiff == nil || report.FirstStateKeyDiff.Eye != RightEye {
		t.Fatalf("unexpected first state key diff: %+v", report.FirstStateKeyDiff)
	}
	if len(report.Impacts) != 1 || report.Impacts[0].Eye != RightEye {
		t.Fatalf("stencil fault must affect only the right eye: %+v", report.Impacts)
	}
	impact := report.Impacts[0]
	if impact.Seq != 5 || impact.BatchID != "b06-highlight-r" {
		t.Fatalf("first impact = %+v, want seq 5 b06", impact)
	}
	wantRegion := Rect{MinX: 25, MinY: 8, MaxX: 33, MaxY: 13}
	if impact.Region != wantRegion || impact.PixelCount != 40 {
		t.Fatalf("impact region=%+v count=%d, want %+v 40", impact.Region, impact.PixelCount, wantRegion)
	}
}

func TestPollutionSpreadsWhenIsolationDisabled(t *testing.T) {
	registry := NewDemoRegistry()
	cfg := testConfig()

	isolated := NewEngine(cfg, registry)
	if _, err := isolated.Run(CloneBatches(DemoBatches())); err != nil {
		t.Fatal(err)
	}

	polluted := CloneBatches(DemoBatches())
	for i := range polluted {
		switch polluted[i].ID {
		case "b09-mono-glow-r":
			polluted[i].State.Material = PollutedMaterial()
			polluted[i].State.Stencil = StencilState{}
			polluted[i].State.Scissor = Scissor{}
			polluted[i].GeometryName = GeomMarker
		case "b10-inherit-marker-r":
			polluted[i].GeometryName = GeomMarker
		}
	}
	dirtyCfg := cfg
	dirtyCfg.DisableStateIsolation = true
	dirty := NewEngine(dirtyCfg, registry)
	if _, err := dirty.Run(polluted); err != nil {
		t.Fatal(err)
	}

	region, count, ok := PixelDiff(isolated.Framebuffer(RightEye), dirty.Framebuffer(RightEye))
	if !ok {
		t.Fatal("expected visible pollution when state isolation is disabled")
	}
	if count <= 40 {
		t.Fatalf("pollution did not spread beyond the first batch: count=%d", count)
	}
	wantRegion := Rect{MinX: 25, MinY: 8, MaxX: 37, MaxY: 23}
	if region != wantRegion {
		t.Fatalf("pollution region=%+v, want %+v", region, wantRegion)
	}
	if _, _, leftDirty := PixelDiff(isolated.Framebuffer(LeftEye), dirty.Framebuffer(LeftEye)); leftDirty {
		t.Fatal("the constructed accident must stay on the right eye")
	}
}
