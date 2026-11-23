package depthlife

import (
	"fmt"
	"sync"
	"time"
)

// EventKind enumerates every lifecycle transition that is auditable.
type EventKind string

const (
	EvCleared           EventKind = "cleared"
	EvLoaded            EventKind = "loaded"
	EvFormatChanged     EventKind = "format-changed"
	EvParsed            EventKind = "parsed"
	EvParseReused       EventKind = "parse-reused"
	EvMaskApplied       EventKind = "mask-applied"
	EvMaskRejected      EventKind = "mask-rejected"
	EvMaskReused        EventKind = "mask-reused"
	EvSaved             EventKind = "saved"
	EvReuseRejected     EventKind = "reuse-rejected"
	EvStaleEye          EventKind = "stale-eye"
	EvStaleAttachment   EventKind = "stale-attachment"
	EvInvalidTransition EventKind = "invalid-transition"
)

// Event is one immutable audit record.
type Event struct {
	Seq       uint64
	Time      time.Time
	Machine   Machine
	Eye       Eye
	Kind      EventKind
	State     State
	AttID     string
	FormatVer uint32
	Detail    string
}

// Logger receives ordered lifecycle events. Implementations must be safe for
// concurrent use.
type Logger interface {
	Log(Event)
}

// MemoryLogger keeps events in memory and supports snapshot/count queries. It
// is the default logger and the primary tool for tests asserting on stale
// eye data.
type MemoryLogger struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryLogger() *MemoryLogger { return &MemoryLogger{} }

func (l *MemoryLogger) Log(ev Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ev.Seq = uint64(len(l.events)) + 1
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	l.events = append(l.events, ev)
}

// Events returns a copy of all recorded events.
func (l *MemoryLogger) Events() []Event {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Event, len(l.events))
	copy(out, l.events)
	return out
}

// CountByKind returns how many events of kind were recorded for eye. Pass the
// zero eye to count across both eyes.
func (l *MemoryLogger) CountByKind(kind EventKind, eye Eye) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, ev := range l.events {
		if ev.Kind == kind && (eye == 0 || ev.Eye == eye) {
			n++
		}
	}
	return n
}

// Last returns the most recent event of kind for eye and whether one exists.
func (l *MemoryLogger) Last(kind EventKind, eye Eye) (Event, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := len(l.events) - 1; i >= 0; i-- {
		ev := l.events[i]
		if ev.Kind == kind && (eye == 0 || ev.Eye == eye) {
			return ev, true
		}
	}
	return Event{}, false
}

// Reset discards all events.
func (l *MemoryLogger) Reset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = nil
}

// StdLogger adapts the standard library log package without taking a hard
// dependency on any third-party package.
type StdLogger struct{ P Logger }

func (s StdLogger) Log(ev Event) {
	if s.P != nil {
		s.P.Log(ev)
	}
	fmt.Printf("[%s] machine=%s eye=%s state=%s att=%s fmt=%d seq=%d %s\n",
		ev.Kind, ev.Machine, ev.Eye, ev.State, ev.AttID, ev.FormatVer, ev.Seq, ev.Detail)
}
