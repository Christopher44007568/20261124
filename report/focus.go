package report

import (
	"image"
	"image/color"
	"math"

	"timewarp/scene"
	"timewarp/warp"
)

const focusGap = 6

// RectAround returns a clamped pixel rectangle centered on u,v.
func RectAround(u, v float64, w, h, pad int, frameW, frameH int) image.Rectangle {
	w += pad * 2
	h += pad * 2
	x0 := int(math.Round(u)) - w/2
	y0 := int(math.Round(v)) - h/2
	return clampRect(image.Rect(x0, y0, x0+w, y0+h), frameW, frameH)
}

func ExpandRect(r image.Rectangle, pad, frameW, frameH int) image.Rectangle {
	r = r.Inset(-pad)
	return clampRect(r, frameW, frameH)
}

func clampRect(r image.Rectangle, w, h int) image.Rectangle {
	if r.Min.X < 0 {
		r.Min.X = 0
	}
	if r.Min.Y < 0 {
		r.Min.Y = 0
	}
	if r.Max.X > w {
		r.Max.X = w
	}
	if r.Max.Y > h {
		r.Max.Y = h
	}
	if r.Dx() < 1 {
		r.Max.X = r.Min.X + 1
	}
	if r.Dy() < 1 {
		r.Max.Y = r.Min.Y + 1
	}
	return r
}

type regionComponent struct {
	bounds  image.Rectangle
	total   int
	covered int
}

func regionComponents(f *scene.Frame, want scene.Region, coveredMask []bool) []regionComponent {
	seen := make([]bool, len(f.Region))
	var out []regionComponent
	stack := make([]int, 0, 1024)
	for start, reg := range f.Region {
		if reg != want || seen[start] {
			continue
		}
		seen[start] = true
		stack = append(stack[:0], start)
		minX, minY := start%f.W, start/f.W
		maxX, maxY := minX, minY
		n := 0
		covered := 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := i%f.W, i/f.W
			n++
			if len(coveredMask) == len(f.Region) && coveredMask[i] {
				covered++
			}
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
			if x > 0 {
				if j := i - 1; !seen[j] && f.Region[j] == want {
					seen[j] = true
					stack = append(stack, j)
				}
			}
			if x+1 < f.W {
				if j := i + 1; !seen[j] && f.Region[j] == want {
					seen[j] = true
					stack = append(stack, j)
				}
			}
			if y > 0 {
				if j := i - f.W; !seen[j] && f.Region[j] == want {
					seen[j] = true
					stack = append(stack, j)
				}
			}
			if y+1 < f.H {
				if j := i + f.W; !seen[j] && f.Region[j] == want {
					seen[j] = true
					stack = append(stack, j)
				}
			}
		}
		out = append(out, regionComponent{
			bounds: image.Rect(minX, minY, maxX+1, maxY+1),
			total:  n, covered: covered,
		})
	}
	return out
}

// LargestRegionBounds returns the bounding box of the largest 4-connected
// component of a semantic region.
func LargestRegionBounds(f *scene.Frame, want scene.Region) image.Rectangle {
	best := image.Rectangle{}
	bestN := 0
	for _, c := range regionComponents(f, want, nil) {
		if c.total > bestN {
			bestN = c.total
			best = c.bounds
		}
	}
	return best
}

// CoveredRegionBounds prefers a semantic component that is visible in both
// warps. It keeps a clipped floor glint from being selected as the specular
// focus when a fully reprojected wall glint is available.
func CoveredRegionBounds(f *scene.Frame, rot, depth *warp.Result, want scene.Region) image.Rectangle {
	mask := make([]bool, len(f.Region))
	for i := range mask {
		if i < len(rot.Reject) && i < len(depth.Reject) {
			mask[i] = rot.Reject[i] == warp.RejectNone && depth.Reject[i] == warp.RejectNone
		}
	}
	best := image.Rectangle{}
	bestCovered, bestTotal := 0, 0
	for _, c := range regionComponents(f, want, mask) {
		if c.covered > bestCovered || (c.covered == bestCovered && c.total > bestTotal) {
			bestCovered, bestTotal = c.covered, c.total
			best = c.bounds
		}
	}
	if bestCovered == 0 {
		return LargestRegionBounds(f, want)
	}
	return best
}

// FocusStrip places GT, rotation, and depth crops side by side without
// smoothing, so one-pixel sampling errors remain visible.
func FocusStrip(gt, rot, depth *image.RGBA, r image.Rectangle, scale int) *image.RGBA {
	if scale < 1 {
		scale = 1
	}
	tw, th := r.Dx()*scale, r.Dy()*scale
	w := tw*3 + focusGap*2 + 4
	h := th + 4
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	fill(out, color.RGBA{R: 7, G: 9, B: 14, A: 255})
	copyScaled(out, gt, r, 2, 2, scale)
	copyScaled(out, rot, r, 2+tw+focusGap, 2, scale)
	copyScaled(out, depth, r, 2+(tw+focusGap)*2, 2, scale)
	return out
}

func copyScaled(dst *image.RGBA, src *image.RGBA, r image.Rectangle, ox, oy, scale int) {
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			sx, sy := r.Min.X+x, r.Min.Y+y
			c := src.RGBAAt(sx, sy)
			for yy := 0; yy < scale; yy++ {
				for xx := 0; xx < scale; xx++ {
					dst.SetRGBA(ox+x*scale+xx, oy+y*scale+yy, c)
				}
			}
		}
	}
}

func fill(img *image.RGBA, c color.RGBA) {
	for y := 0; y < img.Rect.Dy(); y++ {
		for x := 0; x < img.Rect.Dx(); x++ {
			img.SetRGBA(x, y, c)
		}
	}
}
