package main

import (
	"fmt"
	"os"
	"text/tabwriter"
)

func human(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMiB", float64(n)/float64(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0fKiB", float64(n)/float64(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func scenarios() []Scenario {
	base := EyeConfig{512, 512, FmtRGBA8}
	big := EyeConfig{1024, 1024, FmtRGBA8}
	half := EyeConfig{512, 512, FmtRGBA16F}
	small := EyeConfig{256, 256, FmtRGB8}
	mid := EyeConfig{768, 768, FmtRGBA8}
	pair := func(c EyeConfig) [2]EyeConfig { return [2]EyeConfig{c, c} }
	none := -1
	return []Scenario{
		{Name: "baseline 512px RGBA8 x90f", Frames: 90, Cfg: pair(base),
			PauseFrom: none, BurstFrom: none},
		{Name: "delay eye0 +3t 512px RGBA8", Frames: 90, Cfg: pair(base),
			PauseFrom: none, BurstFrom: none, DelayEye: 0, Delay: 3},
		{Name: "pause eye1 [30,60) 512px RGBA8", Frames: 90, Cfg: pair(base),
			PauseEye: 1, PauseFrom: 30, PauseTo: 60, BurstFrom: none},
		{Name: "hotresize eye0 512->1024 @30", Frames: 90, Cfg: pair(base),
			Resizes: []ResizeEvent{{30, 0, big}}, PauseFrom: none, BurstFrom: none},
		{Name: "hotresize eye0 + pause eye1", Frames: 90, Cfg: pair(base),
			Resizes:  []ResizeEvent{{30, 0, big}},
			PauseEye: 1, PauseFrom: 20, PauseTo: 70, BurstFrom: none},
		{Name: "multi-resize eye0 x3 @20/40/60", Frames: 90, Cfg: pair(base),
			Resizes:   []ResizeEvent{{20, 0, mid}, {40, 0, big}, {60, 0, small}},
			PauseFrom: none, BurstFrom: none},
		{Name: "hotresize tight budget 5MiB", Frames: 90, Cfg: pair(base),
			Budget: 5 << 20, Resizes: []ResizeEvent{{30, 0, big}},
			PauseFrom: none, BurstFrom: none},
		{Name: "burst eye0 [20,50) RGBA16F", Frames: 90, Cfg: pair(half),
			PauseFrom: none, BurstEye: 0, BurstFrom: 20, BurstTo: 50},
		{Name: "small 256px RGB8 x150f", Frames: 150, Cfg: pair(small),
			PauseFrom: none, BurstFrom: none},
	}
}

func main() {
	strategies := []struct {
		name string
		make func(*Sim) Strategy
	}{
		{"independent", func(s *Sim) Strategy { return NewIndependent(s) }},
		{"atlas", func(s *Sim) Strategy { return NewAtlas(s) }},
		{"ring", func(s *Sim) Strategy { return NewRing(s) }},
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "SCENARIO\tSTRATEGY\tPEAK MEM\tCOPIED\tEXPIRED\tALLOC FAIL\tSTALLS\tCONTENT ERR")
	totals := map[string]Metrics{}
	bad := false
	for _, sc := range scenarios() {
		for _, st := range strategies {
			m := Run(sc, st.make)
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n",
				sc.Name, st.name, human(m.PeakMem), human(m.CopiedBytes),
				m.ExpiredFrames, m.AllocFailures, m.Stalls, m.ContentErrors)
			t := totals[st.name]
			t.PeakMem += m.PeakMem
			t.CopiedBytes += m.CopiedBytes
			t.ExpiredFrames += m.ExpiredFrames
			t.AllocFailures += m.AllocFailures
			t.Stalls += m.Stalls
			t.ContentErrors += m.ContentErrors
			totals[st.name] = t
			if m.Stalls > 0 || m.ContentErrors > 0 {
				bad = true
			}
		}
	}
	fmt.Fprintln(tw, "\t\t\t\t\t\t\t")
	for _, st := range strategies {
		t := totals[st.name]
		fmt.Fprintf(tw, "TOTAL\t%s\t%s\t%s\t%d\t%d\t%d\t%d\n",
			st.name, human(t.PeakMem), human(t.CopiedBytes),
			t.ExpiredFrames, t.AllocFailures, t.Stalls, t.ContentErrors)
	}
	tw.Flush()

	fmt.Println()
	fmt.Println("Invariant: undisturbed eye never stalls, presented pixels never corrupt.")
	for _, st := range strategies {
		t := totals[st.name]
		status := "PASS"
		if t.Stalls > 0 || t.ContentErrors > 0 {
			status = "FAIL"
		}
		fmt.Printf("  %-11s %s (stalls=%d contentErr=%d)\n", st.name, status, t.Stalls, t.ContentErrors)
	}
	fmt.Println()
	fmt.Println("Verdict: independent buffers are the most robust layout. Each eye")
	fmt.Println("owns its own buffers: hot resize, pause and alloc failure never touch")
	fmt.Println("the other eye (0 cross-eye copies, 0 stalls). Ring shares a")
	fmt.Println("slot pool - flexible and lazy, but expires frames under bursts and")
	fmt.Println("tight budgets. Atlas has the lowest steady-state footprint (single")
	fmt.Println("buffer) but couples both eyes: every resize reallocates the whole")
	fmt.Println("atlas and migrates the healthy eye's pixels (see multi-resize row),")
	fmt.Println("its realloc is all-or-nothing, and in-place submits risk tearing.")
	if bad {
		os.Exit(1)
	}
}
