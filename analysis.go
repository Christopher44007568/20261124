package stereoholes

import (
	"image"
	"math"
)

// Bounds returns the tight bounding box of pixels with the given category.
func (f *Frame) Bounds(category string) (image.Rectangle, bool) {
	minX, minY := f.Width, f.Height
	maxX, maxY := -1, -1
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			if f.Category[f.idx(x, y)] != category {
				continue
			}
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x > maxX {
				maxX = x
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if maxX < 0 {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX+1, maxY+1), true
}

// VisibilityMatch reports whether a target sample's surface is visible at the
// corresponding source-eye coordinate.
func VisibilityMatch(target, source *Frame, x, y int) bool {
	i := target.idx(x, y)
	if target.Depth[i] <= 0 {
		return false
	}
	world := target.Camera.Unproject(float64(x)+0.5, float64(y)+0.5, target.Depth[i])
	sx, sy, _, ok := source.Camera.Project(world)
	if !ok {
		return false
	}
	ix, iy := int(sx), int(sy)
	if !source.Camera.Contains(ix, iy) {
		return false
	}
	si := source.idx(ix, iy)
	if source.Depth[si] <= 0 {
		return false
	}
	depthTol := 0.025 + 0.012*target.Depth[i]
	return source.ObjectID[si] == target.ObjectID[i] &&
		math.Abs(source.Depth[si]-target.Depth[i]) <= depthTol
}

// CategoryMismatch returns the bounds and count of target pixels whose
// category is visible only in the target eye.
func CategoryMismatch(target, source *Frame, category string) (image.Rectangle, int) {
	r := image.Rectangle{}
	count := 0
	for y := 0; y < target.Height; y++ {
		for x := 0; x < target.Width; x++ {
			i := target.idx(x, y)
			if target.Category[i] != category || VisibilityMatch(target, source, x, y) {
				continue
			}
			if count == 0 {
				r = image.Rect(x, y, x+1, y+1)
			} else {
				r = r.Union(image.Rect(x, y, x+1, y+1))
			}
			count++
		}
	}
	return r, count
}
