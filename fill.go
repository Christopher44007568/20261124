package stereoholes

import (
	"container/heap"
	"math"
)

type RejectReason string

const (
	ReasonNone           RejectReason = ""
	ReasonNoOverlap      RejectReason = "mask_no_overlap"
	ReasonMaskSurface    RejectReason = "mask_surface_mismatch"
	ReasonSourceTracking RejectReason = "source_tracking_invalid"
	ReasonDepth          RejectReason = "depth_mismatch"
	ReasonTargetTracking RejectReason = "target_tracking_invalid"
	ReasonSupport        RejectReason = "mask_asymmetric_support"
)

var holeColor = Vec3{0.012, 0.016, 0.024}

// FillOptions controls the conservative cross-eye validation.
type FillOptions struct {
	DepthAbsTolerance float64
	DepthRelTolerance float64
	MinSupport        int
	SupportRadiusPx   float64
}

func DefaultFillOptions() FillOptions {
	return FillOptions{
		DepthAbsTolerance: 0.025,
		DepthRelTolerance: 0.012,
		MinSupport:        5,
		SupportRadiusPx:   2.25,
	}
}

func (o FillOptions) depthTolerance(depth float64) float64 {
	return o.DepthAbsTolerance + o.DepthRelTolerance*math.Abs(depth)
}

type PixelStatus uint8

const (
	StatusOutside PixelStatus = iota
	StatusFilled
	StatusRejected
)

type FillStats struct {
	HolePixels int
	Filled     int
	Rejected   int
	Reasons    map[RejectReason]int
}

type FillResult struct {
	RGB    []Vec3
	Status []PixelStatus
	Reason []RejectReason
	Stats  FillStats
}

func newFillResult(f *Frame) FillResult {
	n := f.Width * f.Height
	r := FillResult{
		RGB:    append([]Vec3(nil), f.RGB...),
		Status: make([]PixelStatus, n),
		Reason: make([]RejectReason, n),
	}
	r.Stats.Reasons = make(map[RejectReason]int)
	return r
}

func (r *FillResult) copyPixel(i int, c Vec3) {
	r.RGB[i] = c
}

func (r *FillResult) reject(i int, reason RejectReason) {
	r.RGB[i] = holeColor
	r.Status[i] = StatusRejected
	r.Reason[i] = reason
	r.Stats.Rejected++
	if reason != ReasonNone {
		r.Stats.Reasons[reason]++
	}
}

type warpCandidate struct {
	valid       bool
	sx, sy      int
	tx, ty      float64
	targetDepth float64
	dist2       float64
	color       Vec3
}

func makeCandidates(target, source *Frame, holes []bool) []warpCandidate {
	out := make([]warpCandidate, target.Width*target.Height)
	for sy := 0; sy < source.Height; sy++ {
		for sx := 0; sx < source.Width; sx++ {
			si := source.idx(sx, sy)
			if source.Depth[si] <= 0 {
				continue
			}
			world := source.Camera.Unproject(float64(sx)+0.5, float64(sy)+0.5, source.Depth[si])
			tx, ty, depth, ok := target.Camera.Project(world)
			if !ok {
				continue
			}
			cx := int(math.Floor(tx))
			cy := int(math.Floor(ty))
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					x, y := cx+dx, cy+dy
					if !target.Camera.Contains(x, y) {
						continue
					}
					ti := target.idx(x, y)
					if !holes[ti] {
						continue
					}
					old := &out[ti]
					dxp := tx - (float64(x) + 0.5)
					dyp := ty - (float64(y) + 0.5)
					dist2 := dxp*dxp + dyp*dyp
					closerSurface := depth < old.targetDepth-1e-5
					sameSurface := math.Abs(depth-old.targetDepth) <= 1e-5
					if !old.valid || closerSurface || (sameSurface && dist2 < old.dist2) {
						*old = warpCandidate{
							valid: true, sx: sx, sy: sy, tx: tx, ty: ty,
							targetDepth: depth, dist2: dist2, color: source.RGB[si],
						}
					}
				}
			}
		}
	}
	return out
}

func supportCount(target, source *Frame, c warpCandidate, o FillOptions) int {
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			sx, sy := c.sx+dx, c.sy+dy
			if !source.Camera.Contains(sx, sy) {
				continue
			}
			si := source.idx(sx, sy)
			if source.Depth[si] <= 0 {
				continue
			}
			world := source.Camera.Unproject(float64(sx)+0.5, float64(sy)+0.5, source.Depth[si])
			tx, ty, depth, ok := target.Camera.Project(world)
			if !ok {
				continue
			}
			if math.Abs(tx-c.tx) <= o.SupportRadiusPx &&
				math.Abs(ty-c.ty) <= o.SupportRadiusPx &&
				math.Abs(depth-c.targetDepth) <= o.depthTolerance(depth) {
				count++
			}
		}
	}
	return count
}

// CrossEyeFill warps the other eye into the target camera and fills only
// target holes whose source sample survives all consistency checks.
func CrossEyeFill(target, source *Frame, holes []bool, o FillOptions) FillResult {
	res := newFillResult(target)
	cands := makeCandidates(target, source, holes)
	for i, isHole := range holes {
		if !isHole {
			continue
		}
		res.Stats.HolePixels++
		res.RGB[i] = holeColor
		res.Status[i] = StatusRejected
		c := cands[i]
		if !c.valid {
			res.reject(i, ReasonNoOverlap)
			continue
		}
		si := source.idx(c.sx, c.sy)
		if !source.Stable[si] || source.Ambiguous[si] {
			res.reject(i, ReasonSourceTracking)
			continue
		}
		if target.ObjectID[i] != 0 && source.ObjectID[si] != target.ObjectID[i] {
			res.reject(i, ReasonMaskSurface)
			continue
		}
		if target.Depth[i] > 0 &&
			math.Abs(c.targetDepth-target.Depth[i]) > o.depthTolerance(target.Depth[i]) {
			res.reject(i, ReasonDepth)
			continue
		}
		if !target.Stable[i] || target.Ambiguous[i] {
			res.reject(i, ReasonTargetTracking)
			continue
		}
		if supportCount(target, source, c, o) < o.MinSupport {
			res.reject(i, ReasonSupport)
			continue
		}
		res.Status[i] = StatusFilled
		res.RGB[i] = c.color
		res.Stats.Filled++
	}
	return res
}

// UnguardedCrossEyeFill is the diagnostic baseline: it still remaps the
// screen coordinate, but accepts every source sample without validation.
func UnguardedCrossEyeFill(target, source *Frame, holes []bool) FillResult {
	res := newFillResult(target)
	cands := makeCandidates(target, source, holes)
	for i, isHole := range holes {
		if !isHole {
			continue
		}
		res.Stats.HolePixels++
		if cands[i].valid {
			res.RGB[i] = cands[i].color
			res.Status[i] = StatusFilled
			res.Stats.Filled++
			continue
		}
		x, y := i%target.Width, i/target.Width
		si := source.idx(x, y)
		if source.Depth[si] > 0 {
			res.RGB[i] = source.RGB[si]
			res.Status[i] = StatusFilled
			res.Stats.Filled++
			continue
		}
		res.reject(i, ReasonNoOverlap)
	}
	return res
}

// OriginalEye leaves the selected holes unfilled.
func OriginalEye(target *Frame, holes []bool) FillResult {
	res := newFillResult(target)
	for i, isHole := range holes {
		if !isHole {
			continue
		}
		res.Stats.HolePixels++
		res.reject(i, ReasonNone)
	}
	return res
}

type queueItem struct {
	dist  float64
	index int
	owner int
}

type fillQueue []queueItem

func (q fillQueue) Len() int            { return len(q) }
func (q fillQueue) Less(i, j int) bool  { return q[i].dist < q[j].dist }
func (q fillQueue) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *fillQueue) Push(x interface{}) { *q = append(*q, x.(queueItem)) }
func (q *fillQueue) Pop() interface{} {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}

// NoCrossEyeFill is a same-eye baseline: hole colors propagate only from
// valid boundary pixels of the target eye.
func NoCrossEyeFill(target *Frame, holes []bool) FillResult {
	res := newFillResult(target)
	n := target.Width * target.Height
	for i, isHole := range holes {
		if isHole {
			res.Stats.HolePixels++
			res.RGB[i] = holeColor
		}
	}
	dist := make([]float64, n)
	owner := make([]int, n)
	for i := range dist {
		dist[i] = math.Inf(1)
		owner[i] = -1
	}
	q := &fillQueue{}
	heap.Init(q)

	addSeed := func(i int) {
		if dist[i] == 0 {
			return
		}
		dist[i] = 0
		owner[i] = i
		heap.Push(q, queueItem{dist: 0, index: i, owner: i})
	}
	for y := 0; y < target.Height; y++ {
		for x := 0; x < target.Width; x++ {
			i := target.idx(x, y)
			if holes[i] {
				continue
			}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if target.Camera.Contains(nx, ny) && holes[target.idx(nx, ny)] {
						addSeed(i)
					}
				}
			}
		}
	}

	steps := [8][3]float64{
		{-1, 0, 1}, {1, 0, 1}, {0, -1, 1}, {0, 1, 1},
		{-1, -1, math.Sqrt2}, {1, -1, math.Sqrt2},
		{-1, 1, math.Sqrt2}, {1, 1, math.Sqrt2},
	}
	for q.Len() > 0 {
		item := heap.Pop(q).(queueItem)
		if item.dist > dist[item.index] {
			continue
		}
		if holes[item.index] && res.Status[item.index] == StatusOutside {
			res.RGB[item.index] = target.RGB[item.owner]
			res.Status[item.index] = StatusFilled
			res.Stats.Filled++
		}
		x, y := item.index%target.Width, item.index/target.Width
		for _, s := range steps {
			nx, ny := x+int(s[0]), y+int(s[1])
			if !target.Camera.Contains(nx, ny) {
				continue
			}
			ni := target.idx(nx, ny)
			if !holes[ni] {
				continue
			}
			nd := item.dist + s[2]
			if nd < dist[ni] {
				dist[ni] = nd
				owner[ni] = item.owner
				heap.Push(q, queueItem{dist: nd, index: ni, owner: item.owner})
			}
		}
	}
	for i, isHole := range holes {
		if isHole && res.Status[i] == StatusOutside {
			res.reject(i, ReasonNoOverlap)
		}
	}
	return res
}
