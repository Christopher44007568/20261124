package optics

import (
	"fmt"
	"math"
)

// Validate checks a candidate config for required fields, version
// compatibility, value sanity, and left/right format agreement.
// It returns nil when the config is fully usable.
func Validate(cfg *Config) error {
	verr := &ValidationError{}

	if cfg.Version < MinVersion || cfg.Version > CurrentVersion {
		verr.add("version", fmt.Sprintf("unsupported version %d (supported range %d..%d)",
			cfg.Version, MinVersion, CurrentVersion))
	}

	validateEye(verr, string(LeftEye), cfg.Left)
	validateEye(verr, string(RightEye), cfg.Right)

	if cfg.Left != nil && cfg.Right != nil {
		validateEyePair(verr, cfg.Left, cfg.Right)
	}

	if len(verr.Problems) > 0 {
		return verr
	}
	return nil
}

func validateEye(verr *ValidationError, side string, eye *EyeParams) {
	if eye == nil {
		verr.add(side, "missing entire per-eye parameter block")
		return
	}
	prefix := side + "."

	if eye.Distortion == nil {
		verr.add(prefix+"distortion", "missing field")
	} else if len(eye.Distortion.Coefficients) == 0 {
		verr.add(prefix+"distortion.coefficients", "missing field: at least one coefficient required")
	} else {
		for i, k := range eye.Distortion.Coefficients {
			if math.IsNaN(k) || math.IsInf(k, 0) {
				verr.add(fmt.Sprintf("%sdistortion.coefficients[%d]", prefix, i), "not a finite number")
			}
		}
	}

	if eye.ChromaticAberration == nil {
		verr.add(prefix+"chromatic_aberration", "missing field")
	} else {
		for i, s := range eye.ChromaticAberration.Scale {
			if math.IsNaN(s) || math.IsInf(s, 0) || s <= 0 {
				verr.add(fmt.Sprintf("%schromatic_aberration.scale[%d]", prefix, i),
					"scale must be a positive finite number")
			}
		}
	}

	if eye.PanelCenter == nil {
		verr.add(prefix+"panel_center", "missing field")
	} else {
		if !inUnit(eye.PanelCenter.X) {
			verr.add(prefix+"panel_center.x", "must be within [0,1]")
		}
		if !inUnit(eye.PanelCenter.Y) {
			verr.add(prefix+"panel_center.y", "must be within [0,1]")
		}
	}

	if eye.SafetyBoundary == nil {
		verr.add(prefix+"safety_boundary", "missing field")
	} else {
		b := eye.SafetyBoundary
		if !(b.MinX < b.MaxX) || !(b.MinY < b.MaxY) {
			verr.add(prefix+"safety_boundary", "min must be strictly less than max on both axes")
		}
		if !inUnit(b.MinX) || !inUnit(b.MaxX) || !inUnit(b.MinY) || !inUnit(b.MaxY) {
			verr.add(prefix+"safety_boundary", "edges must be within [0,1]")
		}
	}
}

// validateEyePair ensures both eyes use the same parameter formats so a
// single rendering pipeline can consume them interchangeably.
func validateEyePair(verr *ValidationError, left, right *EyeParams) {
	if left.Distortion != nil && right.Distortion != nil {
		ln, rn := len(left.Distortion.Coefficients), len(right.Distortion.Coefficients)
		if ln != rn {
			verr.add("distortion.coefficients",
				fmt.Sprintf("left/right format mismatch: left has %d coefficients, right has %d", ln, rn))
		}
	}
}

func inUnit(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1
}
