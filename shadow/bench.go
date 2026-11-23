package shadow

import (
	"math"
	"time"
)

type strategyAccum struct {
	frames        int
	casterSum     int
	reuseSum      float64
	peakCasters   int
	peakShadow    int
	peakResources int
	compared      int
	mismatch      int
	elapsed       time.Duration
}

var strategyOrder = []Strategy{
	SharedStrategy,
	IndependentStrategy,
	HybridStrategy,
}

// RunBenchmark rotates the light and turns the head rapidly while rebuilding
// the three shadow layouts. Numeric results are deterministic for a Config.
func RunBenchmark(cfg Config) Report {
	cfg = normalizeConfig(cfg)
	scene := DefaultScene()
	acc := map[Strategy]*strategyAccum{}
	for _, strategy := range strategyOrder {
		acc[strategy] = &strategyAccum{}
	}
	failures := map[Strategy][]FailureProbe{}

	for frame := 0; frame < cfg.Frames; frame++ {
		phase := float64(frame) / float64(cfg.Frames)
		yaw := 0.42 * math.Sin(2*math.Pi*6*phase)
		pitch := scene.BasePitch + 0.035*math.Sin(2*math.Pi*5*phase)
		azimuth := -1.05 + 2*math.Pi*cfg.LightTurns*phase
		elevation := 0.95 + 0.14*math.Sin(2*math.Pi*3*phase+0.4)
		light := LightDirection(azimuth, elevation)
		masks := scene.CasterMasks(yaw, pitch)
		eyes := [2]Eye{
			scene.Pose(-1, yaw, pitch),
			scene.Pose(1, yaw, pitch),
		}
		refs := [2][]uint8{
			ReferenceShadows(scene, masks, light, eyes[0], cfg.Width, cfg.Height),
			ReferenceShadows(scene, masks, light, eyes[1], cfg.Width, cfg.Height),
		}
		for _, strategy := range strategyOrder {
			start := time.Now()
			store := BuildShadowStore(strategy, scene, masks, light,
				cfg.MapSize, cfg.TileSize, BothEyes)
			errs := [2]ImageError{}
			for i, eye := range eyes {
				img := StrategyShadows(scene, store, eye, cfg.Width, cfg.Height)
				errs[i] = CompareImages(refs[i], img)
			}
			a := acc[strategy]
			a.frames++
			a.casterSum += store.CasterSet
			a.reuseSum += store.ReuseRate()
			if store.CasterSet > a.peakCasters {
				a.peakCasters = store.CasterSet
			}
			if n := store.Bytes(); n > a.peakShadow {
				a.peakShadow = n
			}
			resourceBytes := store.Bytes() + store.CasterSet*8 +
				len(masks)*2 + len(store.CasterEyes)
			if resourceBytes > a.peakResources {
				a.peakResources = resourceBytes
			}
			for _, e := range errs {
				a.compared += e.Compared
				a.mismatch += e.Mismatch
			}
			a.elapsed += time.Since(start)
		}
	}

	probeMasks := scene.CasterMasks(0.17, scene.BasePitch)
	probeLight := LightDirection(-0.6, 0.95)
	for _, strategy := range strategyOrder {
		failures[strategy] = probeEyeFailures(scene, probeMasks, probeLight,
			cfg, strategy)
	}

	report := Report{
		Config: cfg,
		Scene:  makeSceneReport(scene),
	}
	allPreserved := true
	for _, strategy := range strategyOrder {
		a := acc[strategy]
		sr := StrategyReport{
			Name:              string(strategy),
			Frames:            a.frames,
			AvgCasters:        float64(a.casterSum) / float64(maxInt(1, a.frames)),
			PeakCasters:       a.peakCasters,
			AvgReusePct:       100 * a.reuseSum / float64(maxInt(1, a.frames)),
			PixelErrorPct:     pct(a.mismatch, a.compared),
			PixelErrorCount:   a.mismatch,
			ComparedPixels:    a.compared,
			PeakShadowBytes:   a.peakShadow,
			PeakResourceBytes: a.peakResources,
			ElapsedMS:         float64(a.elapsed.Microseconds()) / 1000,
			Failures:          failures[strategy],
		}
		for _, p := range sr.Failures {
			allPreserved = allPreserved && p.Preserved
		}
		report.Strategies = append(report.Strategies, sr)
	}
	report.Invariants = runInvariantChecks(scene, probeMasks, probeLight, cfg,
		allPreserved)
	return report
}

func probeEyeFailures(scene Scene, masks []EyeMask, light Vec3, cfg Config,
	strategy Strategy) []FailureProbe {
	store := BuildShadowStore(strategy, scene, masks, light,
		cfg.MapSize, cfg.TileSize, BothEyes)
	var probes []FailureProbe
	for _, disabled := range []EyeMask{RightEye, LeftEye} {
		survivor := LeftEye
		if disabled == LeftEye {
			survivor = RightEye
		}
		eye := scene.Pose(-1, 0.17, scene.BasePitch)
		if survivor == RightEye {
			eye = scene.Pose(1, 0.17, scene.BasePitch)
		}
		ref := ReferenceShadows(scene, masks, light, eye, cfg.Width, cfg.Height)
		savedActive := store.Active
		store.SetActive(BothEyes)
		before := StrategyShadows(scene, store, eye, cfg.Width, cfg.Height)
		beforeErr := CompareImages(ref, before)

		store.SetActive(survivor)
		after := StrategyShadows(scene, store, eye, cfg.Width, cfg.Height)
		afterErr := CompareImages(ref, after)
		changed := countChangedGround(before, after)
		split := eyeSplitTexels(store)
		exclusive := countExclusiveCasters(store.CasterEyes)
		probes = append(probes, FailureProbe{
			DisabledEye:      eyeName(disabled),
			SurvivingEye:     eyeName(survivor),
			BeforeErrorPct:   pct(beforeErr.Mismatch, beforeErr.Compared),
			AfterErrorPct:    pct(afterErr.Mismatch, afterErr.Compared),
			ChangedPixels:    changed,
			EyeSplitTexels:   split,
			ExclusiveCasters: exclusive,
			Preserved: changed == 0 && afterErr.Mismatch <= beforeErr.Mismatch &&
				(split > 0 || exclusive > 0),
		})
		store.SetActive(savedActive)
	}
	return probes
}

func eyeSplitTexels(store *ShadowStore) int {
	n := 0
	for iz := 0; iz < store.Grid.Size; iz++ {
		for ix := 0; ix < store.Grid.Size; ix++ {
			p := store.Grid.texel(ix, iz)
			if store.Sample(p, LeftEye) != store.Sample(p, RightEye) {
				n++
			}
		}
	}
	return n
}

func countChangedGround(a, b []uint8) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	changed := 0
	for i := 0; i < n; i++ {
		if a[i] != sky && b[i] != sky && a[i] != b[i] {
			changed++
		}
	}
	return changed
}
