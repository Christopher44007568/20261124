package gaze

import (
	"errors"
	"fmt"
)

// Eye identifies the left or the right eye.
type Eye int

const (
	LeftEye Eye = iota
	RightEye
)

func (e Eye) String() string {
	switch e {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	default:
		return fmt.Sprintf("eye(%d)", int(e))
	}
}

// Point is a normalized screen coordinate in [0,1]x[0,1].
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Level is the foveated quality tier of a sample cell.
type Level uint8

const (
	Peripheral Level = iota
	Parafoveal
	Foveal
	// LevelCount is the number of quality tiers.
	LevelCount Level = 3
)

// Params describes the radial quality boundaries of one eye.
//
// Every eye carries its own parameters so a shared mask can never move or
// reshape another eye's fovea. Distances are normalized by grid size.
type Params struct {
	FovealRadius     float64 `json:"fovealRadius"`
	ParafovealRadius float64 `json:"parafovealRadius"`
	// FallbackLevel is emitted for every cell while the eye is occluded.
	FallbackLevel Level `json:"fallbackLevel"`
}

// DefaultParams returns conservative, deterministic boundaries.
func DefaultParams() Params {
	return Params{
		FovealRadius:     0.10,
		ParafovealRadius: 0.24,
		FallbackLevel:    Peripheral,
	}
}

// Validate rejects inverted or out-of-range boundaries.
func (p Params) Validate() error {
	if p.FovealRadius <= 0 || p.ParafovealRadius <= 0 {
		return errors.New("gaze: quality radii must be positive")
	}
	if p.FovealRadius >= p.ParafovealRadius {
		return errors.New("gaze: foveal radius must be smaller than parafoveal radius")
	}
	if p.FallbackLevel >= LevelCount {
		return errors.New("gaze: invalid fallback level")
	}
	return nil
}

// EyeConfig is one eye's state for a single frame.
type EyeConfig struct {
	Eye      Eye    `json:"eye"`
	Gaze     Point  `json:"gaze"`
	Occluded bool   `json:"occluded"`
	Params   Params `json:"params"`
}
