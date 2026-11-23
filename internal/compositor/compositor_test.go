package compositor

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func decisionAt(t *testing.T, result Result, eye Eye, x, y int) PixelDecision {
	t.Helper()
	for _, eyeResult := range result.Eyes {
		if eyeResult.Eye != eye {
			continue
		}
		for _, decision := range eyeResult.Decisions {
			if decision.X == x && decision.Y == y {
				return decision
			}
		}
	}
	t.Fatalf("no decision for %s eye at (%d,%d)", eye, x, y)
	return PixelDecision{}
}

func candidateByID(decision PixelDecision, id string) CandidateDecision {
	for _, candidate := range decision.Candidates {
		if candidate.LayerID == id {
			return candidate
		}
	}
	return CandidateDecision{}
}

func TestDemoComparisonOrdersHandPanelAndNotification(t *testing.T) {
	comparison := DemoComparison()
	if len(comparison.Submissions) != 3 {
		t.Fatalf("got %d submissions, want 3", len(comparison.Submissions))
	}

	for _, result := range comparison.Submissions {
		if result.Summary.Pixels != 192 {
			t.Errorf("%s: got %d pixels, want 192", result.Mode, result.Summary.Pixels)
		}
		if result.Summary.WrongOcclusions != 0 {
			t.Errorf("%s: got %d occlusion errors, want 0", result.Mode, result.Summary.WrongOcclusions)
		}
		for _, eye := range []Eye{LeftEye, RightEye} {
			if got := decisionAt(t, result, eye, 7, 3).Winner; got != "hand" {
				t.Errorf("%s/%s: hand overlap selected %q", result.Mode, eye, got)
			}
			if got := decisionAt(t, result, eye, 3, 3).Winner; got != "panel" {
				t.Errorf("%s/%s: panel overlap selected %q", result.Mode, eye, got)
			}
			if got := decisionAt(t, result, eye, 1, 0).Winner; got != "notice" {
				t.Errorf("%s/%s: notification-only pixel selected %q", result.Mode, eye, got)
			}
		}
	}
}

func TestFullDepthUsesPerEyeNearestSurface(t *testing.T) {
	comparison := DemoComparison()
	full := comparison.Submissions[2]
	if full.Mode != DepthFull {
		t.Fatalf("got mode %s, want full", full.Mode)
	}
	if full.Summary.InvalidDepthSamples != 0 {
		t.Fatalf("full depth has %d invalid samples", full.Summary.InvalidDepthSamples)
	}
	for _, eye := range []Eye{LeftEye, RightEye} {
		decision := decisionAt(t, full, eye, 7, 3)
		if decision.Reason != ReasonNearestDepth {
			t.Errorf("%s eye: reason %s, want nearest depth", eye, decision.Reason)
		}
		hand := candidateByID(decision, "hand")
		notice := candidateByID(decision, "notice")
		if !hand.Selected || hand.Status != StatusSelected {
			t.Errorf("%s eye: hand candidate not selected: %+v", eye, hand)
		}
		if notice.Selected || notice.Status != StatusOccluded {
			t.Errorf("%s eye: notification candidate should be occluded: %+v", eye, notice)
		}
		if notice.System != true {
			t.Errorf("%s eye: notification should remain marked as a system layer", eye)
		}
	}
}

func TestSparseDepthFallsBackWithoutSystemPriority(t *testing.T) {
	comparison := DemoComparison()
	sparse := comparison.Submissions[1]
	if sparse.Mode != DepthSparse {
		t.Fatalf("got mode %s, want sparse", sparse.Mode)
	}
	if sparse.Summary.IncompleteDepthPixels == 0 {
		t.Fatal("sparse submission did not report incomplete depth pixels")
	}
	if sparse.Summary.OrderFallbacks == 0 {
		t.Fatal("sparse submission did not report order fallbacks")
	}
	if sparse.Summary.WrongOcclusions != 0 {
		t.Fatalf("sparse submission has %d errors", sparse.Summary.WrongOcclusions)
	}
	decision := decisionAt(t, sparse, RightEye, 6, 3)
	if decision.Winner != "hand" {
		t.Fatalf("sparse fallback selected %q, want hand", decision.Winner)
	}
	if decision.Reason != ReasonIncompleteDepth && decision.Reason != ReasonNoValidDepth {
		t.Fatalf("sparse fallback reason %s, want an order fallback", decision.Reason)
	}
	if !decision.Ambiguous {
		t.Fatal("sparse fallback should be marked ambiguous")
	}
}

func TestNoDepthUsesExplicitOrderOnly(t *testing.T) {
	comparison := DemoComparison()
	noDepth := comparison.Submissions[0]
	if noDepth.Mode != DepthNone {
		t.Fatalf("got mode %s, want none", noDepth.Mode)
	}
	decision := decisionAt(t, noDepth, LeftEye, 7, 3)
	if decision.Reason != ReasonNoDepth {
		t.Fatalf("got reason %s, want no-depth painter order", decision.Reason)
	}
	if decision.Winner != "hand" {
		t.Fatalf("got winner %q, want hand", decision.Winner)
	}
	for _, candidate := range decision.Candidates {
		if candidate.DepthValid {
			t.Fatalf("no-depth candidate %s unexpectedly has depth", candidate.LayerID)
		}
		if candidate.LayerID == "notice" && candidate.Selected {
			t.Fatal("system notification was promoted without depth")
		}
	}
}

func TestQuantizationTieKeepsStableFrontSurface(t *testing.T) {
	diagnostics := DemoDiagnostics()
	diag := diagnostics[0]
	if diag.Safe.Summary.QuantizationAmbiguous == 0 {
		t.Fatal("quantization ambiguity was not recorded")
	}
	if diag.Safe.Summary.WrongOcclusions != 0 {
		t.Fatalf("safe policy has %d occlusion errors", diag.Safe.Summary.WrongOcclusions)
	}
	if diag.Bad == nil || diag.Bad.Summary.WrongOcclusions == 0 {
		t.Fatal("system-always-front negative control should fail")
	}
	decision := decisionAt(t, diag.Safe, RightEye, 0, 0)
	if decision.Winner != "hand" {
		t.Fatalf("quantized tie selected %q, want hand", decision.Winner)
	}
	if decision.Reason != ReasonQuantizationTie || !decision.Ambiguous {
		t.Fatalf("tie was not recorded: reason=%s ambiguous=%v", decision.Reason, decision.Ambiguous)
	}
}

func TestInvalidDepthFallsBackInsteadOfBecomingInfinity(t *testing.T) {
	diag := DemoDiagnostics()[1]
	if diag.Safe.Summary.IncompleteDepthPixels == 0 {
		t.Fatal("invalid depth was not recorded")
	}
	if diag.Safe.Summary.WrongOcclusions != 0 {
		t.Fatalf("safe fallback has %d errors", diag.Safe.Summary.WrongOcclusions)
	}
	if diag.Bad == nil || diag.Bad.Summary.WrongOcclusions == 0 {
		t.Fatal("ignore-incomplete negative control should fail")
	}
	decision := decisionAt(t, diag.Safe, LeftEye, 0, 0)
	if decision.Winner != "hand" {
		t.Fatalf("fallback selected %q, want hand", decision.Winner)
	}
	hand := candidateByID(decision, "hand")
	if hand.DepthValid || !hand.Selected || hand.Status != StatusInvalid {
		t.Fatalf("invalid hand should be selected via order: %+v", hand)
	}
	if candidateByID(decision, "notice").Selected {
		t.Fatal("system notification was promoted after invalid depth")
	}
}

func TestRightEyeNeverReadsLeftEyeDepth(t *testing.T) {
	diag := DemoDiagnostics()[2]
	if diag.Safe.Summary.WrongOcclusions != 0 {
		t.Fatalf("safe stereo has %d errors", diag.Safe.Summary.WrongOcclusions)
	}
	if got := decisionAt(t, diag.Safe, LeftEye, 0, 0).Winner; got != "hand" {
		t.Fatalf("left eye selected %q, want hand", got)
	}
	right := decisionAt(t, diag.Safe, RightEye, 0, 0)
	if right.Winner != "panel" {
		t.Fatalf("right eye selected %q, want panel", right.Winner)
	}
	for _, candidate := range right.Candidates {
		if candidate.SourceEye != RightEye {
			t.Fatalf("right-eye decision read %s depth for %s", candidate.SourceEye, candidate.LayerID)
		}
	}
	if diag.Bad == nil {
		t.Fatal("missing left-eye-for-right negative control")
	}
	if got := decisionAt(t, *diag.Bad, RightEye, 0, 0).Winner; got != "hand" {
		t.Fatalf("negative control selected %q, want hand (to prove the bug)", got)
	}
	if diag.Bad.Summary.WrongOcclusions == 0 {
		t.Fatal("left-eye-for-right negative control should record an error")
	}
}

func TestJSONReportContainsNoNaN(t *testing.T) {
	payload, err := json.Marshal(DemoComparison())
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}
	text := strings.ToLower(string(payload))
	if strings.Contains(text, "nan") || strings.Contains(text, "+inf") || strings.Contains(text, "-inf") {
		t.Fatal("JSON report contains a non-finite depth value")
	}
}

func TestInvalidDepthPredicate(t *testing.T) {
	for _, depth := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), 0, -1} {
		if !invalidDepth(depth) {
			t.Errorf("%v should be invalid", depth)
		}
	}
	if invalidDepth(0.5) {
		t.Fatal("0.5 should be valid")
	}
}
