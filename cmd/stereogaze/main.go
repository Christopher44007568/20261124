package main

import (
	"flag"
	"fmt"
	"os"

	"stereogaze/internal/gaze"
	"stereogaze/internal/record"
	"stereogaze/internal/scenario"
)

func main() {
	output := flag.String("out", "out", "output directory for PNG frames, GIFs and records")
	gridSize := flag.Int("grid", 32, "sampling grid size per eye")
	frameCount := flag.Int("frames", 16, "frames generated per scenario")
	flag.Parse()

	if err := run(*output, *gridSize, *frameCount); err != nil {
		fmt.Fprintln(os.Stderr, "stereogaze:", err)
		os.Exit(1)
	}
}

func run(output string, gridSize, frameCount int) error {
	recorder, err := record.NewRecorder(output)
	if err != nil {
		return err
	}

	for _, current := range scenario.All(frameCount) {
		metrics := recorder.BeginScenario(current.Name, gridSize)
		for _, frame := range current.Frames {
			left, err := gaze.BuildMask(frame.Left, gridSize)
			if err != nil {
				return fmt.Errorf("%s frame %d left eye: %w", current.Name, frame.Index, err)
			}
			right, err := gaze.BuildMask(frame.Right, gridSize)
			if err != nil {
				return fmt.Errorf("%s frame %d right eye: %w", current.Name, frame.Index, err)
			}
			report, err := gaze.AnalyzeSharing(left, right)
			if err != nil {
				return fmt.Errorf("%s frame %d sharing: %w", current.Name, frame.Index, err)
			}
			if err := recorder.AddFrame(metrics, current.Name, frame.Index, gridSize, left, right, report); err != nil {
				return err
			}
		}
		recorder.CommitScenario(metrics)
		fmt.Printf("%s: %d frames recorded\n", current.Name, len(metrics.Frames))
	}
	if err := recorder.Finish(); err != nil {
		return err
	}
	fmt.Println("records written to", output)
	return nil
}
