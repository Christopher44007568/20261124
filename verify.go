package depthmip

import (
	"fmt"
	"math"
)

// CellError pinpoints the first mip cell whose reduced value disagrees with
// the bound recomputed from the base-level footprint.
type CellError struct {
	Level     int       // mip level where the error was found (1 = first reduced level)
	X, Y      int       // cell coordinates within that level
	Dir       Direction // reduction path being verified
	Got       float64   // value stored in the chain
	Want      float64   // bound recomputed from the base-level footprint
	Footprint [4]int    // base-level texel rect [x0,y0,x1,y1) the cell covers
}

func (e *CellError) Error() string {
	return fmt.Sprintf("level %d cell (%d,%d): %s reduction = %g, want %g from base footprint [%d,%d)-[%d,%d)",
		e.Level, e.X, e.Y, e.Dir, e.Got, e.Want,
		e.Footprint[0], e.Footprint[1], e.Footprint[2], e.Footprint[3])
}

// footprintOf maps a level-L cell to its half-open base-level texel rect.
func footprintOf(base *Buffer, level, x, y int) [4]int {
	s := 1 << level
	return [4]int{
		x * s,
		y * s,
		min((x+1)*s, base.W),
		min((y+1)*s, base.H),
	}
}

// boundOver recomputes the min/max bound directly from base texels.
func boundOver(base *Buffer, fp [4]int, dir Direction) float64 {
	acc := base.At(fp[0], fp[1])
	for y := fp[1]; y < fp[3]; y++ {
		for x := fp[0]; x < fp[2]; x++ {
			v := base.At(x, y)
			if dir == Min {
				acc = math.Min(acc, v)
			} else {
				acc = math.Max(acc, v)
			}
		}
	}
	return acc
}

// VerifyChain checks every level of a mip chain against bounds recomputed
// from the base buffer, restricted to the cells visible through vp. It
// returns a *CellError describing the first mismatch (scanning levels from
// coarse coverage of the base upward, row-major within a level), or nil.
func VerifyChain(base *Buffer, levels []*Buffer, dir Direction, vp Viewport) error {
	if dir != Min && dir != Max {
		return fmt.Errorf("invalid reduction direction %s", dir)
	}
	if err := vp.Validate(); err != nil {
		return err
	}
	if len(levels) == 0 || levels[0] != base {
		return fmt.Errorf("chain must start at the supplied base buffer")
	}
	if err := base.validate(); err != nil {
		return err
	}
	for level := 1; level < len(levels); level++ {
		lvl := levels[level]
		if lvl == nil || lvl.W <= 0 || lvl.H <= 0 || len(lvl.Data) == 0 {
			return fmt.Errorf("level %d: empty layer in chain", level)
		}
		ew, eh := (base.W+(1<<level)-1)>>level, (base.H+(1<<level)-1)>>level
		if lvl.W != ew || lvl.H != eh {
			return fmt.Errorf("level %d: dimensions %dx%d, want %dx%d", level, lvl.W, lvl.H, ew, eh)
		}
		if err := lvl.validate(); err != nil {
			return fmt.Errorf("level %d: %w", level, err)
		}
		clip := vp.rectAtLevel(level, lvl.W, lvl.H)
		for y := clip.Y0; y < clip.Y1; y++ {
			for x := clip.X0; x < clip.X1; x++ {
				fp := footprintOf(base, level, x, y)
				want := boundOver(base, fp, dir)
				if got := lvl.At(x, y); got != want {
					return &CellError{Level: level, X: x, Y: y, Dir: dir, Got: got, Want: want, Footprint: fp}
				}
			}
		}
	}
	return nil
}
