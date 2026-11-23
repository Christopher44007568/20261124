// Package reproject implements a versioned, asynchronous reprojection task
// scheduler. It deliberately does not depend on a graphics API: frames and
// staging buffers are plain byte slices owned by the scheduler.
package reproject

import (
	"context"
	"errors"
	"time"
)

// FrameVersion identifies an input frame. Versions must increase monotonically.
type FrameVersion uint64

// PoseVersion identifies the pose used to start a reprojection task. Versions
// must increase monotonically.
type PoseVersion uint64

// TaskID identifies a queued reprojection task.
type TaskID uint64

// BufferID identifies an input or output buffer owned by a task, or a buffer
// retained for display after a successful commit.
type BufferID uint64

// TaskState is the complete state machine for a reprojection task.
type TaskState uint8

const (
	TaskQueued TaskState = iota + 1
	TaskRunning
	TaskComputed
	TaskCommitted
	TaskDiscarded
	TaskCancelled
	TaskSuperseded
	TaskFailed
)

func (s TaskState) String() string {
	switch s {
	case TaskQueued:
		return "queued"
	case TaskRunning:
		return "running"
	case TaskComputed:
		return "computed"
	case TaskCommitted:
		return "committed"
	case TaskDiscarded:
		return "discarded"
	case TaskCancelled:
		return "cancelled"
	case TaskSuperseded:
		return "superseded"
	case TaskFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Terminal reports whether no further task transition is possible.
func (s TaskState) Terminal() bool {
	switch s {
	case TaskCommitted, TaskDiscarded, TaskCancelled, TaskSuperseded, TaskFailed:
		return true
	default:
		return false
	}
}

// Frame is an input frame submitted to the scheduler. Pixels are copied at
// submission, so the caller may reuse its slice immediately.
type Frame struct {
	Version FrameVersion
	Pixels  []byte
}

// ReprojectionInput is the immutable input snapshot observed by a WorkFunc.
type ReprojectionInput struct {
	TaskID       TaskID
	FrameVersion FrameVersion
	PoseVersion  PoseVersion
	Pixels       []byte
}

// StagingBuffer is exclusively owned by one running task. Work functions write
// only here; the currently displayed buffer is never exposed to a WorkFunc.
type StagingBuffer struct {
	id    BufferID
	owner TaskID
	data  []byte
}

// ID returns the stable identity of this staging buffer.
func (b *StagingBuffer) ID() BufferID { return b.id }

// Owner returns the only task allowed to write this staging buffer.
func (b *StagingBuffer) Owner() TaskID { return b.owner }

// Len returns the fixed capacity of the staging buffer.
func (b *StagingBuffer) Len() int { return len(b.data) }

// Write copies p into the start of the staging buffer.
func (b *StagingBuffer) Write(p []byte) (int, error) {
	if len(p) > len(b.data) {
		return 0, ErrOutputTooLarge
	}
	return copy(b.data, p), nil
}

// Bytes returns a copy of the staging buffer contents.
func (b *StagingBuffer) Bytes() []byte {
	return append([]byte(nil), b.data...)
}

// WorkFunc performs the CPU-side reprojection into dst. Cancellation is
// cooperative: implementations should stop early and return when ctx is done.
type WorkFunc func(ctx context.Context, in ReprojectionInput, dst *StagingBuffer) error

// TaskRecord is the reviewable, replayable record for one task.
type TaskRecord struct {
	ID               TaskID
	InputFrame       FrameVersion
	PoseVersion      PoseVersion
	State            TaskState
	SubmittedAt      time.Time
	StartedAt        time.Time
	CompletedAt      time.Time
	FinishedAt       time.Time
	FinalUsedVersion PoseVersion
	InputBuffer      BufferID
	OutputBuffer     BufferID
	Reason           string
	Error            string
}

// ResourceKind distinguishes task-owned inputs and outputs from the retained
// display buffer.
type ResourceKind string

const (
	ResourceInput   ResourceKind = "input"
	ResourceOutput  ResourceKind = "output"
	ResourceDisplay ResourceKind = "display"
)

// ResourceInfo describes a currently live buffer. Owner is zero for a buffer
// retained by the scheduler for display.
type ResourceInfo struct {
	ID        BufferID
	Kind      ResourceKind
	Owner     TaskID
	Frame     FrameVersion
	Pose      PoseVersion
	Len       int
	Presented bool
}

// DisplayedFrame is the most recent successfully committed output.
type DisplayedFrame struct {
	TaskID       TaskID
	FrameVersion FrameVersion
	PoseVersion  PoseVersion
	BufferID     BufferID
	Pixels       []byte
}

// Snapshot is an immutable view of the scheduler at one instant.
type Snapshot struct {
	CurrentFrame FrameVersion
	CurrentPose  PoseVersion
	Displayed    *DisplayedFrame
	Tasks        []TaskRecord
	Resources    []ResourceInfo
}

// Config controls a Scheduler.
type Config struct {
	// Workers is the number of concurrent reprojection workers. Defaults to 2.
	Workers int
	// QueueSize bounds queued work. Defaults to 64.
	QueueSize int
	// Work is required.
	Work WorkFunc
	// Now is injectable for deterministic completion timestamps. Defaults to
	// time.Now.
	Now func() time.Time
}

var (
	ErrInvalidVersion = errors.New("reproject: version must be greater than the current version")
	ErrInvalidConfig  = errors.New("reproject: invalid configuration")
	ErrNoPose         = errors.New("reproject: set a pose before submitting a frame")
	ErrClosed         = errors.New("reproject: scheduler is closed")
	ErrTaskNotFound   = errors.New("reproject: task not found")
	ErrTaskNotActive  = errors.New("reproject: task is not queued or running")
	ErrOutputTooLarge = errors.New("reproject: output exceeds staging buffer capacity")
	ErrNotOwner       = errors.New("reproject: resource is not owned by task")
	ErrInvalidReplay  = errors.New("reproject: invalid event replay")
)
