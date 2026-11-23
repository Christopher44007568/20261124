package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

func ms(d time.Duration) string {
	return strconv.FormatFloat(d.Seconds()*1000, 'f', 3, 64)
}

func rad(v float64) string {
	return strconv.FormatFloat(v, 'f', 9, 64)
}

// WriteRecordsCSV writes every accepted prediction with error and overshoot.
func WriteRecordsCSV(path string, results []RunResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	header := []string{
		"scenario", "mode", "eye", "submit_ms", "scanout_ms",
		"true_yaw_rad", "pred_yaw_rad", "abs_err_rad", "abs_err_deg",
		"signed_err_rad", "overshoot_rad", "gap_age_ms",
	}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, res := range results {
		for _, r := range res.Records {
			row := []string{
				r.Scenario, r.Mode.String(), r.Eye, ms(r.SubmitAt), ms(r.ScanoutAt),
				rad(r.TrueYaw), rad(r.PredYaw), rad(r.AbsErrRad),
				rad(rad2deg(r.AbsErrRad)), rad(r.SignedErr), rad(r.Overshoot),
				ms(r.GapAge),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
	}
	w.Flush()
	return w.Error()
}

// WriteRejectsCSV writes every refused sample and query with its reason.
func WriteRejectsCSV(path string, results []RunResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"scenario", "mode", "kind", "at_ms", "target_ms", "reason",
	}); err != nil {
		return err
	}
	for _, res := range results {
		for _, rj := range res.Rejects {
			row := []string{
				res.Scenario, rj.Mode.String(), rj.Kind, ms(rj.At),
				ms(rj.Target), string(rj.Reason),
			}
			if err := w.Write(row); err != nil {
				return err
			}
		}
	}
	w.Flush()
	return w.Error()
}

// WriteSummaryCSV writes one aggregated row per scenario x mode x eye.
func WriteSummaryCSV(path string, results []RunResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"scenario", "mode", "eye", "count",
		"mean_abs_err_rad", "mean_abs_err_deg",
		"rmse_rad", "rmse_deg", "max_abs_err_rad", "max_abs_err_deg",
		"max_overshoot_rad", "max_overshoot_deg",
	}); err != nil {
		return err
	}
	for _, res := range results {
		for _, eye := range []string{"left", "right", "both"} {
			s := res.Stats[eye]
			if err := w.Write([]string{
				res.Scenario, res.Mode.String(), eye, strconv.Itoa(s.Count),
				rad(s.MeanAbsRad), rad(rad2deg(s.MeanAbsRad)),
				rad(s.RMSE), rad(rad2deg(s.RMSE)),
				rad(s.MaxAbsRad), rad(rad2deg(s.MaxAbsRad)),
				rad(s.MaxOvershoot), rad(rad2deg(s.MaxOvershoot)),
			}); err != nil {
				return err
			}
		}
	}
	w.Flush()
	return w.Error()
}

// WriteSummaryText renders a human-readable comparison table.
func WriteSummaryText(path string, results []RunResult, cfg Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return RenderSummaryText(f, results, cfg)
}

// RenderSummaryText is separated for testability.
func RenderSummaryText(w io.Writer, results []RunResult, cfg Config) error {
	if _, err := fmt.Fprintln(w, "Head-pose timing experiment (deterministic, stdlib only)"); err != nil {
		return err
	}
	fmt.Fprintf(w, "sample rate: 1000 Hz; eye offsets: left %v, right %v\n", LeftEyeOffset, RightEyeOffset)
	fmt.Fprintf(w, "history window: %v; horizon: %v; velocity EMA tau: %v; |omega| clamp: %.1f rad/s\n\n",
		cfg.HistoryWindow, cfg.Horizon, cfg.VelocityTau, cfg.MaxAngularVel)

	for _, res := range results {
		fmt.Fprintf(w, "scenario=%s mode=%s rejects=%d\n", res.Scenario, res.Mode.String(), len(res.Rejects))
		for _, eye := range []string{"left", "right"} {
			s := res.Stats[eye]
			fmt.Fprintf(w,
				"  %-5s n=%4d mean=%7.3f deg rmse=%7.3f deg max=%7.3f deg maxOvershoot=%7.3f deg\n",
				eye, s.Count, rad2deg(s.MeanAbsRad), rad2deg(s.RMSE),
				rad2deg(s.MaxAbsRad), rad2deg(s.MaxOvershoot))
		}
		fmt.Fprintln(w)
	}
	return nil
}

// EnsureOutputDir creates the output directory if needed.
func EnsureOutputDir(dir string) error {
	return os.MkdirAll(filepath.Clean(dir), 0o755)
}
