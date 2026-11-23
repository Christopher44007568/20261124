package main

// Scenario describes one test run. Disabled features use -1 sentinels.
type Scenario struct {
	Name      string
	Frames    int
	Cfg       [2]EyeConfig
	Budget    int64 // 0 = unlimited
	Resizes   []ResizeEvent
	PauseEye  int // eye that stops submitting in [PauseFrom, PauseTo)
	PauseFrom int
	PauseTo   int
	DelayEye  int // eye whose frames commit Delay ticks late
	Delay     int
	BurstEye  int // eye that submits twice per tick in [BurstFrom, BurstTo)
	BurstFrom int
	BurstTo   int
}

// ResizeEvent hot-changes one eye's resolution/format at a given tick.
type ResizeEvent struct {
	Tick int
	Eye  int
	To   EyeConfig
}

func (sc Scenario) disturbed() [2]bool {
	var d [2]bool
	for _, r := range sc.Resizes {
		d[r.Eye] = true
	}
	if sc.PauseFrom >= 0 {
		d[sc.PauseEye] = true
	}
	if sc.Delay > 0 {
		d[sc.DelayEye] = true
	}
	if sc.BurstFrom >= 0 {
		d[sc.BurstEye] = true
	}
	return d
}

// Run executes one scenario against one strategy and collects metrics.
func Run(sc Scenario, newStrat func(*Sim) Strategy) Metrics {
	alloc := &Allocator{Budget: sc.Budget}
	m := &Metrics{}
	sim := &Sim{Alloc: alloc, Metrics: m, Cfg: sc.Cfg}
	st := newStrat(sim)
	for e := 0; e < 2; e++ {
		st.Configure(e, sc.Cfg[e])
	}
	disturbed := sc.disturbed()
	submitted := [2]int{-1, -1}

	for t := 0; t < sc.Frames; t++ {
		for _, r := range sc.Resizes {
			if t == r.Tick {
				sim.Cfg[r.Eye] = r.To
				st.Configure(r.Eye, r.To)
			}
		}
		for eye := 0; eye < 2; eye++ {
			if sc.PauseFrom >= 0 && eye == sc.PauseEye && t >= sc.PauseFrom && t < sc.PauseTo {
				continue // paused eye produces nothing
			}
			delay := 0
			if sc.Delay > 0 && eye == sc.DelayEye {
				delay = sc.Delay
			}
			seq := t - delay
			if seq < 0 {
				continue
			}
			st.Submit(eye, seq)
			submitted[eye] = seq
			if sc.BurstFrom >= 0 && eye == sc.BurstEye && t >= sc.BurstFrom && t < sc.BurstTo {
				st.Submit(eye, seq) // fast producer: second frame same tick
			}
		}
		for eye := 0; eye < 2; eye++ {
			seq, ok := st.Present(eye)
			if !ok {
				continue
			}
			if !checkFrame(st.Front(eye), eye, seq) {
				m.ContentErrors++
			}
			// The undisturbed eye must always present its freshest frame:
			// it must never be held back by the other eye's invalid resources.
			if !disturbed[eye] && seq < submitted[eye] {
				m.Stalls++
			}
		}
	}
	m.PeakMem = alloc.Peak
	m.AllocFailures = alloc.Failures
	return *m
}
