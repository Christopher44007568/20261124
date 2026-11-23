package pose

import "sync"

// Domain is the identity of a clock. Samples from different clocks never
// share ordering, interpolation or extrapolation: there is no timestamp
// conversion here, so a sample can never leak across domains.
type Domain string

// Hub owns one independent Buffer per clock domain. A query for an unknown
// domain yields StatusEmpty rather than falling back to another domain.
type Hub struct {
	mu      sync.RWMutex
	cfg     Config
	buffers map[Domain]*Buffer
}

// NewHub builds a domain registry. Every per-domain buffer inherits cfg.
func NewHub(cfg Config) *Hub {
	return &Hub{cfg: cfg, buffers: make(map[Domain]*Buffer)}
}

// Buffer returns the buffer for a domain, creating it on first use.
func (h *Hub) Buffer(d Domain) *Buffer {
	h.mu.RLock()
	buf, ok := h.buffers[d]
	h.mu.RUnlock()
	if ok {
		return buf
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if buf, ok = h.buffers[d]; ok {
		return buf
	}
	buf = New(h.cfg)
	h.buffers[d] = buf
	return buf
}

// Insert routes one sample to its own clock-domain buffer.
func (h *Hub) Insert(d Domain, s Sample) { h.Buffer(d).Insert(s) }

// Domains lists the domains that have seen at least one buffer allocation.
func (h *Hub) Domains() []Domain {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]Domain, 0, len(h.buffers))
	for d := range h.buffers {
		out = append(out, d)
	}
	return out
}
