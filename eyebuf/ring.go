package main

const ringSlots = 4

type slot struct {
	blk *Block
	eye int
	seq int
}

// Ring: a small pool of slots shared by both eyes. Producers grab a free (or
// the oldest non-presented) slot; the presented slot is never stolen. Memory
// is pooled across eyes, so a resize only reallocates individual slots.
type Ring struct {
	sim          *Sim
	cfg          [2]EyeConfig
	slots        [ringSlots]*slot
	latest       [2]int
	presentedSeq [2]int
}

func NewRing(sim *Sim) *Ring {
	return &Ring{sim: sim, latest: [2]int{-1, -1}}
}

func (s *Ring) Name() string { return "ring" }

func (s *Ring) Configure(eye int, cfg EyeConfig) { s.cfg[eye] = cfg } // lazy

// victim picks a slot that is not the latest for either eye, preferring empty
// slots, then size matches, then the oldest sequence.
func (s *Ring) victim(eye int) int {
	want := s.cfg[eye].Bytes()
	match, any := -1, -1
	for i, sl := range s.slots {
		if i == s.latest[0] || i == s.latest[1] {
			continue
		}
		if sl == nil || sl.blk == nil {
			return i
		}
		if sl.blk.Size == want && (match == -1 || sl.seq < s.slots[match].seq) {
			match = i
		}
		if any == -1 || sl.seq < s.slots[any].seq {
			any = i
		}
	}
	if match != -1 {
		return match
	}
	if any != -1 {
		return any
	}
	return s.latest[eye] // unreachable with 4 slots / 2 eyes
}

func (s *Ring) Submit(eye, seq int) {
	idx := s.victim(eye)
	sl := s.slots[idx]
	if sl == nil {
		sl = &slot{}
		s.slots[idx] = sl
	}
	want := s.cfg[eye].Bytes()
	if sl.blk == nil || sl.blk.Size != want {
		nb := s.sim.Alloc.Alloc(want)
		if nb == nil {
			s.sim.Metrics.ExpiredFrames++
			return
		}
		s.sim.Alloc.Free(sl.blk)
		sl.blk = nb
	}
	if s.latest[eye] >= 0 {
		if prev := s.slots[s.latest[eye]]; prev.seq > s.presentedSeq[eye] {
			s.sim.Metrics.ExpiredFrames++ // superseded before ever presented
		}
	}
	fillFrame(sl.blk.Buf, eye, seq)
	s.sim.Metrics.CopiedBytes += sl.blk.Size
	sl.eye = eye
	sl.seq = seq
	s.latest[eye] = idx
}

func (s *Ring) Present(eye int) (int, bool) {
	if s.latest[eye] < 0 {
		return 0, false
	}
	s.presentedSeq[eye] = s.slots[s.latest[eye]].seq
	return s.presentedSeq[eye], true
}

func (s *Ring) Front(eye int) []byte {
	if s.latest[eye] < 0 || s.slots[s.latest[eye]] == nil {
		return nil
	}
	return s.slots[s.latest[eye]].blk.Buf
}
