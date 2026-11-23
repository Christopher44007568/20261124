package shadow

import "bytes"

func (s *ShadowStore) buildHybrid(scene Scene, masks []EyeMask, light Vec3, tileSize int) {
	if tileSize < 1 {
		tileSize = 8
	}
	s.TileSize = tileSize
	s.TilesX = (s.Grid.Size + tileSize - 1) / tileSize
	s.TilesZ = s.TilesX
	s.Blocks = make([]tileBlock, s.TilesX*s.TilesZ)

	for tz := 0; tz < s.TilesZ; tz++ {
		for tx := 0; tx < s.TilesX; tx++ {
			tile := tz*s.TilesX + tx
			var left, right []uint8
			if s.Active.Has(LeftEye) {
				left = s.buildTile(scene, masks, light, tx, tz, LeftEye)
			}
			if s.Active.Has(RightEye) {
				right = s.buildTile(scene, masks, light, tx, tz, RightEye)
			}
			if left != nil && right != nil && bytes.Equal(left, right) {
				s.Blocks[tile] = tileBlock{shared: true, left: left, right: left}
				s.SharedTile++
				continue
			}
			s.Blocks[tile] = tileBlock{left: left, right: right}
		}
	}
}

func (s *ShadowStore) buildTile(scene Scene, masks []EyeMask, light Vec3,
	tx, tz int, eye EyeMask) []uint8 {
	w, h := s.tileExtent(tx, tz)
	data := make([]uint8, w*h)
	for z := 0; z < h; z++ {
		for x := 0; x < w; x++ {
			p := s.Grid.texel(tx*s.TileSize+x, tz*s.TileSize+z)
			if shadowAt(scene, masks, light, p, eye) {
				data[z*w+x] = 1
			}
		}
	}
	return data
}

func (s *ShadowStore) tileExtent(tx, tz int) (int, int) {
	w := s.TileSize
	if remain := s.Grid.Size - tx*s.TileSize; remain < w {
		w = remain
	}
	h := s.TileSize
	if remain := s.Grid.Size - tz*s.TileSize; remain < h {
		h = remain
	}
	return w, h
}

func (s *ShadowStore) sampleHybrid(ix, iz int, eye EyeMask) bool {
	tx, tz := ix/s.TileSize, iz/s.TileSize
	tile := tz*s.TilesX + tx
	if tile < 0 || tile >= len(s.Blocks) {
		return false
	}
	b := s.Blocks[tile]
	data := b.left
	if eye == RightEye && !b.shared {
		data = b.right
	}
	if data == nil {
		return false
	}
	w, _ := s.tileExtent(tx, tz)
	lx, lz := ix-tx*s.TileSize, iz-tz*s.TileSize
	return data[lz*w+lx] != 0
}
