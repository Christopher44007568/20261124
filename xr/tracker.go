package xr

import "math"

// Tracker simulates a 6-Dof tracking source. It owns an analytic ground-truth
// trajectory in a canonical room frame and reports poses relative to the
// current tracking origin. Tracking can be muted (loss) and the origin can be
// swapped (relocalization into another room).
type Tracker struct {
	clock *Clock

	// Analytic motion parameters (SI units, t in seconds).
	v0   Vec3    // initial linear velocity
	linA float64 // sinusoidal linear acceleration amplitude
	linW float64 // linear acceleration angular frequency
	yaw0 float64 // initial heading
	wY0  float64 // constant yaw-rate component
	angA float64 // sinusoidal angular acceleration amplitude
	angW float64 // angular acceleration frequency

	origin     string
	originFrom Canonical // transform canonical -> current origin
	muteFrom   float64
	mutedUntil float64
	reloc      *RelocEvent
}

// Canonical is the room-invariant ground-truth transform (canonical -> origin).
type Canonical = RigidTransform

func NewTracker(clock *Clock) *Tracker {
	return &Tracker{
		clock: clock,
		v0:    Vec3{X: 0.8, Z: 0.15},
		linA:  1.2, linW: 0.9,
		yaw0: 0.1, wY0: 0.25,
		angA: 0.6, angW: 0.7,
		origin:     "room-A",
		originFrom: IdentityTransform, // origin A == canonical frame
	}
}

// GroundTruth returns the device pose in the canonical room frame at time t.
func (tk *Tracker) GroundTruth(tMs float64) Pose {
	t := tMs / 1000.0
	// Position: v0*t plus double integral of linA*sin(linW*t) on X.
	sinFrac := math.Sin(tk.linW*t) - tk.linW*t
	x := tk.v0.X*t + tk.linA/(tk.linW*tk.linW)*sinFrac
	z := tk.v0.Z * t
	// Heading: constant rate plus double integral of angA*sin(angW*t).
	sinFracY := math.Sin(tk.angW*t) - tk.angW*t
	yaw := tk.yaw0 + tk.wY0*t + tk.angA/(tk.angW*tk.angW)*sinFracY
	return Pose{P: Vec3{X: x, Y: 1.5, Z: z}, Q: QuatFromAxisAngle(Vec3{0, 1, 0}, yaw)}
}

// Velocities returns ground-truth linear (world frame) and angular (local
// frame) velocity at time t, used to quantify extrapolation error.
func (tk *Tracker) Velocities(tMs float64) (lin, ang Vec3) {
	t := tMs / 1000.0
	lin = Vec3{X: tk.v0.X - tk.linA/tk.linW*(1-math.Cos(tk.linW*t)), Z: tk.v0.Z}
	worldOmega := Vec3{Y: tk.wY0 - tk.angA/tk.angW*(1-math.Cos(tk.angW*t))}
	inv := Quat{W: tk.GroundTruth(tMs).Q.W, X: -tk.GroundTruth(tMs).Q.X,
		Y: -tk.GroundTruth(tMs).Q.Y, Z: -tk.GroundTruth(tMs).Q.Z}
	ang = inv.Rotate(worldOmega)
	return lin, ang
}

// ScheduleOutage mutes every sample in the half-open interval [fromMs, toMs).
func (tk *Tracker) ScheduleOutage(fromMs, toMs float64) {
	tk.muteFrom, tk.mutedUntil = fromMs, toMs
}

// Relocate swaps the tracking origin. newFromCanonical maps the canonical
// room frame into the new origin frame (a large offset for cross-room moves).
func (tk *Tracker) Relocate(atMs float64, newOrigin string, newFromCanonical Canonical) {
	old := tk.originFrom
	tk.reloc = &RelocEvent{
		AtMs:       atMs,
		FromOrigin: tk.origin,
		ToOrigin:   newOrigin,
		// Origin transforms map canonical -> origin. A point p in the old
		// origin satisfies p = A(c); hence c = A^-1(p) and
		// p_new = B(A^-1(p)) = (B o A^-1)(p).
		Delta: composeBInvA(newFromCanonical, old),
	}
	tk.origin = newOrigin
	tk.originFrom = newFromCanonical
}

// composeBInvA returns B o A^-1: maps an old-origin point (frame A) into the
// new origin (frame B), where both A and B map canonical -> origin.
func composeBInvA(b, a RigidTransform) RigidTransform {
	aInv := a.Invert()
	// (B o A^-1)(p) = Rb*(Rai*p + Tai) + Tb
	//               = (Rb*Rai)*p + (Rb*Tai + Tb)
	return RigidTransform{
		Q: b.Q.Multiply(aInv.Q),
		T: b.Q.Rotate(aInv.T).Add(b.T),
	}
}

// Poll returns the latest available sample at the current clock time, or nil
// while tracking is muted. A pending reloc event is delivered exactly once,
// alongside the first sample expressed in the new origin.
func (tk *Tracker) Poll() (*Sample, *RelocEvent) {
	now := tk.clock.Now()
	if now >= tk.muteFrom && now < tk.mutedUntil {
		return nil, nil
	}
	var ev *RelocEvent
	if tk.reloc != nil && now >= tk.reloc.AtMs {
		ev, tk.reloc = tk.reloc, nil
	}
	g := tk.GroundTruth(now)
	s := &Sample{TMs: now, Pose: g.Transform(tk.originFrom), Origin: tk.origin}
	return s, ev
}

// SpawnAnchor creates a virtual anchor at the device's current ground-truth
// position and records it in the tracker's current origin frame.
func (tk *Tracker) SpawnAnchor(id, color string) Anchor {
	g := tk.GroundTruth(tk.clock.Now())
	return Anchor{ID: id, Color: color, Pose: g.Transform(tk.originFrom), Origin: tk.origin}
}

// ReprojectAnchor re-expresses an anchor when the origin changes.
func (tk *Tracker) ReprojectAnchor(a Anchor, ev RelocEvent) Anchor {
	if a.Origin != ev.FromOrigin {
		return a
	}
	return Anchor{ID: a.ID, Color: a.Color, Pose: a.Pose.Transform(ev.Delta), Origin: ev.ToOrigin}
}

// AnchorWorldError measures an anchor's stored pose against its ground-truth
// canonical position (the anchor should remain glued to the real world).
func (tk *Tracker) AnchorWorldError(a Anchor, canonicalPose Pose) float64 {
	// Stored pose is in current origin; map it back to canonical via the
	// inverse of the origin transform.
	inCanonical := a.Pose.Transform(tk.originFrom.Invert())
	return PositionError(inCanonical, canonicalPose)
}
