package depthlife

import "fmt"

// Save builds and caches the template for an eye whose mask has passed. A
// rejected or missing mask is refused so a failing template is never
// persisted as if it had passed.
func (m *Manager) Save(eye Eye) (*Template, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, nil, err
	}
	if s.att == nil {
		return nil, nil, fmt.Errorf("%w: save requires Load first", ErrNotLoaded)
	}
	if s.depth == nil {
		m.emit(s, EvInvalidTransition, "save before parse")
		return nil, nil, fmt.Errorf("%w: save requires Parse first", ErrNotParsed)
	}
	if s.mask == nil {
		m.emit(s, EvInvalidTransition, "save before mask")
		return nil, nil, fmt.Errorf("%w: save requires ApplyMask first", ErrNotMasked)
	}
	if gerr := m.guard(s, s.depth); gerr != nil {
		return nil, nil, gerr
	}
	if gerr := m.guard(s, s.mask); gerr != nil {
		return nil, nil, gerr
	}
	if !s.mask.Passed {
		m.emit(s, EvMaskRejected, "save refused: mask did not pass")
		return nil, nil,
			fmt.Errorf("%w: coverage=%.4f below threshold", ErrTemplateRejected, s.mask.Coverage)
	}
	s.template = &Template{
		Machine:   m.machine,
		Eye:       s.eye,
		AttID:     s.att.ID,
		Version:   s.att.Version,
		Width:     s.depth.Width,
		Height:    s.depth.Height,
		Z:         append([]uint16(nil), s.depth.Z...),
		Threshold: s.mask.Threshold,
		Coverage:  s.mask.Coverage,
		Passed:    true,
		Mask:      append([]bool(nil), s.mask.Mask...),
	}
	blob, err := s.template.Encode()
	if err != nil {
		s.template = nil
		return nil, nil, err
	}
	s.state = Saved
	m.emit(s, EvSaved, fmt.Sprintf("template saved bytes=%d", len(blob)))
	return s.template, blob, nil
}

// FrameResult is what Reuse hands back for a frame.
type FrameResult struct {
	Eye      Eye
	FrameSeq uint64
	Reused   bool
	State    State
	Depth    *DepthResult
	Mask     *MaskResult
	Template *Template
}

// Reuse reports whether the artifacts cached for eye are still valid for the
// attachment identified by attID. It performs no mutation. Any attempt to
// cross eye boundaries or reuse another attachment is logged (stale-eye /
// stale-attachment / reuse-rejected) and returns an error instead of
// returning stale data.
func (m *Manager) Reuse(eye Eye, attID string, frameSeq uint64) (*FrameResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, err
	}
	if s.att == nil {
		m.emit(s, EvInvalidTransition, "reuse on empty slot")
		return nil, ErrEmptySlot
	}
	if s.att.ID != attID {
		m.emit(s, EvReuseRejected,
			fmt.Sprintf("cached att=%s requested for att=%s frame=%d",
				s.att.ID, attID, frameSeq))
		return nil, fmt.Errorf("%w: cached=%s requested=%s",
			ErrAttachmentMoved, s.att.ID, attID)
	}
	if s.depth != nil {
		if gerr := m.guard(s, s.depth); gerr != nil {
			m.emit(s, EvReuseRejected, "stale eye data blocked on reuse")
			return nil, gerr
		}
	}
	s.frameSeq = frameSeq
	return &FrameResult{
		Eye:      s.eye,
		FrameSeq: frameSeq,
		Reused:   s.depth != nil,
		State:    s.state,
		Depth:    s.depth,
		Mask:     s.mask,
		Template: s.template,
	}, nil
}

// Restore decodes a saved template blob and verifies it belongs to the
// requesting eye and machine before binding it to the slot. A foreign eye or
// machine tag is treated as stale data and refused with a stale-eye event.
func (m *Manager) Restore(eye Eye, blob []byte) (*Template, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return nil, err
	}
	t, err := DecodeTemplate(blob)
	if err != nil {
		return nil, err
	}
	if t.Machine != m.machine || t.Eye != eye {
		m.emit(s, EvStaleEye,
			fmt.Sprintf("restore blob machine=%s eye=%s into machine=%s eye=%s",
				t.Machine, t.Eye, m.machine, eye))
		return nil, fmt.Errorf("%w: blob machine=%s eye=%s", ErrStaleEye, t.Machine, t.Eye)
	}
	s.template = t
	if s.state < Saved {
		s.state = Saved
	}
	m.emit(s, EvSaved, "template restored from blob")
	return t, nil
}
