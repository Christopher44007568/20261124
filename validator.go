package reproject

import (
	"errors"
	"math"
)

// Limits controls conservative surface validation.
type Limits struct {
	MinHistoryConfidence float32
	MinCurrentConfidence float32
	DepthBias            float32
	DepthRelativeSlope   float32
	DisocclusionSlope    float32
	MinNormalDot         float32
}

// DefaultLimits intentionally reject only material surface changes.
func DefaultLimits() Limits {
	return Limits{
		MinHistoryConfidence: 0.15,
		MinCurrentConfidence: 0.05,
		DepthBias:            0.02,
		DepthRelativeSlope:   0.03,
		DisocclusionSlope:    0.015,
		MinNormalDot:         float32(math.Cos(math.Pi / 4)),
	}
}

// Stats summarizes a rejection mask without hiding per-pixel reasons.
type Stats struct {
	Total             int
	Accepted          int
	Rejected          int
	NoHistory         int
	LowHistory        int
	CurrentInvalid    int
	DepthMismatch     int
	NormalMismatch    int
	Disocclusion      int
	RejectionFraction float32
}

// Validator performs CPU-only history reprojection and surface validation.
type Validator struct {
	limits Limits
}

// NewValidator validates threshold ranges before any frame is evaluated.
func NewValidator(limits Limits) (Validator, error) {
	if limits.MinHistoryConfidence < 0 || limits.MinHistoryConfidence > 1 {
		return Validator{}, errors.New("MinHistoryConfidence must be in [0,1]")
	}
	if limits.MinCurrentConfidence < 0 || limits.MinCurrentConfidence > 1 {
		return Validator{}, errors.New("MinCurrentConfidence must be in [0,1]")
	}
	if limits.DepthBias < 0 || limits.DepthRelativeSlope < 0 || limits.DisocclusionSlope < 0 {
		return Validator{}, errors.New("depth thresholds must be non-negative")
	}
	if limits.MinNormalDot < -1 || limits.MinNormalDot > 1 {
		return Validator{}, errors.New("MinNormalDot must be in [-1,1]")
	}
	return Validator{limits: limits}, nil
}

// Evaluate reprojects every current pixel into the previous camera.
func (v Validator) Evaluate(current Frame, currentCamera Camera, history Frame, historyCamera Camera) (Mask, Stats, error) {
	if !current.wellFormed() || !history.wellFormed() {
		return Mask{}, Stats{}, errors.New("frame slices must match width and height")
	}
	if current.Width != history.Width || current.Height != history.Height {
		return Mask{}, Stats{}, errors.New("current and history dimensions must match")
	}
	if err := validateCamera(currentCamera); err != nil {
		return Mask{}, Stats{}, err
	}
	if err := validateCamera(historyCamera); err != nil {
		return Mask{}, Stats{}, err
	}

	mask := Mask{
		Width:  current.Width,
		Height: current.Height,
		Pixels: make([]PixelResult, current.Width*current.Height),
	}
	stats := Stats{Total: len(mask.Pixels)}

	for y := 0; y < current.Height; y++ {
		for x := 0; x < current.Width; x++ {
			result := v.evaluatePixel(x, y, current, currentCamera, history, historyCamera)
			mask.Pixels[current.index(x, y)] = result
			if result.Valid {
				stats.Accepted++
			} else {
				stats.Rejected++
				switch result.Reason {
				case RejectNoHistory:
					stats.NoHistory++
				case RejectLowHistoryConfidence:
					stats.LowHistory++
				case RejectCurrentInvalid:
					stats.CurrentInvalid++
				case RejectDepthMismatch:
					stats.DepthMismatch++
				case RejectNormalMismatch:
					stats.NormalMismatch++
				case RejectDisocclusion:
					stats.Disocclusion++
				}
			}
		}
	}
	stats.RejectionFraction = float32(stats.Rejected) / float32(stats.Total)
	return mask, stats, nil
}

func (v Validator) evaluatePixel(x, y int, current Frame, currentCamera Camera, history Frame, historyCamera Camera) PixelResult {
	idx := current.index(x, y)
	result := PixelResult{HistoryX: -1, HistoryY: -1}
	currentDepth := current.Depth[idx]
	currentConfidence := current.Confidence[idx]
	if currentDepth <= 0 || currentConfidence < v.limits.MinCurrentConfidence {
		result.Reason = RejectCurrentInvalid
		return result
	}

	worldPoint := unproject(x, y, current.Width, current.Height, currentDepth, currentCamera)
	hx, hy, historyDepth := project(worldPoint, historyCamera, history.Width, history.Height)
	if hx < 0 || hy < 0 {
		result.Reason = RejectNoHistory
		return result
	}

	historyIndex := history.index(hx, hy)
	historyConfidence := history.Confidence[historyIndex]
	sampledDepth := history.Depth[historyIndex]
	result.HistoryX = hx
	result.HistoryY = hy
	result.Confidence = historyConfidence
	if sampledDepth <= 0 || historyConfidence < v.limits.MinHistoryConfidence {
		result.Reason = RejectLowHistoryConfidence
		return result
	}

	depthDifference := sampledDepth - historyDepth
	result.DepthScore = abs32(depthDifference)
	depthThreshold := v.limits.DepthBias + v.limits.DepthRelativeSlope*max32(currentDepth, sampledDepth)
	if depthDifference > v.limits.DepthBias+v.limits.DisocclusionSlope*historyDepth {
		result.Reason = RejectDisocclusion
		return result
	}
	if result.DepthScore > depthThreshold {
		result.Reason = RejectDepthMismatch
		return result
	}

	currentNormal := normalize(current.Normals[idx])
	historyNormalWorld := viewNormalToWorld(history.Normals[historyIndex], historyCamera)
	historyNormalCurrent := worldNormalToView(historyNormalWorld, currentCamera)
	normalDot := dot(currentNormal, historyNormalCurrent)
	result.NormalScore = 1 - normalDot
	if normalDot < v.limits.MinNormalDot {
		result.Reason = RejectNormalMismatch
		return result
	}

	result.Valid = true
	result.ReprojectedOK = true
	return result
}

func validateCamera(c Camera) error {
	if c.FX <= 0 || c.FY <= 0 {
		return errors.New("camera focal lengths must be positive")
	}
	if nearlyZero(c.Right) || nearlyZero(c.Up) || nearlyZero(c.Forward) {
		return errors.New("camera axes must be non-zero")
	}
	if abs32(dot(c.Right, c.Up)) > 1e-4 || abs32(dot(c.Right, c.Forward)) > 1e-4 || abs32(dot(c.Up, c.Forward)) > 1e-4 {
		return errors.New("camera axes must be orthogonal")
	}
	if abs32(dot(c.Right, c.Right)-1) > 1e-4 || abs32(dot(c.Up, c.Up)-1) > 1e-4 || abs32(dot(c.Forward, c.Forward)-1) > 1e-4 {
		return errors.New("camera axes must be unit length")
	}
	return nil
}

func nearlyZero(v Vec3) bool {
	return v.X == 0 && v.Y == 0 && v.Z == 0
}

func unproject(x, y, width, height int, depth float32, c Camera) Vec3 {
	ndcX := 2*(float32(x)+0.5)/float32(width) - 1
	ndcY := 1 - 2*(float32(y)+0.5)/float32(height)
	view := Vec3{X: ndcX * depth / c.FX, Y: ndcY * depth / c.FY, Z: depth}
	return add(c.Origin, scale(c.Right, view.X), scale(c.Up, view.Y), scale(c.Forward, view.Z))
}

func project(world Vec3, c Camera, width, height int) (int, int, float32) {
	relative := sub(world, c.Origin)
	viewX := dot(relative, c.Right)
	viewY := dot(relative, c.Up)
	viewZ := dot(relative, c.Forward)
	if viewZ <= 0 {
		return -1, -1, viewZ
	}
	ndcX := c.FX * viewX / viewZ
	ndcY := c.FY * viewY / viewZ
	px := int((ndcX + 1) * 0.5 * float32(width))
	py := int((1 - ndcY) * 0.5 * float32(height))
	if px < 0 || px >= width || py < 0 || py >= height {
		return -1, -1, viewZ
	}
	return px, py, viewZ
}

func viewNormalToWorld(normal Vec3, c Camera) Vec3 {
	return normalize(add(scale(c.Right, normal.X), scale(c.Up, normal.Y), scale(c.Forward, normal.Z)))
}

func worldNormalToView(normal Vec3, c Camera) Vec3 {
	return Vec3{X: dot(normal, c.Right), Y: dot(normal, c.Up), Z: dot(normal, c.Forward)}
}

func add(parts ...Vec3) Vec3 {
	var result Vec3
	for _, part := range parts {
		result.X += part.X
		result.Y += part.Y
		result.Z += part.Z
	}
	return result
}

func scale(v Vec3, s float32) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
