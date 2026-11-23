package warp

import (
	"math"

	"timewarp/geom"
)

type depthTexel struct {
	z      float64
	srcU   float64
	srcV   float64
	srcIdx int
}

// DepthWarp forward-projects every source pixel with valid depth, rasterizes
// its projected footprint into a target z-buffer, then samples only from the
// source frame for covered target pixels. Pixels without coverage are holes:
// they are not filled with invented depth or clamped border color.
func DepthWarp(src *Source, in geom.Intrinsics, from, to geom.Pose) *Result {
	out := newResult(in.W, in.H)
	out.Stats.Evaluated = in.W * in.H
	tex := make([]depthTexel, len(out.Pix))
	for i := range tex {
		tex[i].z = math.Inf(1)
	}

	for y := 0; y < src.H; y++ {
		for x := 0; x < src.W; x++ {
			si := y*src.W + x
			if !src.DepthValid[si] || src.Depth[si] <= 0 {
				out.Stats.SourceNoDepth++
				continue
			}
			depth := src.Depth[si]

			cu, cv, cz, centerOK := projectPoint(from, to, float64(x), float64(y), depth, in)
			if !centerOK || cu < 0 || cv < 0 || cu > float64(in.W-1) || cv > float64(in.H-1) {
				out.Stats.SourceOffscreen++
			}

			corners := [4][3]float64{
				{float64(x) - 0.5, float64(y) - 0.5, depth},
				{float64(x) + 0.5, float64(y) - 0.5, depth},
				{float64(x) + 0.5, float64(y) + 0.5, depth},
				{float64(x) - 0.5, float64(y) + 0.5, depth},
			}
			var px, py, pz [4]float64
			allBehind := true
			for k := range corners {
				u, v, z, ok := projectPoint(from, to, corners[k][0], corners[k][1], corners[k][2], in)
				px[k], py[k], pz[k] = u, v, z
				if ok {
					allBehind = false
				}
			}
			if allBehind {
				continue
			}
			validCorners := true
			for k := 0; k < 4; k++ {
				if pz[k] <= 1e-6 {
					validCorners = false
					break
				}
			}
			if !validCorners {
				continue
			}
			if allOutside(px, py, in.W, in.H) {
				continue
			}
			if centerOK && cz > 1e-6 {
				minX, maxX, minY, maxY := bounds(px, py)
				spacing := math.Max(maxX-minX, maxY-minY)
				radius := math.Min(2.75, math.Max(0.72, 0.72*spacing))
				splatCenter(&tex, in.W, in.H, cu, cv, cz, float64(x), float64(y), si, radius)
			}

			// Two triangles form the projected pixel footprint. The source
			// corner coordinates are interpolated perspective-correctly.
			rasterTriangle(out, in, tex,
				px[0], py[0], pz[0], corners[0][0], corners[0][1], si,
				px[1], py[1], pz[1], corners[1][0], corners[1][1],
				px[2], py[2], pz[2], corners[2][0], corners[2][1])
			rasterTriangle(out, in, tex,
				px[0], py[0], pz[0], corners[0][0], corners[0][1], si,
				px[2], py[2], pz[2], corners[2][0], corners[2][1],
				px[3], py[3], pz[3], corners[3][0], corners[3][1])
		}
	}

	sealOnePixelCracks(&tex, in.W, in.H)

	for y := 0; y < in.H; y++ {
		for x := 0; x < in.W; x++ {
			i := y*in.W + x
			t := tex[i]
			if math.IsInf(t.z, 1) {
				out.Reject[i] = RejectNoCoverage
				out.Stats.NoCoverage++
				continue
			}
			c, ok := sampleBilinear(src.Pix, src.W, src.H, t.srcU, t.srcV)
			if !ok {
				// Footprints at the source border can overhang by half a
				// pixel. Fall back to the source texel that produced the
				// fragment; never wrap or clamp to an off-screen address.
				c = src.Pix[t.srcIdx]
			}
			out.Pix[i] = c
			out.Depth[i] = t.z
			out.DepthValid[i] = true
			out.Stats.Valid++
		}
	}
	out.Stats.Rejected = out.Stats.Evaluated - out.Stats.Valid
	return out
}

func bounds(px, py [4]float64) (minX, maxX, minY, maxY float64) {
	minX, maxX = px[0], px[0]
	minY, maxY = py[0], py[0]
	for k := 1; k < 4; k++ {
		minX = math.Min(minX, px[k])
		maxX = math.Max(maxX, px[k])
		minY = math.Min(minY, py[k])
		maxY = math.Max(maxY, py[k])
	}
	return minX, maxX, minY, maxY
}

func splatCenter(tex *[]depthTexel, w, h int, cu, cv, z, su, sv float64, srcIdx int, radius float64) {
	minX := int(math.Floor(cu - radius))
	maxX := int(math.Ceil(cu + radius))
	minY := int(math.Floor(cv - radius))
	maxY := int(math.Ceil(cv + radius))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > w-1 {
		maxX = w - 1
	}
	if maxY > h-1 {
		maxY = h - 1
	}
	r2 := radius * radius
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx, dy := float64(x)-cu, float64(y)-cv
			if dx*dx+dy*dy > r2 {
				continue
			}
			i := y*w + x
			if z < (*tex)[i].z-1e-6 {
				(*tex)[i] = depthTexel{z: z, srcU: su, srcV: sv, srcIdx: srcIdx}
			}
		}
	}
}

func sealOnePixelCracks(tex *[]depthTexel, w, h int) {
	// A splat seam is a one-pixel hole with at least two covered cardinal
	// neighbours. Reuse one covered source texel rather than interpolating a
	// new color or fetching a border pixel; real disocclusions stay holes.
	src := *tex
	dst := append([]depthTexel(nil), src...)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			if !math.IsInf(src[i].z, 1) {
				continue
			}
			neighbors := [4]int{i - 1, i + 1, i - w, i + w}
			count := 0
			best := depthTexel{z: math.Inf(1)}
			for _, ni := range neighbors {
				n := src[ni]
				if math.IsInf(n.z, 1) {
					continue
				}
				count++
				if n.z < best.z {
					best = n
				}
			}
			if count < 2 {
				continue
			}
			dst[i] = best
		}
	}
	*tex = dst
}

func projectPoint(from, to geom.Pose, u, v, depth float64, in geom.Intrinsics) (float64, float64, float64, bool) {
	p := in.Unproject(u, v, depth)
	q := to.WorldToCam(from.CamToWorld(p))
	return in.Project(q)
}

func allOutside(px, py [4]float64, w, h int) bool {
	minX, maxX, minY, maxY := bounds(px, py)
	return maxX < 0 || maxY < 0 || minX > float64(w-1) || minY > float64(h-1)
}

func rasterTriangle(out *Result, in geom.Intrinsics, tex []depthTexel,
	x0, y0, z0, u0, v0 float64, si0 int,
	x1, y1, z1, u1, v1 float64,
	x2, y2, z2, u2, v2 float64) {
	if z0 <= 1e-6 || z1 <= 1e-6 || z2 <= 1e-6 {
		return
	}
	den := (y1-y2)*(x0-x2) + (x2-x1)*(y0-y2)
	if math.Abs(den) < 1e-12 {
		return
	}
	minX := int(math.Floor(math.Min(x0, math.Min(x1, x2))))
	maxX := int(math.Ceil(math.Max(x0, math.Max(x1, x2))))
	minY := int(math.Floor(math.Min(y0, math.Min(y1, y2))))
	maxY := int(math.Ceil(math.Max(y0, math.Max(y1, y2))))
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > in.W-1 {
		maxX = in.W - 1
	}
	if maxY > in.H-1 {
		maxY = in.H - 1
	}
	const eps = 1e-7
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x), float64(y)
			b0 := ((y1-y2)*(px-x2) + (x2-x1)*(py-y2)) / den
			b1 := ((y2-y0)*(px-x2) + (x0-x2)*(py-y2)) / den
			b2 := 1 - b0 - b1
			if b0 < -eps || b1 < -eps || b2 < -eps {
				continue
			}
			q0, q1, q2 := b0/z0, b1/z1, b2/z2
			q := q0 + q1 + q2
			if q <= 1e-12 {
				continue
			}
			z := 1 / q
			su := (q0*u0 + q1*u1 + q2*u2) / q
			sv := (q0*v0 + q1*v1 + q2*v2) / q
			i := y*out.W + x
			if z < tex[i].z-1e-6 {
				tex[i] = depthTexel{z: z, srcU: su, srcV: sv, srcIdx: si0}
			}
		}
	}
}
