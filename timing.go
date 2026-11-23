package rollingshutter

import (
	"fmt"
	"time"
)

// ReadoutDirection describes the physical order in which image rows are exposed.
type ReadoutDirection int

const (
	TopToBottom ReadoutDirection = iota + 1
	BottomToTop
)

func (d ReadoutDirection) String() string {
	switch d {
	case TopToBottom:
		return "top-to-bottom"
	case BottomToTop:
		return "bottom-to-top"
	default:
		return "unknown"
	}
}

// Reversed reports whether rows are read out from the bottom upwards.
func (d ReadoutDirection) Reversed() bool {
	return d == BottomToTop
}

// RowTimingConfig defines the rolling-shutter clock independently from camera
// pose. FirstExposureStart is the exposure start of the first row in readout
// order, not necessarily image row zero.
type RowTimingConfig struct {
	FirstExposureStart time.Duration
	RowPeriod          time.Duration
	ExposureDuration   time.Duration
	PoseSampleOffset   time.Duration
	Direction          ReadoutDirection
}

// RowTiming is a validated row clock.
type RowTiming struct {
	config RowTimingConfig
}

// NewRowTiming validates a row clock.
func NewRowTiming(cfg RowTimingConfig) (RowTiming, error) {
	if cfg.RowPeriod <= 0 {
		return RowTiming{}, fmt.Errorf("row period must be positive: %s", cfg.RowPeriod)
	}
	if cfg.ExposureDuration <= 0 {
		return RowTiming{}, fmt.Errorf("exposure duration must be positive: %s", cfg.ExposureDuration)
	}
	if cfg.PoseSampleOffset < 0 || cfg.PoseSampleOffset > cfg.ExposureDuration {
		return RowTiming{}, fmt.Errorf(
			"pose sample offset %s must be in [0, %s]",
			cfg.PoseSampleOffset, cfg.ExposureDuration,
		)
	}
	if cfg.Direction != TopToBottom && cfg.Direction != BottomToTop {
		return RowTiming{}, fmt.Errorf("invalid readout direction: %d", cfg.Direction)
	}
	return RowTiming{config: cfg}, nil
}

func (t RowTiming) Config() RowTimingConfig {
	return t.config
}

// Validate checks that the clock was built from a valid configuration.
func (t RowTiming) Validate() error {
	_, err := NewRowTiming(t.config)
	return err
}

// RowWindow contains all row timing values. ExposureStart, ReadoutOrder and
// PoseSampleTime are intentionally separate values.
type RowWindow struct {
	Row               int
	Height            int
	ReadoutOrder      int
	ExposureStart     time.Duration
	ExposureEnd       time.Duration
	PoseSampleTime    time.Duration
	Direction         ReadoutDirection
	DirectionReversed bool
}

// Row returns the timing window for a physical image row.
func (t RowTiming) Row(row, height int) (RowWindow, error) {
	if err := t.Validate(); err != nil {
		return RowWindow{}, err
	}
	if height <= 0 {
		return RowWindow{}, fmt.Errorf("frame height must be positive: %d", height)
	}
	if row < 0 || row >= height {
		return RowWindow{}, fmt.Errorf("row %d outside [0, %d)", row, height)
	}
	order := row
	if t.config.Direction == BottomToTop {
		order = height - 1 - row
	}
	start := t.config.FirstExposureStart + time.Duration(order)*t.config.RowPeriod
	return RowWindow{
		Row:               row,
		Height:            height,
		ReadoutOrder:      order,
		ExposureStart:     start,
		ExposureEnd:       start + t.config.ExposureDuration,
		PoseSampleTime:    start + t.config.PoseSampleOffset,
		Direction:         t.config.Direction,
		DirectionReversed: t.config.Direction.Reversed(),
	}, nil
}

// GlobalShutterReference is the chosen common capture instant. It is the pose
// sample time of the first row in readout order.
func (t RowTiming) GlobalShutterReference() time.Duration {
	return t.config.FirstExposureStart + t.config.PoseSampleOffset
}

// SignedTimeError returns sample time minus the global-shutter reference.
func (t RowTiming) SignedTimeError(window RowWindow) time.Duration {
	return window.PoseSampleTime - t.GlobalShutterReference()
}

// FrameSpan is the time from the first exposure start to the last exposure end.
func (t RowTiming) FrameSpan(height int) (time.Duration, error) {
	if err := t.Validate(); err != nil {
		return 0, err
	}
	if height <= 0 {
		return 0, fmt.Errorf("frame height must be positive: %d", height)
	}
	return time.Duration(height-1)*t.config.RowPeriod + t.config.ExposureDuration, nil
}
