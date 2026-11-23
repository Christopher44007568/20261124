// Command foveqdemo runs a scripted VR workload through the joint quality
// controller and the per-eye budget scheduler, then writes telemetry JSON.
// It uses only the Go standard library.
package main

import (
	"fmt"
	"os"

	"foveq"
)

func main() {
	ladder := foveq.DefaultLadder()
	tel := foveq.NewTelemetry(2048)
	cfg := foveq.DefaultConfig(ladder)
	cfg.Telemetry = tel

	controller, err := foveq.NewController(cfg)
	if err != nil {
		fatal(err)
	}
	scheduler, err := foveq.NewScheduler(foveq.SchedulerConfig{
		WeightLeft: 3, WeightRight: 2,
		QuantumMicros: 1000, StarvationFrames: 2,
	})
	if err != nil {
		fatal(err)
	}

	type phase struct {
		name       string
		frames     int
		gpu        float64
		center     float64
		peripheral float64
		gaze       float64
		spike      bool
		failed     bool
	}
	phases := []phase{
		{"steady", 20, 4000, 0.03, 0.05, 30, false, false},
		{"quality demand", 18, 4200, 0.28, 0.10, 40, false, false},
		{"fast saccade", 12, 4300, 0.28, 0.10, 320, false, false},
		{"gaze settles", 16, 4300, 0.28, 0.10, 45, false, false},
		{"load burst", 10, 7800, 0.05, 0.06, 40, true, false},
		{"sustained overrun", 14, 8200, 0.05, 0.06, 40, false, false},
		{"frame failure", 1, 5000, 0.05, 0.06, 40, false, true},
		{"recovery", 24, 3800, 0.03, 0.04, 30, false, false},
	}

	var frame int64
	for _, ph := range phases {
		for range ph.frames {
			frame++
			sample := foveq.Sample{
				Frame: frame, GPUMicros: ph.gpu,
				CenterError: ph.center, PeripheralError: ph.peripheral,
				GazeVelocity: ph.gaze, LoadSpike: ph.spike, Failed: ph.failed,
			}
			profile, decision := controller.Observe(sample)

			demand := profile.FrameBudgetMicros / 2
			allocs := scheduler.Allocate([2]float64{demand, demand}, profile.FrameBudgetMicros)
			_ = allocs

			if frame == 1 || decision.FromTier != decision.ToTier {
				fmt.Printf("frame %-4d %-18s %s reasons=%v\n",
					frame, ph.name, profile, decision.Reasons)
			}
		}
	}

	stats := scheduler.Stats()
	fmt.Printf("\nscheduler served L=%d R=%d starvations L=%d R=%d shares=%.2f/%.2f\n",
		stats.Served[0], stats.Served[1],
		stats.Starvations[0], stats.Starvations[1],
		scheduler.Shares()[0], scheduler.Shares()[1])

	out, err := os.Create("telemetry.json")
	if err != nil {
		fatal(err)
	}
	defer out.Close()
	if err := tel.Dump(out); err != nil {
		fatal(err)
	}
	fmt.Printf("\nwrote %d decisions, %d events, %d trajectory points to telemetry.json\n",
		len(tel.Decisions()), len(tel.Events()), len(tel.Trajectory()))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "foveqdemo:", err)
	os.Exit(1)
}
