package pose

// QueryResult is the outcome of a request for the pose at one time.
type QueryResult struct {
	Sample Sample
	Status int
	// Distance to the nearest buffered timestamp (negative when querying the
	// past, positive the future); zero on an exact hit.
	NearestDelta Timestamp
}

// Available reports whether Sample holds a usable pose.
func (r QueryResult) Available() bool {
	return r.Status == StatusOK || r.Status == StatusExtrapolated
}

// Query resamples the buffered trajectory at time t. Translation uses linear
// interpolation and orientation uses slerp, so the two channels follow their
// own geometry. Extrapolation is allowed only within Config.Extrapolate;
// interpolation is refused across gaps larger than Config.MaxGap.
func (b *Buffer) Query(t Timestamp) QueryResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stats.Queries++

	if b.size == 0 {
		b.stats.Unavailable++
		return QueryResult{Status: StatusEmpty, NearestDelta: 0}
	}

	oldest, newest := b.at(0), b.at(b.size-1)
	if t < oldest.Time {
		b.stats.Unavailable++
		return QueryResult{Status: StatusBeforeWindow, NearestDelta: t - oldest.Time}
	}
	if t > newest.Time {
		if t-newest.Time > b.cfg.Extrapolate {
			b.stats.Unavailable++
			return QueryResult{Status: StatusAfterWindow, NearestDelta: t - newest.Time}
		}
		r := b.extrapolateLocked(newest, t)
		b.stats.Extrapolations++
		return r
	}

	if pos, ok := b.search(t); ok {
		b.stats.ExactHits++
		return QueryResult{Sample: b.at(pos), Status: StatusOK}
	}
	pos := b.insertionIndex(t) // first sample strictly after t
	prev, next := b.at(pos-1), b.at(pos)
	gap := next.Time - prev.Time
	if gap > b.cfg.MaxGap {
		b.stats.Unavailable++
		return QueryResult{
			Status:       StatusGapTooLarge,
			NearestDelta: nearestDelta(t, prev.Time, next.Time),
		}
	}

	frac := float64(t-prev.Time) / float64(gap)
	out := Sample{
		Time: t,
		Pos:  Lerp(prev.Pos, next.Pos, frac),
		Rot:  Slerp(prev.Rot, next.Rot, frac),
	}
	b.stats.Interpolations++
	return QueryResult{Sample: out, Status: StatusOK, NearestDelta: 0}
}

// QueryNearest returns the closest stored sample without resampling. It obeys
// the same window and extrapolation limits as Query so a nearest-neighbor
// consumer cannot silently use data that is too far away.
func (b *Buffer) QueryNearest(t Timestamp) QueryResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stats.Queries++

	if b.size == 0 {
		b.stats.Unavailable++
		return QueryResult{Status: StatusEmpty}
	}

	oldest, newest := b.at(0), b.at(b.size-1)
	switch {
	case t < oldest.Time:
		if oldest.Time-t > b.cfg.Extrapolate {
			b.stats.Unavailable++
			return QueryResult{Status: StatusBeforeWindow, NearestDelta: t - oldest.Time}
		}
		b.stats.NearestFallback++
		return QueryResult{Sample: oldest, Status: StatusExtrapolated, NearestDelta: t - oldest.Time}
	case t > newest.Time:
		if t-newest.Time > b.cfg.Extrapolate {
			b.stats.Unavailable++
			return QueryResult{Status: StatusAfterWindow, NearestDelta: t - newest.Time}
		}
		b.stats.NearestFallback++
		return QueryResult{Sample: newest, Status: StatusExtrapolated, NearestDelta: t - newest.Time}
	}

	if pos, ok := b.search(t); ok {
		b.stats.ExactHits++
		return QueryResult{Sample: b.at(pos), Status: StatusOK}
	}
	pos := b.insertionIndex(t)
	prev, next := b.at(pos-1), b.at(pos)
	if t-prev.Time <= next.Time-t {
		b.stats.NearestFallback++
		return QueryResult{Sample: prev, Status: StatusOK, NearestDelta: t - prev.Time}
	}
	b.stats.NearestFallback++
	return QueryResult{Sample: next, Status: StatusOK, NearestDelta: t - next.Time}
}

func (b *Buffer) extrapolateLocked(anchor Sample, t Timestamp) QueryResult {
	// Hold the last observed pose. Pose extrapolation beyond a short horizon is
	// deliberately avoided: angular/linear velocity estimates from two noisy
	// samples diverge quickly, and callers receive StatusAfterWindow instead.
	return QueryResult{
		Sample:       Sample{Time: t, Pos: anchor.Pos, Rot: anchor.Rot},
		Status:       StatusExtrapolated,
		NearestDelta: t - anchor.Time,
	}
}

func nearestDelta(t, prev, next Timestamp) Timestamp {
	d0, d1 := t-prev, next-t
	if d0 <= d1 {
		return d0
	}
	return -d1
}

// Oldest and Newest expose the current window edges.
func (b *Buffer) Oldest() (Sample, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size == 0 {
		return Sample{}, false
	}
	return b.at(0), true
}

// Newest returns the most recent buffered sample.
func (b *Buffer) Newest() (Sample, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.size == 0 {
		return Sample{}, false
	}
	return b.at(b.size - 1), true
}
