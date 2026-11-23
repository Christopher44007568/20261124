// Package depthmip implements reversed-Z depth buffer mip reduction with
// strict validation: unknown reduction directions, NaN/Inf depths and empty
// layers are hard errors, never silently replaced by fallback values.
package depthmip

import (
	"errors"
	"fmt"
	"math"
)

// Direction selects which extreme of a texel block survives reduction.
// Under reversed-Z (1 = near plane, 0 = far plane) Min keeps the farthest
// texel and Max keeps the nearest texel of each block.
type Direction int

const (
	Min Direction = iota // farthest texel wins (reversed-Z far bound)
	Max                  // nearest texel wins (reversed-Z near bound)
)

func (d Direction) String() string {
	switch d {
	case Min:
		return "min"
	case Max:
		return "max"
	default:
		return fmt.Sprintf("Direction(%d)", int(d))
	}
}

// ParseDirection converts a direction name; anything else is an error so a
// typo can never silently degrade into a default reduction.
func ParseDirection(s string) (Direction, error) {
	switch s {
	case "min":
		return Min, nil
	case "max":
		return Max, nil
	default:
		return 0, fmt.Errorf("unknown reduction direction %q: want \"min\" or \"max\"", s)
	}
}

// Buffer is one mip level of a reversed-Z depth buffer, row-major.
type Buffer struct {
	W, H int
	Data []float64
}

// NewBuffer allocates a w x h level; non-positive dimensions are rejected.
func NewBuffer(w, h int) (*Buffer, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("buffer dimensions must be positive, got %dx%d", w, h)
	}
	return &Buffer{W: w, H: h, Data: make([]float64, w*h)}, nil
}

// At returns the depth at texel (x, y).
func (b *Buffer) At(x, y int) float64 { return b.Data[y*b.W+x] }

// validate rejects NaN and Inf so invalid depth can never propagate
// through a reduction as a silent fallback.
func (b *Buffer) validate() error {
	if len(b.Data) != b.W*b.H {
		return fmt.Errorf("corrupt buffer: %dx%d needs %d texels, has %d", b.W, b.H, b.W*b.H, len(b.Data))
	}
	for i, v := range b.Data {
		switch {
		case math.IsNaN(v):
			return fmt.Errorf("NaN depth at texel (%d,%d)", i%b.W, i/b.W)
		case math.IsInf(v, 0):
			return fmt.Errorf("infinite depth at texel (%d,%d)", i%b.W, i/b.W)
		}
	}
	return nil
}

// Reduce builds the next mip level. Blocks are 2x2; along odd edges the
// partial block (2x1, 1x2 or 1x1) is reduced over the texels that exist,
// so edge texels are never dropped nor duplicated.
func (b *Buffer) Reduce(dir Direction) (*Buffer, error) {
	if b == nil || b.W <= 0 || b.H <= 0 || len(b.Data) == 0 {
		return nil, errors.New("cannot reduce an empty layer")
	}
	if dir != Min && dir != Max {
		return nil, fmt.Errorf("invalid reduction direction %s", dir)
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	nw, nh := (b.W+1)/2, (b.H+1)/2
	out := &Buffer{W: nw, H: nh, Data: make([]float64, nw*nh)}
	for y := 0; y < nh; y++ {
		sy, ey := 2*y, min(2*y+2, b.H)
		for x := 0; x < nw; x++ {
			sx, ex := 2*x, min(2*x+2, b.W)
			acc := b.Data[sy*b.W+sx]
			for yy := sy; yy < ey; yy++ {
				for xx := sx; xx < ex; xx++ {
					v := b.Data[yy*b.W+xx]
					if dir == Min {
						acc = math.Min(acc, v)
					} else {
						acc = math.Max(acc, v)
					}
				}
			}
			out.Data[y*nw+x] = acc
		}
	}
	return out, nil
}

// Chain reduces b down to a 1x1 level, returning [base, mip1, ..., mipN].
func (b *Buffer) Chain(dir Direction) ([]*Buffer, error) {
	if b == nil || b.W <= 0 || b.H <= 0 || len(b.Data) == 0 {
		return nil, errors.New("cannot build a mip chain from an empty layer")
	}
	if err := b.validate(); err != nil {
		return nil, err
	}
	levels := []*Buffer{b}
	for cur := b; cur.W > 1 || cur.H > 1; {
		next, err := cur.Reduce(dir)
		if err != nil {
			return nil, fmt.Errorf("level %d: %w", len(levels), err)
		}
		levels = append(levels, next)
		cur = next
	}
	return levels, nil
}
