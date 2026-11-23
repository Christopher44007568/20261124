package replay

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	builtinFramePeriod  = int64(16_666_667)
	builtinDeadline     = int64(8_000_000)
	builtinComposite    = int64(8_100_000)
	builtinScanout      = int64(15_000_000)
	builtinInitialAt    = int64(-2_000_000)
	builtinInitialStamp = int64(-3_000_000)
)

func BuiltinScenarios() []Scenario {
	return []Scenario{
		deadlineMinusOneScenario(),
		deadlinePlusOneScenario(),
		duplicateTimestampScenario(),
		trackingLossScenario(),
	}
}

func ResolveScenarios(spec string) ([]Scenario, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || spec == "all" {
		return BuiltinScenarios(), nil
	}
	for _, scenario := range BuiltinScenarios() {
		if scenario.Name == spec {
			return []Scenario{scenario}, nil
		}
	}
	scenario, err := LoadScenario(spec)
	if err != nil {
		return nil, err
	}
	return []Scenario{scenario}, nil
}

func LoadScenario(path string) (Scenario, error) {
	file, err := os.Open(path)
	if err != nil {
		return Scenario{}, err
	}
	defer file.Close()
	scenario, err := DecodeScenario(file)
	if err != nil {
		return Scenario{}, fmt.Errorf("%s: %w", filepath.Clean(path), err)
	}
	return scenario, nil
}

func DecodeScenario(reader io.Reader) (Scenario, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var scenario Scenario
	if err := decoder.Decode(&scenario); err != nil {
		return Scenario{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Scenario{}, fmt.Errorf("unexpected trailing JSON")
	}
	if err := scenario.Validate(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

func baseScenario(name, description string) Scenario {
	return Scenario{
		SchemaVersion:     SchemaVersion,
		Name:              name,
		Description:       description,
		FrameCount:        6,
		FirstSubmitNs:     0,
		FramePeriodNs:     builtinFramePeriod,
		DeadlineOffsetNs:  builtinDeadline,
		CompositeOffsetNs: builtinComposite,
		ScanoutOffsetNs:   builtinScanout,
	}
}

func initialPose() []InputEvent {
	return []InputEvent{
		{
			AtNs: builtinInitialAt, Kind: "pose", Eye: EyeLeft,
			TimestampNs: builtinInitialStamp, YawDeg: -1.5, PitchDeg: 0.5,
			Note: "initial left eye pose",
		},
		{
			AtNs: builtinInitialAt, Kind: "pose", Eye: EyeRight,
			TimestampNs: builtinInitialStamp, YawDeg: -1.5, PitchDeg: 0.5,
			Note: "initial right eye pose",
		},
	}
}

func submitAt(index int) int64 {
	return int64(index) * builtinFramePeriod
}

func deadlineAt(index int) int64 {
	return submitAt(index) + builtinDeadline
}

func deadlineMinusOneScenario() Scenario {
	scenario := baseScenario(
		"deadline-minus-1ms",
		"Both eye poses arrive 1 ms before F0003 deadline and replace its camera.",
	)
	scenario.Events = append(initialPose(),
		InputEvent{
			AtNs: deadlineAt(2) - 1_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: submitAt(2) + 3_000_000, YawDeg: 4.5, PitchDeg: 1.2,
			Note: "on-time left update",
		},
		InputEvent{
			AtNs: deadlineAt(2) - 1_000_000, Kind: "pose", Eye: EyeRight,
			TimestampNs: submitAt(2) + 3_000_000, YawDeg: 4.5, PitchDeg: 1.2,
			Note: "on-time right update",
		},
	)
	return scenario
}

func deadlinePlusOneScenario() Scenario {
	scenario := baseScenario(
		"deadline-plus-1ms",
		"Both eye poses arrive 1 ms after F0003 deadline, queue for F0004, and replace only F0004.",
	)
	scenario.Events = append(initialPose(),
		InputEvent{
			AtNs: deadlineAt(2) + 1_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: submitAt(2) + 3_000_000, YawDeg: 7.0, PitchDeg: -1.0,
			Note: "late for F0003, queued for F0004",
		},
		InputEvent{
			AtNs: deadlineAt(2) + 1_000_000, Kind: "pose", Eye: EyeRight,
			TimestampNs: submitAt(2) + 3_000_000, YawDeg: 7.0, PitchDeg: -1.0,
			Note: "late for F0003, queued for F0004",
		},
	)
	return scenario
}

func duplicateTimestampScenario() Scenario {
	firstStamp := submitAt(1) + 2_000_000
	scenario := baseScenario(
		"duplicate-timestamp",
		"Eye packets reuse capture timestamps; duplicates never advance pose or matrix versions.",
	)
	scenario.Events = append(initialPose(),
		InputEvent{
			AtNs: deadlineAt(1) - 1_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: firstStamp, YawDeg: 3.0, Note: "first left sample",
		},
		InputEvent{
			AtNs: deadlineAt(1) - 1_000_000, Kind: "pose", Eye: EyeRight,
			TimestampNs: firstStamp, YawDeg: 3.0, Note: "first right sample",
		},
		InputEvent{
			AtNs: deadlineAt(2) - 2_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: firstStamp, YawDeg: 90, Note: "duplicate left timestamp",
		},
		InputEvent{
			AtNs: deadlineAt(2) - 2_000_000, Kind: "pose", Eye: EyeRight,
			TimestampNs: firstStamp, YawDeg: 90, Note: "duplicate right timestamp",
		},
		InputEvent{
			AtNs: deadlineAt(2) + 1_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: firstStamp, YawDeg: 45, Note: "late duplicate left timestamp",
		},
	)
	return scenario
}

func trackingLossScenario() Scenario {
	scenario := baseScenario(
		"tracking-loss",
		"F0003 loses right-eye tracking and holds the previous matrix; F0004 recovers both eyes.",
	)
	scenario.Events = append(initialPose(),
		InputEvent{
			AtNs: deadlineAt(2) - 2_000_000, Kind: "pose", Eye: EyeLeft,
			TimestampNs: submitAt(2) + 2_000_000, YawDeg: 5.0,
			Note: "left update before loss",
		},
		InputEvent{
			AtNs: deadlineAt(2) - 1_000_000, Kind: "tracking_lost", Eye: EyeRight,
			Note: "right eye tracking lost before F0003 deadline",
		},
		InputEvent{
			AtNs: deadlineAt(3) - 1_000_000, Kind: "pose", Eye: EyeRight,
			TimestampNs: submitAt(3) + 2_000_000, YawDeg: 5.0,
			Note: "right eye tracking recovered",
		},
	)
	return scenario
}
