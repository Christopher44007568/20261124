package main

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"rollingshutter"
)

func TestRunEmitsCurvatureAndTimeError(t *testing.T) {
	var output bytes.Buffer
	if err := run([]string{"-summary-only"}, &output); err != nil {
		t.Fatal(err)
	}
	var analysis rollingshutter.Analysis
	if err := json.Unmarshal(output.Bytes(), &analysis); err != nil {
		t.Fatal(err)
	}
	if analysis.Summary.StraightLineCurvature.MaxPixels <= 0 {
		t.Fatalf("curvature = %+v", analysis.Summary.StraightLineCurvature)
	}
	if analysis.Summary.MaxAbsTimeErrorNS <= 0 {
		t.Fatalf("time error = %d", analysis.Summary.MaxAbsTimeErrorNS)
	}
	if analysis.Rows != nil {
		t.Fatal("summary-only output contains rows")
	}
}

func TestRunMarksReversedReadoutAndMissingPose(t *testing.T) {
	var output bytes.Buffer
	err := run([]string{
		"-summary-only",
		"-direction", "bottom-to-top",
		"-drop-pose-start", "2ms",
		"-drop-pose-end", "2500us",
	}, &output)
	if err != nil {
		t.Fatal(err)
	}
	var analysis rollingshutter.Analysis
	if err := json.Unmarshal(output.Bytes(), &analysis); err != nil {
		t.Fatal(err)
	}
	if !analysis.Summary.DirectionReversed {
		t.Fatal("direction reversal not marked")
	}
	if analysis.Summary.PoseMissingRows == 0 {
		t.Fatal("missing pose rows not counted")
	}
	if !slices.Contains(analysis.Summary.Flags, rollingshutter.FlagReadoutDirectionReversed) ||
		!slices.Contains(analysis.Summary.Flags, rollingshutter.FlagPoseSamplesMissing) {
		t.Fatalf("flags = %v", analysis.Summary.Flags)
	}
}

func TestRunRejectsUnknownDirection(t *testing.T) {
	if err := run([]string{"-direction", "sideways"}, &bytes.Buffer{}); err == nil {
		t.Fatal("expected unknown direction error")
	}
}
