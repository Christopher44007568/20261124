package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"latelatch/internal/replay"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "replay:", err)
		os.Exit(1)
	}
}

func run() error {
	scenarioSpec := flag.String("scenario", "all", "built-in scenario name, JSON file, or all")
	outputDir := flag.String("out", "artifacts/replay", "directory for replay artifacts")
	list := flag.Bool("list", false, "list built-in scenarios")
	quiet := flag.Bool("quiet", false, "only report failures")
	flag.Parse()

	if *list {
		for _, scenario := range replay.BuiltinScenarios() {
			fmt.Printf("%-22s %s\n", scenario.Name, scenario.Description)
		}
		return nil
	}
	scenarios, err := replay.ResolveScenarios(*scenarioSpec)
	if err != nil {
		return err
	}
	for _, scenario := range scenarios {
		directory := filepath.Join(*outputDir, safeName(scenario.Name))
		result, err := replay.Run(scenario)
		if err != nil {
			return fmt.Errorf("%s: %w", scenario.Name, err)
		}
		if err := replay.WriteResult(directory, result); err != nil {
			return fmt.Errorf("%s: %w", scenario.Name, err)
		}
		if !result.Checks.OK {
			return fmt.Errorf("%s: consistency checks failed: %s", scenario.Name, strings.Join(result.Checks.Errors, "; "))
		}
		if !*quiet {
			fmt.Printf(
				"%-22s frames=%d matrices=%d replaced=%d lateQueued=%d reads=%d checks=ok -> %s\n",
				scenario.Name,
				result.Summary.FrameCount,
				result.Summary.MatricesCreated,
				result.Summary.FramesReplaced,
				result.Summary.LateQueuedEvents,
				result.Summary.OldResourceReads,
				directory,
			)
		}
	}
	return nil
}

func safeName(name string) string {
	var builder strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-', char == '_', char == '.':
			builder.WriteRune(char)
		default:
			builder.WriteRune('_')
		}
	}
	if builder.Len() == 0 {
		return "scenario"
	}
	return builder.String()
}
