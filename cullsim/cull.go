package main

import (
	"fmt"
	"math"
)

// Params groups the visibility tunables shared by every strategy.
type Params struct {
	FovHalf     float64 // eye frustum half-angle (radians)
	MaxDist     float64 // eye view distance
	EyeSep      float64 // interpupillary distance
	LightRadius float64 // moving light influence radius
}

func DefaultParams() Params {
	return Params{
		FovHalf:     60 * math.Pi / 180,
		MaxDist:     260,
		EyeSep:      9,
		LightRadius: 165,
	}
}

// FrameEnv is the per-frame simulation state.
type FrameEnv struct {
	Scene *Scene
	P     Params
	Light Vec2
	Pose  ViewerPose
	EyeL  Vec2
	EyeR  Vec2
}

func NewFrameEnv(s *Scene, p Params, t float64) *FrameEnv {
	pose := ViewerAt(t)
	el, er := pose.Eyes(p.EyeSep)
	return &FrameEnv{Scene: s, P: p, Light: LightAt(t), Pose: pose, EyeL: el, EyeR: er}
}

// Lit reports whether object i is inside the moving light's radius and has
// line-of-sight to it.
func (e *FrameEnv) Lit(i int) bool {
	o := e.Scene.Objects[i]
	if o.Pos.Dist(e.Light) > e.P.LightRadius {
		return false
	}
	return !Blocked(e.Scene.Walls, e.Light, o.Pos)
}

// VisibleFrom reports whether object i lies inside the frustum rooted at
// eye and has line-of-sight to it.
func (e *FrameEnv) VisibleFrom(eye Vec2, i int) bool {
	o := e.Scene.Objects[i]
	if !InCone(eye, e.Pose.Yaw, e.P.FovHalf, e.P.MaxDist, o.Pos) {
		return false
	}
	return !Blocked(e.Scene.Walls, eye, o.Pos)
}

// Required is the ground-truth set: objects that must appear in the final
// image, i.e. lit and visible to at least one eye (binocular union).
func (e *FrameEnv) Required() map[int]bool {
	req := make(map[int]bool)
	for i := range e.Scene.Objects {
		if !e.Lit(i) {
			continue
		}
		if e.VisibleFrom(e.EyeL, i) || e.VisibleFrom(e.EyeR, i) {
			req[i] = true
		}
	}
	return req
}

// coneQuery returns all object indices inside either eye cone, widened by
// extraAng (radians) and extraDist, without any line-of-sight test. This
// models a cheap spatial-index lookup.
func (e *FrameEnv) coneQuery(pose ViewerPose, extraAng, extraDist float64) []int {
	el, er := pose.Eyes(e.P.EyeSep)
	half := e.P.FovHalf + extraAng
	maxd := e.P.MaxDist + extraDist
	var out []int
	for i, o := range e.Scene.Objects {
		if InCone(el, pose.Yaw, half, maxd, o.Pos) || InCone(er, pose.Yaw, half, maxd, o.Pos) {
			out = append(out, i)
		}
	}
	return out
}

// Strategy selects candidate objects to render for each frame.
//
// Pipeline model: fetching candidates takes one frame, so a strategy may
// only use pose information available up to the PREVIOUS frame, while the
// ground truth (Required) is evaluated at the current display pose. This
// is exactly where quick head turns hurt: the query is always one frame
// stale unless the strategy predicts.
type Strategy interface {
	Name() string
	Candidates(env *FrameEnv) []int
	// Observe feeds back the confirmed rendered set and the miss count of
	// the frame just finished, so strategies can adapt the next query.
	Observe(rendered []int, misses int)
	// Track records the measured pose of the finished frame.
	Track(env *FrameEnv)
}

// --- Strategy 1: no prediction, reuses the last measured pose as-is ------

type NoneStrategy struct {
	last ViewerPose
	has  bool
}

func (NoneStrategy) Name() string { return "none" }

func (n *NoneStrategy) Candidates(env *FrameEnv) []int {
	if !n.has {
		return env.coneQuery(env.Pose, 0, 0)
	}
	return env.coneQuery(n.last, 0, 0)
}

func (NoneStrategy) Observe([]int, int) {}

func (n *NoneStrategy) Track(env *FrameEnv) {
	n.last = env.Pose
	n.has = true
}

// ColdQuery is shared by all strategies for the very first frame, before
// any pose history exists (zero-latency assumption at startup).
func ColdQuery(env *FrameEnv, margin, distPad float64) []int {
	return env.coneQuery(env.Pose, margin, distPad)
}

// --- Strategy 2: shared union of last frame's both-eye visible set --------

type UnionStrategy struct {
	prev    []int
	started bool
}

func (UnionStrategy) Name() string { return "union" }

func (u *UnionStrategy) Candidates(env *FrameEnv) []int {
	if !u.started {
		return ColdQuery(env, 0, 0)
	}
	return append([]int(nil), u.prev...)
}

func (u *UnionStrategy) Observe(rendered []int, _ int) {
	u.prev = append([]int(nil), rendered...)
	u.started = true
}

func (UnionStrategy) Track(*FrameEnv) {}

// --- Strategy 3: path prediction with adaptive query expansion ------------

type PredictStrategy struct {
	last    ViewerPose
	prev    ViewerPose
	hasLast bool
	hasPrev bool
	margin  float64 // extra half-angle (radians), grows after a miss
	distPad float64 // extra view distance, grows together with margin
	minMarg float64
	maxMarg float64
	frame   int
	Log     bool
}

func NewPredictStrategy() *PredictStrategy {
	return &PredictStrategy{
		minMarg: 4 * math.Pi / 180,
		maxMarg: 45 * math.Pi / 180,
		margin:  4 * math.Pi / 180,
		distPad: 12,
	}
}

func (PredictStrategy) Name() string { return "predict" }

// PredictPose linearly extrapolates the head path one frame ahead from
// the two most recent measured poses.
func (p *PredictStrategy) PredictPose() ViewerPose {
	vel := p.last.Pos.Sub(p.prev.Pos)
	dYaw := NormAngle(p.last.Yaw - p.prev.Yaw)
	return ViewerPose{Pos: p.last.Pos.Add(vel), Yaw: p.last.Yaw + dYaw}
}

func (p *PredictStrategy) Candidates(env *FrameEnv) []int {
	if !p.hasLast {
		return ColdQuery(env, p.margin, p.distPad)
	}
	set := make(map[int]bool)
	// Swept volume: cone at the last known pose plus cone at the
	// predicted display pose, both widened by the adaptive margin.
	for _, i := range env.coneQuery(p.last, p.margin, p.distPad) {
		set[i] = true
	}
	if p.hasPrev {
		pred := p.PredictPose()
		for _, i := range env.coneQuery(pred, p.margin, p.distPad) {
			set[i] = true
		}
	}
	out := make([]int, 0, len(set))
	for i := range set {
		out = append(out, i)
	}
	return out
}

// Observe implements the auto-expansion rule: any miss means the
// prediction failed, so the next query widens; clean frames let it relax
// back toward the minimum margin.
func (p *PredictStrategy) Observe(_ []int, misses int) {
	if misses > 0 {
		p.margin = math.Min(p.margin*1.7+2*math.Pi/180, p.maxMarg)
		p.distPad = math.Min(p.distPad*1.7+8, 120)
		if p.Log {
			fmt.Printf("[predict] frame %d: %d miss(es) -> query expands: margin=%.1f deg distPad=%.0f\n",
				p.frame, misses, p.margin*180/math.Pi, p.distPad)
		}
	} else {
		p.margin = math.Max(p.margin*0.92, p.minMarg)
		p.distPad = math.Max(p.distPad*0.92, 12)
	}
	p.frame++
}

// Track records the pose for the next extrapolation step.
func (p *PredictStrategy) Track(env *FrameEnv) {
	p.prev = p.last
	p.last = env.Pose
	p.hasPrev = p.hasLast
	p.hasLast = true
}
