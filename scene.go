package stereocull

import "math/rand"

// SceneKind identifies which parameter family a scene exercises.
type SceneKind string

const (
	SceneWide   SceneKind = "wide-angle"
	SceneNarrow SceneKind = "narrow-angle"
	SceneIPD    SceneKind = "ipd-shift"
	SceneNear   SceneKind = "near-boundary"
)

// Scene bundles geometry with the stereo configuration used to interpret it.
type Scene struct {
	Kind      SceneKind
	Config    StereoConfig
	Instances []Instance
}

// Special instance names so reports can trace hand-placed corner cases.
const (
	NameLeftOnly  = "left-only"
	NameRightOnly = "right-only"
	NameGapGhost  = "union-gap-ghost"
	NameLeftKiss  = "left-boundary-kiss"
	NameRightKiss = "right-boundary-kiss"
	NameNearKiss  = "near-plane-kiss"
)

// BuildScene constructs a deterministic lattice plus hand-placed instances
// that exercise single-eye visibility, union false positives and boundary
// tangency for the given parameter family.
func BuildScene(kind SceneKind, cfg StereoConfig) Scene {
	rng := rand.New(rand.NewSource(int64(seedFor(kind))))
	items := gridInstances(rng)

	halfW, halfH := cfg.HalfExtents()
	eye := cfg.IPD / 2

	// Frusta overlap once depth reaches converge = near*IPD/(2*halfW).
	converge := cfg.Near * cfg.IPD / (2 * halfW)
	// Side-only cases live just beyond convergence; gap ghosts must stay
	// between the near plane and convergence where the frusta are disjoint.
	dSide := cfg.Near * 1.4
	if dSide < converge*1.1 {
		dSide = converge * 1.1
	}
	dGap := (cfg.Near + converge) / 2

	edgesAt := func(d float64) (lInner, lOuter, rOuter, rInner float64) {
		hw := halfW * d / cfg.Near
		return eye - hw, eye + hw, -eye - hw, -eye + hw
	}
	_, xLeftOuter, xRightOuter, _ := edgesAt(dSide)

	add := func(name string, center Vec3, radius float64) {
		items = append(items, Instance{
			ID:     len(items),
			Name:   name,
			Center: center,
			Radius: radius,
		})
	}

	kissR := cfg.Near * 0.01
	// Left-only: inside the left outer frustum, outside the right one.
	add(NameLeftOnly, Vec3{X: xLeftOuter - 2*kissR, Y: 0, Z: -dSide}, kissR)
	// Right-only: symmetric case for the right eye.
	add(NameRightOnly, Vec3{X: xRightOuter + 2*kissR, Y: 0, Z: -dSide}, kissR)
	// Gap ghost: accepted by the union hull but inside neither eye frustum.
	if converge > cfg.Near {
		hwGap := halfW * dGap / cfg.Near
		leftRightEdge := eye - hwGap
		rightLeftEdge := -eye + hwGap
		if leftRightEdge-rightLeftEdge > 4*kissR {
			add(NameGapGhost, Vec3{X: (leftRightEdge + rightLeftEdge) / 2, Y: 0, Z: -dGap}, kissR)
		}
	}
	// Side-plane tangency: sphere center one radius inside the outer planes.
	add(NameLeftKiss, Vec3{X: xLeftOuter - kissR, Y: halfH * dSide / cfg.Near * 0.4, Z: -dSide}, kissR)
	add(NameRightKiss, Vec3{X: xRightOuter + kissR, Y: -halfH * dSide / cfg.Near * 0.4, Z: -dSide}, kissR)
	// Near-plane tangency: straddles the near clip plane.
	add(NameNearKiss, Vec3{X: 0, Y: 0, Z: -(cfg.Near - kissR)}, kissR)

	return Scene{Kind: kind, Config: cfg, Instances: items}
}

func gridInstances(rng *rand.Rand) []Instance {
	const (
		nx, ny, nz = 44, 34, 44
		spacing    = 1.3
		radius     = 0.06
	)
	items := make([]Instance, 0, nx*ny*nz)
	id := 0
	for ix := 0; ix < nx; ix++ {
		for iy := 0; iy < ny; iy++ {
			for iz := 0; iz < nz; iz++ {
				jitter := func() float64 { return (rng.Float64() - 0.5) * spacing * 0.25 }
				center := Vec3{
					X: (float64(ix)-(nx-1)/2.0)*spacing + jitter(),
					Y: (float64(iy)-(ny-1)/2.0)*spacing + jitter(),
					Z: -0.2 - float64(iz)*spacing + jitter(),
				}
				items = append(items, Instance{
					ID:     id,
					Name:   "grid",
					Center: center,
					Radius: radius,
				})
				id++
			}
		}
	}
	return items
}

func seedFor(kind SceneKind) int {
	switch kind {
	case SceneWide:
		return 101
	case SceneNarrow:
		return 202
	case SceneIPD:
		return 303
	default:
		return 404
	}
}

// DefaultScenes returns the four required parameter families.
func DefaultScenes() []Scene {
	return []Scene{
		BuildScene(SceneWide, StereoConfig{IPD: 0.6, Near: 0.1, Far: 12, VerticalFOV: 1.9, Aspect: 1.6}),
		BuildScene(SceneNarrow, StereoConfig{IPD: 0.16, Near: 0.1, Far: 12, VerticalFOV: 0.55, Aspect: 1.6}),
		BuildScene(SceneIPD, StereoConfig{IPD: 0.22, Near: 0.1, Far: 12, VerticalFOV: 1.0, Aspect: 1.6}),
		BuildScene(SceneNear, StereoConfig{IPD: 0.2, Near: 0.05, Far: 12, VerticalFOV: 1.0, Aspect: 1.6}),
	}
}
