package stereoholes

import (
	"image"
	"math"
)

type HoleGroup struct {
	Name string
	Rect image.Rectangle
	Mask []bool
}

type HoleSet struct {
	Mask   []bool
	Groups []HoleGroup
	Width  int
	Height int
}

func (h *HoleSet) Group(name string) *HoleGroup {
	for i := range h.Groups {
		if h.Groups[i].Name == name {
			return &h.Groups[i]
		}
	}
	return nil
}

func centeredRect(b image.Rectangle, w, h int) image.Rectangle {
	w = min(w, b.Dx())
	h = min(h, b.Dy())
	cx, cy := b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2
	return image.Rect(cx-w/2, cy-h/2, cx+(w+1)/2, cy+(h+1)/2)
}

// DemoHoles places holes on a shared surface, an asymmetric overlap, a wall
// corner, a one-eye highlight, and a transparent sheet.
func DemoHoles(target *Frame) *HoleSet {
	n := target.Width * target.Height
	set := &HoleSet{Mask: make([]bool, n), Width: target.Width, Height: target.Height}
	addRect := func(name string, r image.Rectangle) {
		r = r.Intersect(image.Rect(0, 0, target.Width, target.Height))
		if r.Empty() {
			return
		}
		g := HoleGroup{Name: name, Rect: r, Mask: make([]bool, n)}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				i := y*target.Width + x
				g.Mask[i] = true
				set.Mask[i] = true
			}
		}
		set.Groups = append(set.Groups, g)
	}
	addCentered := func(name, category string, w, h int) {
		if b, ok := target.Bounds(category); ok {
			addRect(name, centeredRect(b, w, h))
		}
	}

	if b, ok := target.Bounds("common"); ok {
		cy := b.Min.Y + b.Dy()/2
		addRect("shared-surface", image.Rect(b.Min.X+2, cy-18, b.Min.X+18, cy+18))
		addRect("asymmetric-mask", image.Rect(b.Min.X+37, cy-18, b.Max.X, cy+18))
	}
	if b, ok := target.Bounds("wall-corner-one-eye"); ok {
		cy := b.Min.Y + b.Dy()/2
		addRect("wall-corner", image.Rect(b.Min.X-2, cy-32, b.Max.X+2, cy+32))
	}

	if b, ok := target.Bounds("specular-edge"); ok {
		cy := b.Min.Y + b.Dy()/2
		addRect("specular-only", image.Rect(b.Min.X-2, cy-42, b.Max.X+2, cy+42))
	}
	addCentered("transparent-only", "transparent-sheet", 64, 68)
	return set
}

// MeanDisparity measures the average horizontal source shift for visible
// target samples, excluding holes.
func MeanDisparity(target, source *Frame, holes []bool) float64 {
	sum, count := 0.0, 0
	for y := 0; y < target.Height; y += 4 {
		for x := 0; x < target.Width; x += 4 {
			i := target.idx(x, y)
			if holes[i] || target.Depth[i] <= 0 {
				continue
			}
			world := target.Camera.Unproject(float64(x)+0.5, float64(y)+0.5, target.Depth[i])
			sx, _, _, ok := source.Camera.Project(world)
			if !ok {
				continue
			}
			sum += math.Abs(float64(x) + 0.5 - sx)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return sum / float64(count)
}
