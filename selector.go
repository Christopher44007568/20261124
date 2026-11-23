package posetimeline

// Selection is the result of a re-projection pose lookup.
type Selection struct {
	Sample     Sample
	GlobalTime int64
	Age        int64 // targetTime - GlobalTime, always >= 0
}

// SelectForReprojection returns the pose to re-project with for the given eye
// and targetTime on the primary clock.
//
// Selection rule: the most recently ACCEPTED sample whose translated
// timestamp is <= targetTime. Future, duplicate, expired and backwards
// samples are never returned, and the lookup scans every accepted entry
// instead of assuming the last array element is the newest. When maxAge > 0
// the chosen pose must also be no older than targetTime-maxAge. A broken or
// empty timeline yields ok=false; poses are never extrapolated into the
// future.
func (v *Validator) SelectForReprojection(eye Eye, targetTime, maxAge int64) (Selection, bool) {
	if int(eye) >= len(v.eyes) {
		return Selection{}, false
	}
	state := v.eyes[eye]
	if state.broken || len(state.accepted) == 0 {
		return Selection{}, false
	}

	best := -1
	for i, entry := range state.accepted {
		if entry.globalTime > targetTime {
			continue // never re-project from a future pose
		}
		if best == -1 || entry.globalTime > state.accepted[best].globalTime {
			best = i
		}
	}
	if best == -1 {
		return Selection{}, false
	}
	entry := state.accepted[best]
	age := targetTime - entry.globalTime
	if maxAge > 0 && age > maxAge {
		return Selection{}, false
	}
	return Selection{
		Sample:     entry.sample,
		GlobalTime: entry.globalTime,
		Age:        age,
	}, true
}
