package shadow

import "testing"

func testFrame() (Scene, []EyeMask, Vec3) {
	s := DefaultScene()
	return s, s.CasterMasks(0, s.BasePitch), LightDirection(-0.6, 0.95)
}

func TestAllStrategiesMatchPerEyeTexels(t *testing.T) {
	s, masks, light := testFrame()
	for _, strategy := range []Strategy{SharedStrategy, IndependentStrategy, HybridStrategy} {
		store := BuildShadowStore(strategy, s, masks, light, 48, 8, BothEyes)
		for iz := 0; iz < store.Grid.Size; iz++ {
			for ix := 0; ix < store.Grid.Size; ix++ {
				p := store.Grid.texel(ix, iz)
				for _, eye := range []EyeMask{LeftEye, RightEye} {
					want := shadowAt(s, masks, light, p, eye)
					if got := store.Sample(p, eye); got != want {
						t.Fatalf("%s eye %d texel (%d,%d) = %v, want %v",
							strategy, eye, ix, iz, got, want)
					}
				}
			}
		}
	}
}

func TestStrategyCasterAndReuseMetrics(t *testing.T) {
	s, masks, light := testFrame()
	shared := BuildShadowStore(SharedStrategy, s, masks, light, 48, 8, BothEyes)
	indep := BuildShadowStore(IndependentStrategy, s, masks, light, 48, 8, BothEyes)
	hybrid := BuildShadowStore(HybridStrategy, s, masks, light, 48, 8, BothEyes)

	if shared.CasterSet != 5 || hybrid.CasterSet != 5 {
		t.Fatalf("shared/hybrid caster sets = %d/%d, want 5/5",
			shared.CasterSet, hybrid.CasterSet)
	}
	if indep.CasterSet != 8 {
		t.Fatalf("independent caster set = %d, want 8", indep.CasterSet)
	}
	if shared.ReuseRate() != 1 || indep.ReuseRate() != 0 {
		t.Fatalf("shared/independent reuse = %v/%v, want 1/0",
			shared.ReuseRate(), indep.ReuseRate())
	}
	if hybrid.ReuseRate() <= 0 || hybrid.ReuseRate() > 1 {
		t.Fatalf("hybrid reuse = %v, want (0,1]", hybrid.ReuseRate())
	}
}

func TestSetActiveDoesNotDeleteOtherEye(t *testing.T) {
	s, masks, light := testFrame()
	for _, strategy := range []Strategy{SharedStrategy, IndependentStrategy, HybridStrategy} {
		store := BuildShadowStore(strategy, s, masks, light, 48, 8, BothEyes)
		before := make([]bool, store.Grid.Size*store.Grid.Size)
		points := make([]Vec2, len(before))
		for iz := 0; iz < store.Grid.Size; iz++ {
			for ix := 0; ix < store.Grid.Size; ix++ {
				i := iz*store.Grid.Size + ix
				points[i] = store.Grid.texel(ix, iz)
				before[i] = store.Sample(points[i], LeftEye)
			}
		}
		store.SetActive(LeftEye)
		for i, p := range points {
			if got := store.Sample(p, LeftEye); got != before[i] {
				t.Fatalf("%s left shadow changed after right eye failure at %v",
					strategy, p)
			}
		}
	}
}
