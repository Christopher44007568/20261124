package panelsampling

import (
	"errors"
	"fmt"
	"math"
)

var (
	// ErrNilTexture is returned when a sample has no source texture.
	ErrNilTexture = errors.New("panelsampling: nil texture")
	// ErrEyeMismatch prevents a request for one eye from touching another
	// eye's texture.
	ErrEyeMismatch = errors.New("panelsampling: request eye does not match texture eye")
	// ErrInvalidPolicy is returned when an unknown boundary policy is used.
	ErrInvalidPolicy = errors.New("panelsampling: invalid boundary policy")
	// ErrInvalidChannel is returned for an unknown channel.
	ErrInvalidChannel = errors.New("panelsampling: invalid color channel")
)

// SampleRequest identifies one source sample. Chromatic aberration should
// issue one request per channel with that channel's exact source coordinate.
type SampleRequest struct {
	Eye     Eye
	Stage   Stage
	Channel Channel
	UV      Coord
}

// Sample is the result of a single source sample. Valid only describes
// whether the boundary policy produced a usable color sample; HasDepth must
// be checked separately before writing depth.
type Sample struct {
	Color       Color
	Depth       float32
	HasDepth    bool
	Valid       bool
	OutOfBounds bool
	Source      Coord
	Resolved    Coord
}

// Sampler applies one boundary policy to every request while optionally
// recording out-of-panel coordinates.
type Sampler struct {
	policy Policy
	log    *OOBLog
}

// NewSampler creates a sampler. A nil log disables recording; callers that
// need to audit boundary behavior should provide one.
func NewSampler(policy Policy, log *OOBLog) (*Sampler, error) {
	if !policy.valid() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidPolicy, policy)
	}
	return &Sampler{policy: policy, log: log}, nil
}

// Policy returns the sampler's boundary policy.
func (s *Sampler) Policy() Policy {
	if s == nil {
		return Policy(255)
	}
	return s.policy
}

// IsOutsidePanel reports whether a coordinate lies outside [0, 1] or is not
// finite. Non-finite coordinates are always outside.
func IsOutsidePanel(coord Coord) bool {
	return !coord.finite() ||
		coord.X < 0 || coord.X > 1 ||
		coord.Y < 0 || coord.Y > 1
}

// Sample resolves one source coordinate. It records the original coordinate
// for every out-of-panel request, including repeated requests for the same
// coordinate.
func (s *Sampler) Sample(texture *Texture, request SampleRequest) (Sample, error) {
	if texture == nil {
		return Sample{}, ErrNilTexture
	}
	if request.Eye != texture.eye {
		return Sample{}, fmt.Errorf("%w: request=%s texture=%s",
			ErrEyeMismatch, request.Eye, texture.eye)
	}
	if !request.Channel.valid() {
		return Sample{}, fmt.Errorf("%w: %d", ErrInvalidChannel, request.Channel)
	}
	if s == nil || !s.policy.valid() {
		return Sample{}, ErrInvalidPolicy
	}

	outside := IsOutsidePanel(request.UV)
	result := Sample{
		Source:      request.UV,
		OutOfBounds: outside,
	}

	var recovered bool
	if !outside {
		result = s.sampleInside(texture, request.UV)
		result.Source = request.UV
		result.OutOfBounds = false
		recovered = result.Valid
	} else {
		result.Resolved, recovered = s.fallback(request.UV)
		if recovered {
			filtered := s.sampleInside(texture, result.Resolved)
			filtered.Source = request.UV
			filtered.OutOfBounds = true
			filtered.Resolved = result.Resolved
			result = filtered
		}
	}

	if outside {
		s.log.Record(OutOfBoundsRecord{
			Policy:     s.policy,
			Eye:        request.Eye,
			Stage:      request.Stage,
			Channel:    request.Channel,
			Source:     request.UV,
			Resolved:   result.Resolved,
			Recovered:  recovered,
			DepthValid: result.Valid && result.HasDepth,
		})
	}

	return result, nil
}

func (s *Sampler) fallback(source Coord) (Coord, bool) {
	switch s.policy {
	case PolicyClamp:
		if !source.finite() {
			return source, false
		}
		return Coord{
			X: clamp(source.X, 0, 1),
			Y: clamp(source.Y, 0, 1),
		}, true
	case PolicyRepeat:
		if !source.finite() {
			return source, false
		}
		return Coord{
			X: wrapUnit(source.X),
			Y: wrapUnit(source.Y),
		}, true
	case PolicyTransparentBlack:
		return source, false
	default:
		return source, false
	}
}

func (s *Sampler) sampleInside(texture *Texture, coord Coord) Sample {
	result := Sample{Resolved: coord}
	if !coord.finite() || coord.X < 0 || coord.X > 1 || coord.Y < 0 || coord.Y > 1 {
		return result
	}

	x := coord.X * float64(texture.width-1)
	y := coord.Y * float64(texture.height-1)
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)

	x1 := x0 + 1
	y1 := y0 + 1
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= texture.width {
		x1 = texture.width - 1
	}
	if y1 >= texture.height {
		y1 = texture.height - 1
	}

	taps := [...]struct {
		x      int
		y      int
		weight float64
	}{
		{x0, y0, (1 - fx) * (1 - fy)},
		{x1, y0, fx * (1 - fy)},
		{x0, y1, (1 - fx) * fy},
		{x1, y1, fx * fy},
	}

	var color Color
	var depth float64
	hasDepth := true
	for _, tap := range taps {
		if tap.weight == 0 {
			continue
		}
		texel := texture.texels[tap.y*texture.width+tap.x]
		color.R += texel.Color.R * tap.weight
		color.G += texel.Color.G * tap.weight
		color.B += texel.Color.B * tap.weight
		color.A += texel.Color.A * tap.weight
		if !texel.HasDepth {
			hasDepth = false
			continue
		}
		depth += float64(texel.Depth) * tap.weight
	}

	result.Color = color
	result.Valid = true
	if hasDepth {
		result.Depth = float32(depth)
		result.HasDepth = true
	}
	return result
}

func clamp(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func wrapUnit(value float64) float64 {
	wrapped := math.Mod(value, 1)
	if wrapped < 0 {
		wrapped++
	}
	return wrapped
}
