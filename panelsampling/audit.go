package panelsampling

import "sync"

// OutOfBoundsRecord captures one source coordinate that left its panel. A
// record is emitted for every channel and every call; records are not
// deduplicated. Sequence is assigned by OOBLog.
type OutOfBoundsRecord struct {
	Sequence uint64
	Policy   Policy
	Eye      Eye
	Stage    Stage
	Channel  Channel
	Source   Coord
	Resolved Coord
	// Recovered is true when clamp or repeat produced a sample from this
	// texture. It is false for transparent-black, non-finite coordinates, and
	// any other invalid sample.
	Recovered  bool
	DepthValid bool
}

// OOBLog records all out-of-bounds samples. It is safe for concurrent use.
type OOBLog struct {
	mu      sync.Mutex
	nextSeq uint64
	records []OutOfBoundsRecord
}

// NewOOBLog creates an empty recorder.
func NewOOBLog() *OOBLog {
	return &OOBLog{}
}

// Record appends a record and assigns its sequence number.
func (l *OOBLog) Record(record OutOfBoundsRecord) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextSeq++
	record.Sequence = l.nextSeq
	l.records = append(l.records, record)
}

// Snapshot returns a copy of all records in insertion order.
func (l *OOBLog) Snapshot() []OutOfBoundsRecord {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]OutOfBoundsRecord, len(l.records))
	copy(result, l.records)
	return result
}

// Len returns the number of recorded out-of-bounds samples.
func (l *OOBLog) Len() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.records)
}

// Reset removes all records. Sequence numbering continues from its previous
// value so records from a reset log are still distinguishable.
func (l *OOBLog) Reset() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = l.records[:0]
}
