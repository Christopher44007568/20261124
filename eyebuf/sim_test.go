package main

import "testing"

func allStrategies() map[string]func(*Sim) Strategy {
	return map[string]func(*Sim) Strategy{
		"independent": func(s *Sim) Strategy { return NewIndependent(s) },
		"atlas":       func(s *Sim) Strategy { return NewAtlas(s) },
		"ring":        func(s *Sim) Strategy { return NewRing(s) },
	}
}

// The undisturbed eye must never be stalled by the other eye's invalidated
// resources, and presented pixels must stay bit-identical to what was
// produced, across every scenario and every layout.
func TestIsolationAndImageStability(t *testing.T) {
	for _, sc := range scenarios() {
		for name, mk := range allStrategies() {
			m := Run(sc, mk)
			if m.Stalls > 0 {
				t.Errorf("%s/%s: healthy eye stalled %d times", sc.Name, name, m.Stalls)
			}
			if m.ContentErrors > 0 {
				t.Errorf("%s/%s: %d corrupted presented frames", sc.Name, name, m.ContentErrors)
			}
		}
	}
}

// A paused eye must keep presenting its last frame, unchanged, while the
// other eye hot-resizes underneath it.
func TestPausedEyeKeepsPicture(t *testing.T) {
	base := EyeConfig{512, 512, FmtRGBA8}
	sc := Scenario{
		Name: "resize-under-pause", Frames: 60,
		Cfg:      [2]EyeConfig{base, base},
		Resizes:  []ResizeEvent{{30, 0, EyeConfig{1024, 1024, FmtRGBA8}}},
		PauseEye: 1, PauseFrom: 10, PauseTo: 60,
		BurstFrom: -1,
	}
	for name, mk := range allStrategies() {
		alloc := &Allocator{}
		m := &Metrics{}
		sim := &Sim{Alloc: alloc, Metrics: m, Cfg: sc.Cfg}
		st := mk(sim)
		st.Configure(0, sc.Cfg[0])
		st.Configure(1, sc.Cfg[1])
		lastSeq := -1
		for tick := 0; tick < sc.Frames; tick++ {
			for _, r := range sc.Resizes {
				if tick == r.Tick {
					st.Configure(r.Eye, r.To)
				}
			}
			if !(tick >= sc.PauseFrom && tick < sc.PauseTo) {
				st.Submit(0, tick)
			}
			if tick < sc.PauseFrom {
				st.Submit(1, tick)
			}
			seq, ok := st.Present(1)
			if !ok {
				t.Fatalf("%s: paused eye lost its picture at tick %d", name, tick)
			}
			if lastSeq >= 0 && tick >= sc.PauseFrom && seq != lastSeq {
				t.Fatalf("%s: paused eye frame changed %d -> %d", name, lastSeq, seq)
			}
			if !checkFrame(st.Front(1), 1, seq) {
				t.Fatalf("%s: paused eye pixels corrupted at tick %d", name, tick)
			}
			lastSeq = seq
		}
	}
}
