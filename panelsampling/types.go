// Package panelsampling validates per-eye panel sampling at the boundaries of
// distortion, chromatic-aberration, and reprojection paths.
//
// Coordinates are normalized to [0, 1]. Pixel centers correspond to 0 and 1;
// a coordinate outside that interval is an out-of-panel sample. Color values
// are premultiplied RGBA so transparent regions filter without adding color.
package panelsampling

import (
	"errors"
	"fmt"
	"math"
)

// Eye identifies the panel being sampled. Textures are never shared or
// wrapped across eyes.
type Eye uint8

const (
	EyeLeft Eye = iota
	EyeRight
)

func (e Eye) String() string {
	switch e {
	case EyeLeft:
		return "left"
	case EyeRight:
		return "right"
	default:
		return fmt.Sprintf("eye(%d)", e)
	}
}

func (e Eye) valid() bool {
	return e == EyeLeft || e == EyeRight
}

// Policy is the action taken when a source coordinate leaves the panel.
type Policy uint8

const (
	// PolicyClamp uses the nearest edge texels.
	PolicyClamp Policy = iota
	// PolicyRepeat wraps within the same eye's texture. It never indexes a
	// different eye's texture.
	PolicyRepeat
	// PolicyTransparentBlack returns premultiplied transparent black and no
	// depth. Samples using this fallback are invalid for depth writes.
	PolicyTransparentBlack
)

func (p Policy) String() string {
	switch p {
	case PolicyClamp:
		return "clamp"
	case PolicyRepeat:
		return "repeat"
	case PolicyTransparentBlack:
		return "transparent_black"
	default:
		return fmt.Sprintf("policy(%d)", p)
	}
}

func (p Policy) valid() bool {
	return p == PolicyClamp || p == PolicyRepeat || p == PolicyTransparentBlack
}

// Stage identifies the operation that produced a source coordinate. It is
// retained in the out-of-bounds record for diagnostics.
type Stage uint8

const (
	StageDirect Stage = iota
	StageDistortion
	StageChromaticAberration
	StageReprojection
)

func (s Stage) String() string {
	switch s {
	case StageDirect:
		return "direct"
	case StageDistortion:
		return "distortion"
	case StageChromaticAberration:
		return "chromatic_aberration"
	case StageReprojection:
		return "reprojection"
	default:
		return fmt.Sprintf("stage(%d)", s)
	}
}

// Channel identifies the color channel being sampled.
type Channel uint8

const (
	ChannelRGB Channel = iota
	ChannelR
	ChannelG
	ChannelB
)

func (c Channel) String() string {
	switch c {
	case ChannelRGB:
		return "rgb"
	case ChannelR:
		return "r"
	case ChannelG:
		return "g"
	case ChannelB:
		return "b"
	default:
		return fmt.Sprintf("channel(%d)", c)
	}
}

func (c Channel) valid() bool {
	return c == ChannelRGB || c == ChannelR || c == ChannelG || c == ChannelB
}

// Coord is a normalized panel coordinate.
type Coord struct {
	X float64
	Y float64
}

func (c Coord) finite() bool {
	return !math.IsNaN(c.X) && !math.IsInf(c.X, 0) &&
		!math.IsNaN(c.Y) && !math.IsInf(c.Y, 0)
}

// Color is premultiplied RGBA in [0, 1].
type Color struct {
	R float64
	G float64
	B float64
	A float64
}

// Transparent is the premultiplied transparent-black fallback.
var Transparent = Color{}

// Texel contains color and optional depth. A missing depth is deliberate: it
// prevents transparent or otherwise invalid regions from synthesizing depth.
type Texel struct {
	Color    Color
	Depth    float32
	HasDepth bool
}

// Texture is a single eye's panel. It intentionally has no link to the other
// eye, so repeat wrapping cannot escape this texture.
type Texture struct {
	eye    Eye
	width  int
	height int
	texels []Texel
}

// NewTexture validates and copies a single-eye texture.
func NewTexture(eye Eye, width, height int, texels []Texel) (*Texture, error) {
	if !eye.valid() {
		return nil, fmt.Errorf("panelsampling: invalid eye %d", eye)
	}
	if width <= 0 || height <= 0 {
		return nil, errors.New("panelsampling: texture dimensions must be positive")
	}
	if width > math.MaxInt/height {
		return nil, errors.New("panelsampling: texture dimensions overflow")
	}
	if len(texels) != width*height {
		return nil, fmt.Errorf("panelsampling: got %d texels, want %d", len(texels), width*height)
	}

	copied := make([]Texel, len(texels))
	for i, texel := range texels {
		if err := validateTexel(texel); err != nil {
			return nil, fmt.Errorf("panelsampling: texel %d: %w", i, err)
		}
		copied[i] = texel
	}

	return &Texture{
		eye:    eye,
		width:  width,
		height: height,
		texels: copied,
	}, nil
}

// MustTexture is a test and fixture helper for panicking on invalid textures.
func MustTexture(eye Eye, width, height int, texels []Texel) *Texture {
	texture, err := NewTexture(eye, width, height, texels)
	if err != nil {
		panic(err)
	}
	return texture
}

func validateTexel(texel Texel) error {
	components := [...]float64{texel.Color.R, texel.Color.G, texel.Color.B, texel.Color.A}
	for _, component := range components {
		if math.IsNaN(component) || math.IsInf(component, 0) ||
			component < 0 || component > 1 {
			return fmt.Errorf("color component %v is outside [0, 1]", component)
		}
	}
	if texel.Color.R > texel.Color.A ||
		texel.Color.G > texel.Color.A ||
		texel.Color.B > texel.Color.A {
		return errors.New("color is not premultiplied")
	}
	if texel.HasDepth &&
		(math.IsNaN(float64(texel.Depth)) || math.IsInf(float64(texel.Depth), 0) || texel.Depth < 0) {
		return fmt.Errorf("invalid depth %v", texel.Depth)
	}
	return nil
}

// Eye returns the texture's eye.
func (t *Texture) Eye() Eye {
	if t == nil {
		return Eye(255)
	}
	return t.eye
}

// Width returns the texture width in texels.
func (t *Texture) Width() int {
	if t == nil {
		return 0
	}
	return t.width
}

// Height returns the texture height in texels.
func (t *Texture) Height() int {
	if t == nil {
		return 0
	}
	return t.height
}

// TexelAt returns a copy of a texel. Coordinates must already be in bounds.
func (t *Texture) TexelAt(x, y int) (Texel, bool) {
	if t == nil || x < 0 || y < 0 || x >= t.width || y >= t.height {
		return Texel{}, false
	}
	return t.texels[y*t.width+x], true
}
