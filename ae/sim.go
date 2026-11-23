package ae

import "fmt"

type Sample struct {
	Frame          int
	SceneCut       bool
	EV             float64
	Control        float64
	Mean           float64
	HighFraction   float64
	Generation     uint64
	WindowFrames   int
	StaleZoneReads uint64
}

type Trace struct {
	Scenario    Scenario
	Mode        MeterMode
	Samples     []Sample
	Performance Performance
}

type Performance struct {
	StartEV           float64
	FinalEV           float64
	ConvergenceFrames int
	OvershootEV       float64
	OvershootPercent  float64
	PeakExcursionEV   float64
	Settled           bool
}

func RunScenario(
	scenario Scenario,
	mode MeterMode,
	meterCfg MeterConfig,
	controlCfg ControlConfig,
) (Trace, error) {
	if scenario.LastTransition < 1 || scenario.LastTransition >= len(scenario.Frames) {
		return Trace{}, fmt.Errorf("scenario %q has invalid transition frame %d",
			scenario.Name, scenario.LastTransition)
	}
	meter, err := NewMeter(mode, meterCfg)
	if err != nil {
		return Trace{}, err
	}
	controller, err := NewController(controlCfg, InitialEV(scenario, controlCfg))
	if err != nil {
		return Trace{}, err
	}

	trace := Trace{
		Scenario: scenario,
		Mode:     mode,
		Samples:  make([]Sample, 0, len(scenario.Frames)),
	}
	for i, frame := range scenario.Frames {
		exposed := ApplyExposure(frame, controller.EV())
		measurement, err := meter.Measure(exposed)
		if err != nil {
			return Trace{}, fmt.Errorf("%s/%s frame %d: %w", scenario.Name, mode, i, err)
		}
		stats := meter.Stats()
		if measurement.StaleZoneReads != stats.StaleZoneReads {
			return Trace{}, fmt.Errorf("%s/%s frame %d: stale-zone counter mismatch",
				scenario.Name, mode, i)
		}
		trace.Samples = append(trace.Samples, Sample{
			Frame:          i,
			SceneCut:       frame.SceneCut,
			EV:             controller.EV(),
			Control:        measurement.Control,
			Mean:           measurement.Mean,
			HighFraction:   measurement.HighFraction,
			Generation:     measurement.Generation,
			WindowFrames:   measurement.WindowFrames,
			StaleZoneReads: measurement.StaleZoneReads,
		})
		controller.Step(measurement.Control)
	}
	trace.Performance = AnalyzePerformance(trace.Samples, scenario.LastTransition)
	return trace, nil
}

func Compare(
	scenarios []Scenario,
	modes []MeterMode,
	meterCfg MeterConfig,
	controlCfg ControlConfig,
) ([]Trace, error) {
	traces := make([]Trace, 0, len(scenarios)*len(modes))
	for _, scenario := range scenarios {
		for _, mode := range modes {
			trace, err := RunScenario(scenario, mode, meterCfg, controlCfg)
			if err != nil {
				return nil, err
			}
			traces = append(traces, trace)
		}
	}
	return traces, nil
}
