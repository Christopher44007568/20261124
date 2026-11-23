// Package optics implements a versioned per-eye optical configuration
// with validation and distortion mapping checks.
package optics

import (
	"encoding/json"
	"fmt"
)

// CurrentVersion is the only config schema version this build accepts.
const CurrentVersion = 2

// MinVersion is the oldest schema version that can still be migrated.
const MinVersion = 1

// Eye identifies which eye a parameter set belongs to.
type Eye string

const (
	LeftEye  Eye = "left"
	RightEye Eye = "right"
)

// Distortion holds a radial polynomial model: r' = r*(1 + k1*r^2 + k2*r^4 + ...).
type Distortion struct {
	Coefficients []float64 `json:"coefficients"`
}

// ChromaticAberration holds per-channel radial scale factors (R, G, B).
type ChromaticAberration struct {
	Scale [3]float64 `json:"scale"`
}

// PanelCenter is the optical center in normalized panel coordinates [0,1].
type PanelCenter struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SafetyBoundary is the inscribed safe rectangle in normalized coordinates.
type SafetyBoundary struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

// EyeParams groups every per-eye optical parameter.
type EyeParams struct {
	Distortion          *Distortion          `json:"distortion"`
	ChromaticAberration *ChromaticAberration `json:"chromatic_aberration"`
	PanelCenter         *PanelCenter         `json:"panel_center"`
	SafetyBoundary      *SafetyBoundary      `json:"safety_boundary"`
}

// Config is the full versioned stereo optical configuration.
type Config struct {
	Version int        `json:"version"`
	Left    *EyeParams `json:"left"`
	Right   *EyeParams `json:"right"`
}

// FieldError pinpoints one invalid or missing field.
type FieldError struct {
	Field  string // dotted path, e.g. "left.distortion.coefficients"
	Reason string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// ValidationError aggregates every problem found in a candidate config.
type ValidationError struct {
	Problems []FieldError
}

func (e *ValidationError) Error() string {
	out := fmt.Sprintf("config rejected (%d problem(s)):", len(e.Problems))
	for _, p := range e.Problems {
		out += "\n  - " + p.Error()
	}
	return out
}

func (e *ValidationError) add(field, reason string) {
	e.Problems = append(e.Problems, FieldError{Field: field, Reason: reason})
}

// Decode parses raw JSON into a Config without validating it.
func Decode(data []byte) (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config is not valid JSON: %w", err)
	}
	return &cfg, nil
}

// Encode serializes a config for persistence.
func Encode(cfg *Config) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}
