package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"foveasched/internal/sim"
)

type summary struct {
	Name        string        `json:"name"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Metrics     []sim.Metrics `json:"metrics"`
}

type payload struct {
	Scenarios []summary            `json:"scenarios"`
	Reports   []sim.ScenarioReport `json:"reports,omitempty"`
}

func main() {
	scenarioArg := flag.String("scenario", "all", "scenario name, or all")
	policyArg := flag.String("policy", "all", "policy name, or all")
	jsonOut := flag.Bool("json", false, "emit JSON")
	includeFrames := flag.Bool("frames", false, "include frame-level data in JSON")
	flag.Parse()

	scenarios, err := selectScenarios(*scenarioArg)
	if err != nil {
		fail(err)
	}
	policies, err := selectPolicies(*policyArg)
	if err != nil {
		fail(err)
	}

	reports := sim.RunAll(scenarios, policies)
	if *jsonOut {
		writeJSON(reports, scenarios, *includeFrames)
		return
	}
	writeTable(reports)
}

func selectScenarios(value string) ([]sim.Scenario, error) {
	if value == "all" {
		return sim.StandardScenarios(), nil
	}
	scenario, err := sim.ScenarioByName(value)
	if err != nil {
		return nil, err
	}
	return []sim.Scenario{scenario}, nil
}

func selectPolicies(value string) ([]sim.Policy, error) {
	if value == "all" {
		return sim.Policies[:], nil
	}
	policy, err := sim.ParsePolicy(value)
	if err != nil {
		return nil, err
	}
	return []sim.Policy{policy}, nil
}

func writeJSON(reports []sim.ScenarioReport, scenarios []sim.Scenario, includeFrames bool) {
	out := payload{}
	for _, scenario := range scenarios {
		item := summary{
			Name:        scenario.Name,
			Title:       scenario.Title,
			Description: scenario.Description,
		}
		for _, report := range reports {
			if report.Scenario.Name == scenario.Name {
				item.Metrics = append(item.Metrics, report.Metrics)
			}
		}
		out.Scenarios = append(out.Scenarios, item)
	}
	if includeFrames {
		out.Reports = reports
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		fail(err)
	}
}

func writeTable(reports []sim.ScenarioReport) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(writer, "scenario\tpolicy\tcenter_err\tperiph_err\tsample_use\teye_use\tredraw_use\tcommit\tfallback_b/e/p/f\tfloor")
	for _, report := range reports {
		m := report.Metrics
		fmt.Fprintf(
			writer,
			"%s\t%s\t%.3f\t%.3f\t%.1f%%\t%.1f%%\t%.1f%%\t%.1f%%\t%d/%d/%d/%d\t%d/%d\n",
			m.Scenario,
			m.Policy,
			m.CenterError,
			m.PeripheralError,
			100*m.SampleUse,
			100*m.EyeBufferUse,
			100*m.RedrawUse,
			100*m.CompletionRate,
			m.Fallbacks.Budget,
			m.Fallbacks.EyeCap,
			m.Fallbacks.Prediction,
			m.Fallbacks.Floor,
			m.FloorPreservedFrames,
			m.Frames,
		)
	}
	if err := writer.Flush(); err != nil {
		fail(err)
	}

	fmt.Println("\n回退列按帧计数：budget/eye-cap/prediction/floor；floor 为守住最低质量帧数。")
	for _, policy := range sim.Policies {
		fmt.Printf("- %-14s %s\n", policy, policy.Description())
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "foveasched: %v\n", err)
	if strings.Contains(err.Error(), "unknown") {
		fmt.Fprintln(os.Stderr, "scenarios: baseline, budget-drop, prediction-fail, single-eye-overrun, combined")
		fmt.Fprintln(os.Stderr, "policies: equal, gaze-region, history-error")
	}
	os.Exit(2)
}
