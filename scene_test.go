package gazedenoise

import (
	"math"
	"reflect"
	"testing"
)

func TestSceneDeterministicAndComplete(t *testing.T) {
	a, err := GenerateScene(SceneCombined, 96, 72, 0.03, 77)
	if err != nil {
		t.Fatal(err)
	}
	b, err := GenerateScene(SceneCombined, 96, 72, 0.03, 77)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a.Truth, b.Truth) || !reflect.DeepEqual(a.Noisy, b.Noisy) {
		t.Fatal("same seed produced different scene data")
	}
	centre, periphery := a.Truth.CountMask()
	if centre == 0 || periphery == 0 {
		t.Fatalf("mask has empty partition: centre=%d periphery=%d", centre, periphery)
	}
	if len(a.BoundaryDistance) != 96*72 {
		t.Fatalf("boundary distance has %d values", len(a.BoundaryDistance))
	}
	for i, valid := range a.Noisy.Valid {
		if !valid && a.Truth.Valid[i] == false {
			t.Fatalf("truth pixel %d should be valid before corruption", i)
		}
	}
}

func TestBoundaryHasNormalDepthAndLumaSteps(t *testing.T) {
	scene, err := GenerateScene(SceneBoundaryStep, 128, 96, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	f := scene.Truth
	for y := 1; y < f.Height-1; y++ {
		for x := 1; x < f.Width-1; x++ {
			i := f.Index(x, y)
			if !f.Mask[i] {
				continue
			}
			for _, n := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				q := f.Index(x+n[0], y+n[1])
				if f.Mask[q] {
					continue
				}
				normalDot := f.Normal[i].Dot(f.Normal[q])
				depthJump := math.Abs(f.Depth[i] - f.Depth[q])
				lumaJump := math.Abs(f.Luma[i] - f.Luma[q])
				if normalDot > 0.99 || depthJump < 0.4 || lumaJump < 0.05 {
					t.Fatalf("weak boundary at (%d,%d): dot=%.4f depth=%.4f luma=%.4f",
						x, y, normalDot, depthJump, lumaJump)
				}
				return
			}
		}
	}
	t.Fatal("no adjacent centre/periphery pair found")
}

func TestStressFeaturesCrossGazeBoundary(t *testing.T) {
	cases := []struct {
		kind    SceneKind
		feature string
	}{
		{SceneThinRod, "thin_rod"},
		{SceneTranslucent, "translucent_edge"},
		{SceneExposureJump, "exposure_jump"},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			scene, err := GenerateScene(tc.kind, 160, 120, 0.01, 9)
			if err != nil {
				t.Fatal(err)
			}
			mask := scene.Feature(tc.feature)
			if len(mask) == 0 {
				t.Fatalf("missing feature %q", tc.feature)
			}
			inside, outside := 0, 0
			for i, selected := range mask {
				if !selected {
					continue
				}
				if scene.Truth.Mask[i] {
					inside++
				} else {
					outside++
				}
			}
			if inside == 0 || outside == 0 {
				t.Fatalf("%s does not cross boundary: inside=%d outside=%d", tc.feature, inside, outside)
			}
		})
	}
}

func TestInvalidSceneInputs(t *testing.T) {
	if _, err := GenerateScene(SceneCombined, 10, 10, 0.01, 1); err == nil {
		t.Fatal("expected small-image error")
	}
	if _, err := GenerateScene(SceneCombined, 64, 48, -1, 1); err == nil {
		t.Fatal("expected negative-noise error")
	}
	if _, err := GenerateScene(SceneKind("missing"), 64, 48, 0.01, 1); err == nil {
		t.Fatal("expected unknown-scene error")
	}
}
