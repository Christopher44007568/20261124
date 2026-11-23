package main

import (
	"math"
	"testing"
)

func TestWrapAngle(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{math.Pi / 2, math.Pi / 2},
		{math.Pi, -math.Pi},
		{-math.Pi, -math.Pi},
		{3 * math.Pi / 2, -math.Pi / 2},
		{-3 * math.Pi / 2, math.Pi / 2},
		{4 * math.Pi, 0},
		{-4 * math.Pi, 0},
	}
	for _, c := range cases {
		if got := wrapAngle(c.in); math.Abs(got-c.want) > 1e-12 {
			t.Errorf("wrapAngle(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestConversionsAndClamp(t *testing.T) {
	if math.Abs(rad2deg(deg2rad(90))-90) > 1e-12 {
		t.Fatal("degree/radian round trip failed")
	}
	if got := clampFloat(15, -1, 1); got != 1 {
		t.Errorf("clampFloat upper bound = %v, want 1", got)
	}
	if got := clampFloat(-15, -1, 1); got != -1 {
		t.Errorf("clampFloat lower bound = %v, want -1", got)
	}
	if got := clampFloat(0.5, -1, 1); got != 0.5 {
		t.Errorf("clampFloat in range = %v, want 0.5", got)
	}
}
