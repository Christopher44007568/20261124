package hzb

import (
	"fmt"
	"math"
	"time"
)

// SharePolicy controls cross-eye / cross-frame layer reuse.
type SharePolicy struct {
	// SharedFromLevel is the near/far split: levels >= this are "far" layers
	// that may be shared; levels below are always rebuilt per eye.
	SharedFromLevel int
	// RecheckShared enables conservative replay for "occluded" verdicts
	// decided on a shared or stale layer (other eye / older frame).
	RecheckShared bool
	// RecheckCoarse enables conservative replay for "occluded" verdicts
	// decided on any far layer (>= SharedFromLevel), even the eye's own.
	RecheckCoarse bool
}

// Compatible reports whether two cameras may share far HZB layers: identical
// viewport and identical clip planes (depth ranges must be comparable).
func Compatible(a, b *Camera) bool {
	if a.Width != b.Width || a.Height != b.Height {
		return false
	}
	const eps = 1e-9
	return math.Abs(a.Near-b.Near) < eps && math.Abs(a.Far-b.Far) < eps
}

// FalseCullEvent records a wrong "occluded" verdict together with the exact
// layer and camera that produced it, so every error is traceable.
type FalseCullEvent struct {
	ObjectID      string
	Eye           string // camera that received the wrong verdict
	Frame         int    // frame in which the verdict was made
	Level         int    // deciding mip level
	CellX, CellY  int    // deciding cell (min corner of the tested range)
	LayerCamera   string // camera that produced the layer
	LayerFrame    int    // frame that produced the layer
	LayerShared   bool   // layer was reused from another eye / older frame
	ObjMinZ       float64
	LayerMaxZ     float64
	VisiblePixels int // ground-truth visible pixels that were wrongly culled
}

func (e FalseCullEvent) String() string {
	return fmt.Sprintf("false-cull obj=%s eye=%s frame=%d level=%d cell=(%d,%d) "+
		"layer[cam=%s frame=%d shared=%t] objMinZ=%.3f layerMaxZ=%.3f lostPx=%d",
		e.ObjectID, e.Eye, e.Frame, e.Level, e.CellX, e.CellY,
		e.LayerCamera, e.LayerFrame, e.LayerShared, e.ObjMinZ, e.LayerMaxZ, e.VisiblePixels)
}

// Stats aggregates culling statistics for one or both eyes.
type Stats struct {
	Queries            int
	Visible            int
	Occluded           int
	SharedLayerQueries int // queries decided on a shared/reused layer
	Replays            int // conservative replays performed
	ReplaySaves        int // replays that flipped occluded -> visible
	ReplayPixels       int // mip0 pixels scanned by replays
	ReplayCells        int // near-layer cells visited by replays
	FalseCulls         int
	FalseCullPixels    int
	SharedCells        int // right-eye cells reused instead of rebuilt
	TotalCells         int // right-eye cells in total
	BuildTime          time.Duration
	QueryTime          time.Duration
	FalseCullEvents    []FalseCullEvent
}

// ReuseRatio is the fraction of the second eye's pyramid that was shared.
func (s Stats) ReuseRatio() float64 {
	if s.TotalCells == 0 {
		return 0
	}
	return float64(s.SharedCells) / float64(s.TotalCells)
}

// SharedQueryRatio is the fraction of queries decided on shared layers.
func (s Stats) SharedQueryRatio() float64 {
	if s.Queries == 0 {
		return 0
	}
	return float64(s.SharedLayerQueries) / float64(s.Queries)
}

// StereoHZB holds one pyramid per eye; far layers of the right eye may alias
// the left eye's layers instead of being rebuilt.
type StereoHZB struct {
	Left, Right *Pyramid
	Frame       int
	Policy      SharePolicy
	Sharing     bool
	Stats       Stats
}

// BuildStereo renders nothing itself; it takes per-eye depth buffers and
// builds the pyramids according to the sharing policy.
func BuildStereo(camL, camR *Camera, dbL, dbR *DepthBuffer, frame int, pol SharePolicy) *StereoHZB {
	start := time.Now()
	s := &StereoHZB{Frame: frame, Policy: pol}
	s.Left = NewPyramid(dbL, camL.ID, frame, 0)
	if pol.SharedFromLevel > 0 && Compatible(camL, camR) {
		near := NewPyramid(dbR, camR.ID, frame, pol.SharedFromLevel)
		s.Right = ReuseFarLevels(near, s.Left, pol.SharedFromLevel)
		s.Sharing = true
	} else {
		s.Right = NewPyramid(dbR, camR.ID, frame, 0)
	}
	for _, lv := range s.Right.Levels {
		s.Stats.TotalCells += len(lv.Data)
		if lv.Shared {
			s.Stats.SharedCells += len(lv.Data)
		}
	}
	s.Stats.BuildTime = time.Since(start)
	return s
}

// ReuseFarLevels assembles a pyramid whose near levels [0, fromLevel) come
// from `near` (current frame, per-eye) and whose far levels are reused from
// `far` (another eye or an older frame). Reused levels keep their original
// provenance and are marked Shared so wrong verdicts stay traceable.
func ReuseFarLevels(near, far *Pyramid, fromLevel int) *Pyramid {
	out := &Pyramid{CameraID: near.CameraID, Frame: near.Frame}
	out.Levels = append(out.Levels, near.Levels...)
	for l := fromLevel; l < len(far.Levels); l++ {
		lv := far.Levels[l]
		lv.Shared = true
		out.Levels = append(out.Levels, lv)
	}
	return out
}

// PyramidFor returns the pyramid of the given eye (0 = left, 1 = right).
func (s *StereoHZB) PyramidFor(eye int) *Pyramid {
	if eye == 1 {
		return s.Right
	}
	return s.Left
}

// ValidateEye classifies every target for one eye and checks each verdict
// against ground truth, recording traceable false-cull events.
func (s *StereoHZB) ValidateEye(cam *Camera, eye int, db *DepthBuffer, targets []Object) {
	ValidateTargets(s.PyramidFor(eye), cam, db, targets, s.Frame, s.Policy, &s.Stats)
}

// Classify runs one occlusion query on the pyramid, applying the conservative
// replay policy when the deciding layer is shared, stale, or coarse.
func (p *Pyramid) Classify(cam *Camera, box AABB, frame int, pol SharePolicy, st *Stats) (bool, QueryResult) {
	t0 := time.Now()
	defer func() { st.QueryTime += time.Since(t0) }()
	res := p.Query(cam, box)
	st.Queries++
	lv := p.Levels[res.Level]
	if lv.Shared || lv.SourceCamera != cam.ID {
		st.SharedLayerQueries++
	}
	if !res.Occluded {
		st.Visible++
		return true, res
	}
	stale := lv.Shared || lv.SourceCamera != cam.ID || lv.SourceFrame != frame
	coarse := res.Level >= pol.SharedFromLevel
	if (stale && pol.RecheckShared) || (coarse && pol.RecheckCoarse) {
		rep := p.Recheck(cam, box, res)
		st.Replays++
		st.ReplayPixels += rep.Pixels
		st.ReplayCells += rep.CellsVisited
		if !rep.Occluded {
			st.ReplaySaves++
			st.Visible++
			return true, res
		}
	}
	st.Occluded++
	return false, res
}

// ValidateTargets classifies every target and records a traceable event for
// each false cull (HZB says occluded, ground truth says visible).
func ValidateTargets(p *Pyramid, cam *Camera, db *DepthBuffer, targets []Object, frame int, pol SharePolicy, st *Stats) {
	for _, t := range targets {
		visible, res := p.Classify(cam, t.Box, frame, pol, st)
		if visible || !GroundTruthVisible(cam, db, t.Box) {
			continue
		}
		lv := p.Levels[res.Level]
		ev := FalseCullEvent{
			ObjectID:      t.ID,
			Eye:           cam.ID,
			Frame:         frame,
			Level:         res.Level,
			CellX:         res.MinCellX,
			CellY:         res.MinCellY,
			LayerCamera:   lv.SourceCamera,
			LayerFrame:    lv.SourceFrame,
			LayerShared:   lv.Shared,
			ObjMinZ:       res.ObjMinZ,
			LayerMaxZ:     res.LayerMaxZ,
			VisiblePixels: VisiblePixels(cam, db, t.Box),
		}
		st.FalseCulls++
		st.FalseCullPixels += ev.VisiblePixels
		st.FalseCullEvents = append(st.FalseCullEvents, ev)
	}
}
