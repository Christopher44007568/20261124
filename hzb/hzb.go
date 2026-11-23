package hzb

import "math"

// OcclusionEpsilon is the depth margin for the occlusion test.
const OcclusionEpsilon = 1e-4

// Level is one mip of a hierarchical Z-buffer. Level 0 holds raw depth
// (+Inf = empty); levels >= 1 hold max-pooled occluder depth where 0 means
// "no occluder in this cell". Provenance fields make every layer traceable
// back to the camera and frame that produced it.
type Level struct {
	W, H         int
	Data         []float64
	SourceCamera string
	SourceFrame  int
	Shared       bool // true when reused from another eye or an older frame
}

// Pyramid is a per-eye hierarchical Z-buffer.
type Pyramid struct {
	CameraID string
	Frame    int
	Levels   []Level
}

func levelCount(w, h int) int {
	n := 1
	for w > 1 || h > 1 {
		w = (w + 1) / 2
		h = (h + 1) / 2
		n++
	}
	return n
}

// NewPyramid builds HZB levels [0, maxLevels); maxLevels <= 0 builds the full
// chain down to 1x1.
func NewPyramid(db *DepthBuffer, camID string, frame, maxLevels int) *Pyramid {
	total := levelCount(db.W, db.H)
	if maxLevels <= 0 || maxLevels > total {
		maxLevels = total
	}
	p := &Pyramid{CameraID: camID, Frame: frame}
	mip := Level{
		W: db.W, H: db.H, Data: make([]float64, len(db.Depth)),
		SourceCamera: camID, SourceFrame: frame,
	}
	copy(mip.Data, db.Depth)
	p.Levels = append(p.Levels, mip)
	for len(p.Levels) < maxLevels {
		p.Levels = append(p.Levels, downsample(p.Levels[len(p.Levels)-1]))
	}
	return p
}

// downsample max-pools 2x2 cells. Empty (+Inf) children contribute 0 so that
// empty space never occludes anything.
func downsample(prev Level) Level {
	w, h := (prev.W+1)/2, (prev.H+1)/2
	next := Level{
		W: w, H: h, Data: make([]float64, w*h),
		SourceCamera: prev.SourceCamera, SourceFrame: prev.SourceFrame,
		Shared: prev.Shared,
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m := 0.0
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					sx, sy := 2*x+dx, 2*y+dy
					if sx >= prev.W || sy >= prev.H {
						continue
					}
					v := prev.Data[sy*prev.W+sx]
					if math.IsInf(v, 1) {
						v = 0
					}
					if v > m {
						m = v
					}
				}
			}
			next.Data[y*w+x] = m
		}
	}
	return next
}

// QueryResult describes how the HZB decided an occlusion query.
type QueryResult struct {
	Occluded  bool
	Level     int // deciding mip level
	MinCellX  int
	MinCellY  int
	MaxCellX  int
	MaxCellY  int
	MinPixX   int // clamped screen-space bbox of the box
	MinPixY   int
	MaxPixX   int
	MaxPixY   int
	ObjMinZ   float64 // nearest view-z of the box
	LayerMaxZ float64 // farthest occluder z found in the layer
}

// Query tests an AABB against the pyramid. A box is occluded only when its
// nearest corner is farther than the farthest occluder stored in its cells.
func (p *Pyramid) Query(cam *Camera, box AABB) QueryResult {
	res := QueryResult{}
	minZ := math.Inf(1)
	fMinX, fMinY := math.Inf(1), math.Inf(1)
	fMaxX, fMaxY := math.Inf(-1), math.Inf(-1)
	for _, c := range box.Corners() {
		v := cam.ToView(c)
		if v.Z < minZ {
			minZ = v.Z
		}
		sx, sy, _, ok := cam.Project(c)
		if !ok {
			continue
		}
		fMinX = math.Min(fMinX, sx)
		fMinY = math.Min(fMinY, sy)
		fMaxX = math.Max(fMaxX, sx)
		fMaxY = math.Max(fMaxY, sy)
	}
	res.ObjMinZ = minZ
	if minZ < cam.Near {
		return res // straddles the near plane: cannot cull
	}
	if fMaxX < 0 || fMaxY < 0 || fMinX >= float64(cam.Width) || fMinY >= float64(cam.Height) {
		return res // off screen: nothing to cull here
	}
	res.MinPixX = clampi(int(math.Floor(fMinX)), 0, cam.Width-1)
	res.MinPixY = clampi(int(math.Floor(fMinY)), 0, cam.Height-1)
	res.MaxPixX = clampi(int(math.Ceil(fMaxX)), 0, cam.Width-1)
	res.MaxPixY = clampi(int(math.Ceil(fMaxY)), 0, cam.Height-1)

	level := 0
	fw, fh := fMaxX-fMinX+1, fMaxY-fMinY+1
	for level < len(p.Levels)-1 && (fw > 2 || fh > 2) {
		fw /= 2
		fh /= 2
		level++
	}
	res.Level = level
	lv := p.Levels[level]
	res.MinCellX = clampi(res.MinPixX>>level, 0, lv.W-1)
	res.MinCellY = clampi(res.MinPixY>>level, 0, lv.H-1)
	res.MaxCellX = clampi(res.MaxPixX>>level, 0, lv.W-1)
	res.MaxCellY = clampi(res.MaxPixY>>level, 0, lv.H-1)
	res.LayerMaxZ = regionMax(lv, res.MinCellX, res.MinCellY, res.MaxCellX, res.MaxCellY)
	res.Occluded = res.LayerMaxZ > 0 && minZ > res.LayerMaxZ+OcclusionEpsilon
	return res
}

func regionMax(lv Level, x0, y0, x1, y1 int) float64 {
	m := 0.0
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			v := lv.Data[y*lv.W+x]
			if math.IsInf(v, 1) {
				continue
			}
			if v > m {
				m = v
			}
		}
	}
	return m
}
