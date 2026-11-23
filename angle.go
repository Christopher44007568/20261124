package main

import "math"

// wrapAngle normalizes an angle in radians to the half-open interval [-pi, pi).
// Yaw is a single degree of freedom here, so every error is reduced with this
// function to stay independent of full head rotations.
func wrapAngle(a float64) float64 {
	a = math.Mod(a, 2*math.Pi)
	if a >= math.Pi {
		a -= 2 * math.Pi
	}
	if a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func deg2rad(d float64) float64 { return d * math.Pi / 180.0 }

func rad2deg(r float64) float64 { return 180.0 * r / math.Pi }

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
