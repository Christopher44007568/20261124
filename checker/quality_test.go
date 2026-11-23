package checker

import "testing"

// limits are per-scene quality ceilings for well-posed content. The saved
// peripheral samples must be "bought back" without a visible loss.
var limits = map[string]struct {
	lumaRMSE, hueMAE, coverageMAE, bias float64
	dice                                float64
}{
	"analytic_blob":       {0.010, 0.020, 0.010, 0.20, 0.99},
	"procedural_fbm":      {0.025, 0.020, 0.040, 0.20, 0.96},
	"thin_diagonal_lines": {0.280, 0.100, 0.080, 0.45, 0.60},
	"checker_16px":        {0.200, 0.020, 0.040, 0.20, 0.96},
	"hue_sweep":           {0.010, 0.020, 0.005, 0.20, 0.99},
}

func TestWellposedQuality(t *testing.T) {
	for _, sc := range Scenes() {
		if !sc.Wellposed {
			continue
		}
		lim, ok := limits[sc.Name]
		if !ok {
			t.Fatalf("no quality limits registered for scene %q", sc.Name)
		}
		m, _, _, _ := RunScene(sc)
		if m.LumaRMSE > lim.lumaRMSE {
			t.Errorf("scene %s: luma RMSE %.4f > %.4f", sc.Name, m.LumaRMSE, lim.lumaRMSE)
		}
		if m.HueMAE > lim.hueMAE {
			t.Errorf("scene %s: hue MAE %.4f > %.4f", sc.Name, m.HueMAE, lim.hueMAE)
		}
		if m.CoverageMAE > lim.coverageMAE {
			t.Errorf("scene %s: coverage MAE %.4f > %.4f", sc.Name, m.CoverageMAE, lim.coverageMAE)
		}
		if m.CoverageDice < lim.dice {
			t.Errorf("scene %s: coverage Dice %.4f < %.4f", sc.Name, m.CoverageDice, lim.dice)
		}
		if absSigned(m.Bias) > lim.bias {
			t.Errorf("scene %s: normalized bias %.4f > %.4f", sc.Name, m.Bias, lim.bias)
		}
	}
}

func TestAliasedScenesOnlyRequireAperiodicityAndCenter(t *testing.T) {
	// Patterns at or below the 2:2 sampling period are information-theoretically
	// un-reconstructable from one frame, so absolute fidelity cannot be
	// demanded. The reconstruction must still avoid inventing a fixed checker
	// period and must never touch the center.
	for _, sc := range Scenes() {
		if sc.Wellposed {
			continue
		}
		m, _, _, _ := RunScene(sc)
		if m.CenterRMSE != 0 {
			t.Fatalf("scene %s: center must stay exact, RMSE %g", sc.Name, m.CenterRMSE)
		}
		if m.PeriodAC > 0.12 && sc.Name == "checker_8px_aliased" {
			t.Errorf("scene %s: aliased content must not lock a block period, AC %.4f", sc.Name, m.PeriodAC)
		}
	}
}

func absSigned(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
