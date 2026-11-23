package render

import (
	"math"

	"stereoval/internal/geo"
)

// WarpResult is a reprojected image with provenance masks used by the
// metrics package.
type WarpResult struct {
	Frame
	SrcPose   geo.Pose
	DstPose   geo.Pose
	PoseTag   string // identifies which pose version produced the frame
	Hole      []bool // true where no source pixel was available
	Filled    []bool // true where a hole was filled by CrossEyeFill
	FillFromE geo.Eye
	HasFill   bool
}

// ForwardWarpDepth reprojects the historical image src (captured at srcPose)
// into the current frame using inverse warping. dstGeom supplies the current
// frame's camera-space depth (the validator always renders current geometry):
// each destination pixel is unprojected to a world point, mapped into the
// source camera and bilinearly sampled. This is sub-pixel accurate. Pixels
// whose source location is off-image or requires a missing source depth are
// marked as holes (disocclusions); no inpainting happens here.
func ForwardWarpDepth(src, dstGeom *Frame, k geo.Intrinsics, srcPose, dstPose geo.Pose, poseTag string) *WarpResult {
	out := &WarpResult{
		Frame:   *newFrame(k.W, k.H),
		SrcPose: srcPose,
		DstPose: dstPose,
		PoseTag: poseTag,
		Hole:    make([]bool, k.W*k.H),
		Filled:  make([]bool, k.W*k.H),
	}
	for y := 0; y < k.H; y++ {
		for x := 0; x < k.W; x++ {
			i := out.idx(x, y)
			z := dstGeom.Depth[i]
			if math.IsInf(z, 1) {
				out.Hole[i] = true
				continue
			}
			world := k.Unproject(float64(x)+0.5, float64(y)+0.5, z, dstPose)
			cam := srcPose.ToCamera(world)
			su, sv, _, ok := k.Project(cam)
			if !ok {
				out.Hole[i] = true
				continue
			}
			g, ok := sampleBilinear(src, su, sv)
			if !ok {
				out.Hole[i] = true
				continue
			}
			out.Gray[i] = g
			out.Depth[i] = z
		}
	}
	return out
}

// sampleBilinear returns the bilinearly interpolated gray value. It fails
// when any of the four neighbors is a hole, preserving disocclusion edges.
func sampleBilinear(f *Frame, u, v float64) (float64, bool) {
	x0 := int(math.Floor(u))
	y0 := int(math.Floor(v))
	if x0 < 0 || y0 < 0 || x0+1 >= f.W || y0+1 >= f.H {
		return 0, false
	}
	tx := u - float64(x0)
	ty := v - float64(y0)
	idx := [4]int{f.idx(x0, y0), f.idx(x0+1, y0), f.idx(x0, y0+1), f.idx(x0+1, y0+1)}
	var vals [4]float64
	valid := [4]float64{1, 1, 1, 1}
	sumW := 0.0
	for j, ix := range idx {
		if math.IsNaN(f.Gray[ix]) {
			valid[j] = 0
			continue
		}
		vals[j] = f.Gray[ix]
	}
	w := [4]float64{(1 - tx) * (1 - ty), tx * (1 - ty), (1 - tx) * ty, tx * ty}
	var acc float64
	for j := range vals {
		w[j] *= valid[j]
		sumW += w[j]
		acc += vals[j] * w[j]
	}
	if sumW == 0 {
		return 0, false
	}
	return acc / sumW, true
}

// CrossEyeFill fills holes in dst from the other eye's historical image.
//
// For each hole pixel the current world point is mapped into the donor eye's
// source camera and sampled there (a different viewpoint thanks to the stereo
// baseline), which is what makes cross-eye completion geometrically possible.
// Crucially no per-eye disparity correction is applied: the donor samples the
// donor viewpoint directly and is stamped at the destination pixel, so the
// filled region carries an inconsistent disparity. That deliberate distortion
// (plus the hard depth discontinuity at the fill seam) is what the metrics
// must detect.
func CrossEyeFill(dst *WarpResult, donorSrc *Frame, donorSrcPose geo.Pose, k geo.Intrinsics, from geo.Eye) int {
	filled := 0
	for y := 0; y < k.H; y++ {
		for x := 0; x < k.W; x++ {
			i := dst.idx(x, y)
			if !dst.Hole[i] {
				continue
			}
			// Holes carry no depth; estimate the surface from the nearest
			// known neighboring destination depth.
			z := neighborDepth(dst.Depth, k.W, k.H, x, y)
			if math.IsInf(z, 1) {
				continue
			}
			world := k.Unproject(float64(x)+0.5, float64(y)+0.5, z, dst.DstPose)
			dcam := donorSrcPose.ToCamera(world)
			du, dv, _, ok := k.Project(dcam)
			if !ok {
				continue
			}
			g, ok := sampleBilinear(donorSrc, du, dv)
			if !ok {
				continue
			}
			dst.Gray[i] = g
			dst.Depth[i] = z
			dst.Filled[i] = true
			dst.Hole[i] = false
			filled++
		}
	}
	if filled > 0 {
		dst.HasFill = true
		dst.FillFromE = from
	}
	return filled
}

// neighborDepth returns the nearest finite depth around a hole pixel.
func neighborDepth(depth []float64, w, h, x, y int) float64 {
	for r := 1; r <= 4; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if dx != r && dx != -r && dy != r && dy != -r {
					continue
				}
				nx, ny := x+dx, y+dy
				if nx < 0 || ny < 0 || nx >= w || ny >= h {
					continue
				}
				if d := depth[ny*w+nx]; !math.IsInf(d, 1) {
					return d
				}
			}
		}
	}
	return math.Inf(1)
}

// HoleRatio is the fraction of pixels that were never covered by the warp.
func (w *WarpResult) HoleRatio() float64 {
	n := 0
	for _, h := range w.Hole {
		if h {
			n++
		}
	}
	return float64(n) / float64(len(w.Hole))
}

// FillRatio is the fraction of pixels produced by cross-eye fill.
func (w *WarpResult) FillRatio() float64 {
	if !w.HasFill {
		return 0
	}
	n := 0
	for _, f := range w.Filled {
		if f {
			n++
		}
	}
	return float64(n) / float64(len(w.Filled))
}
