package pose

import (
	"math"
	"sync"
)

// Query statuses. These describe *why* a pose at a requested time was or was
// not produced, which makes clock-rollback and out-of-window failures explicit
// instead of silently returning stale data.
const (
	StatusOK           = iota // exact match or interpolation between samples
	StatusExtrapolated        // within Extrapolate of the buffered horizon
	StatusEmpty               // no samples have ever been inserted
	StatusBeforeWindow        // requested time is older than the oldest sample
	StatusAfterWindow         // requested time is beyond horizon + Extrapolate
	StatusGapTooLarge         // bracketing gap exceeds MaxInterpolateGap
)

// StatusName maps a status code to a stable diagnostic label.
func StatusName(status int) string {
	switch status {
	case StatusOK:
		return "OK"
	case StatusExtrapolated:
		return "extrapolated"
	case StatusEmpty:
		return "empty"
	case StatusBeforeWindow:
		return "before_window"
	case StatusAfterWindow:
		return "after_window"
	case StatusGapTooLarge:
		return "gap_too_large"
	default:
		return "unknown"
	}
}

// Config sizes the fixed-length history. Zero values fall back to defaults.
type Config struct {
	Capacity        int       // ring length in samples
	Extrapolate     Timestamp // allowed forward/backward extension past data
	MaxGap          Timestamp // gaps larger than this are not interpolated
	OnGapExceeded   func(gap Timestamp)
	OnClockRollback func(delta Timestamp)
}

// Stats accumulates lifecycle counters since the buffer was created.
type Stats struct {
	Inserts         int64
	Duplicates      int64 // same-timestamp sample replaced an earlier one
	Evictions       int64 // oldest sample dropped because the ring was full
	LargestGap      Timestamp
	GapsOverLimit   int64
	ClockRollbacks  int64
	Queries         int64
	ExactHits       int64
	Interpolations  int64
	Extrapolations  int64
	NearestFallback int64
	Unavailable     int64
}

// Buffer is a fixed-capacity, time-ordered ring of pose samples from one
// clock domain. It is safe for concurrent use.
type Buffer struct {
	cfg   Config
	mu    sync.Mutex
	ring  []Sample
	head  int // index of the oldest sample
	size  int
	last  Timestamp // timestamp of the newest sample (monotonic per domain)
	stats Stats
}

// New builds a buffer with validated/defaulted configuration.
func New(cfg Config) *Buffer {
	if cfg.Capacity <= 0 {
		cfg.Capacity = 1024
	}
	if cfg.Extrapolate <= 0 {
		cfg.Extrapolate = Timestamp(25_000_000) // 25 ms
	}
	if cfg.MaxGap <= 0 {
		cfg.MaxGap = Timestamp(500_000_000) // 500 ms
	}
	return &Buffer{cfg: cfg, ring: make([]Sample, cfg.Capacity), last: math.MinInt64}
}

// Stats returns a copy of the running counters.
func (b *Buffer) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.stats
}

// Len reports how many samples currently occupy the ring.
func (b *Buffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.size
}

func (b *Buffer) slot(i int) int { return (b.head + i) % b.cfg.Capacity }

func (b *Buffer) at(i int) Sample { return b.ring[b.slot(i)] }

// Insert places a sample into time order. Samples may arrive out of order:
// the ring stays sorted by timestamp. A repeated timestamp overwrites the
// earlier sample instead of duplicating it; a timestamp older than every
// buffered sample is still retained (clock rollback), evicting the newest one
// if necessary.
func (b *Buffer) Insert(s Sample) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stats.Inserts++

	if b.size > 0 && s.Time < b.last {
		b.stats.ClockRollbacks++
		if b.cfg.OnClockRollback != nil {
			b.cfg.OnClockRollback(b.last - s.Time)
		}
	}

	if b.size > 0 {
		newest := b.at(b.size - 1)
		if s.Time == newest.Time { // duplicate of newest: overwrite in place
			b.ring[b.slot(b.size-1)] = s
			b.stats.Duplicates++
			return
		}
		if s.Time < newest.Time {
			if pos, ok := b.search(s.Time); ok { // duplicate inside the ring
				b.ring[b.slot(pos)] = s
				b.stats.Duplicates++
				return
			}
		}
	}

	if b.size == b.cfg.Capacity {
		// A sample older than the oldest one evicts the newest (clock
		// rollback); a normal in-window or newer sample evicts the oldest.
		if s.Time < b.at(0).Time {
			b.size--
		} else {
			b.head = (b.head + 1) % b.cfg.Capacity
			b.size--
		}
		b.stats.Evictions++
	}

	pos := b.insertionIndex(s.Time)
	for i := b.size; i > pos; i-- {
		b.ring[b.slot(i)] = b.ring[b.slot(i-1)]
	}
	b.ring[b.slot(pos)] = s
	b.size++
	b.last = b.at(b.size - 1).Time
	b.scanGapsLocked()
}

// scanGapsLocked walks every adjacent pair. Out-of-order insertion can enlarge
// a gap anywhere in the window, so checking only the newest pair would miss
// outages that arrived late.
func (b *Buffer) scanGapsLocked() {
	if b.size < 2 {
		return
	}
	var largest Timestamp
	var over int64
	for i := 1; i < b.size; i++ {
		gap := b.at(i).Time - b.at(i-1).Time
		if gap > largest {
			largest = gap
		}
		if gap > b.cfg.MaxGap {
			over++
		}
	}
	b.stats.LargestGap = largest
	b.stats.GapsOverLimit = over
	if largest > b.cfg.MaxGap && b.cfg.OnGapExceeded != nil {
		b.cfg.OnGapExceeded(largest)
	}
}

// RecomputeGaps refreshes gap statistics; it is mostly useful in tests.
func (b *Buffer) RecomputeGaps() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.scanGapsLocked()
}

// search returns the logical index of an exact timestamp, if present.
func (b *Buffer) search(t Timestamp) (int, bool) {
	lo, hi := 0, b.size
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		switch {
		case b.at(mid).Time < t:
			lo = mid + 1
		case b.at(mid).Time > t:
			hi = mid
		default:
			return mid, true
		}
	}
	return lo, false
}

func (b *Buffer) insertionIndex(t Timestamp) int {
	lo, hi := 0, b.size
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if b.at(mid).Time < t {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}
