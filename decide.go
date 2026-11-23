package foveq

// Observe ingests one frame sample, runs the joint rule engine and returns
// the profile that should be active for the next frame. It never mutates the
// ladder and never adjusts any rendering knob independently of the tier.
func (c *Controller) Observe(s Sample) (Profile, Decision) {
	<-c.mu
	defer func() { c.mu <- struct{}{} }()

	c.frame++
	frame := c.frame
	fromTier := c.tier
	reasons := make([]string, 0, 6)

	sm := smoothedInputs{
		GPU:        c.gpuEMA.update(s.GPUMicros),
		Center:     c.centerEMA.update(s.CenterError),
		Peripheral: c.periphEMA.update(s.PeripheralError),
		Gaze:       c.gazeEMA.update(s.GazeVelocity),
	}

	spike := s.LoadSpike || frame < c.spikeUntil
	if s.LoadSpike {
		c.spikeUntil = frame + c.cfg.SpikeCooldownFrames
	}
	fastGaze := sm.Gaze >= c.cfg.FastGazeDps

	profile, _ := c.cfg.Ladder.At(c.tier)
	budget := profile.FrameBudgetMicros
	overrun := sm.GPU > budget
	headroom := budget - sm.GPU

	d := Decision{
		Frame:    frame,
		Raw:      s,
		Smoothed: sm,
		Budget:   budget,
		Headroom: headroom,
		FromTier: fromTier,
		ToTier:   fromTier,
		Spike:    spike,
		FastGaze: fastGaze,
		InTrial:  c.inTrial,
		LastGood: c.lastGood,
	}

	// Failures always win: an aborted frame cannot justify more quality.
	if s.Failed {
		c.handleFailure(frame, &d, &reasons)
	} else if c.inTrial {
		c.handleTrial(frame, sm, overrun, &d, &reasons)
	} else {
		c.handleSteady(frame, sm, spike, fastGaze, overrun, headroom, &d, &reasons)
	}

	if len(reasons) == 0 {
		reasons = append(reasons, ReasonHealthy)
	}
	d.Reasons = reasons
	d.LastGood = c.lastGood

	// A clean, on-budget, non-trial frame confirms the current tier. Note the
	// order matters: trial rollback may have just restored lastGood's tier,
	// and an over-budget frame must never overwrite that confirmation.
	if !s.Failed && !overrun && !c.inTrial && !spike && c.tier == c.lastGood {
		c.lastGood = c.tier
	}

	c.wasSpike = spike
	c.wasFastGaze = fastGaze

	newProfile, _ := c.cfg.Ladder.At(c.tier)
	d.ToTier = c.tier
	c.emit(d, newProfile)
	return newProfile, d
}

// handleFailure rolls a failed trial back to the last known-good profile, or
// downshifts by one tier in steady state. The rejected tier is put in a
// failure cooldown so it cannot be immediately reselected.
func (c *Controller) handleFailure(frame int64, d *Decision, reasons *[]string) {
	*reasons = append(*reasons, ReasonFrameFailure)
	c.record(d.Frame, EventFrameFailure, map[string]any{
		"tier": c.tier, "last_good": c.lastGood,
	})

	if c.inTrial {
		rejected := c.trialTo
		c.blockedUntil[rejected] = frame + c.cfg.FailureCooldownFrames
		c.tier = c.trialFrom
		c.inTrial = false
		*reasons = append(*reasons, ReasonTrialRollback)
		c.record(d.Frame, EventTrialRollback, map[string]any{
			"from": rejected, "to": c.trialFrom,
			"blocked_until_frame": c.blockedUntil[rejected],
		})
		c.shiftLocked(frame)
		return
	}

	c.blockedUntil[c.tier] = frame + c.cfg.FailureCooldownFrames
	c.requestShift(frame, d, reasons, -1, ReasonFrameFailure)
}

// handleTrial evaluates a tier that was recently selected but not yet
// confirmed. Sustained overrun or any failure rolls back to the previous
// usable profile; surviving the full trial window promotes it as last-good.
func (c *Controller) handleTrial(
	frame int64, sm smoothedInputs, overrun bool,
	d *Decision, reasons *[]string,
) {
	*reasons = append(*reasons, ReasonTrial)
	if overrun {
		c.overCounter++
	} else {
		c.overCounter = 0
	}
	if c.overCounter >= c.cfg.OverrunHoldFrames {
		rejected := c.trialTo
		c.blockedUntil[rejected] = frame + c.cfg.FailureCooldownFrames
		c.tier = c.trialFrom
		c.inTrial = false
		c.overCounter = 0
		*reasons = append(*reasons, ReasonTrialRollback)
		c.record(d.Frame, EventTrialRollback, map[string]any{
			"from": rejected, "to": c.trialFrom,
			"blocked_until_frame": c.blockedUntil[rejected],
		})
		c.shiftLocked(frame)
		return
	}
	if frame >= c.trialUntil {
		c.inTrial = false
		*reasons = append(*reasons, ReasonTrialPromote)
		c.record(d.Frame, EventTrialPromote, map[string]any{
			"tier": c.tier, "trial_frames": c.cfg.TrialFrames,
		})
	}
}

// handleSteady applies the hysteresis rule engine in normal operation.
func (c *Controller) handleSteady(
	frame int64, sm smoothedInputs, spike, fastGaze, overrun bool, headroom float64,
	d *Decision, reasons *[]string,
) {
	// --- Downshift path: GPU pressure, with sustained hold + boundary clamp.
	if overrun {
		c.overCounter++
		*reasons = append(*reasons, ReasonGPUOverrun)
		c.record(d.Frame, EventGpuPressure, map[string]any{
			"gpu_smoothed": sm.GPU, "budget": d.Budget,
			"consecutive": c.overCounter,
		})
		if spike {
			*reasons = append(*reasons, ReasonLoadSpike)
			c.record(d.Frame, EventSpikeFreeze, map[string]any{"direction": "down"})
		}
		if c.overCounter >= c.cfg.OverrunHoldFrames && !spike {
			c.requestShift(frame, d, reasons, -1, ReasonGPUOverrun)
		}
		c.upCounter = 0
		return
	}
	c.overCounter = 0

	// --- Upshift path: error demand + headroom, gated by hysteresis.
	demand := sm.Center >= c.cfg.CenterErrorThreshold ||
		sm.Peripheral >= c.cfg.PeripheralErrorThreshold
	if !demand {
		c.upCounter = 0
		*reasons = append(*reasons, ReasonNoQualityDemand)
		return
	}
	if sm.Center >= c.cfg.CenterErrorThreshold {
		*reasons = append(*reasons, ReasonCenterError)
	}
	if sm.Peripheral >= c.cfg.PeripheralErrorThreshold {
		*reasons = append(*reasons, ReasonPeripheralError)
	}

	if headroom < d.Budget*c.cfg.HeadroomRatio {
		c.upCounter = 0
		*reasons = append(*reasons, ReasonHeadroomLack)
		return
	}
	if spike {
		c.upCounter = 0
		*reasons = append(*reasons, ReasonLoadSpike)
		c.record(d.Frame, EventSpikeFreeze, map[string]any{"direction": "up"})
		return
	}
	if fastGaze {
		c.upCounter = 0
		*reasons = append(*reasons, ReasonFastGaze)
		c.record(d.Frame, EventGazeFreeze, map[string]any{
			"gaze_smoothed": sm.Gaze,
			"threshold":     c.cfg.FastGazeDps,
		})
		return
	}

	c.upCounter++
	if c.upCounter < c.cfg.UpshiftHoldFrames {
		*reasons = append(*reasons, ReasonUpshiftHeld)
		c.record(d.Frame, EventBlockedUp, map[string]any{
			"held_frames": c.upCounter,
			"required":    c.cfg.UpshiftHoldFrames,
		})
		return
	}
	c.requestShift(frame, d, reasons, +1, "")
}

// requestShift attempts a one-tier move in direction (+1 up, -1 down). It
// enforces the global shift cooldown, failure-cooldown tiers and ladder
// boundaries, so a downshift can never cross the minimum tier.
func (c *Controller) requestShift(
	frame int64, d *Decision, reasons *[]string, direction int, trigger string,
) {
	if frame-c.lastShift < c.cfg.ShiftCooldownFrames && c.lastShift != 0 {
		*reasons = append(*reasons, ReasonCooldown)
		c.record(d.Frame, EventCooldown, map[string]any{
			"frames_since_shift": frame - c.lastShift,
			"cooldown":           c.cfg.ShiftCooldownFrames,
		})
		return
	}

	target := c.cfg.Ladder.clampTier(c.tier + direction)
	if target == c.tier {
		*reasons = append(*reasons, ReasonBoundaryClamp)
		c.record(d.Frame, EventClamp, map[string]any{
			"tier": c.tier, "direction": direction,
		})
		return
	}
	if until, blocked := c.blockedUntil[target]; blocked && frame < until {
		*reasons = append(*reasons, ReasonTierBlocked)
		c.record(d.Frame, EventBlockedUp, map[string]any{
			"target_tier": target, "blocked_until_frame": until,
		})
		return
	}

	kind := EventUpshift
	if direction < 0 {
		kind = EventDownshift
	}
	prev := c.tier
	c.tier = target
	c.lastShift = frame
	c.upCounter = 0
	c.overCounter = 0

	if direction > 0 {
		// Every promoted tier starts on probation and must survive TrialFrames.
		c.inTrial = true
		c.trialFrom = prev
		c.trialTo = target
		c.trialUntil = frame + c.cfg.TrialFrames
		c.upCounter = 0
		c.overCounter = 0
		if trigger != "" {
			*reasons = append(*reasons, trigger)
		}
	}
	c.record(d.Frame, kind, map[string]any{
		"from": prev, "to": target, "trigger": trigger,
	})
}

// shiftLocked records that a tier change just happened, resetting cooldowns.
func (c *Controller) shiftLocked(frame int64) {
	c.lastShift = frame
	c.upCounter = 0
	c.overCounter = 0
}

func (c *Controller) record(frame int64, kind EventKind, detail map[string]any) {
	if c.cfg.Telemetry != nil {
		c.cfg.Telemetry.RecordEvent(frame, kind, detail)
	}
}

func (c *Controller) emit(d Decision, p Profile) {
	if c.cfg.Telemetry == nil {
		return
	}
	c.cfg.Telemetry.RecordDecision(d)
	c.cfg.Telemetry.RecordResolution(ResolutionPoint{
		Frame:        d.Frame,
		Tier:         p.Tier,
		EyeWidth:     p.EyeWidth,
		EyeHeight:    p.EyeHeight,
		FoveaScale:   p.FoveaScale,
		BudgetMicros: p.FrameBudgetMicros,
	})
}
