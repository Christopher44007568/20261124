package main

import "math"

// CenterTracker moves the fovea center with hysteresis: a candidate
// position must persist for Confirm consecutive frames before the
// center commits, and a Cooldown follows each switch. A single-frame
// outlier therefore never moves the center.
type CenterTracker struct {
	CX, CY    float64
	Threshold float64
	Confirm   int
	Cooldown  int
	Switches  int

	candX, candY float64
	candN        int
	cool         int
}

func NewCenterTracker(x, y float64) *CenterTracker {
	return &CenterTracker{CX: x, CY: y, Threshold: 60, Confirm: 3, Cooldown: 8}
}

func (t *CenterTracker) Update(x, y float64, valid bool) {
	if t.cool > 0 {
		t.cool--
	}
	if !valid {
		t.candN = 0
		return
	}
	if math.Hypot(x-t.CX, y-t.CY) <= t.Threshold {
		t.candN = 0
		return
	}
	if t.candN > 0 && math.Hypot(x-t.candX, y-t.candY) <= t.Threshold/2 {
		t.candN++
	} else {
		t.candX, t.candY = x, y
		t.candN = 1
	}
	if t.candN >= t.Confirm && t.cool == 0 {
		t.CX, t.CY = x, y
		t.candN = 0
		t.cool = t.Cooldown
		t.Switches++
	}
}
