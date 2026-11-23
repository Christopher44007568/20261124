package raster

import "stereowarp/geom"

// CoverageMask marks every pixel covered by glass (or all) mesh triangles as
// seen by cam. It ignores depth: the mask identifies screen regions whose
// color depends on alpha blending and therefore cannot be reused blindly.
func CoverageMask(cam geom.Camera, meshes []*Mesh, transparentOnly bool) []bool {
	mask := make([]bool, cam.Width*cam.Height)
	r := Renderer{}
	for _, m := range meshes {
		if transparentOnly && !m.Material.Transparent() {
			continue
		}
		pr := r.project(m, cam)
		for _, tri := range m.Triangles {
			a, b, c := pr[tri[0]], pr[tri[1]], pr[tri[2]]
			if !a.inside || !b.inside || !c.inside {
				continue
			}
			markTriangle(mask, cam.Width, cam.Height, a, b, c)
		}
	}
	return mask
}

func markTriangle(mask []bool, w, h int, a, b, c projected) {
	minX := max(int(min(a.u, b.u, c.u)), 0)
	maxX := min(int(max(a.u, b.u, c.u))+1, w)
	minY := max(int(min(a.v, b.v, c.v)), 0)
	maxY := min(int(max(a.v, b.v, c.v))+1, h)
	area := edge(a.u, a.v, b.u, b.v, c.u, c.v)
	if area == 0 {
		return
	}
	invA := 1.0 / area
	for y := minY; y < maxY; y++ {
		py := float64(y) + 0.5
		for x := minX; x < maxX; x++ {
			px := float64(x) + 0.5
			l0 := edge(b.u, b.v, c.u, c.v, px, py) * invA
			l1 := edge(c.u, c.v, a.u, a.v, px, py) * invA
			l2 := edge(a.u, a.v, b.u, b.v, px, py) * invA
			if l0 >= 0 && l1 >= 0 && l2 >= 0 {
				mask[y*w+x] = true
			}
		}
	}
}
