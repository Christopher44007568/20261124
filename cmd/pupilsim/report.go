package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"

	"pupilsim/internal/plot"
	"pupilsim/internal/sim"
)

// writeCSV dumps per-frame metrics.
func writeCSV(path string, frames []sim.Frame) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"t_s", "drift_l_arcmin", "drift_r_arcmin",
		"disparity_arcmin", "corner_mm", "jump_arcmin",
		"offset_jump_arcmin", "lost"})
	for _, fr := range frames {
		w.Write([]string{
			ftoa(fr.T), ftoa(fr.DriftL), ftoa(fr.DriftR),
			ftoa(fr.Disparity), ftoa(fr.CornerMM), ftoa(fr.Jump),
			ftoa(fr.OffsetJump),
			strconv.FormatBool(fr.Lost),
		})
	}
	return w.Error()
}

// writeChart renders drift, disparity, and loss bands for one run.
func writeChart(path string, sc sim.Scenario, comp string, frames []sim.Frame) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	n := len(frames)
	dl := make([]float64, n)
	dr := make([]float64, n)
	disp := make([]float64, n)
	lost := make([]bool, n)
	yMax := 0.0
	for i, fr := range frames {
		dl[i] = fr.DriftL
		dr[i] = fr.DriftR
		disp[i] = fr.Disparity
		lost[i] = fr.Lost
		if fr.DriftL > yMax {
			yMax = fr.DriftL
		}
		if fr.DriftR > yMax {
			yMax = fr.DriftR
		}
	}
	title := fmt.Sprintf("%s / %s (arcmin)", sc.Name, comp)
	series := []plot.Series{
		{Name: "drift-l", Color: palette[0], Y: dl},
		{Name: "drift-r", Color: palette[1], Y: dr},
		{Name: "disparity", Color: palette[2], Y: disp},
		{Name: "loss", Color: palette[3], Y: plot.LossBands(lost, yMax)},
	}
	return plot.Chart(f, title, "arcmin", series, 0)
}

// writeSummary dumps the aggregate table.
func writeSummary(path string, sums []sim.Summary) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"scenario", "compensator", "max_drift_arcmin",
		"rms_drift_arcmin", "max_corner_mm", "max_disparity_arcmin",
		"rms_disparity_arcmin", "max_jump_arcmin", "max_jump_loss_arcmin",
		"max_offset_jump_arcmin", "max_offset_jump_loss_arcmin"})
	for _, s := range sums {
		w.Write([]string{s.Scenario, s.Compensator,
			ftoa(s.MaxDrift), ftoa(s.RMSDrift), ftoa(s.MaxCornerMM),
			ftoa(s.MaxDisparity), ftoa(s.RMSDisparity),
			ftoa(s.MaxJump), ftoa(s.MaxJumpLoss),
			ftoa(s.MaxOffsetJump), ftoa(s.MaxOffsetJumpLoss)})
	}
	return w.Error()
}

// printTable echoes the summary to stdout.
func printTable(sums []sim.Summary) {
	fmt.Printf("%-9s %-9s %9s %9s %10s %11s %9s %10s %10s\n",
		"scenario", "comp", "maxDrift", "rmsDrift", "cornerMM",
		"maxDispar", "maxJump", "jumpLoss", "offJmpLos")
	for _, s := range sums {
		fmt.Printf("%-9s %-9s %9.2f %9.2f %10.2f %11.2f %9.2f %10.2f %10.2f\n",
			s.Scenario, s.Compensator, s.MaxDrift, s.RMSDrift,
			s.MaxCornerMM, s.MaxDisparity, s.MaxJump, s.MaxJumpLoss,
			s.MaxOffsetJumpLoss)
	}
}

func ftoa(v float64) string { return strconv.FormatFloat(v, 'f', 4, 64) }
