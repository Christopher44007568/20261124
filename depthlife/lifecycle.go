package depthlife

import "fmt"

// Load decodes raw and binds the attachment to one eye slot.
//
// Loading the identical attachment again is an idempotent cross-frame reuse:
// the slot keeps its derived artifacts and a parse-reuse event is not
// fabricated. A different payload with a changed format version wipes the
// slot (format change invalidates every derived artifact) and emits
// format-changed. A different payload with the same version replaces the
// attachment without leaving stale depth/mask/template data behind.
func (m *Manager) Load(eye Eye, raw []byte, frameSeq uint64) (*Attachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, err
	}
	att, err := DecodeAttachment(raw)
	if err != nil {
		return nil, err
	}
	if s.att != nil && s.att.ID == att.ID {
		s.frameSeq = frameSeq
		m.emit(s, EvLoaded, fmt.Sprintf("same attachment re-bound frame=%d", frameSeq))
		return &att, nil
	}
	versionChanged := s.att != nil && s.att.Version != att.Version
	if versionChanged {
		s.depth = nil
		s.mask = nil
		s.template = nil
		s.state = Empty
		m.emitVersion(s, EvFormatChanged, att.Version,
			fmt.Sprintf("format %d -> %d; derived artifacts invalidated",
				s.att.Version, att.Version))
	}
	s.att = &att
	s.depth = nil
	s.mask = nil
	s.template = nil
	s.state = Loaded
	s.frameSeq = frameSeq
	m.emit(s, EvLoaded, fmt.Sprintf("attachment loaded frame=%d ver=%d", frameSeq, att.Version))
	return &att, nil
}

// Parse parses the loaded attachment for the eye. When the depth result for
// the current attachment and eye already exists, it is reused across frames
// and a parse-reused event is emitted instead of re-decoding every sample.
func (m *Manager) Parse(eye Eye) (*DepthResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, err
	}
	if s.att == nil {
		m.emit(s, EvInvalidTransition, "parse before load")
		return nil, fmt.Errorf("%w: parse requires Load first", ErrNotLoaded)
	}
	if s.depth != nil {
		if gerr := m.guard(s, s.depth); gerr != nil {
			return nil, gerr
		}
		m.emit(s, EvParseReused, "depth result reused across frames")
		return s.depth, nil
	}
	s.depth = parseDepth(*s.att, eye)
	s.mask = nil
	s.template = nil
	if s.state < Parsed {
		s.state = Parsed
	}
	m.emit(s, EvParsed,
		fmt.Sprintf("depth parsed samples=%d mean=%d", len(s.depth.Z), s.depth.MeanZ))
	return s.depth, nil
}

// ApplyMask computes the threshold mask for a parsed eye slot. The result is
// cached for cross-frame reuse as long as the attachment does not change.
// Rejected masks stay visible in the slot (state Masked, Passed=false) so
// callers and logs can distinguish "rejected" from "not attempted"; saving a
// rejected template is refused.
func (m *Manager) ApplyMask(eye Eye, threshold uint16, minCoverage float64) (*MaskResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, err
	}
	if s.att == nil {
		return nil, fmt.Errorf("%w: mask requires Load first", ErrNotLoaded)
	}
	if s.depth == nil {
		m.emit(s, EvInvalidTransition, "mask before parse")
		return nil, fmt.Errorf("%w: mask requires Parse first", ErrNotParsed)
	}
	if gerr := m.guard(s, s.depth); gerr != nil {
		return nil, gerr
	}
	if s.mask != nil && s.mask.Threshold == threshold {
		if gerr := m.guard(s, s.mask); gerr != nil {
			return nil, gerr
		}
		m.emit(s, EvMaskReused, "mask reused across frames")
		return s.mask, nil
	}
	s.mask = applyMask(s.depth, threshold, minCoverage)
	s.template = nil
	s.state = Masked
	if s.mask.Passed {
		m.emit(s, EvMaskApplied,
			fmt.Sprintf("mask passed coverage=%.4f threshold=%d", s.mask.Coverage, threshold))
	} else {
		m.emit(s, EvMaskRejected,
			fmt.Sprintf("mask rejected coverage=%.4f threshold=%d min=%.4f",
				s.mask.Coverage, threshold, minCoverage))
	}
	return s.mask, nil
}
