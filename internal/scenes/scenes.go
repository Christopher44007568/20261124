// Package scenes builds deterministic synthetic sequences. Each scene isolates
// a different failure mode: high-frequency detail, chroma edges, and temporal
// motion.
package scenes

import (
	"fmt"
	"math"

	"fidelitycheck/internal/imaging"
)

type Kind string

const (
	FrequencyGradient Kind = "frequency-gradient"
	ColoredEdges      Kind = "colored-edges"
	SlowMotion        Kind = "slow-motion"
)

var AllKinds = []Kind{FrequencyGradient, ColoredEdges, SlowMotion}

func ParseKind(s string) (Kind, error) {
	for _, kind := range AllKinds {
		if string(kind) == s {
			return kind, nil
		}
	}
	return "", fmt.Errorf("unknown scene %q (want %s, %s, or %s)", s, FrequencyGradient, ColoredEdges, SlowMotion)
}

type Spec struct {
	Kind       Kind
	Width      int
	Height     int
	MaskRadius float64
	Frames     int
}

type Sequence struct {
	Name      string
	Kind      Kind
	Frames    []*imaging.Frame
	Mask      *imaging.Mask
	Center    *imaging.Mask
	Periphery *imaging.Mask
}

func Build(spec Spec) (*Sequence, error) {
	if spec.Width < 16 || spec.Height < 16 {
		return nil, fmt.Errorf("scene resolution %dx%d is too small", spec.Width, spec.Height)
	}
	if spec.MaskRadius < 2 {
		return nil, fmt.Errorf("mask radius %.2f is too small", spec.MaskRadius)
	}
	if spec.Frames < 1 {
		return nil, fmt.Errorf("frame count must be positive")
	}
	if spec.Frames < 3 && spec.Kind == SlowMotion {
		return nil, fmt.Errorf("slow-motion scene needs at least 3 frames")
	}

	mask := imaging.CircleMask(spec.Width, spec.Height, spec.MaskRadius)
	center := imaging.Intersect(imaging.Disk(
		spec.Width,
		spec.Height,
		math.Max(2, spec.MaskRadius*0.55),
	), mask)
	periphery := imaging.Intersect(imaging.Ring(
		spec.Width,
		spec.Height,
		spec.MaskRadius*0.65,
		spec.MaskRadius,
	), mask)
	s := &Sequence{
		Name:      string(spec.Kind),
		Kind:      spec.Kind,
		Mask:      mask,
		Center:    center,
		Periphery: periphery,
	}

	for i := 0; i < spec.Frames; i++ {
		var frame *imaging.Frame
		switch spec.Kind {
		case FrequencyGradient:
			frame = frequencyFrame(spec.Width, spec.Height, i)
		case ColoredEdges:
			frame = edgesFrame(spec.Width, spec.Height, i)
		case SlowMotion:
			frame = motionFrame(spec.Width, spec.Height, i, spec.Frames)
		default:
			return nil, fmt.Errorf("unsupported scene %q", spec.Kind)
		}
		s.Frames = append(s.Frames, frame)
	}
	return s, nil
}

// Corrupted returns the observable input sequence. Missing pixels are filled
// with mid-gray rather than black so reconstructors cannot exploit an
// unintended zero-value shortcut.
func (s *Sequence) Corrupted() []*imaging.Frame {
	out := make([]*imaging.Frame, len(s.Frames))
	for i, frame := range s.Frames {
		c := frame.Clone()
		s.Mask.Apply(c, imaging.RGB{R: 0.5, G: 0.5, B: 0.5})
		out[i] = c
	}
	return out
}

func frequencyFrame(w, h, frame int) *imaging.Frame {
	f := imaging.NewFrame(w, h)
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	maxR := math.Hypot(cx, cy)
	phaseShift := 0.015 * float64(frame)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			r := math.Hypot(dx, dy)
			// The instantaneous radial frequency falls from 0.21 cycles per
			// pixel at the center to 0.018 at the corners.
			const f0, f1 = 0.21, 0.018
			phase := 2 * math.Pi * (f0*r - (f0-f1)*r*r/(2*maxR))
			phase += phaseShift
			v := 0.50 + 0.43*math.Cos(phase)
			angle := math.Atan2(dy, dx)
			tint := 0.04 * math.Cos(angle*2)
			f.Set(x, y, imaging.RGB{
				R: v + tint,
				G: v,
				B: v - tint,
			})
		}
	}
	return f
}

var edgePalette = []imaging.RGB{
	{R: 0.96, G: 0.10, B: 0.12},
	{R: 0.08, G: 0.82, B: 0.22},
	{R: 0.09, G: 0.25, B: 0.96},
	{R: 0.98, G: 0.82, B: 0.06},
	{R: 0.90, G: 0.12, B: 0.78},
	{R: 0.04, G: 0.82, B: 0.84},
	{R: 0.98, G: 0.42, B: 0.04},
	{R: 0.12, G: 0.12, B: 0.16},
}

func edgesFrame(w, h, frame int) *imaging.Frame {
	f := imaging.NewFrame(w, h)
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	band := math.Max(12, math.Min(float64(w), float64(h))/8)
	angleShift := 0.002 * float64(frame)
	phaseShift := 0.35 * float64(frame)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			angle := math.Atan2(dy, dx) + math.Pi + angleShift
			sector := int(angle/(math.Pi/4)) % len(edgePalette)
			ring := int(math.Floor((math.Hypot(dx, dy)+phaseShift)/band)) % 2
			c := edgePalette[sector]
			if ring == 1 {
				c = edgePalette[(sector+3)%len(edgePalette)]
				c = imaging.RGB{
					R: 0.25 + 0.65*c.R,
					G: 0.25 + 0.65*c.G,
					B: 0.25 + 0.65*c.B,
				}
			}
			shade := 0.92 + 0.08*math.Cos((float64(x)-cx)/(float64(w)*0.7))
			f.Set(x, y, c.Scale(shade))
		}
	}
	return f
}

func motionFrame(w, h, frame, frames int) *imaging.Frame {
	f := imaging.NewFrame(w, h)
	cx := float64(w-1) / 2
	cy := float64(h-1) / 2
	minDim := math.Min(float64(w), float64(h))
	targetR := math.Max(8, minDim*0.095)
	const speed = 2.0
	tx := cx + (float64(frame)-float64(frames-1)/2)*speed
	ty := cy + 1.5*math.Sin(float64(frame)*0.45)

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u := (float64(x) - cx) / float64(w)
			v := (float64(y) - cy) / float64(h)
			base := 0.30 + 0.10*math.Sin(2*math.Pi*(2.2*u+0.7*v))
			base += 0.05 * math.Cos(2*math.Pi*(0.8*u-1.3*v))

			dx := float64(x) - tx
			dy := float64(y) - ty
			d := math.Hypot(dx, dy)
			if d <= targetR {
				stripe := int(math.Floor((float64(x)-tx+targetR)/4)) % 2
				var c imaging.RGB
				if stripe == 0 {
					c = imaging.RGB{R: 0.97, G: 0.86, B: 0.12}
				} else {
					c = imaging.RGB{R: 0.86, G: 0.08, B: 0.12}
				}
				// Round off the disk boundary to avoid a binary alpha edge.
				blend := math.Min(1, (targetR-d)/1.5)
				bg := imaging.RGB{R: base, G: base, B: base + 0.02}
				f.Set(x, y, bg.Add(c.Sub(bg).Scale(blend)))
				continue
			}
			f.Set(x, y, imaging.RGB{R: base, G: base, B: base + 0.02})
		}
	}
	return f
}
