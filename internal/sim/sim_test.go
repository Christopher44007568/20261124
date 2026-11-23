package sim

import (
	"math"
	"reflect"
	"testing"
)

func TestStandardScenariosStayWithinHardLimits(t *testing.T) {
	for _, scenario := range StandardScenarios() {
		for _, policy := range Policies {
			report := RunScenario(scenario, policy)
			if len(report.Frames) != len(scenario.Frames) {
				t.Fatalf("%s/%s: got %d frames, want %d", scenario.Name, policy, len(report.Frames), len(scenario.Frames))
			}
			for i, frame := range report.Frames {
				budget := scenario.Frames[i].Budget
				if !frame.FloorPreserved {
					t.Errorf("%s/%s frame %d: floor not preserved", scenario.Name, policy, i)
				}
				if len(frame.Submitted) != 4 {
					t.Errorf("%s/%s frame %d: submitted %d/4 tasks", scenario.Name, policy, i, len(frame.Submitted))
				}
				if frame.SamplesUsed > budget.Samples {
					t.Errorf("%s/%s frame %d: samples %d > %d", scenario.Name, policy, i, frame.SamplesUsed, budget.Samples)
				}
				if frame.RedrawUsed > budget.Redraw {
					t.Errorf("%s/%s frame %d: redraw %d > %d", scenario.Name, policy, i, frame.RedrawUsed, budget.Redraw)
				}
				for _, eye := range Eyes {
					if frame.EyeBufferUsed[eye] > budget.EyeBuffer[eye] {
						t.Errorf("%s/%s frame %d: %s buffer %d > %d", scenario.Name, policy, i, eye, frame.EyeBufferUsed[eye], budget.EyeBuffer[eye])
					}
				}
				assertAccounting(t, scenario.Name, policy, frame)
			}
			if report.Metrics.CompletionRate != 1 {
				t.Errorf("%s/%s: completion %.3f, want 1", scenario.Name, policy, report.Metrics.CompletionRate)
			}
		}
	}
}

func TestFloorFailureCommitsNothing(t *testing.T) {
	scenario := Scenario{
		Name: "under-floor",
		Frames: []Frame{{
			Index:        0,
			Budget:       Budget{Samples: 11, Redraw: 8, EyeBuffer: [2]int{8, 8}},
			PredictionOK: true,
		}},
	}
	for _, policy := range Policies {
		report := RunScenario(scenario, policy)
		frame := report.Frames[0]
		if frame.FloorPreserved {
			t.Errorf("%s: floor unexpectedly preserved", policy)
		}
		if len(frame.Submitted) != 0 || frame.SamplesUsed != 0 || frame.RedrawUsed != 0 {
			t.Errorf("%s: unsubmitted work was counted: %+v", policy, frame)
		}
		if frame.EyeBufferUsed != [2]int{0, 0} {
			t.Errorf("%s: unsubmitted eye buffer counted: %+v", policy, frame.EyeBufferUsed)
		}
		if report.Metrics.SubmittedTasks != 0 || report.Metrics.CompletionRate != 0 {
			t.Errorf("%s: completion accounting accepted unsubmitted work", policy)
		}
		if report.Metrics.Fallbacks.Floor != 1 {
			t.Errorf("%s: floor fallback count = %d, want 1", policy, report.Metrics.Fallbacks.Floor)
		}
	}
}

func TestPredictionTripsOnThirdConsecutiveFailure(t *testing.T) {
	scenario := Scenario{Name: "prediction-edge"}
	for i, ok := range []bool{false, false, true, false, false, false} {
		scenario.Frames = append(scenario.Frames, Frame{
			Index:        i,
			Budget:       normalBudget(),
			PredictionOK: ok,
		})
	}
	report := RunScenario(scenario, PolicyEqual)
	wantHold := []bool{false, false, false, false, false, true}
	for i, want := range wantHold {
		if got := report.Frames[i].PredictionHold; got != want {
			t.Errorf("frame %d: prediction hold = %v, want %v", i, got, want)
		}
	}
	if report.Metrics.Fallbacks.Prediction != 1 {
		t.Errorf("prediction fallbacks = %d, want 1", report.Metrics.Fallbacks.Prediction)
	}

	hold := report.Frames[5]
	for _, submission := range hold.Submitted {
		if submission.Tier != TierMin {
			t.Errorf("prediction hold submitted tier %s, want min", submission.Tier)
		}
	}
}

func TestSingleEyeCapCannotBorrow(t *testing.T) {
	scenario := SingleEyeOverrunScenario()
	for _, policy := range Policies {
		report := RunScenario(scenario, policy)
		if report.Metrics.Fallbacks.EyeCap == 0 {
			t.Errorf("%s: expected eye-cap fallback events", policy)
		}
		for _, frame := range report.Frames {
			if frame.EyeBufferUsed[Left] > 4 {
				t.Errorf("%s frame %d: left eye used %d > 4", policy, frame.Index, frame.EyeBufferUsed[Left])
			}
			if frame.EyeBufferUsed[Right] > 7 {
				t.Errorf("%s frame %d: right eye used %d > 7", policy, frame.Index, frame.EyeBufferUsed[Right])
			}
		}
	}
}

func TestPoliciesProduceDistinctTradeoffs(t *testing.T) {
	drop := map[Policy]Metrics{}
	for _, policy := range Policies {
		drop[policy] = RunScenario(BudgetDropScenario(), policy).Metrics
	}
	if !(drop[PolicyGazeRegion].CenterError < drop[PolicyEqual].CenterError &&
		drop[PolicyEqual].CenterError < drop[PolicyHistoricalError].CenterError) {
		t.Errorf("budget-drop center ordering: gaze=%.3f equal=%.3f history=%.3f",
			drop[PolicyGazeRegion].CenterError, drop[PolicyEqual].CenterError, drop[PolicyHistoricalError].CenterError)
	}
	if !(drop[PolicyHistoricalError].PeripheralError < drop[PolicyEqual].PeripheralError &&
		drop[PolicyEqual].PeripheralError < drop[PolicyGazeRegion].PeripheralError) {
		t.Errorf("budget-drop periphery ordering: history=%.3f equal=%.3f gaze=%.3f",
			drop[PolicyHistoricalError].PeripheralError, drop[PolicyEqual].PeripheralError, drop[PolicyGazeRegion].PeripheralError)
	}

	single := map[Policy]Metrics{}
	for _, policy := range Policies {
		single[policy] = RunScenario(SingleEyeOverrunScenario(), policy).Metrics
	}
	if !(single[PolicyHistoricalError].PeripheralError < single[PolicyEqual].PeripheralError) {
		t.Errorf("single-eye history periphery %.3f not below equal %.3f",
			single[PolicyHistoricalError].PeripheralError, single[PolicyEqual].PeripheralError)
	}
	if !(single[PolicyEqual].CenterError < single[PolicyHistoricalError].CenterError) {
		t.Errorf("single-eye equal center %.3f not below history %.3f",
			single[PolicyEqual].CenterError, single[PolicyHistoricalError].CenterError)
	}
}

func TestRunIsDeterministic(t *testing.T) {
	scenario := CombinedScenario()
	for _, policy := range Policies {
		first := RunScenario(scenario, policy)
		second := RunScenario(scenario, policy)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("%s: repeated run differs", policy)
		}
	}
}

func TestMetricErrorAveragesMatchFrames(t *testing.T) {
	for _, scenario := range StandardScenarios() {
		for _, policy := range Policies {
			report := RunScenario(scenario, policy)
			var center, peripheral float64
			for _, frame := range report.Frames {
				center += frame.CenterError
				peripheral += frame.PeripheralError
			}
			center /= float64(len(report.Frames))
			peripheral /= float64(len(report.Frames))
			if math.Abs(center-report.Metrics.CenterError) > 1e-9 {
				t.Errorf("%s/%s: center aggregate %.9f != %.9f", scenario.Name, policy, report.Metrics.CenterError, center)
			}
			if math.Abs(peripheral-report.Metrics.PeripheralError) > 1e-9 {
				t.Errorf("%s/%s: peripheral aggregate %.9f != %.9f", scenario.Name, policy, report.Metrics.PeripheralError, peripheral)
			}
		}
	}
}

func assertAccounting(t *testing.T, scenario string, policy Policy, frame FrameResult) {
	t.Helper()
	var samples, redraw int
	var eye [2]int
	for _, submission := range frame.Submitted {
		samples += submission.Spec.Samples
		redraw += submission.Spec.Redraw
		eye[submission.Key.Eye] += submission.Spec.EyeBuffer
	}
	if samples != frame.SamplesUsed || redraw != frame.RedrawUsed || eye != frame.EyeBufferUsed {
		t.Errorf("%s/%s frame %d: result usage does not match submissions", scenario, policy, frame.Index)
	}
	if frame.SubmittedTasks() != len(frame.Submitted) {
		t.Errorf("%s/%s frame %d: submitted-task count mismatch", scenario, policy, frame.Index)
	}
}
