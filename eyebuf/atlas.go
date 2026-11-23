package main

// Atlas packs both eyes into one contiguous allocation. Any resize reallocates
// the whole atlas: old and new coexist during the switch (peak spike) and the
// untouched eye's committed picture must be migrated to stay on screen.
type Atlas struct {
	sim  *Sim
	cfg  [2]EyeConfig
	blk  *Block
	size [2]int64
	off  [2]int64
	ok   [2]bool // region matches cfg
	has  [2]bool // region holds a committed frame
	seq  [2]int
}

func NewAtlas(sim *Sim) *Atlas { return &Atlas{sim: sim} }

func (s *Atlas) Name() string { return "atlas" }

func (s *Atlas) Configure(eye int, cfg EyeConfig) {
	s.cfg[eye] = cfg
	need := [2]int64{s.cfg[0].Bytes(), s.cfg[1].Bytes()}
	if s.blk != nil && s.size == need {
		return
	}
	nb := s.sim.Alloc.Alloc(need[0] + need[1])
	if nb == nil {
		s.ok[eye] = false // cannot grow: resized eye is stuck, other eye keeps old region
		return
	}
	other := 1 - eye
	if s.blk != nil && s.ok[other] && s.has[other] {
		dst := int64(0)
		if other == 1 {
			dst = need[0]
		}
		src := s.blk.Buf[s.off[other] : s.off[other]+s.size[other]]
		copy(nb.Buf[dst:dst+s.size[other]], src)
		s.sim.Metrics.CopiedBytes += s.size[other] // healthy eye dragged along
	}
	s.sim.Alloc.Free(s.blk)
	s.blk = nb
	s.size = need
	s.off = [2]int64{0, need[0]}
	s.ok = [2]bool{true, true}
	s.has[eye] = false // resized eye's old pixels are gone until next submit
}

func (s *Atlas) Submit(eye, seq int) {
	if s.blk == nil || !s.ok[eye] {
		s.sim.Metrics.ExpiredFrames++
		return
	}
	region := s.blk.Buf[s.off[eye] : s.off[eye]+s.size[eye]]
	fillFrame(region, eye, seq)
	s.sim.Metrics.CopiedBytes += s.size[eye]
	s.seq[eye] = seq
	s.has[eye] = true
}

func (s *Atlas) Present(eye int) (int, bool) {
	return s.seq[eye], s.has[eye]
}

func (s *Atlas) Front(eye int) []byte {
	if s.blk == nil || !s.has[eye] {
		return nil
	}
	return s.blk.Buf[s.off[eye] : s.off[eye]+s.size[eye]]
}
