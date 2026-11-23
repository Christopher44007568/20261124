package main

import "testing"

func boundOne(inst *Instance) []BoundInstance {
	return []BoundInstance{{Inst: inst, World: Identity()}}
}

// A sliver in the wedge between the two frusta must get zero coverage
// from the left eye and positive coverage from the right eye.
func TestFrustumOnlySliver(t *testing.T) {
	rig := NewRig()
	inst := &Instance{ID: 0, Name: "right-frustum-only", Box: OBB{Center: Vec3{4.5, 0, -3}, Half: Vec3{0.03, 0.5, 0.03}}}
	covL, _ := RasterizeEye(rig.Cams[EyeLeft], boundOne(inst), renderW, renderH)
	covR, _ := RasterizeEye(rig.Cams[EyeRight], boundOne(inst), renderW, renderH)
	if covL[0] != 0 {
		t.Errorf("left eye coverage = %d, want 0", covL[0])
	}
	if covR[0] == 0 {
		t.Errorf("right eye coverage = 0, want > 0")
	}
}

// A box centered on the view axis must be covered by both eyes.
func TestCenterBoxHasCoverage(t *testing.T) {
	rig := NewRig()
	inst := &Instance{ID: 0, Name: "center", Box: OBB{Center: Vec3{0, 0, -3}, Half: Vec3{0.5, 0.5, 0.5}}}
	for eye := 0; eye < 2; eye++ {
		cov, _ := RasterizeEye(rig.Cams[eye], boundOne(inst), renderW, renderH)
		if cov[0] == 0 {
			t.Errorf("eye %d coverage = 0, want > 0", eye)
		}
	}
}

// A box fully beyond the far plane must get zero coverage even though
// only the near plane is geometrically clipped.
func TestFarPlaneClipsCoverage(t *testing.T) {
	rig := NewRig()
	inst := &Instance{ID: 0, Name: "too-far", Box: OBB{Center: Vec3{0, 0, -(farDist + 8.4)}, Half: Vec3{8, 8, 8}}}
	cov, _ := RasterizeEye(rig.Cams[EyeLeft], boundOne(inst), renderW, renderH)
	if cov[0] != 0 {
		t.Errorf("coverage = %d, want 0 for box beyond far plane", cov[0])
	}
}

// A box straddling the near plane must still be rasterized (clipped, not
// rejected), and the culler must report the near-plane crossing.
func TestNearPlaneCrossing(t *testing.T) {
	rig := NewRig()
	inst := &Instance{ID: 0, Name: "near-cross", Box: OBB{Center: Vec3{0, 0, -0.13}, Half: Vec3{0.05, 0.05, 0.05}}}
	bound := boundOne(inst)
	cov, _ := RasterizeEye(rig.Cams[EyeLeft], bound, renderW, renderH)
	if cov[0] == 0 {
		t.Errorf("coverage = 0, want > 0 for near-crossing box")
	}
	center, axes, half := bound[0].WorldOBB()
	ev := CullOBB(rig.Frusta[EyeLeft], center, axes, half)
	if !ev.Visible || !ev.CrossesNear {
		t.Errorf("evidence = %+v, want visible and CrossesNear", ev)
	}
}

// The wall must fully hide the target from the left eye but not the
// right eye (single-eye occlusion ground truth).
func TestSingleEyeOcclusion(t *testing.T) {
	rig := NewRig()
	id := 0
	bound := Flatten(scenarioSingleEyeOcclusion().build(&id))
	var targetID int
	for _, b := range bound {
		if b.Inst.Name == "hidden-target" {
			targetID = b.Inst.ID
		}
	}
	covL, _ := RasterizeEye(rig.Cams[EyeLeft], bound, renderW, renderH)
	covR, _ := RasterizeEye(rig.Cams[EyeRight], bound, renderW, renderH)
	if covL[targetID] != 0 {
		t.Errorf("left eye target coverage = %d, want 0 (occluded)", covL[targetID])
	}
	if covR[targetID] == 0 {
		t.Errorf("right eye target coverage = 0, want > 0")
	}
}

// Classification counts must close to the input total for every scenario.
func TestCountsClose(t *testing.T) {
	rig := NewRig()
	for _, sc := range allScenarios() {
		id := 0
		bound := Flatten(sc.build(&id))
		_, counts := ClassifyAll(bound, rig.Frusta)
		if !counts.Closed() {
			t.Errorf("scenario %s: counts %d != total %d", sc.name, counts.Sum(), counts.Total)
		}
	}
}
