package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func firstSaccadeOnset(frames []Frame, after int) int {
	for i := after + 1; i < len(frames); i++ {
		if frames[i].Saccade && !frames[i-1].Saccade {
			return i
		}
	}
	return -1
}

func main() {
	cfg := SimConfig{
		Frames:    720,
		Width:     640,
		Height:    360,
		NumSacc:   8,
		NumBlink:  3,
		Seed:      7,
		AnomalyAt: 200,
	}
	frames, saccs := Simulate(cfg)

	strategies := []Strategy{
		NewNoPredict(),
		NewConstantDelay(2, 6),
		NewPredictor(5, 10, 20, 1.25),
	}
	results := make([]*Result, 0, len(strategies))
	for _, s := range strategies {
		results = append(results, Run(s, frames, saccs))
	}
	printTable(results)
	if os.Getenv("DEBUG") != "" {
		fmt.Println("saccade onsets:")
		for _, sc := range saccs {
			fmt.Printf("  onset=%d dur=%d\n", sc.Onset, sc.Dur)
		}
		for _, s := range strategies {
			fmt.Println(s.Name(), "events:")
			for _, e := range s.Events() {
				fmt.Printf("  start=%d end=%d pred=%d\n", e.Start, e.End, e.PredictedOnset)
			}
		}
	}

	outDir := "out"
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	onset := firstSaccadeOnset(frames, 120)
	for _, r := range results {
		for _, at := range []int{onset - 3, onset + 2, onset + 6} {
			img := RenderFrame(cfg, frames[at], r.Decisions[at])
			path := filepath.Join(outDir, fmt.Sprintf("%s_f%03d.png", r.Name, at))
			SavePNG(path, img)
		}
	}
	fmt.Println("rendered frames around saccade onset", onset, "into", outDir)
}
