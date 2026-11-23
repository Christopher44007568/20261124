package shadow

func normalizeConfig(c Config) Config {
	if c.Frames < 1 {
		c.Frames = 1
	}
	if c.MapSize < 8 {
		c.MapSize = 8
	}
	if c.TileSize < 1 {
		c.TileSize = 1
	}
	if c.Width < 8 {
		c.Width = 8
	}
	if c.Height < 8 {
		c.Height = 8
	}
	if c.LightTurns <= 0 {
		c.LightTurns = 1.5
	}
	return c
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func eyeName(eye EyeMask) string {
	if eye == LeftEye {
		return "left"
	}
	if eye == RightEye {
		return "right"
	}
	return "both"
}

func countExclusiveCasters(masks []EyeMask) int {
	n := 0
	for _, m := range masks {
		if m == LeftEye || m == RightEye {
			n++
		}
	}
	return n
}

func maskName(m EyeMask) string {
	switch m {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	case BothEyes:
		return "both"
	default:
		return "none"
	}
}

func casterKindName(k CasterKind) string {
	if k == RodCaster {
		return "rod"
	}
	return "box"
}

func makeSceneReport(scene Scene) SceneReport {
	masks := scene.CasterMasks(0, scene.BasePitch)
	out := SceneReport{
		EyeSeparation: scene.EyeSeparation,
	}
	for i, c := range scene.Casters {
		out.Casters = append(out.Casters, CasterReport{
			Name: c.Name, Kind: casterKindName(c.Kind),
			Height: c.Height, Eyes: maskName(masks[i]),
		})
		switch masks[i] {
		case LeftEye, RightEye:
			out.ExclusiveCount++
		case BothEyes:
			out.CrossEyeCount++
		}
	}
	return out
}

func runInvariantChecks(scene Scene, masks []EyeMask, light Vec3, cfg Config,
	allPreserved bool) InvariantReport {
	shared := BuildShadowStore(SharedStrategy, scene, masks, light,
		cfg.MapSize, cfg.TileSize, BothEyes)
	sharedOK := countExclusiveCasters(shared.CasterEyes) > 0
	for i, m := range masks {
		if shared.CasterEyes[i] != m {
			sharedOK = false
			break
		}
	}
	for iz := 0; iz < shared.Grid.Size && sharedOK; iz++ {
		for ix := 0; ix < shared.Grid.Size; ix++ {
			p := shared.Grid.texel(ix, iz)
			for _, eye := range []EyeMask{LeftEye, RightEye} {
				if shared.Sample(p, eye) != shadowAt(scene, masks, light, p, eye) {
					sharedOK = false
					break
				}
			}
		}
	}

	indep := BuildShadowStore(IndependentStrategy, scene, masks, light,
		cfg.MapSize, cfg.TileSize, BothEyes)
	hybrid := BuildShadowStore(HybridStrategy, scene, masks, light,
		cfg.MapSize, cfg.TileSize, BothEyes)
	hybridOK := true
	for iz := 0; iz < indep.Grid.Size && hybridOK; iz++ {
		for ix := 0; ix < indep.Grid.Size; ix++ {
			p := indep.Grid.texel(ix, iz)
			for _, eye := range []EyeMask{LeftEye, RightEye} {
				if indep.Sample(p, eye) != hybrid.Sample(p, eye) {
					hybridOK = false
					break
				}
			}
		}
	}
	return InvariantReport{
		SharedPreservesEyeBits: sharedOK,
		FailureKeepsShadows:    allPreserved,
		HybridMatchesReference: hybridOK,
	}
}
