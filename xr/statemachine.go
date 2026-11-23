package xr

import "math"

// State of the XR render pipeline.
type State int

const (
	StateNormal State = iota
	StateLost
	StateRelocalizing
	StateRecovery
)

func (s State) String() string {
	switch s {
	case StateLost:
		return "LOST"
	case StateRelocalizing:
		return "RELOCALIZING"
	case StateRecovery:
		return "RECOVERY"
	default:
		return "NORMAL"
	}
}

// Policy selects how the pipeline reacts while samples are unavailable.
type Policy int

const (
	// PolicyFreeze keeps the last known pose on screen (no motion at all).
	PolicyFreeze Policy = iota
	// PolicyInertial extrapolates with last velocities up to MaxInertialMs,
	// then degrades to a freeze.
	PolicyInertial
	// PolicyFade behaves like freeze while fading the frame to black and
	// fades content back in after relocalization.
	PolicyFade
)

func (p Policy) String() string {
	switch p {
	case PolicyInertial:
		return "inertial"
	case PolicyFade:
		return "fade"
	default:
		return "freeze"
	}
}

// Config holds the tunable time budgets of the state machine.
type Config struct {
	LostTimeoutMs float64 // no-sample interval before NORMAL -> LOST
	MaxInertialMs float64 // inertial extrapolation budget
	ClockJumpMs   float64 // |delta| above which a sample is treated as a clock jump
	FadeOutMs     float64 // time to reach full black after loss (fade policy)
	FadeInMs      float64 // time to restore full opacity after reloc
	BlackAlpha    float64 // alpha at/above which the frame counts as black
	TrailLength   int     // rendered pose samples kept as the trail
}

func DefaultConfig() Config {
	return Config{
		LostTimeoutMs: 80, MaxInertialMs: 120, ClockJumpMs: 200,
		FadeOutMs: 150, FadeInMs: 200, BlackAlpha: 0.99, TrailLength: 12,
	}
}

// FrameReport is what the renderer receives each tick.
type FrameReport struct {
	TMs          float64
	State        State
	RenderPose   Pose
	Alpha        float64 // 1 = fully visible content, 0 = black frame
	Black        bool
	Extrapolated bool
	Frozen       bool
	AgeMs        float64 // age of the underlying tracker sample
	ClockJumped  bool
	Note         string
}

// Machine is the four-state render state machine.
type Machine struct {
	cfg    Config
	policy Policy

	state   State
	last    *Sample
	prev    *Sample
	linVel  Vec3
	angVel  Vec3
	haveVel bool

	// LOST bookkeeping.
	lostSinceMs float64
	// RECOVERY bookkeeping.
	recoverSinceMs float64
	recoverFrom    Pose
	recoverTo      Pose
	holdRelocFrame bool
	// Latest processed reloc event (for anchor/trail re-projection).
	lastReloc *RelocEvent
	// Monotonic clock of the previous tick for jump detection.
	lastTickMs    float64
	haveTick      bool
	clockJumpSeen bool
	// Trail of rendered poses in the current origin frame.
	trail []Pose
}

func NewMachine(cfg Config, policy Policy) *Machine {
	return &Machine{cfg: cfg, policy: policy, state: StateNormal}
}

func (m *Machine) State() State           { return m.state }
func (m *Machine) Trail() []Pose          { return m.trail }
func (m *Machine) LastReloc() *RelocEvent { return m.lastReloc }
func (m *Machine) recoverFromPose() Pose  { return m.recoverFrom }

// Observe is the single entry point fed once per rendered frame.
func (m *Machine) Observe(nowMs float64, s *Sample, ev *RelocEvent, anchors []Anchor) FrameReport {
	m.clockJumpSeen = false
	if m.haveTick {
		if d := math.Abs(nowMs - m.lastTickMs); d > m.cfg.ClockJumpMs {
			m.clockJumpSeen = true
		}
	}
	m.lastTickMs, m.haveTick = nowMs, true

	if ev != nil {
		m.applyReloc(nowMs, *ev)
	}
	if s != nil {
		m.ingest(nowMs, *s, ev != nil)
	}

	switch m.state {
	case StateNormal:
		return m.tickNormal(nowMs)
	case StateLost:
		return m.tickLost(nowMs)
	case StateRelocalizing:
		return m.tickRelocalizing(nowMs)
	default:
		return m.tickRecovery(nowMs)
	}
}

func (m *Machine) ingest(nowMs float64, s Sample, relocThisTick bool) {
	if m.last != nil && m.clockJumpSeen {
		// Timestamps are no longer comparable: drop history so the age of the
		// sample restarts from zero instead of firing a phantom loss timeout.
		m.last, m.prev, m.haveVel = nil, nil, false
	}
	recovering := m.state == StateLost && !relocThisTick
	if m.last != nil && s.Origin == m.last.Origin {
		dt := (s.TMs - m.last.TMs) / 1000.0
		if dt > 0 {
			base := m.last.Pose
			if m.prev != nil && m.prev.Origin == s.Origin {
				base = m.prev.Pose
			}
			m.linVel = s.Pose.P.Sub(m.last.Pose.P).Scale(1 / dt)
			m.angVel = AngularVelocity(yawBetween(base.Q, s.Pose.Q), dt, s.Pose.Q)
			m.haveVel = true
		}
	}
	m.prev = m.last
	m.last = &s
	if recovering && (m.prev == nil || m.prev.Origin == s.Origin) {
		// Same-origin sample after a loss: ease the frozen frame onto the
		// fresh sample instead of snapping, so RECOVERY covers both losses
		// and relocalizations.
		m.state = StateRecovery
		m.recoverSinceMs = nowMs
		m.recoverFrom = m.renderFrozen(nowMs)
		m.recoverTo = s.Pose
	}
}

// yawBetween extracts the signed Y-component angle between two headings that
// differ only by yaw (the simulated trajectory rotates around world Y only).
func yawBetween(a, b Quat) float64 {
	inv := Quat{W: a.W, X: -a.X, Y: -a.Y, Z: -a.Z}
	r := inv.Multiply(b)
	return 2 * math.Atan2(r.Y, r.W)
}

func (m *Machine) applyReloc(nowMs float64, ev RelocEvent) {
	// Re-project the frozen pose, velocities and the whole trail into the new
	// origin so legacy content keeps working instead of teleporting.
	if m.last != nil && m.last.Origin == ev.FromOrigin {
		m.last.Pose = m.last.Pose.Transform(ev.Delta)
		m.last.Origin = ev.ToOrigin
	}
	if m.prev != nil && m.prev.Origin == ev.FromOrigin {
		m.prev.Pose = m.prev.Pose.Transform(ev.Delta)
		m.prev.Origin = ev.ToOrigin
	}
	m.linVel = ev.Delta.Q.Rotate(m.linVel)
	for i := range m.trail {
		m.trail[i] = m.trail[i].Transform(ev.Delta)
	}
	rel := ev
	m.lastReloc = &rel
	m.state = StateRelocalizing
	m.holdRelocFrame = true
	m.recoverSinceMs = nowMs
	m.recoverFrom = m.renderFrozen(nowMs)
	if m.last != nil {
		m.recoverTo = m.last.Pose
	} else {
		m.recoverTo = m.recoverFrom
	}
}

func (m *Machine) tickNormal(nowMs float64) FrameReport {
	r := FrameReport{TMs: nowMs, State: StateNormal, Alpha: 1}
	if m.last == nil {
		// Cold start: stay in NORMAL and emit black until the first sample;
		// RELOCALIZING is reserved for an actual origin swap.
		r.Alpha, r.Black, r.Note = 0, true, "waiting for first sample"
		return r
	}
	r.AgeMs = nowMs - m.last.TMs
	if r.AgeMs > m.cfg.LostTimeoutMs {
		m.state = StateLost
		m.lostSinceMs = nowMs - r.AgeMs
		return m.tickLost(nowMs)
	}
	if r.AgeMs <= 1e-9 {
		r.RenderPose = m.last.Pose
	} else {
		// Fresh enough: short inertial fill between sample arrivals.
		r.RenderPose = Extrapolate(m.last.Pose, m.linVel, m.angVel, r.AgeMs/1000)
		r.Extrapolated = r.AgeMs > 1
	}
	r.ClockJumped = m.clockJumpSeen
	m.pushTrail(r.RenderPose)
	return r
}

func (m *Machine) tickLost(nowMs float64) FrameReport {
	r := FrameReport{TMs: nowMs, State: StateLost}
	age := nowMs - m.last.TMs
	r.AgeMs = age
	gap := nowMs - m.lostSinceMs

	switch m.policy {
	case PolicyInertial:
		if gap <= m.cfg.MaxInertialMs {
			r.RenderPose = Extrapolate(m.last.Pose, m.linVel, m.angVel, age/1000)
			r.Extrapolated = true
			r.Alpha = 1
		} else {
			r.RenderPose = m.last.Pose
			r.Frozen = true
			r.Alpha = 1
			r.Note = "inertial budget exhausted -> freeze"
		}
	case PolicyFade:
		r.RenderPose = m.last.Pose
		r.Frozen = true
		r.Alpha = 1 - math.Min(1, gap/m.cfg.FadeOutMs)
	default:
		r.RenderPose = m.last.Pose
		r.Frozen = true
		r.Alpha = 1
	}
	r.Black = r.Alpha <= 1-m.cfg.BlackAlpha
	r.ClockJumped = m.clockJumpSeen
	m.pushTrail(r.RenderPose)
	return r
}

func (m *Machine) tickRelocalizing(nowMs float64) FrameReport {
	r := FrameReport{TMs: nowMs, State: StateRelocalizing}
	if m.last == nil {
		// Nothing to show before the first sample ever arrives: hard black.
		r.Alpha = 0
		r.Black = true
		r.Note = "no pose available"
		return r
	}
	if m.holdRelocFrame {
		// Hold one frame in RELOCALIZING while the new origin is established.
		r.RenderPose = m.last.Pose
		r.Alpha = m.entryAlpha()
		r.Note = "establishing new tracking origin"
		m.holdRelocFrame = false
		return r
	}
	m.recoverTo = m.last.Pose
	m.state = StateRecovery
	return m.tickRecovery(nowMs)
}

func (m *Machine) entryAlpha() float64 {
	if m.policy == PolicyFade {
		return 0
	}
	return 1
}

func (m *Machine) tickRecovery(nowMs float64) FrameReport {
	r := FrameReport{TMs: nowMs, State: StateRecovery}
	prog := math.Min(1, (nowMs-m.recoverSinceMs)/m.cfg.FadeInMs)
	// Ease the pose from the frozen frame toward the relocated sample.
	e := prog * prog * (3 - 2*prog) // smoothstep
	r.RenderPose = lerpPose(m.recoverFrom, m.recoverTo, e)
	if m.policy == PolicyFade {
		r.Alpha = prog
	} else {
		r.Alpha = 1
	}
	r.Black = r.Alpha <= 1-m.cfg.BlackAlpha
	r.Note = "first-frame pose correction active"
	m.pushTrail(r.RenderPose)
	if prog >= 1 {
		m.state = StateNormal
		r.State = StateNormal
		r.Note = ""
	}
	return r
}

func (m *Machine) renderFrozen(nowMs float64) Pose {
	if m.last == nil {
		return Pose{}
	}
	return m.last.Pose
}

func (m *Machine) pushTrail(p Pose) {
	m.trail = append(m.trail, p)
	if len(m.trail) > m.cfg.TrailLength {
		m.trail = m.trail[len(m.trail)-m.cfg.TrailLength:]
	}
}

func lerpPose(a, b Pose, t float64) Pose {
	return Pose{
		P: a.P.Add(b.P.Sub(a.P).Scale(t)),
		Q: slerp(a.Q, b.Q, t),
	}
}

// slerp for unit quaternions (NLERP fallback for near-parallel spins).
func slerp(a, b Quat, t float64) Quat {
	dot := a.W*b.W + a.X*b.X + a.Y*b.Y + a.Z*b.Z
	if dot < 0 {
		b = Quat{W: -b.W, X: -b.X, Y: -b.Y, Z: -b.Z}
		dot = -dot
	}
	if dot > 0.9995 {
		q := Quat{
			W: a.W + (b.W-a.W)*t, X: a.X + (b.X-a.X)*t,
			Y: a.Y + (b.Y-a.Y)*t, Z: a.Z + (b.Z-a.Z)*t,
		}
		n := math.Sqrt(q.W*q.W + q.X*q.X + q.Y*q.Y + q.Z*q.Z)
		return Quat{q.W / n, q.X / n, q.Y / n, q.Z / n}
	}
	theta := math.Acos(dot)
	sin := math.Sin(theta)
	w1, w2 := math.Sin((1-t)*theta)/sin, math.Sin(t*theta)/sin
	return Quat{
		W: a.W*w1 + b.W*w2, X: a.X*w1 + b.X*w2,
		Y: a.Y*w1 + b.Y*w2, Z: a.Z*w1 + b.Z*w2,
	}
}
