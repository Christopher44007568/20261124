package warp

import (
	"timewarp/geom"
)

// RotationWarp maps each target ray through the relative rotation and samples
// the previous color frame. Translation is ignored, and no depth is read or
// written. Rays leaving the source frame are rejected, never clamped.
func RotationWarp(src *Source, in geom.Intrinsics, from, to geom.Pose) *Result {
	out := newResult(in.W, in.H)
	rel := rotationDelta(from, to)
	out.Stats.Evaluated = in.W * in.H
	for y := 0; y < in.H; y++ {
		for x := 0; x < in.W; x++ {
			i := y*in.W + x
			u, v := PixelCoord(x, y)
			dir := rel.MulVec(in.CamRay(u, v))
			if dir.Z <= 1e-9 {
				out.Reject[i] = RejectBehindCamera
				out.Stats.BehindCamera++
				continue
			}
			su := in.FX*dir.X/dir.Z + in.CX
			sv := in.CY - in.FY*dir.Y/dir.Z
			if !in.InImage(su, sv) {
				out.Reject[i] = RejectOutOfBounds
				out.Stats.OutOfBounds++
				continue
			}
			c, ok := sampleBilinear(src.Pix, src.W, src.H, su, sv)
			if !ok {
				out.Reject[i] = RejectOutOfBounds
				out.Stats.OutOfBounds++
				continue
			}
			out.Pix[i] = c
			out.Stats.Valid++
		}
	}
	out.Stats.Rejected = out.Stats.Evaluated - out.Stats.Valid
	return out
}
