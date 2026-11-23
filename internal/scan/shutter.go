package scan

import "math"

type ScanDirection int

const (
	TopToBottom ScanDirection = iota
	BottomToTop
)

// Shutter maps image rows to capture times. ReadoutSec is the time from the
// first scanned row to the last scanned row.
type Shutter struct {
	Camera     Camera
	ReadoutSec float64
	FrameSec   float64
	Direction  ScanDirection
}

func DefaultShutter() Shutter {
	return Shutter{
		Camera:     DefaultCamera(),
		ReadoutSec: 1.0 / 60.0,
		FrameSec:   1.0 / 60.0,
		Direction:  TopToBottom,
	}
}

// RowTime returns the exposure time of a discrete row.
func (s Shutter) RowTime(frameStart float64, row int) float64 {
	if row < 0 {
		row = 0
	}
	if row >= s.Camera.Height {
		row = s.Camera.Height - 1
	}
	return s.RowTimeAtY(frameStart, float64(row))
}

// RowTimeAtY accepts a continuous row coordinate. Fractional rows are useful
// when fitting a segment centre or when refining a projected landmark row.
func (s Shutter) RowTimeAtY(frameStart, y float64) float64 {
	h := float64(s.Camera.Height - 1)
	if h <= 0 {
		return frameStart
	}
	u := y / h
	if s.Direction == BottomToTop {
		u = 1 - u
	}
	return frameStart + clamp(u, 0, 1)*s.ReadoutSec
}

// RowAtTime is the inverse of RowTimeAtY.
func (s Shutter) RowAtTime(frameStart, t float64) float64 {
	if s.ReadoutSec <= 0 {
		return 0
	}
	u := clamp((t-frameStart)/s.ReadoutSec, 0, 1)
	if s.Direction == BottomToTop {
		u = 1 - u
	}
	return u * float64(s.Camera.Height-1)
}

func (s Shutter) MidTime(frameStart float64) float64 {
	return frameStart + s.ReadoutSec/2
}

func clamp(v, lo, hi float64) float64 {
	if math.IsNaN(v) {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
