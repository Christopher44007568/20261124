package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	outDir := flag.String("out", "out", "directory for CSV and text reports")
	flag.Parse()

	if err := EnsureOutputDir(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "cannot create output dir: %v\n", err)
		os.Exit(1)
	}

	cfg := DefaultConfig()
	modes := []Mode{ModeZOH, ModeVelocity, ModeClamped}
	var results []RunResult
	for _, tr := range AllTrajectories() {
		for _, mode := range modes {
			results = append(results, Run(tr, mode, cfg))
		}
	}

	files := map[string]func(string) error{
		"records.csv": func(p string) error { return WriteRecordsCSV(p, results) },
		"rejects.csv": func(p string) error { return WriteRejectsCSV(p, results) },
		"summary.csv": func(p string) error { return WriteSummaryCSV(p, results) },
		"summary.txt": func(p string) error { return WriteSummaryText(p, results, cfg) },
	}
	for name, write := range files {
		path := filepath.Join(*outDir, name)
		if err := write(path); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	totalRejects := 0
	for _, r := range results {
		totalRejects += len(r.Rejects)
	}
	fmt.Printf("wrote %d runs (%d records rows grouped in files), %d invalid records to %s\n",
		len(results), countRecords(results), totalRejects, *outDir)
}

func countRecords(results []RunResult) int {
	n := 0
	for _, r := range results {
		n += len(r.Records)
	}
	return n
}
