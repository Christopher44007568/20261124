package posetimeline

// ClockRegistry translates ticks from secondary clock domains onto the
// primary domain. Each secondary domain needs an affine offset derived from a
// shared sync event:
//
//	primaryTick = secondaryTick + offset
//
// No rate estimation is performed: when drift matters the caller must refresh
// the offset. A domain without a registered offset is treated as unknown and
// its samples are rejected rather than compared across domains.
type ClockRegistry struct {
	offsets map[string]int64
}

// NewClockRegistry creates an empty registry. The primary domain ("") is
// always known and has a zero offset.
func NewClockRegistry() *ClockRegistry {
	return &ClockRegistry{offsets: map[string]int64{}}
}

// Register records the offset for a clock domain. Registering "" resets the
// primary-domain offset, which is normally left at zero.
func (r *ClockRegistry) Register(clock string, offset int64) {
	r.offsets[clock] = offset
}

// Known reports whether ticks from clock can be translated.
func (r *ClockRegistry) Known(clock string) bool {
	if clock == "" {
		return true
	}
	_, ok := r.offsets[clock]
	return ok
}

// Translate maps one tick from clock onto the primary domain.
func (r *ClockRegistry) Translate(clock string, tick int64) (int64, bool) {
	if clock == "" {
		return tick, true
	}
	offset, ok := r.offsets[clock]
	if !ok {
		return 0, false
	}
	return tick + offset, true
}
