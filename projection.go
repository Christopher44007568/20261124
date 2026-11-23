package rollingshutter

import (
	"errors"
	"fmt"
	"math"
)

var (
	ErrPointNotVisible = errors.New("point is not in front of camera")
	ErrLineMissesRow   = errors.New("projected line does not cross row")
)

// Camera is a pinhole camera. Pixel coordinates use x right and y down. The
// principal point is specified in continuous pixel coordinates.
type Camera struct {
	Width  int     `json:"width"`
	Height int     `json:"height"`
	FX     float64 `json:"fx"`
	FY     float64 `json:"fy"`
	CX     float64 `json:"cx"`
	CY     float64 `json:"cy"`
	Near   float64 `json:"near"`
}

// DefaultCamera returns a 640x480, 800-pixel-focal-length camera.
func DefaultCamera() Camera {
	return Camera{
		Width:  640,
		Height: 480,
		FX:     800,
		FY:     800,
		CX:     320,
		CY:     240,
		Near:   0.05,
	}
}

func (c Camera) Validate() error {
	if c.Width <= 0 || c.Height <= 0 {
		return fmt.Errorf("camera size must be positive: %dx%d", c.Width, c.Height)
	}
	if c.FX <= 0 || c.FY <= 0 {
		return fmt.Errorf("camera focal lengths must be positive: fx=%g fy=%g", c.FX, c.FY)
	}
	if c.Near <= 0 {
		return fmt.Errorf("camera near plane must be positive: %g", c.Near)
	}
	return nil
}

// Vec2 is a continuous image coordinate in pixels.
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Project projects a world point with one pose.
func (c Camera) Project(pose Pose, world Vec3) (Vec2, error) {
	camera := pose.WorldToCamera(world)
	if !isFinite(camera.X) || !isFinite(camera.Y) || !isFinite(camera.Z) || camera.Z <= c.Near {
		return Vec2{}, fmt.Errorf("%w: z=%g", ErrPointNotVisible, camera.Z)
	}
	return Vec2{
		X: c.FX*camera.X/camera.Z + c.CX,
		Y: c.FY*camera.Y/camera.Z + c.CY,
	}, nil
}

// LineSegment is a finite straight feature in world space.
type LineSegment struct {
	A Vec3 `json:"a"`
	B Vec3 `json:"b"`
}

func (line LineSegment) at(fraction float64) Vec3 {
	return line.A.Add(line.B.Sub(line.A).Scale(fraction))
}

// LineProjection is the intersection of one image row with the projection of
// a world-space straight line under the pose for that row.
type LineProjection struct {
	Image     Vec2    `json:"image"`
	World     Vec3    `json:"world"`
	Parameter float64 `json:"parameter"`
}

// ProjectLineAtRow solves for the point on line whose projection has the
// requested row coordinate, using the pose belonging to that row.
func ProjectLineAtRow(camera Camera, pose Pose, line LineSegment, row int) (LineProjection, error) {
	if err := camera.Validate(); err != nil {
		return LineProjection{}, err
	}
	if row < 0 || row >= camera.Height {
		return LineProjection{}, fmt.Errorf("row %d outside [0, %d)", row, camera.Height)
	}
	start, err := camera.Project(pose, line.A)
	if err != nil {
		return LineProjection{}, fmt.Errorf("line endpoint A: %w", err)
	}
	end, err := camera.Project(pose, line.B)
	if err != nil {
		return LineProjection{}, fmt.Errorf("line endpoint B: %w", err)
	}
	target := float64(row) + 0.5
	lowY, highY := math.Min(start.Y, end.Y), math.Max(start.Y, end.Y)
	if target < lowY || target > highY {
		return LineProjection{}, fmt.Errorf(
			"%w %d: projected y range [%.3f, %.3f]",
			ErrLineMissesRow, row, lowY, highY,
		)
	}
	if math.Abs(end.Y-start.Y) < 1e-12 {
		return LineProjection{}, fmt.Errorf("%w %d: projected line is horizontal", ErrLineMissesRow, row)
	}

	ascending := start.Y <= end.Y
	low, high := 0.0, 1.0
	var best LineProjection
	for iteration := 0; iteration < 64; iteration++ {
		middle := (low + high) / 2
		world := line.at(middle)
		image, err := camera.Project(pose, world)
		if err != nil {
			return LineProjection{}, fmt.Errorf("projection at parameter %.9f: %w", middle, err)
		}
		best = LineProjection{Image: image, World: world, Parameter: middle}
		if (ascending && image.Y < target) || (!ascending && image.Y > target) {
			low = middle
		} else {
			high = middle
		}
	}
	return best, nil
}

// ProjectedPoint is one row's sample of a projected straight line.
type ProjectedPoint struct {
	Row int
	X   float64
	Y   float64
}

// CurvatureResult describes how much a straight world line bends in an image.
type CurvatureResult struct {
	Computed    bool    `json:"computed"`
	MaxPixels   float64 `json:"maxPixels"`
	RMSWarpPx   float64 `json:"rmsPixels"`
	AtRow       int     `json:"atRow"`
	ChordLength float64 `json:"chordLengthPixels"`
}

// StraightLineCurvature measures maximum perpendicular deviation from the
// chord joining the first and last projected points. Points must be distinct
// row samples; order determines the chord.
func StraightLineCurvature(points []ProjectedPoint) CurvatureResult {
	if len(points) < 2 {
		return CurvatureResult{}
	}
	first, last := points[0], points[len(points)-1]
	dx, dy := last.X-first.X, last.Y-first.Y
	chord := math.Hypot(dx, dy)
	if chord == 0 {
		return CurvatureResult{}
	}

	result := CurvatureResult{Computed: true, ChordLength: chord}
	var sumSquares float64
	for _, point := range points {
		distance := math.Abs((point.X-first.X)*dy-(point.Y-first.Y)*dx) / chord
		sumSquares += distance * distance
		if distance > result.MaxPixels {
			result.MaxPixels = distance
			result.AtRow = point.Row
		}
	}
	result.RMSWarpPx = math.Sqrt(sumSquares / float64(len(points)))
	return result
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
