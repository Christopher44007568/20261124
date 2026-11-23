package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"compositor/internal/compositor"
)

func run(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("compositor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	format := flags.String("format", "markdown", "output format: markdown or json")
	outPath := flags.String("out", "-", "output path, or - for stdout")
	if err := flags.Parse(args); err != nil {
		return err
	}

	comparison := compositor.DemoComparison()
	var output io.Writer = stdout
	var file *os.File
	if *outPath != "-" {
		var err error
		file, err = os.Create(*outPath)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file
	}

	switch strings.ToLower(*format) {
	case "markdown", "md":
		_, err := io.WriteString(output, compositor.RenderMarkdown(comparison))
		return err
	case "json":
		encoder := json.NewEncoder(output)
		encoder.SetIndent("", "  ")
		return encoder.Encode(comparison)
	default:
		return fmt.Errorf("unsupported format %q (use markdown or json)", *format)
	}
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "compositor:", err)
		os.Exit(1)
	}
}
