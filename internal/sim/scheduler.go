package sim

type allocation struct {
	tiers [2][2]Tier
	used  resourceUse
	flags FallbackFlags
}

type resourceUse struct {
	samples int
	redraw  int
	eye     [2]int
}

func (u resourceUse) add(spec TierSpec) resourceUse {
	u.samples += spec.Samples
	u.redraw += spec.Redraw
	return u
}

func (a *allocation) reserveFloor(profile Profile) bool {
	for _, key := range JobKeys() {
		a.tiers[key.Eye][key.Region] = TierMin
	}
	used, ok := a.usage(profile)
	if !ok {
		a.used = resourceUse{}
		return false
	}
	a.used = used
	return true
}

func (a *allocation) usage(profile Profile) (resourceUse, bool) {
	var used resourceUse
	for _, key := range JobKeys() {
		spec := profile.Spec(key.Region, a.tiers[key.Eye][key.Region])
		used.samples += spec.Samples
		used.redraw += spec.Redraw
		used.eye[key.Eye] += spec.EyeBuffer
	}
	return used, true
}

func (a *allocation) fits(profile Profile, budget Budget) (bool, FallbackFlags) {
	var used resourceUse
	for _, key := range JobKeys() {
		spec := profile.Spec(key.Region, a.tiers[key.Eye][key.Region])
		used = used.add(spec)
		used.eye[key.Eye] += spec.EyeBuffer
	}

	var flags FallbackFlags
	if used.samples > budget.Samples || used.redraw > budget.Redraw {
		flags.Budget = true
	}
	for _, eye := range Eyes {
		if used.eye[eye] > budget.EyeBuffer[eye] {
			flags.EyeCap = true
		}
	}
	if flags.Count() != 0 {
		return false, flags
	}
	a.used = used
	return true, flags
}

func (a *allocation) tryUpgrade(key JobKey, profile Profile, budget Budget) (bool, FallbackFlags) {
	current := a.tiers[key.Eye][key.Region]
	if current >= TierFull {
		return false, FallbackFlags{}
	}
	a.tiers[key.Eye][key.Region] = current + 1
	ok, flags := a.fits(profile, budget)
	if !ok {
		a.tiers[key.Eye][key.Region] = current
		return false, flags
	}
	return true, flags
}

func allocateEqual(a *allocation, profile Profile, budget Budget) FallbackFlags {
	var blocked FallbackFlags
	for {
		progress := false
		for _, key := range JobKeys() {
			if a.tiers[key.Eye][key.Region] == TierFull {
				continue
			}
			ok, flags := a.tryUpgrade(key, profile, budget)
			if ok {
				progress = true
			} else {
				blocked = blocked.Merge(flags)
			}
		}
		if !progress {
			return blocked
		}
	}
}

func allocateGazeRegion(a *allocation, profile Profile, budget Budget) FallbackFlags {
	var blocked FallbackFlags
	flip := false
	for _, region := range Regions {
		for {
			progress := false
			order := [2]Eye{Left, Right}
			if flip {
				order = [2]Eye{Right, Left}
			}
			for _, eye := range order {
				key := JobKey{Eye: eye, Region: region}
				if a.tiers[eye][region] == TierFull {
					continue
				}
				ok, flags := a.tryUpgrade(key, profile, budget)
				if ok {
					progress = true
				} else {
					blocked = blocked.Merge(flags)
				}
			}
			if !progress {
				break
			}
			flip = !flip
		}
	}
	return blocked
}

func allocateHistoricalError(a *allocation, profile Profile, budget Budget, history [2][2]float64) FallbackFlags {
	var blocked FallbackFlags
	var exhausted [2][2]bool
	for {
		best, found := bestUpgrade(a, history, &exhausted)
		if !found {
			return blocked
		}
		ok, flags := a.tryUpgrade(best, profile, budget)
		if ok {
			continue
		}
		exhausted[best.Eye][best.Region] = true
		blocked = blocked.Merge(flags)
	}
}

func bestUpgrade(a *allocation, history [2][2]float64, exhausted *[2][2]bool) (JobKey, bool) {
	var best JobKey
	found := false
	for _, key := range JobKeys() {
		if a.tiers[key.Eye][key.Region] == TierFull || exhausted[key.Eye][key.Region] {
			continue
		}
		if !found || betterHistory(key, best, history) {
			best = key
			found = true
		}
	}
	return best, found
}

func betterHistory(a, b JobKey, history [2][2]float64) bool {
	av := history[a.Eye][a.Region]
	bv := history[b.Eye][b.Region]
	if av != bv {
		return av > bv
	}
	if a.Region != b.Region {
		return a.Region == Center
	}
	return a.Eye == Left
}
