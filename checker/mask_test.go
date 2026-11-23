package checker

import "testing"

func TestMaskCheckerRateAndCenter(t *testing.T) {
	w, h := 20, 20
	center := Rect{X0: 7, Y0: 7, X1: 13, Y1: 13}
	mask := BuildMask(w, h, center, 0)

	// Outside the center, exactly half of the pixels must be shaded: the 2:2
	// checker writes alternating 2x2 blocks, never whole rows or columns.
	peripheral, peripheralShaded := 0, 0
	centerCount, centerShaded := 0, 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if mask.InCenter[idx] {
				centerCount++
				if mask.Shaded[idx] {
					centerShaded++
				}
			} else {
				peripheral++
				if mask.Shaded[idx] {
					peripheralShaded++
				}
			}
		}
	}
	if centerShaded != centerCount {
		t.Fatalf("center must be per-pixel shaded: %d/%d", centerShaded, centerCount)
	}
	if peripheralShaded*2 != peripheral {
		t.Fatalf("peripheral checker rate must be 1/2, got %d/%d", peripheralShaded, peripheral)
	}

	// Complementary phase must shade exactly the opposite peripheral blocks.
	mask1 := BuildMask(w, h, center, 1)
	for i := range mask.Shaded {
		if mask.InCenter[i] {
			if !mask1.Shaded[i] {
				t.Fatal("center must stay shaded in both phases")
			}
			continue
		}
		if mask.Shaded[i] == mask1.Shaded[i] {
			t.Fatal("the two phases must be complementary in the periphery")
		}
	}
}

func TestShadeLeavesSkippedPixelsEmpty(t *testing.T) {
	truth := renderAnalytic(TestSize, TestSize)
	mask := BuildMask(TestSize, TestSize, TestCenter, 0)
	shaded := Shade(truth, mask)
	for i := range truth.Px {
		if mask.Shaded[i] {
			if shaded.Px[i] != truth.Px[i] {
				t.Fatal("shaded pixel must keep its ground-truth color")
			}
			continue
		}
		if shaded.Px[i] != (RGB{}) {
			t.Fatal("skipped peripheral pixel must be left unwritten")
		}
	}
}

func TestReconstructionLocksShadedAndCenterPixels(t *testing.T) {
	for _, sc := range Scenes() {
		truth := sc.Render(TestSize, TestSize)
		mask := BuildMask(TestSize, TestSize, TestCenter, 0)
		shaded := Shade(truth, mask)
		rec := Reconstruct(shaded, mask)

		for i := range truth.Px {
			if !mask.Shaded[i] {
				continue
			}
			if rec.Px[i] != shaded.Px[i] {
				t.Fatalf("scene %s: shaded pixel %d was modified", sc.Name, i)
			}
			if mask.InCenter[i] && rec.Px[i] != truth.Px[i] {
				t.Fatalf("scene %s: center pixel %d was touched by a peripheral rule", sc.Name, i)
			}
		}
	}
}

func TestCenterRMSEIsExactlyZero(t *testing.T) {
	for _, sc := range Scenes() {
		m, _, _, _ := RunScene(sc)
		if m.CenterRMSE != 0 {
			t.Fatalf("scene %s: center RMSE must be zero, got %g", sc.Name, m.CenterRMSE)
		}
	}
}
