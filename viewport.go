package depthmip

import (
	"fmt"
	"math"
)

// Viewport maps the normalized view rect [0,1]x[0,1] onto base-level texels
// via offset + scale. Verification only covers mip cells whose footprint
// intersects this rect, so a scaled-down viewport never validates texels it
// does not display.
type Viewport struct {
	OX, OY float64 // offset in base texels
	SX, SY float64 // scale in base texels per normalized unit
}

// FullViewport returns the viewport covering the whole buffer.
func FullViewport(w, h int) Viewport {
	return Viewport{OX: 0, OY: 0, SX: float64(w), SY: float64(h)}
}

// Validate rejects non-finite or non-positive viewport parameters instead of
// silently clamping them.
func (v Viewport) Validate() error {
	for name, f := range map[string]float64{"OX": v.OX, "OY": v.OY, "SX": v.SX, "SY": v.SY} {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return fmt.Errorf("viewport %s must be finite, got %g", name, f)
		}
	}
	if v.SX <= 0 || v.SY <= 0 {
		return fmt.Errorf("viewport scale must be positive, got %gx%g", v.SX, v.SY)
	}
	return nil
}

// texelRect is a half-open integer rect [X0,X1) x [Y0,Y1).
type texelRect struct{ X0, Y0, X1, Y1 int }

func (r texelRect) empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }

// rectAtLevel returns the viewport rect in level-L texel coordinates,
// clipped to a level of size w x h. Scale and offset shrink by 2^L per level.
func (v Viewport) rectAtLevel(level, w, h int) texelRect {
	s := float64(int(1) << level)
	floor := func(f float64) int { return int(math.Floor(f)) }
	ceil := func(f float64) int { return int(math.Ceil(f)) }
	r := texelRect{
		X0: max(floor(v.OX/s), 0),
		Y0: max(floor(v.OY/s), 0),
		X1: min(ceil((v.OX+v.SX)/s), w),
		Y1: min(ceil((v.OY+v.SY)/s), h),
	}
	return r
}
