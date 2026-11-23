package main

import "testing"

// TestPipelineConservative runs the full two-layer query over the camera
// path and requires zero false culls and zero shared-layer failures.
func TestPipelineConservative(t *testing.T) {
	scene := buildScene()
	_, rigs := GeneratePath(8, ipd)
	for f := range rigs {
		res := RunQuery(scene, &rigs[f], f)
		Validate(scene, &rigs[f], res)
		if res.Metrics.SharedFailure {
			t.Fatalf("frame %d: shared layer culled an object visible to an eye", f)
		}
		if res.Metrics.FalseCulled != 0 {
			t.Fatalf("frame %d: %d false culls", f, res.Metrics.FalseCulled)
		}
	}
}

// TestSharedFailureDetected proves the validator catches a shared-set
// result that excludes an object actually visible to one eye.
func TestSharedFailureDetected(t *testing.T) {
	scene := buildScene()
	_, rigs := GeneratePath(1, ipd)
	rig := &rigs[0]
	res := RunQuery(scene, rig, 0)
	// Find an object that is ground-truth visible to at least one eye.
	victim := -1
	for _, obj := range scene.Objects {
		if GroundTruthVisible(scene, rig.EyePos(0), obj) ||
			GroundTruthVisible(scene, rig.EyePos(1), obj) {
			victim = obj.ID
			break
		}
	}
	if victim < 0 {
		t.Skip("no visible object in frame 0")
	}
	// Simulate a broken shared layer that culled the visible object.
	res.CulledShared[victim] = true
	Validate(scene, rig, res)
	if !res.Metrics.SharedFailure {
		t.Fatal("validator failed to flag shared-set exclusion of a visible object")
	}
	if res.Metrics.FalseCulled == 0 {
		t.Fatal("validator failed to count the false cull")
	}
}

// TestFallbackExpansion ensures a partially covering per-eye occluder
// triggers the conservative range-expansion path.
func TestFallbackExpansion(t *testing.T) {
	scene := &Scene{
		Objects: []Object{{ID: 0, Center: Vec3{0.35, 0, 5}, Radius: 0.4}},
		Occluders: []Occluder{
			// Thin pillar covering only part of the object from the origin.
			{ID: 0, Kind: Pillar, Min: Vec3{-0.2, -1, 2}, Max: Vec3{0.2, 1, 2.3}},
		},
	}
	rig := &StereoRig{Position: Vec3{0, 0, 0}, Yaw: 0, IPD: ipd, Fov: 1.0}
	res := RunQuery(scene, rig, 0)
	if res.Metrics.ConservativeFallback == 0 {
		t.Fatal("expected conservative fallback expansions for partial coverage")
	}
	// The object must survive: partial coverage may never cull it.
	if res.CulledPerEye[0][0] || res.CulledPerEye[1][0] {
		t.Fatal("partially covered object was culled")
	}
}
