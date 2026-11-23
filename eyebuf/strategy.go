package main

// Strategy abstracts one eye-buffer layout. The engine drives it once per
// tick: Configure on (re)size, Submit per produced frame, Present per vsync.
type Strategy interface {
	Name() string
	Configure(eye int, cfg EyeConfig)
	Submit(eye, seq int)
	Present(eye int) (seq int, ok bool)
	Front(eye int) []byte // bytes of the currently presented frame
}

// Independent: each eye owns a private double buffer. A resize only touches
// that eye; the other eye's resources are never reallocated or copied.
type Independent struct {
	sim      *Sim
	cfg      [2]EyeConfig
	front    [2]*Block
	back     [2]*Block
	frontSeq [2]int
	backSeq  [2]int
	dirty    [2]bool
	hasFront [2]bool
}

func NewIndependent(sim *Sim) *Independent { return &Independent{sim: sim} }

func (s *Independent) Name() string { return "independent" }

// Configure only records the new config; reallocation is lazy so the old
// front buffer keeps the previous picture on screen until a new frame lands.
func (s *Independent) Configure(eye int, cfg EyeConfig) {
	s.cfg[eye] = cfg
}

func (s *Independent) ensureBack(eye int) bool {
	want := s.cfg[eye].Bytes()
	if s.back[eye] != nil && s.back[eye].Size == want {
		return true
	}
	b := s.sim.Alloc.Alloc(want)
	if b == nil {
		return false // keep showing the old front; frame is dropped
	}
	s.sim.Alloc.Free(s.back[eye])
	s.back[eye] = b
	return true
}

func (s *Independent) Submit(eye, seq int) {
	if !s.ensureBack(eye) {
		s.sim.Metrics.ExpiredFrames++
		return
	}
	if s.dirty[eye] {
		s.sim.Metrics.ExpiredFrames++ // previous back never reached the screen
	}
	fillFrame(s.back[eye].Buf, eye, seq)
	s.sim.Metrics.CopiedBytes += s.back[eye].Size
	s.backSeq[eye] = seq
	s.dirty[eye] = true
}

func (s *Independent) Present(eye int) (int, bool) {
	if s.dirty[eye] {
		s.front[eye], s.back[eye] = s.back[eye], s.front[eye]
		s.frontSeq[eye] = s.backSeq[eye]
		s.dirty[eye] = false
		s.hasFront[eye] = true
	}
	return s.frontSeq[eye], s.hasFront[eye]
}

func (s *Independent) Front(eye int) []byte {
	if s.front[eye] == nil {
		return nil
	}
	return s.front[eye].Buf
}
