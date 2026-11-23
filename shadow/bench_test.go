package shadow

import "testing"

func TestRunBenchmarkInvariantsAndMetrics(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Frames = 8
	cfg.MapSize = 48
	cfg.Width = 64
	cfg.Height = 40
	r := RunBenchmark(cfg)

	if len(r.Strategies) != 3 {
		t.Fatalf("strategies = %d, want 3", len(r.Strategies))
	}
	if !r.Invariants.SharedPreservesEyeBits {
		t.Error("shared layout lost per-eye bits")
	}
	if !r.Invariants.FailureKeepsShadows {
		t.Error("eye failure removed surviving eye shadows")
	}
	if !r.Invariants.HybridMatchesReference {
		t.Error("hybrid layout differs from per-eye reference")
	}
	byName := map[string]StrategyReport{}
	for _, s := range r.Strategies {
		byName[s.Name] = s
		for _, p := range s.Failures {
			if !p.Preserved || p.ChangedPixels != 0 ||
				(p.EyeSplitTexels == 0 && p.ExclusiveCasters == 0) {
				t.Errorf("%s failure probe %s->%s not preserved: %+v",
					s.Name, p.DisabledEye, p.SurvivingEye, p)
			}
		}
	}
	if byName["shared"].AvgReusePct != 100 {
		t.Errorf("shared reuse = %v, want 100", byName["shared"].AvgReusePct)
	}
	if byName["independent"].AvgReusePct != 0 {
		t.Errorf("independent reuse = %v, want 0", byName["independent"].AvgReusePct)
	}
	hybridReuse := byName["hybrid"].AvgReusePct
	if hybridReuse <= 0 || hybridReuse >= 100 {
		t.Errorf("hybrid reuse = %v, want (0,100)", hybridReuse)
	}
	if byName["independent"].PeakShadowBytes <= byName["shared"].PeakShadowBytes {
		t.Error("independent layout did not peak at more shadow storage")
	}
	if byName["independent"].PeakCasters <= byName["shared"].PeakCasters {
		t.Error("independent layout did not peak at more caster references")
	}
}

func TestLowResolutionStillPreservesCasterVisibility(t *testing.T) {
	cfg := Config{Frames: 2, MapSize: 16, TileSize: 64, Width: 24, Height: 16}
	r := RunBenchmark(cfg)
	if !r.Invariants.SharedPreservesEyeBits {
		t.Error("caster eye bits were lost at low shadow-map resolution")
	}
	if !r.Invariants.FailureKeepsShadows {
		t.Error("eye failure probe failed at low shadow-map resolution")
	}
	for _, s := range r.Strategies {
		for _, p := range s.Failures {
			if p.ExclusiveCasters == 0 {
				t.Errorf("%s probe lost exclusive caster masks", s.Name)
			}
		}
	}
}
