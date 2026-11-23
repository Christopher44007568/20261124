package ae

import (
	"fmt"
	"sort"
)

type MeterMode string

const (
	ModeWholeFrame     MeterMode = "whole-frame"
	ModeZonal          MeterMode = "zonal"
	ModeTemporalWindow MeterMode = "temporal-window"
)

// MeterConfig controls the spatial and temporal extent of metering.
type MeterConfig struct {
	ZonesX        int
	ZonesY        int
	WindowFrames  int
	HighThreshold float64
}

func DefaultMeterConfig() MeterConfig {
	return MeterConfig{
		ZonesX:        12,
		ZonesY:        8,
		WindowFrames:  4,
		HighThreshold: 0.95,
	}
}

func (c MeterConfig) validate(mode MeterMode) error {
	if c.HighThreshold <= 0 || c.HighThreshold > 1 {
		return fmt.Errorf("high threshold %g is outside (0,1]", c.HighThreshold)
	}
	if mode == ModeZonal && (c.ZonesX <= 0 || c.ZonesY <= 0) {
		return fmt.Errorf("invalid zone grid %dx%d", c.ZonesX, c.ZonesY)
	}
	if mode == ModeTemporalWindow && c.WindowFrames <= 0 {
		return fmt.Errorf("window frames must be positive")
	}
	return nil
}

// Measurement is one frame's metering result. Control is the value fed to the
// exposure controller; Mean is always the current frame's plain mean.
type Measurement struct {
	Control        float64
	Mean           float64
	HighFraction   float64
	Generation     uint64
	WindowFrames   int
	StaleZoneReads uint64
}

type MeterStats struct {
	Generation     uint64
	Resets         uint64
	StaleZoneReads uint64
}

type Meter interface {
	Mode() MeterMode
	Measure(frame Frame) (Measurement, error)
	Reset()
	Stats() MeterStats
}

func NewMeter(mode MeterMode, cfg MeterConfig) (Meter, error) {
	if err := cfg.validate(mode); err != nil {
		return nil, err
	}
	switch mode {
	case ModeWholeFrame:
		return &wholeFrameMeter{cfg: cfg}, nil
	case ModeZonal:
		return &zonalMeter{
			cfg:   cfg,
			zones: make([]zoneState, cfg.ZonesX*cfg.ZonesY),
			parts: make([]Histogram, cfg.ZonesX*cfg.ZonesY),
		}, nil
	case ModeTemporalWindow:
		return &temporalMeter{cfg: cfg}, nil
	default:
		return nil, fmt.Errorf("unknown meter mode %q", mode)
	}
}

type wholeFrameMeter struct {
	cfg        MeterConfig
	generation uint64
	resets     uint64
}

func (m *wholeFrameMeter) Mode() MeterMode { return ModeWholeFrame }

func (m *wholeFrameMeter) Reset() {
	m.generation++
	m.resets++
}

func (m *wholeFrameMeter) Stats() MeterStats {
	return MeterStats{Generation: m.generation, Resets: m.resets}
}

func (m *wholeFrameMeter) Measure(frame Frame) (Measurement, error) {
	if err := frame.Validate(); err != nil {
		return Measurement{}, err
	}
	if frame.SceneCut {
		m.Reset()
	}
	var hist Histogram
	hist.AddFrame(frame)
	return Measurement{
		Control:      hist.Mean(),
		Mean:         hist.Mean(),
		HighFraction: hist.HighFraction(m.cfg.HighThreshold),
		Generation:   m.generation,
		WindowFrames: 1,
	}, nil
}

type zoneState struct {
	hist       Histogram
	generation uint64
	valid      bool
}

type zonalMeter struct {
	cfg            MeterConfig
	zones          []zoneState
	parts          []Histogram
	generation     uint64
	resets         uint64
	staleZoneReads uint64
}

func (m *zonalMeter) Mode() MeterMode { return ModeZonal }

func (m *zonalMeter) Reset() {
	m.generation++
	m.resets++
	for i := range m.zones {
		m.zones[i] = zoneState{}
	}
}

func (m *zonalMeter) Stats() MeterStats {
	return MeterStats{
		Generation:     m.generation,
		Resets:         m.resets,
		StaleZoneReads: m.staleZoneReads,
	}
}

func (m *zonalMeter) Measure(frame Frame) (Measurement, error) {
	if err := frame.Validate(); err != nil {
		return Measurement{}, err
	}
	if frame.Width < m.cfg.ZonesX || frame.Height < m.cfg.ZonesY {
		return Measurement{}, fmt.Errorf("frame %dx%d is smaller than zone grid %dx%d",
			frame.Width, frame.Height, m.cfg.ZonesX, m.cfg.ZonesY)
	}
	if frame.SceneCut {
		m.Reset()
	}

	for i := range m.parts {
		m.parts[i].Reset()
	}
	var global Histogram
	global.AddFrame(frame)
	for y := 0; y < frame.Height; y++ {
		zoneY := y * m.cfg.ZonesY / frame.Height
		row := y * frame.Width
		for x := 0; x < frame.Width; x++ {
			zoneX := x * m.cfg.ZonesX / frame.Width
			m.parts[zoneY*m.cfg.ZonesX+zoneX].Add(frame.Luma[row+x])
		}
	}

	zoneMeans := make([]float64, 0, len(m.parts))
	for i := range m.parts {
		// The current generation is committed only after this frame has fully
		// populated the zone. A stale generation is never used for metering.
		m.zones[i] = zoneState{
			hist:       m.parts[i],
			generation: m.generation,
			valid:      m.parts[i].Count > 0,
		}
		if !m.zones[i].valid {
			continue
		}
		if m.zones[i].generation != m.generation {
			m.staleZoneReads++
			continue
		}
		zoneMeans = append(zoneMeans, m.zones[i].hist.Mean())
	}
	sort.Float64s(zoneMeans)
	control := median(zoneMeans)
	return Measurement{
		Control:        control,
		Mean:           global.Mean(),
		HighFraction:   global.HighFraction(m.cfg.HighThreshold),
		Generation:     m.generation,
		WindowFrames:   1,
		StaleZoneReads: m.staleZoneReads,
	}, nil
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}

type temporalMeter struct {
	cfg            MeterConfig
	history        []Histogram
	generation     uint64
	resets         uint64
	staleZoneReads uint64
}

func (m *temporalMeter) Mode() MeterMode { return ModeTemporalWindow }

func (m *temporalMeter) Reset() {
	m.generation++
	m.resets++
	m.history = m.history[:0]
}

func (m *temporalMeter) Stats() MeterStats {
	return MeterStats{
		Generation:     m.generation,
		Resets:         m.resets,
		StaleZoneReads: m.staleZoneReads,
	}
}

func (m *temporalMeter) Measure(frame Frame) (Measurement, error) {
	if err := frame.Validate(); err != nil {
		return Measurement{}, err
	}
	if frame.SceneCut {
		// A new scene starts with an empty window. Keeping old histogram bins
		// here is what leaks a previous frame's highlights into the new scene.
		m.Reset()
	}
	var current Histogram
	current.AddFrame(frame)
	m.history = append(m.history, current)
	if len(m.history) > m.cfg.WindowFrames {
		copy(m.history, m.history[len(m.history)-m.cfg.WindowFrames:])
		m.history = m.history[:m.cfg.WindowFrames]
	}
	var window Histogram
	for _, hist := range m.history {
		window.Merge(hist)
	}
	return Measurement{
		Control:        window.Mean(),
		Mean:           current.Mean(),
		HighFraction:   window.HighFraction(m.cfg.HighThreshold),
		Generation:     m.generation,
		WindowFrames:   len(m.history),
		StaleZoneReads: m.staleZoneReads,
	}, nil
}
