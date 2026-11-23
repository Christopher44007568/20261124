package gaze

import "fmt"

func errSize(size int) error {
	return fmt.Errorf("gaze: grid size must be positive, got %d", size)
}

func errPoint(p Point) error {
	return fmt.Errorf("gaze: gaze point must lie in [0,1]x[0,1], got (%.3f,%.3f)", p.X, p.Y)
}
