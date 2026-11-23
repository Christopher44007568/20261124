package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const totalFrames = 90

// FrameStat holds the per-frame metrics of one strategy.
type FrameStat struct {
	Frame      int
	Candidates int
	Rendered   int
	Misses     int
	Overdraw   int
}

// RunSim executes the whole simulation and returns per-strategy stats plus
// the list of frames in which the union strategy missed something.
func RunSim(snapshot map[int]bool, outDir string, verbose ...bool) (map[string][]FrameStat, []int) {
	scene := BuildScene()
	params := DefaultParams()
	predict := NewPredictStrategy()
	predict.Log = len(verbose) > 0 && verbose[0]
	strategies := []Strategy{&NoneStrategy{}, &UnionStrategy{}, predict}

	stats := map[string][]FrameStat{}
	var unionMissFrames []int

	for f := 0; f < totalFrames; f++ {
		t := float64(f) / float64(totalFrames)
		env := NewFrameEnv(scene, params, t)
		req := env.Required()

		for _, st := range strategies {
			cand := st.Candidates(env)
			candSet := make(map[int]bool, len(cand))
			for _, i := range cand {
				candSet[i] = true
			}
			var rendered []int
			misses, overdraw := 0, 0
			for i := range req {
				if candSet[i] {
					rendered = append(rendered, i)
				} else {
					misses++
				}
			}
			for _, i := range cand {
				if !req[i] {
					overdraw++
				}
			}
			stats[st.Name()] = append(stats[st.Name()], FrameStat{
				Frame: f, Candidates: len(cand), Rendered: len(rendered),
				Misses: misses, Overdraw: overdraw,
			})
			if st.Name() == "union" && misses > 0 {
				unionMissFrames = append(unionMissFrames, f)
			}
			st.Observe(rendered, misses)
			st.Track(env)
			if snapshot[f] {
				path := filepath.Join(outDir,
					fmt.Sprintf("frame_%03d_%s.png", f, st.Name()))
				last := stats[st.Name()][len(stats[st.Name()])-1]
				if err := DrawFrame(env, candSet, req, st.Name(), f, last, path); err != nil {
					fmt.Fprintln(os.Stderr, "snapshot:", err)
				}
			}
		}
	}
	return stats, unionMissFrames
}

// pickSnapshots chooses up to 3 evenly spaced frames from the miss list.
func pickSnapshots(missFrames []int) map[int]bool {
	snap := map[int]bool{}
	if len(missFrames) == 0 {
		snap[totalFrames/2] = true
		return snap
	}
	picks := []int{missFrames[0]}
	if len(missFrames) > 2 {
		picks = append(picks, missFrames[len(missFrames)/2])
	}
	if len(missFrames) > 1 {
		picks = append(picks, missFrames[len(missFrames)-1])
	}
	for _, f := range picks {
		snap[f] = true
	}
	return snap
}

func writeCSV(path string, stats map[string][]FrameStat, order []string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fmt.Fprintln(f, "frame,strategy,candidates,rendered,misses,overdraw")
	for _, name := range order {
		for _, s := range stats[name] {
			fmt.Fprintf(f, "%d,%s,%d,%d,%d,%d\n",
				s.Frame, name, s.Candidates, s.Rendered, s.Misses, s.Overdraw)
		}
	}
	return nil
}

func printSummary(stats map[string][]FrameStat, order []string) {
	fmt.Printf("%-8s %8s %8s %8s %8s %8s %10s\n",
		"strategy", "avgCand", "avgRend", "totMiss", "maxMiss", "totOver", "missFrames")
	for _, name := range order {
		var cand, rend, miss, over, maxMiss, missFrames int
		for _, s := range stats[name] {
			cand += s.Candidates
			rend += s.Rendered
			miss += s.Misses
			over += s.Overdraw
			if s.Misses > maxMiss {
				maxMiss = s.Misses
			}
			if s.Misses > 0 {
				missFrames++
			}
		}
		n := float64(len(stats[name]))
		fmt.Printf("%-8s %8.1f %8.1f %8d %8d %8d %10d\n",
			name, float64(cand)/n, float64(rend)/n, miss, maxMiss, over, missFrames)
	}
}

// clearFrameSnapshots removes stale frame_*.png files from previous runs.
func clearFrameSnapshots(dir string) {
	matches, err := filepath.Glob(filepath.Join(dir, "frame_*.png"))
	if err != nil {
		return
	}
	for _, m := range matches {
		os.Remove(m)
	}
}

func main() {
	outDir := flag.String("out", "out", "output directory for PNG/CSV artifacts")
	flag.Parse()
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Pass 1: discover which frames the union strategy misses on.
	_, unionMiss := RunSim(nil, *outDir)

	// Pass 2: deterministic re-run, capturing snapshots on those frames.
	clearFrameSnapshots(*outDir)
	snap := pickSnapshots(unionMiss)
	stats, _ := RunSim(snap, *outDir, true)

	order := []string{"none", "union", "predict"}
	if err := writeCSV(filepath.Join(*outDir, "metrics.csv"), stats, order); err != nil {
		fmt.Fprintln(os.Stderr, "csv:", err)
	}
	if err := DrawChart(stats, order, filepath.Join(*outDir, "metrics.png")); err != nil {
		fmt.Fprintln(os.Stderr, "chart:", err)
	}

	printSummary(stats, order)

	frames := make([]int, 0, len(snap))
	for f := range snap {
		frames = append(frames, f)
	}
	sort.Ints(frames)
	fmt.Println("snapshot frames:", frames)
	fmt.Println("artifacts written to", *outDir)
}
