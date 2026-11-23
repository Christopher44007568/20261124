// Package depthlife manages the per-eye lifecycle of a depth attachment and
// its parsed template/mask for the host machine and the right machine.
//
// The core invariant is eye-scoped state: every artifact derived from an
// attachment carries the eye it belongs to, and an artifact produced for one
// eye is never accepted as data for the other eye. Stale reuse attempts are
// rejected and recorded in the audit log.
package depthlife

import "errors"

// Eye identifies which optical eye owns a lifecycle slot.
type Eye uint8

const (
	Left Eye = iota + 1
	Right
)

// Valid reports whether e is a known eye.
func (e Eye) Valid() bool { return e == Left || e == Right }

func (e Eye) String() string {
	switch e {
	case Left:
		return "left"
	case Right:
		return "right"
	default:
		return "unknown"
	}
}

// Machine identifies the processing machine. Both machines embed the same
// lifecycle manager; the identity only tags log events and saved blobs.
type Machine uint8

const (
	Host Machine = iota + 1
	RightMachine
)

func (m Machine) String() string {
	switch m {
	case Host:
		return "host"
	case RightMachine:
		return "right-machine"
	default:
		return "unknown"
	}
}

// State is the lifecycle state of one eye slot.
//
//	Empty -> Loaded -> Parsed -> Masked -> Saved
//	any state -> Empty (Clear / format change)
type State uint8

const (
	Empty State = iota
	Loaded
	Parsed
	Masked
	Saved
)

func (s State) String() string {
	switch s {
	case Empty:
		return "empty"
	case Loaded:
		return "loaded"
	case Parsed:
		return "parsed"
	case Masked:
		return "masked"
	case Saved:
		return "saved"
	default:
		return "unknown"
	}
}

// Sentinel errors. Callers can use errors.Is to classify failures.
var (
	ErrInvalidEye       = errors.New("depthlife: invalid eye")
	ErrEmptySlot        = errors.New("depthlife: slot is empty")
	ErrNotLoaded        = errors.New("depthlife: attachment not loaded")
	ErrNotParsed        = errors.New("depthlife: depth not parsed")
	ErrNotMasked        = errors.New("depthlife: mask not applied")
	ErrFormatChanged    = errors.New("depthlife: attachment format changed")
	ErrStaleEye         = errors.New("depthlife: artifact belongs to a different eye")
	ErrAttachmentMoved  = errors.New("depthlife: artifact belongs to a different attachment")
	ErrStaleAttachment  = ErrAttachmentMoved
	ErrTemplateRejected = errors.New("depthlife: template mask rejected")
	ErrBadBlob          = errors.New("depthlife: malformed saved blob")
)
