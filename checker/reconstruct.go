package checker

import "math"

// Reconstruction parameters. The edge scale is expressed in normalized luma
// units: differences well below it are treated as "flat", larger differences
// stop smoothing across an edge.
const (
	edgeScale        = 0.08
	flatSpread       = 0.015
	jitterAmplitude  = 0.004
	refinementPasses = 24
)

type acc struct {
	r, g, b, w float64
}

func (a *acc) add(c RGB, w float64) {
	a.r += c.R * w
	a.g += c.G * w
	a.b += c.B * w
	a.w += w
}

func (a *acc) color() RGB {
	if a.w <= 0 {
		return RGB{}
	}
	return RGB{R: a.r / a.w, G: a.g / a.w, B: a.b / a.w}
}

// hash01 is a deterministic integer hash in [0,1), keyed by pixel position.
// It breaks exact four-way degeneracies without any global periodicity, so
// tie-breaking cannot line up with the checkerboard lattice.
func hash01(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h) / 4294967296.0
}

func edgeWeight(du, sigma float64) float64 {
	r := du / sigma
	return math.Exp(-0.5 * r * r)
}

// Reconstruct fills every skipped peripheral pixel of a shaded frame while
// preserving shaded pixels exactly. The center region is never touched by any
// peripheral rule: BuildMask already shades every center pixel, and the solver
// only ever writes to non-shaded locations.
//
// The result is deterministic and contains no fixed checkerboard-period term:
// interpolation weights depend on local content (edge-directed blending), the
// few exact degeneracies are resolved by a per-pixel coordinate hash, and the
// refinement sweeps alternate direction each pass.
func Reconstruct(shaded *Image, mask *Mask) *Image {
	out := shaded.Clone()
	initUnknown(out, mask)
	refine(out, mask)
	return out
}

// initUnknown gives every skipped pixel a content-adaptive seed from its
// shaded neighborhood before diffusion starts.
func initUnknown(img *Image, mask *Mask) {
	mean := shadedMeanColor(img, mask)

	for y := 0; y < img.H; y++ {
		for x := 0; x < img.W; x++ {
			idx := y*img.W + x
			if mask.Shaded[idx] {
				continue
			}
			var a acc
			// A 5x5 edge-aware stencil: every shaded sample within Chebyshev
			// distance two contributes, weighted by inverse distance and by an
			// edge-stop against the local shaded mean. The wider reach lets the
			// seed cross the two-pixel unshaded gap of a 2:2 block without
			// smearing sharp boundaries.
			lsum, lcount := 0.0, 0.0
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if !mask.At(nx, ny) {
						continue
					}
					c := img.At(nx, ny)
					inv := 1.0 / math.Hypot(float64(dx), float64(dy))
					a.add(c, inv)
					lsum += Luma(c)
					lcount++
				}
			}
			localMean := 0.0
			if lcount > 0 {
				localMean = lsum / lcount
			}
			// Re-weight with an edge-stop against the local shaded mean so a
			// sample across a hard edge is down-weighted rather than averaged.
			var edge acc
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if !mask.At(nx, ny) {
						continue
					}
					c := img.At(nx, ny)
					inv := 1.0 / math.Hypot(float64(dx), float64(dy))
					w := inv * edgeWeight(math.Abs(Luma(c)-localMean), 2*edgeScale)
					edge.add(c, w)
				}
			}
			// Strong-edge handling: if the shaded neighborhood splits into two
			// compact luma clusters, keep only the samples from the dominant
			// side. Interpolating across a hard checker boundary would smear
			// two columns of pixels; clustering preserves the side identity and
			// still interpolates along the edge direction.
			if strong := dominantSide(img, mask, x, y); strong.w > 0 {
				edge = strong
			}

			seed := mean
			if edge.w > 0 {
				seed = edge.color()
			} else if a.w > 0 {
				seed = a.color()
			} else if c, ok := nearestShadedColor(img, mask, x, y); ok {
				seed = c
			}

			if lcount >= 2 {
				if math.Abs(localMean-Luma(seed)) < flatSpread {
					j := (hash01(x, y) - 0.5) * jitterAmplitude
					seed = RGB{
						R: clamp01(seed.R + j),
						G: clamp01(seed.G + j),
						B: clamp01(seed.B + j),
					}
				}
			}
			img.Px[idx] = seed
		}
	}
}

// dominantSide clusters the shaded 5x5 neighborhood into a low/high luma pair
// and returns the larger cluster when the separation is unambiguous. It
// returns a zero-weight accumulator on smooth regions, where clustering would
// be arbitrary.
func dominantSide(img *Image, mask *Mask, x, y int) acc {
	type sample struct {
		c RGB
		l float64
		w float64
	}
	var samples []sample
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if !mask.At(nx, ny) {
				continue
			}
			c := img.At(nx, ny)
			samples = append(samples, sample{
				c: c,
				l: Luma(c),
				w: 1.0 / math.Hypot(float64(dx), float64(dy)),
			})
		}
	}
	if len(samples) < 4 {
		return acc{}
	}
	lo, hi := 1.0, 0.0
	for _, s := range samples {
		lo = math.Min(lo, s.l)
		hi = math.Max(hi, s.l)
	}
	if hi-lo < 0.45 {
		return acc{}
	}
	mid := (hi + lo) / 2
	var low, high acc
	for _, s := range samples {
		if s.l < mid {
			low.add(s.c, s.w)
		} else {
			high.add(s.c, s.w)
		}
	}
	if low.w == 0 || high.w == 0 {
		return acc{}
	}
	// Decide the side with a soft, distance-weighted vote across the whole
	// stencil. A decisive majority is taken directly; an exact tie is resolved
	// by the edge direction (the closest low/high sample pair), so the result
	// is spatial interpolation rather than a nearest copy and can never align
	// with a fixed checker period.
	if high.w > low.w*1.05 {
		return high
	}
	if low.w > high.w*1.05 {
		return low
	}
	nearestLow, nearestHigh := math.Inf(1), math.Inf(1)
	for dy := -2; dy <= 2; dy++ {
		for dx := -2; dx <= 2; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			nx, ny := x+dx, y+dy
			if !mask.At(nx, ny) {
				continue
			}
			d := math.Hypot(float64(dx), float64(dy))
			if Luma(img.At(nx, ny)) < mid {
				nearestLow = math.Min(nearestLow, d)
			} else {
				nearestHigh = math.Min(nearestHigh, d)
			}
		}
	}
	if nearestHigh <= nearestLow {
		return high
	}
	return low
}

// nearestShadedColor expands rings around a pixel until a shaded neighbor is
// found. It only matters at image borders where the immediate block is absent.
func nearestShadedColor(img *Image, mask *Mask, x, y int) (RGB, bool) {
	for r := 1; r < max(img.W, img.H); r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if abs(dx) != r && abs(dy) != r {
					continue
				}
				nx, ny := x+dx, y+dy
				if mask.At(nx, ny) {
					return img.At(nx, ny), true
				}
			}
		}
	}
	return RGB{}, false
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func shadedMeanColor(img *Image, mask *Mask) RGB {
	var a acc
	for i := range img.Px {
		if mask.Shaded[i] {
			a.add(img.Px[i], 1.0)
		}
	}
	if a.w == 0 {
		return RGB{}
	}
	return a.color()
}

// refine runs edge-aware Gauss-Seidel diffusion over unknown pixels. Sweep
// direction alternates each pass so no directional bias can lock onto the
// checkerboard axes.
func refine(img *Image, mask *Mask) {
	dirs := [8][2]int{
		{1, 0}, {-1, 0}, {0, 1}, {0, -1},
		{1, 1}, {1, -1}, {-1, 1}, {-1, -1},
	}
	for pass := 0; pass < refinementPasses; pass++ {
		rev := pass&1 == 1
		for y := 0; y < img.H; y++ {
			yy := y
			if rev {
				yy = img.H - 1 - y
			}
			for x := 0; x < img.W; x++ {
				xx := x
				if rev {
					xx = img.W - 1 - x
				}
				idx := yy*img.W + xx
				if mask.Shaded[idx] {
					continue
				}
				centerLuma := Luma(img.Px[idx])
				var sum acc
				for _, d := range dirs {
					nx, ny := xx+d[0], yy+d[1]
					if nx < 0 || ny < 0 || nx >= img.W || ny >= img.H {
						continue
					}
					nc := img.Px[ny*img.W+nx]
					w := edgeWeight(Luma(nc)-centerLuma, edgeScale)
					if d[0] != 0 && d[1] != 0 {
						w *= 0.5
					}
					sum.add(nc, w)
				}
				if sum.w > 0 {
					c := sum.color()
					img.Px[idx] = RGB{
						R: clamp01(c.R),
						G: clamp01(c.G),
						B: clamp01(c.B),
					}
				}
			}
		}
	}
}
