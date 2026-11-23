package shadow

// Strategy selects how the per-eye shadow maps are laid out.
type Strategy string

const (
	SharedStrategy      Strategy = "shared"
	IndependentStrategy Strategy = "independent"
	HybridStrategy      Strategy = "hybrid"
)

// MapGrid samples a square shadow map over the scene's ground bounds.
type MapGrid struct {
	Size   int
	Bounds Bounds
}

func (g MapGrid) texel(ix, iz int) Vec2 {
	sx := (g.Bounds.MaxX - g.Bounds.MinX) / float64(g.Size)
	sz := (g.Bounds.MaxZ - g.Bounds.MinZ) / float64(g.Size)
	return Vec2{
		X: g.Bounds.MinX + (float64(ix)+0.5)*sx,
		Z: g.Bounds.MinZ + (float64(iz)+0.5)*sz,
	}
}

func (g MapGrid) index(p Vec2) (int, int, bool) {
	if !g.Bounds.Contains(p) {
		return 0, 0, false
	}
	u := (p.X - g.Bounds.MinX) / (g.Bounds.MaxX - g.Bounds.MinX)
	v := (p.Z - g.Bounds.MinZ) / (g.Bounds.MaxZ - g.Bounds.MinZ)
	ix := int(u * float64(g.Size))
	iz := int(v * float64(g.Size))
	if ix >= g.Size {
		ix = g.Size - 1
	}
	if iz >= g.Size {
		iz = g.Size - 1
	}
	return ix, iz, true
}

type tileBlock struct {
	shared bool
	left   []uint8
	right  []uint8
}

// ShadowStore is the combined layout for one strategy. The shared layout uses
// per-texel eye bits; independent uses one layer per eye; hybrid shares
// macro-cells whose two eye masks are identical.
type ShadowStore struct {
	Strategy Strategy
	Grid     MapGrid

	Bits       []uint8
	Layers     [2][]uint8
	TileSize   int
	TilesX     int
	TilesZ     int
	Blocks     []tileBlock
	SharedTile int

	Active     EyeMask
	CasterSet  int
	CasterEyes []EyeMask
}

// BuildShadowStore builds a map for the enabled eye mask.
func BuildShadowStore(strategy Strategy, scene Scene, masks []EyeMask, light Vec3,
	size, tileSize int, active EyeMask) *ShadowStore {
	if size < 1 {
		size = 1
	}
	g := MapGrid{Size: size, Bounds: scene.Bounds}
	store := &ShadowStore{
		Strategy:   strategy,
		Grid:       g,
		Active:     active,
		CasterEyes: append([]EyeMask(nil), masks...),
	}
	switch strategy {
	case IndependentStrategy:
		store.buildIndependent(scene, masks, light)
	case HybridStrategy:
		store.buildHybrid(scene, masks, light, tileSize)
	default:
		store.buildShared(scene, masks, light)
	}
	if strategy == IndependentStrategy {
		store.CasterSet = 0
		if active.Has(LeftEye) {
			store.CasterSet += countMask(masks, LeftEye)
		}
		if active.Has(RightEye) {
			store.CasterSet += countMask(masks, RightEye)
		}
	} else {
		store.CasterSet = activeCasterCount(masks, active)
	}
	return store
}

func shadowAt(scene Scene, masks []EyeMask, light Vec3, p Vec2, eye EyeMask) bool {
	for i, c := range scene.Casters {
		if masks[i].Has(eye) && c.HitsLight(p, light) {
			return true
		}
	}
	return false
}

func activeCasterCount(masks []EyeMask, active EyeMask) int {
	if active == BothEyes {
		return countUnion(masks)
	}
	if active == LeftEye || active == RightEye {
		return countMask(masks, active)
	}
	return 0
}

func (s *ShadowStore) buildShared(scene Scene, masks []EyeMask, light Vec3) {
	s.Bits = make([]uint8, s.Grid.Size*s.Grid.Size)
	for iz := 0; iz < s.Grid.Size; iz++ {
		for ix := 0; ix < s.Grid.Size; ix++ {
			p := s.Grid.texel(ix, iz)
			var bits uint8
			if s.Active.Has(LeftEye) && shadowAt(scene, masks, light, p, LeftEye) {
				bits |= 1
			}
			if s.Active.Has(RightEye) && shadowAt(scene, masks, light, p, RightEye) {
				bits |= 2
			}
			s.Bits[iz*s.Grid.Size+ix] = bits
		}
	}
}

func (s *ShadowStore) buildIndependent(scene Scene, masks []EyeMask, light Vec3) {
	for e, bit := range []EyeMask{LeftEye, RightEye} {
		if !s.Active.Has(bit) {
			continue
		}
		layer := make([]uint8, s.Grid.Size*s.Grid.Size)
		for iz := 0; iz < s.Grid.Size; iz++ {
			for ix := 0; ix < s.Grid.Size; ix++ {
				if shadowAt(scene, masks, light, s.Grid.texel(ix, iz), bit) {
					layer[iz*s.Grid.Size+ix] = 1
				}
			}
		}
		s.Layers[e] = layer
	}
}

// Sample returns the stored shadow bit for an eye. Disabled eyes are still
// readable: SetActive never deletes another eye's map data.
func (s *ShadowStore) Sample(p Vec2, eye EyeMask) bool {
	ix, iz, ok := s.Grid.index(p)
	if !ok {
		return false
	}
	switch s.Strategy {
	case IndependentStrategy:
		e := 0
		if eye == RightEye {
			e = 1
		}
		if s.Layers[e] == nil {
			return false
		}
		return s.Layers[e][iz*s.Grid.Size+ix] != 0
	case HybridStrategy:
		return s.sampleHybrid(ix, iz, eye)
	default:
		bits := s.Bits[iz*s.Grid.Size+ix]
		if eye == LeftEye {
			return bits&1 != 0
		}
		if eye == RightEye {
			return bits&2 != 0
		}
		return false
	}
}

// SetActive flips eye availability without touching stored layers.
func (s *ShadowStore) SetActive(active EyeMask) { s.Active = active }

// ReuseRate is the fraction of active eye tile lookups served by storage
// shared between the two eyes during the build that produced this store.
func (s *ShadowStore) ReuseRate() float64 {
	if s.Active != BothEyes {
		return 0
	}
	switch s.Strategy {
	case IndependentStrategy:
		return 0
	case HybridStrategy:
		if s.TilesX*s.TilesZ == 0 {
			return 0
		}
		return 2 * float64(s.SharedTile) / float64(2*s.TilesX*s.TilesZ)
	default:
		return 1
	}
}

// Bytes is the modeled peak shadow-map storage for this strategy.
func (s *ShadowStore) Bytes() int {
	switch s.Strategy {
	case IndependentStrategy:
		n := 0
		for _, layer := range s.Layers {
			n += len(layer)
		}
		return n
	case HybridStrategy:
		n := 0
		for _, b := range s.Blocks {
			if b.shared {
				n += len(b.left) + 16
				continue
			}
			n += len(b.left) + len(b.right) + 32
		}
		return n
	default:
		return len(s.Bits)
	}
}
