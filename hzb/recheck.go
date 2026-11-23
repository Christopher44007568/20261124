package hzb

// ReplayResult reports the outcome of a conservative replay (recheck) of an
// "occluded" decision against the eye's own near layers.
type ReplayResult struct {
	Occluded       bool
	Pixels         int // mip0 pixels examined by the exact raster pass
	CellsVisited   int // coarse cells visited while descending near layers
	DecidedAtLevel int // level that produced the final verdict (-1 = mip0 raster)
}

// Recheck conservatively re-verifies an "occluded" verdict that came from a
// shared, stale, or coarse layer. It first descends the eye's own near layers
// (cheaper, exact region max), and falls back to an exact per-pixel raster of
// the box against mip0 when dilation keeps the verdict ambiguous.
//
// The replay can only flip "occluded" to "visible", never the reverse, so it
// is conservative: it may miss culls but never introduces false culls.
func (p *Pyramid) Recheck(cam *Camera, box AABB, res QueryResult) ReplayResult {
	rep := ReplayResult{Occluded: true, DecidedAtLevel: res.Level}
	for level := res.Level - 1; level >= 1; level-- {
		lv := p.Levels[level]
		x0 := clampi(res.MinPixX>>level, 0, lv.W-1)
		y0 := clampi(res.MinPixY>>level, 0, lv.H-1)
		x1 := clampi(res.MaxPixX>>level, 0, lv.W-1)
		y1 := clampi(res.MaxPixY>>level, 0, lv.H-1)
		rep.CellsVisited += (x1 - x0 + 1) * (y1 - y0 + 1)
		maxZ := regionMax(lv, x0, y0, x1, y1)
		if maxZ <= 0 || res.ObjMinZ <= maxZ+OcclusionEpsilon {
			rep.Occluded = false
			rep.DecidedAtLevel = level
			return rep
		}
	}
	// Exact pass: rasterize the box and depth-test every covered pixel
	// against the eye's own mip0 (raw depth, +Inf = empty).
	mip := p.Levels[0]
	visible := false
	rasterize(cam, BoxTriangles(box.Min, box.Max), func(x, y int, z float64) bool {
		rep.Pixels++
		if z+OcclusionEpsilon < mip.Data[y*mip.W+x] {
			visible = true
			return true
		}
		return false
	})
	rep.Occluded = !visible
	rep.DecidedAtLevel = -1
	return rep
}
