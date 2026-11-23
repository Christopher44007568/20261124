// Package foveq implements a joint foveated-rendering quality controller.
//
// Eye-buffer size, foveal quality and per-frame GPU budget are never tuned
// independently. They are bound together into an immutable Profile, and the
// controller only moves between whole profiles ("tiers").
package foveq

import (
	"errors"
	"fmt"
)

// Profile is one immutable operating point. All three rendering knobs are
// bound together, so callers cannot adjust the eye buffer, foveal quality or
// frame budget in isolation.
type Profile struct {
	// Tier is the dense index into the ladder, 0 is the lowest tier.
	Tier int
	// Name is a stable, human-readable label.
	Name string
	// EyeWidth and EyeHeight are the per-eye render-target pixels.
	EyeWidth  int
	EyeHeight int
	// FoveaScale is the central-resolution multiplier relative to native.
	FoveaScale float64
	// FrameBudgetMicros is the GPU time budget for one eye frame.
	FrameBudgetMicros float64
}

// Pixels reports the per-eye pixel count for the profile.
func (p Profile) Pixels() int { return p.EyeWidth * p.EyeHeight }

func (p Profile) String() string {
	return fmt.Sprintf(
		"%s[tier=%d %dx%d fovea=%.2f budget=%vus]",
		p.Name, p.Tier, p.EyeWidth, p.EyeHeight, p.FoveaScale, p.FrameBudgetMicros,
	)
}

// Ladder is the ordered, monotonically increasing set of profiles.
type Ladder struct {
	profiles []Profile
}

var (
	errEmptyLadder    = errors.New("foveq: ladder must contain profiles")
	errNotMonotonic   = errors.New("foveq: ladder must be strictly monotonic in buffer, fovea and budget")
	errTierOutOfRange = errors.New("foveq: tier out of range")
)

// NewLadder validates and wraps a set of profiles. The slice is copied so
// later caller mutation cannot change the ladder. Profiles must be ordered
// from lowest to highest and every joint knob must increase strictly.
func NewLadder(profiles []Profile) (*Ladder, error) {
	if len(profiles) == 0 {
		return nil, errEmptyLadder
	}
	cloned := make([]Profile, len(profiles))
	copy(cloned, profiles)
	for i := range cloned {
		cloned[i].Tier = i
		if i == 0 {
			continue
		}
		prev, cur := cloned[i-1], cloned[i]
		if !(cur.EyeWidth > prev.EyeWidth &&
			cur.EyeHeight > prev.EyeHeight &&
			cur.FoveaScale > prev.FoveaScale &&
			cur.FrameBudgetMicros > prev.FrameBudgetMicros) {
			return nil, fmt.Errorf("%w: tier %d -> tier %d", errNotMonotonic, i-1, i)
		}
	}
	return &Ladder{profiles: cloned}, nil
}

// Len returns the number of tiers.
func (l *Ladder) Len() int { return len(l.profiles) }

// At returns the profile at a tier index.
func (l *Ladder) At(tier int) (Profile, error) {
	if tier < 0 || tier >= len(l.profiles) {
		return Profile{}, fmt.Errorf("%w: got %d, have %d tiers", errTierOutOfRange, tier, len(l.profiles))
	}
	return l.profiles[tier], nil
}

// Min and Max tier indexes.
func (l *Ladder) Min() int { return 0 }
func (l *Ladder) Max() int { return len(l.profiles) - 1 }

// clampTier keeps a requested tier inside the ladder bounds. Downshifts can
// never cross the lower boundary and upshifts can never cross the upper one.
func (l *Ladder) clampTier(tier int) int {
	if tier < 0 {
		return 0
	}
	if tier > l.Max() {
		return l.Max()
	}
	return tier
}

// DefaultLadder builds a five-tier stereo ladder. Buffer resolution, foveal
// quality and frame budget rise together from tier 0 to tier 4.
func DefaultLadder() *Ladder {
	ladder, err := NewLadder([]Profile{
		{Name: "eco", EyeWidth: 1280, EyeHeight: 1152, FoveaScale: 0.60, FrameBudgetMicros: 4500},
		{Name: "low", EyeWidth: 1536, EyeHeight: 1344, FoveaScale: 0.72, FrameBudgetMicros: 5500},
		{Name: "mid", EyeWidth: 1824, EyeHeight: 1568, FoveaScale: 0.84, FrameBudgetMicros: 6700},
		{Name: "high", EyeWidth: 2112, EyeHeight: 1824, FoveaScale: 0.94, FrameBudgetMicros: 8000},
		{Name: "ultra", EyeWidth: 2448, EyeHeight: 2112, FoveaScale: 1.00, FrameBudgetMicros: 9500},
	})
	if err != nil {
		panic(err)
	}
	return ladder
}
