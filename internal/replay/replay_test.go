package replay

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scenarioByName(t *testing.T, name string) Scenario {
	t.Helper()
	for _, scenario := range BuiltinScenarios() {
		if scenario.Name == name {
			return scenario
		}
	}
	t.Fatalf("scenario %q not found", name)
	return Scenario{}
}

func mustRun(t *testing.T, name string) *Replay {
	t.Helper()
	result, err := Run(scenarioByName(t, name))
	if err != nil {
		t.Fatalf("Run(%s): %v", name, err)
	}
	if !result.Checks.OK {
		t.Fatalf("Run(%s) checks failed: %v", name, result.Checks.Errors)
	}
	return result
}

func inputByID(t *testing.T, result *Replay, id string) InputRecord {
	t.Helper()
	for _, input := range result.Inputs {
		if input.EventID == id {
			return input
		}
	}
	t.Fatalf("input %s not found", id)
	return InputRecord{}
}

func TestDeadlineMinusOneUpdatesCurrentFrame(t *testing.T) {
	result := mustRun(t, "deadline-minus-1ms")
	if got := len(result.Matrices); got != 2 {
		t.Fatalf("matrix count = %d, want 2", got)
	}
	frame := result.Frames[2]
	if frame.UsedMatrixVersion != 2 || !frame.MatrixReplaced {
		t.Fatalf("F0003 = M%d replaced=%v, want M2 replaced", frame.UsedMatrixVersion, frame.MatrixReplaced)
	}
	if frame.UsedLeftPoseVersion != 2 || frame.UsedRightPoseVersion != 2 {
		t.Fatalf("F0003 pose versions = %d/%d, want 2/2", frame.UsedLeftPoseVersion, frame.UsedRightPoseVersion)
	}
	if result.Frames[1].UsedMatrixVersion != 1 {
		t.Fatalf("F0002 unexpectedly used M%d", result.Frames[1].UsedMatrixVersion)
	}
	if len(result.Timelines.Deadline[2].LateQueuedEventIDs) != 0 {
		t.Fatalf("F0003 unexpectedly queued on-time events: %v", result.Timelines.Deadline[2].LateQueuedEventIDs)
	}
	for _, id := range []string{"E0003", "E0004"} {
		input := inputByID(t, result, id)
		if input.TargetFrameID != "F0003" || input.QueuedFromFrameID != "" || input.Status != "accepted" {
			t.Fatalf("%s = target %s queuedFrom %s status %s", id, input.TargetFrameID, input.QueuedFromFrameID, input.Status)
		}
	}
	if result.Frames[1].ImageSHA256 == result.Frames[2].ImageSHA256 {
		t.Fatal("on-time matrix replacement did not produce a new rendered image")
	}
	if got := len(result.Timelines.Composite[2].OldResourceReadIDs); got != 2 {
		t.Fatalf("F0003 old resource reads = %d, want 2", got)
	}
}

func TestDeadlinePlusOneQueuesForNextFrame(t *testing.T) {
	result := mustRun(t, "deadline-plus-1ms")
	if result.Frames[2].UsedMatrixVersion != 1 || result.Frames[2].MatrixReplaced {
		t.Fatalf("F0003 = M%d replaced=%v, want held M1", result.Frames[2].UsedMatrixVersion, result.Frames[2].MatrixReplaced)
	}
	deadline := result.Timelines.Deadline[2]
	if len(deadline.LateQueuedEventIDs) != 2 {
		t.Fatalf("F0003 late queue = %v, want E0003/E0004", deadline.LateQueuedEventIDs)
	}
	for _, id := range []string{"E0003", "E0004"} {
		input := inputByID(t, result, id)
		if input.TargetFrameID != "F0004" || input.QueuedFromFrameID != "F0003" {
			t.Fatalf("%s = target %s queuedFrom %s, want F0004 from F0003", id, input.TargetFrameID, input.QueuedFromFrameID)
		}
	}
	if result.Frames[3].UsedMatrixVersion != 2 || !result.Frames[3].MatrixReplaced {
		t.Fatalf("F0004 = M%d replaced=%v, want replaced M2", result.Frames[3].UsedMatrixVersion, result.Frames[3].MatrixReplaced)
	}
	if len(result.Timelines.Composite[2].OldResourceReadIDs) != 0 {
		t.Fatal("late queued data must not produce old-resource reads on the missed frame")
	}
	reads := result.Timelines.Composite[3].OldResourceReadIDs
	if len(reads) != 2 {
		t.Fatalf("F0004 old resource reads = %d, want 2", len(reads))
	}
	for _, id := range reads {
		read := findRead(t, result, id)
		if read.ProducerFrameID != "F0003" || read.ProducerMatrixVersion != 1 || read.ConsumerMatrixVersion != 2 {
			t.Fatalf("read %s provenance = %s M%d -> M%d", id, read.ProducerFrameID, read.ProducerMatrixVersion, read.ConsumerMatrixVersion)
		}
	}
}

func TestDuplicateTimestampDoesNotAdvanceVersions(t *testing.T) {
	result := mustRun(t, "duplicate-timestamp")
	if got := len(result.Matrices); got != 2 {
		t.Fatalf("matrix count = %d, duplicates must not create a matrix", got)
	}
	for _, id := range []string{"E0005", "E0006", "E0007"} {
		input := inputByID(t, result, id)
		if input.Status != "duplicate" || input.SampleID != "" || input.PoseVersion != 0 {
			t.Fatalf("%s = status %s sample %q pose %d, want duplicate", id, input.Status, input.SampleID, input.PoseVersion)
		}
	}
	if got := len(result.Timelines.Deadline[2].IgnoredEventIDs); got != 2 {
		t.Fatalf("F0003 ignored = %d, want 2", got)
	}
	if result.Frames[2].UsedMatrixVersion != 2 || result.Frames[3].UsedMatrixVersion != 2 {
		t.Fatalf("duplicate timestamps advanced frame matrices to M%d/M%d", result.Frames[2].UsedMatrixVersion, result.Frames[3].UsedMatrixVersion)
	}
	if result.Summary.DuplicateEvents != 3 {
		t.Fatalf("duplicate summary = %d, want 3", result.Summary.DuplicateEvents)
	}
}

func TestTrackingLossHoldsLastMatrixUntilRecovery(t *testing.T) {
	result := mustRun(t, "tracking-loss")
	frame := result.Frames[2]
	if frame.UsedMatrixVersion != 1 || frame.Resolution != "held_tracking_lost" {
		t.Fatalf("F0003 = M%d %s, want held M1 on tracking loss", frame.UsedMatrixVersion, frame.Resolution)
	}
	if !strings.Contains(frame.TrackingState, "right=lost") {
		t.Fatalf("F0003 tracking state = %s", frame.TrackingState)
	}
	if len(result.Timelines.Composite[2].OldResourceReadIDs) != 0 {
		t.Fatal("tracking-loss hold must not read old resources")
	}
	if inputByID(t, result, "E0003").Status != "accepted" {
		t.Fatal("left pose before loss should still be accepted into the staging state")
	}
	if result.Frames[3].UsedMatrixVersion != 2 {
		t.Fatalf("F0004 recovery used M%d, want M2", result.Frames[3].UsedMatrixVersion)
	}
	if result.Frames[3].UsedLeftSampleID != "S0003" || result.Frames[3].UsedRightSampleID != "S0004" {
		t.Fatalf("F0004 samples = %s/%s, want S0003/S0004", result.Frames[3].UsedLeftSampleID, result.Frames[3].UsedRightSampleID)
	}
}

func TestSingleEyeUpdateWaitsForCompleteStereoPair(t *testing.T) {
	scenario := scenarioByName(t, "deadline-minus-1ms")
	scenario.Name = "single-eye-update"
	scenario.Events = scenario.Events[:3]
	scenario.Events = append(scenario.Events, InputEvent{
		AtNs: deadlineAt(3) - 1_000_000, Kind: "pose", Eye: EyeRight,
		TimestampNs: submitAt(3) + 2_000_000, YawDeg: 4.5, PitchDeg: 1.2,
		Note: "completes the stereo pair",
	})
	result, err := Run(scenario)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Checks.OK {
		t.Fatalf("checks failed: %v", result.Checks.Errors)
	}
	frame := result.Frames[2]
	if frame.UsedMatrixVersion != 1 || frame.Resolution != "held_incomplete_update" {
		t.Fatalf("F0003 = M%d %s, want held complete-pair update", frame.UsedMatrixVersion, frame.Resolution)
	}
	if frame.UsedLeftSampleID != "S0001" || frame.UsedRightSampleID != "S0002" {
		t.Fatalf("F0003 used %s/%s, want old stereo pair", frame.UsedLeftSampleID, frame.UsedRightSampleID)
	}
	if result.Frames[3].UsedMatrixVersion != 2 {
		t.Fatalf("F0004 = M%d, want completed stereo pair", result.Frames[3].UsedMatrixVersion)
	}
	if result.Frames[3].UsedLeftSampleID != "S0003" || result.Frames[3].UsedRightSampleID != "S0004" {
		t.Fatalf("F0004 samples = %s/%s", result.Frames[3].UsedLeftSampleID, result.Frames[3].UsedRightSampleID)
	}
}

func TestEventsAreProcessedByArrivalTime(t *testing.T) {
	scenario := scenarioByName(t, "deadline-minus-1ms")
	scenario.Name = "unsorted-input"
	scenario.Events = scenario.Events[:2]
	scenario.Events = append(scenario.Events,
		InputEvent{
			AtNs: deadlineAt(2) - 1_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: submitAt(2) + 1_000_000, YawDeg: 2,
			Note: "listed first but arrives later and is older",
		},
		InputEvent{
			AtNs: deadlineAt(2) - 2_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: submitAt(2) + 3_000_000, YawDeg: 9,
			Note: "listed second but arrives earlier and is newer",
		},
	)
	result, err := Run(scenario)
	if err != nil {
		t.Fatal(err)
	}
	older := inputByID(t, result, "E0003")
	newer := inputByID(t, result, "E0004")
	if newer.Status != "accepted" || newer.PoseVersion != 2 {
		t.Fatalf("newer capture = %s v%d, want accepted v2", newer.Status, newer.PoseVersion)
	}
	if older.Status != "out_of_order" {
		t.Fatalf("older capture = %s, want out_of_order", older.Status)
	}
}

func findRead(t *testing.T, result *Replay, id string) ResourceReadRecord {
	t.Helper()
	for _, read := range result.ResourceReads {
		if read.ID == id {
			return read
		}
	}
	t.Fatalf("resource read %s not found", id)
	return ResourceReadRecord{}
}

func TestArtifactsAreReplayableAndCrossReferenced(t *testing.T) {
	result := mustRun(t, "deadline-plus-1ms")
	directory := t.TempDir()
	if err := WriteResult(directory, result); err != nil {
		t.Fatalf("WriteResult: %v", err)
	}

	required := []string{
		"replay.json", "summary.json", "audit.csv",
		"timeline-submit.jsonl", "timeline-deadline.jsonl",
		"timeline-composite.jsonl", "timeline-scanout.jsonl",
	}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("missing artifact %s: %v", name, err)
		}
	}

	for _, name := range []string{
		"timeline-submit.jsonl", "timeline-deadline.jsonl",
		"timeline-composite.jsonl", "timeline-scanout.jsonl",
	} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := bytes.Count(data, []byte("\n")); got != result.Summary.FrameCount {
			t.Fatalf("%s lines = %d, want %d", name, got, result.Summary.FrameCount)
		}
	}

	images, err := filepath.Glob(filepath.Join(directory, "frames", "*.png"))
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != result.Summary.FrameCount {
		t.Fatalf("PNG count = %d, want %d", len(images), result.Summary.FrameCount)
	}
	for _, path := range images {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := png.Decode(bytes.NewReader(data)); err != nil {
			t.Fatalf("%s is not a valid PNG: %v", path, err)
		}
	}

	file, err := os.Open(filepath.Join(directory, "audit.csv"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(file).ReadAll()
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != result.Summary.FrameCount+1 {
		t.Fatalf("audit rows = %d, want %d", len(rows), result.Summary.FrameCount+1)
	}
	header := strings.Join(rows[0], ",")
	for _, column := range []string{"matrixVersion", "imageSha256", "oldResourceReadIds", "oldProducerMatrices"} {
		if !strings.Contains(header, column) {
			t.Fatalf("audit header missing %s", column)
		}
	}
	f0004 := rows[4]
	if f0004[0] != "F0004" || f0004[5] != "2" || f0004[13] == "" || f0004[14] != "1;1" {
		t.Fatalf("F0004 audit row = %v", f0004)
	}

	replayJSON, err := os.ReadFile(filepath.Join(directory, "replay.json"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded Replay
	if err := json.Unmarshal(replayJSON, &decoded); err != nil {
		t.Fatalf("decode replay.json: %v", err)
	}
	if check := Validate(&decoded); !check.OK {
		t.Fatalf("decoded replay failed checks: %v", check.Errors)
	}
	for i, frame := range decoded.Frames {
		image, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(frame.ImagePath)))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(image)
		if fmt.Sprintf("%x", sum[:]) != frame.ImageSHA256 {
			t.Fatalf("frame %d image digest mismatch", i+1)
		}
	}
}

func TestValidationCatchesBrokenCrossReferences(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Replay)
	}{
		{
			name: "missing matrix",
			mutate: func(result *Replay) {
				result.Frames[2].UsedMatrixVersion = 99
			},
		},
		{
			name: "image digest",
			mutate: func(result *Replay) {
				result.Frames[1].ImageSHA256 = strings.Repeat("0", 64)
			},
		},
		{
			name: "resource producer matrix",
			mutate: func(result *Replay) {
				result.Resources[0].ProducerMatrixVersion = 99
			},
		},
		{
			name: "old read provenance",
			mutate: func(result *Replay) {
				result.ResourceReads[0].ProducerMatrixVersion = 99
			},
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			result := mustRun(t, "deadline-plus-1ms")
			test.mutate(result)
			if check := Validate(result); check.OK {
				t.Fatal("corrupt cross-reference passed validation")
			}
		})
	}
}

func TestReplayIsDeterministic(t *testing.T) {
	first := mustRun(t, "deadline-minus-1ms")
	second := mustRun(t, "deadline-minus-1ms")
	firstJSON, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("replay metadata is not deterministic")
	}
	for _, frame := range first.Frames {
		if !bytes.Equal(first.Images[frame.ID], second.Images[frame.ID]) {
			t.Fatalf("frame %s image is not deterministic", frame.ID)
		}
	}
}

func TestScenarioJSONDecode(t *testing.T) {
	scenario := scenarioByName(t, "deadline-minus-1ms")
	data, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeScenario(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("DecodeScenario: %v", err)
	}
	if decoded.Name != scenario.Name || len(decoded.Events) != len(scenario.Events) {
		t.Fatalf("decoded scenario = %#v", decoded)
	}
	if _, err := DecodeScenario(strings.NewReader(string(data) + "{}")); err == nil {
		t.Fatal("trailing JSON was accepted")
	}
	unknown := strings.Replace(string(data), `"description"`, `"unknownField":true,"description"`, 1)
	if _, err := DecodeScenario(strings.NewReader(unknown)); err == nil {
		t.Fatal("unknown JSON field was accepted")
	}
}
