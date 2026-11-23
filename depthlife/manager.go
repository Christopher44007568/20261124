package depthlife

import (
	"fmt"
	"sync"
)

// eyeSlot holds the complete lifecycle of one attachment for one eye.
// All pointers are independently owned; the left and right slots never alias
// parsed, mask or template objects.
type eyeSlot struct {
	eye      Eye
	state    State
	att      *Attachment
	depth    *DepthResult
	mask     *MaskResult
	template *Template
	frameSeq uint64
}

// Manager runs the depth/template lifecycle on one machine (host or right
// machine). Create one per machine; managers do not share slots or caches.
type Manager struct {
	mu      sync.Mutex
	machine Machine
	log     Logger
	slots   [2]*eyeSlot
}

// New creates a Manager for machine. A nil logger becomes a MemoryLogger.
func New(machine Machine, log Logger) *Manager {
	if machine != Host && machine != RightMachine {
		panic("depthlife: unknown machine")
	}
	if log == nil {
		log = NewMemoryLogger()
	}
	m := &Manager{machine: machine, log: log}
	m.slots[0] = &eyeSlot{eye: Left, state: Empty}
	m.slots[1] = &eyeSlot{eye: Right, state: Empty}
	return m
}

// Logger exposes the configured audit logger.
func (m *Manager) Logger() Logger { return m.log }

// Machine reports which machine this manager represents.
func (m *Manager) Machine() Machine { return m.machine }

func (m *Manager) slot(eye Eye) (*eyeSlot, error) {
	if !eye.Valid() {
		return nil, ErrInvalidEye
	}
	return m.slots[eye-1], nil
}

func (m *Manager) emit(s *eyeSlot, kind EventKind, detail string) {
	ev := Event{
		Machine:   m.machine,
		Eye:       s.eye,
		Kind:      kind,
		State:     s.state,
		FormatVer: s.formatVersion(),
	}
	if s.att != nil {
		ev.AttID = s.att.ID
	}
	if detail != "" {
		ev.Detail = detail
	}
	m.log.Log(ev)
}

func (m *Manager) emitVersion(s *eyeSlot, kind EventKind, version uint32, detail string) {
	ev := Event{
		Machine:   m.machine,
		Eye:       s.eye,
		Kind:      kind,
		State:     s.state,
		FormatVer: version,
	}
	if s.att != nil {
		ev.AttID = s.att.ID
	}
	if detail != "" {
		ev.Detail = detail
	}
	m.log.Log(ev)
}

func (s *eyeSlot) formatVersion() uint32 {
	if s.att != nil {
		return s.att.Version
	}
	return 0
}

// Clear drops every artifact of one eye and returns the slot to Empty.
func (m *Manager) Clear(eye Eye) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return err
	}
	oldAtt := ""
	if s.att != nil {
		oldAtt = s.att.ID
	}
	s.att = nil
	s.depth = nil
	s.mask = nil
	s.template = nil
	s.state = Empty
	m.emit(s, EvCleared, "cleared slot (was "+oldAtt+")")
	return nil
}

// ClearAll clears both eye slots.
func (m *Manager) ClearAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.slots {
		s.att = nil
		s.depth = nil
		s.mask = nil
		s.template = nil
		s.state = Empty
		m.emit(s, EvCleared, "cleared slot")
	}
}

// SlotSnapshot is a point-in-time, value-only view of an eye slot.
type SlotSnapshot struct {
	Eye         Eye
	State       State
	FrameSeq    uint64
	AttID       string
	FormatVer   uint32
	HasDepth    bool
	HasMask     bool
	HasTemplate bool
	MaskPassed  bool
}

// Snapshot returns the current state of one eye slot.
func (m *Manager) Snapshot(eye Eye) (SlotSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, err := m.slot(eye)
	if err != nil {
		return SlotSnapshot{}, err
	}
	snap := SlotSnapshot{
		Eye:         s.eye,
		State:       s.state,
		FrameSeq:    s.frameSeq,
		HasDepth:    s.depth != nil,
		HasMask:     s.mask != nil,
		HasTemplate: s.template != nil,
	}
	if s.att != nil {
		snap.AttID = s.att.ID
		snap.FormatVer = s.att.Version
	}
	if s.mask != nil {
		snap.MaskPassed = s.mask.Passed
	}
	return snap, nil
}

// States returns the lifecycle state of both eyes.
func (m *Manager) States() map[Eye]State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[Eye]State{Left: m.slots[0].state, Right: m.slots[1].state}
}

// guard verifies that an eye-scoped artifact may be used by the requesting
// eye on the requesting attachment. Any mismatch is logged with the stale-eye
// or stale-attachment event and converted into a sentinel error.
func (m *Manager) guard(s *eyeSlot, art eyeTagged) error {
	if art == nil {
		return ErrEmptySlot
	}
	if got := art.TaggedEye(); got != s.eye {
		m.emit(s, EvStaleEye,
			fmt.Sprintf("artifact tagged eye=%s presented to eye=%s", got, s.eye))
		return fmt.Errorf("%w: artifact eye=%s, requested eye=%s", ErrStaleEye, got, s.eye)
	}
	if s.att != nil && art.TaggedAttachment() != s.att.ID {
		m.emit(s, EvStaleAttachment,
			fmt.Sprintf("artifact att=%s presented for att=%s", art.TaggedAttachment(), s.att.ID))
		return fmt.Errorf("%w: artifact att=%s, current att=%s",
			ErrStaleAttachment, art.TaggedAttachment(), s.att.ID)
	}
	return nil
}
