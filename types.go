package reproject

import "math"

// Vec3 is a plain CPU-side vector. No graphics SDK types are used.
type Vec3 struct {
	X, Y, Z float32
}

// Frame contains the data required to validate a reprojected history sample.
// Depth is positive view-space depth. Normal is normalized and stored in view space.
type Frame struct {
	Width   int
	Height  int
	Depth   []float32
	Normals []Vec3

	// Confidence is in [0,1]. Zero means that the source pixel itself is invalid,
	// for example a transparent edge without a stable surface.
	Confidence []float32
}

// Camera describes an orthonormal world-to-view rotation and a view position.
type Camera struct {
	Right   Vec3
	Up      Vec3
	Forward Vec3
	Origin  Vec3
	FX      float32
	FY      float32
}

// RejectReason explains why a history sample cannot be blended.
type RejectReason uint8

const (
	RejectNone RejectReason = iota
	RejectNoHistory
	RejectLowHistoryConfidence
	RejectCurrentInvalid
	RejectDepthMismatch
	RejectNormalMismatch
	RejectDisocclusion
)

// String returns a stable machine-readable rejection label.
func (r RejectReason) String() string {
	switch r {
	case RejectNone:
		return "none"
	case RejectNoHistory:
		return "no_history"
	case RejectLowHistoryConfidence:
		return "low_history_confidence"
	case RejectCurrentInvalid:
		return "current_invalid"
	case RejectDepthMismatch:
		return "depth_mismatch"
	case RejectNormalMismatch:
		return "normal_mismatch"
	case RejectDisocclusion:
		return "disocclusion"
	default:
		return "unknown"
	}
}

// PixelResult is an explicit decision for one current-frame pixel.
type PixelResult struct {
	HistoryX int
	HistoryY int
	Valid    bool
	Reason   RejectReason

	DepthScore    float32
	NormalScore   float32
	Confidence    float32
	ReprojectedOK bool
}

// Mask is a full-frame validation result.
type Mask struct {
	Width  int
	Height int
	Pixels []PixelResult
}

func dot(a, b Vec3) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

func sub(a, b Vec3) Vec3 {
	return Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}

func normalize(v Vec3) Vec3 {
	length := float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
	if length == 0 {
		return Vec3{}
	}
	return Vec3{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}

func abs32(v float32) float32 {
	return float32(math.Abs(float64(v)))
}

func (f Frame) index(x, y int) int {
	return y*f.Width + x
}

func (f Frame) wellFormed() bool {
	if f.Width <= 0 || f.Height <= 0 {
		return false
	}
	count := f.Width * f.Height
	return len(f.Depth) == count && len(f.Normals) == count && len(f.Confidence) == count
}

// Rejected reports whether the pixel must be marked rather than blended.
func (p PixelResult) Rejected() bool {
	return !p.Valid
}

// RejectedAt treats coordinates outside the frame as unusable history.
func (m Mask) RejectedAt(x, y int) bool {
	if x < 0 || y < 0 || x >= m.Width || y >= m.Height {
		return true
	}
	return m.Pixels[y*m.Width+x].Rejected()
}
