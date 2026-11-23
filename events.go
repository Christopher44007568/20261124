package reproject

import (
	"fmt"
	"sort"
	"time"
)

// EventKind is a scheduler transition recorded in the replay journal.
type EventKind string

const (
	EventFrameAdvanced    EventKind = "frame_advanced"
	EventPoseAdvanced     EventKind = "pose_advanced"
	EventTaskQueued       EventKind = "task_queued"
	EventTaskStarted      EventKind = "task_started"
	EventTaskCompleted    EventKind = "task_completed"
	EventTaskCommitted    EventKind = "task_committed"
	EventTaskDiscarded    EventKind = "task_discarded"
	EventTaskCancelled    EventKind = "task_cancelled"
	EventTaskSuperseded   EventKind = "task_superseded"
	EventTaskFailed       EventKind = "task_failed"
	EventResourceReleased EventKind = "resource_released"
)

// Event is one immutable journal entry. Output is copied on emission and is
// used by Replay to reconstruct the committed display buffer.
type Event struct {
	Seq              uint64
	At               time.Time
	Kind             EventKind
	TaskID           TaskID
	FrameVersion     FrameVersion
	PoseVersion      PoseVersion
	State            TaskState
	FinalUsedVersion PoseVersion
	InputBuffer      BufferID
	OutputBuffer     BufferID
	BufferID         BufferID
	Length           int
	Output           []byte
	Reason           string
	Error            string
}

// Replay reconstructs task records, live resources, and the displayed frame
// solely from the journal. It rejects illegal state transitions and ownership
// transitions.
func Replay(events []Event) (Snapshot, error) {
	r := replayState{
		tasks:     make(map[TaskID]*TaskRecord),
		resources: make(map[BufferID]ResourceInfo),
		outputs:   make(map[TaskID][]byte),
	}
	var previousSeq uint64
	for i := range events {
		ev := events[i]
		if ev.Seq != previousSeq+1 {
			return Snapshot{}, replayError(ev, "non-contiguous sequence %d after %d", ev.Seq, previousSeq)
		}
		previousSeq = ev.Seq
		if err := r.apply(ev); err != nil {
			return Snapshot{}, err
		}
	}
	return r.snapshot(), nil
}

type replayState struct {
	frame     FrameVersion
	pose      PoseVersion
	displayed *DisplayedFrame
	tasks     map[TaskID]*TaskRecord
	order     []TaskID
	resources map[BufferID]ResourceInfo
	outputs   map[TaskID][]byte
}

func (r *replayState) apply(ev Event) error {
	switch ev.Kind {
	case EventFrameAdvanced:
		if ev.FrameVersion <= r.frame {
			return replayError(ev, "frame %d is not newer than %d", ev.FrameVersion, r.frame)
		}
		r.frame = ev.FrameVersion
	case EventPoseAdvanced:
		if ev.PoseVersion <= r.pose {
			return replayError(ev, "pose %d is not newer than %d", ev.PoseVersion, r.pose)
		}
		r.pose = ev.PoseVersion
	case EventTaskQueued:
		if _, ok := r.tasks[ev.TaskID]; ok {
			return replayError(ev, "duplicate task %d", ev.TaskID)
		}
		if ev.FrameVersion != r.frame {
			return replayError(ev, "task frame %d does not match current frame %d", ev.FrameVersion, r.frame)
		}
		if ev.PoseVersion != r.pose {
			return replayError(ev, "task pose %d does not match current pose %d", ev.PoseVersion, r.pose)
		}
		if ev.InputBuffer == ev.OutputBuffer {
			return replayError(ev, "task input and output buffers are identical")
		}
		record := &TaskRecord{
			ID:           ev.TaskID,
			InputFrame:   ev.FrameVersion,
			PoseVersion:  ev.PoseVersion,
			State:        TaskQueued,
			SubmittedAt:  ev.At,
			InputBuffer:  ev.InputBuffer,
			OutputBuffer: ev.OutputBuffer,
		}
		r.tasks[ev.TaskID] = record
		r.order = append(r.order, ev.TaskID)
		r.resources[ev.InputBuffer] = ResourceInfo{
			ID: ev.InputBuffer, Kind: ResourceInput, Owner: ev.TaskID,
			Frame: ev.FrameVersion, Pose: ev.PoseVersion, Len: ev.Length,
		}
		r.resources[ev.OutputBuffer] = ResourceInfo{
			ID: ev.OutputBuffer, Kind: ResourceOutput, Owner: ev.TaskID,
			Frame: ev.FrameVersion, Pose: ev.PoseVersion, Len: ev.Length,
		}
	case EventTaskStarted:
		record, err := r.activeTask(ev)
		if err != nil {
			return err
		}
		if record.State != TaskQueued {
			return replayError(ev, "task %d starts from %s", ev.TaskID, record.State)
		}
		record.State = TaskRunning
		record.StartedAt = ev.At
	case EventTaskCompleted:
		record, err := r.task(ev)
		if err != nil {
			return err
		}
		if !record.CompletedAt.IsZero() {
			return replayError(ev, "task %d completed twice", ev.TaskID)
		}
		record.CompletedAt = ev.At
		record.Error = ev.Error
		if ev.Output != nil {
			r.outputs[ev.TaskID] = append([]byte(nil), ev.Output...)
		}
		if record.State == TaskRunning {
			record.State = TaskComputed
		}
	case EventTaskCommitted:
		record, err := r.task(ev)
		if err != nil {
			return err
		}
		if record.State != TaskComputed {
			return replayError(ev, "task %d commits from %s", ev.TaskID, record.State)
		}
		if ev.FinalUsedVersion != record.PoseVersion {
			return replayError(ev, "task %d committed pose %d, input pose was %d",
				ev.TaskID, ev.FinalUsedVersion, record.PoseVersion)
		}
		resource, ok := r.resources[record.OutputBuffer]
		if !ok || resource.Owner != record.ID || resource.Presented {
			return replayError(ev, "task %d does not own output buffer %d", ev.TaskID, record.OutputBuffer)
		}
		output := append([]byte(nil), r.outputs[record.ID]...)
		if r.displayed != nil {
			if _, ok := r.resources[r.displayed.BufferID]; ok {
				return replayError(ev, "previous display buffer %d was not released", r.displayed.BufferID)
			}
		}
		resource.Kind = ResourceDisplay
		resource.Owner = 0
		resource.Presented = true
		r.resources[resource.ID] = resource
		r.displayed = &DisplayedFrame{
			TaskID:       record.ID,
			FrameVersion: record.InputFrame,
			PoseVersion:  ev.FinalUsedVersion,
			BufferID:     record.OutputBuffer,
			Pixels:       output,
		}
		record.State = TaskCommitted
		record.FinalUsedVersion = ev.FinalUsedVersion
		record.FinishedAt = ev.At
	case EventTaskDiscarded:
		if err := r.finishFrom(ev, TaskComputed, TaskDiscarded); err != nil {
			return err
		}
	case EventTaskCancelled:
		if err := r.abort(ev, TaskCancelled, TaskQueued, TaskRunning); err != nil {
			return err
		}
	case EventTaskSuperseded:
		if err := r.abort(ev, TaskSuperseded, TaskQueued, TaskRunning); err != nil {
			return err
		}
	case EventTaskFailed:
		if err := r.finishFrom(ev, TaskComputed, TaskFailed); err != nil {
			return err
		}
	case EventResourceReleased:
		resource, ok := r.resources[ev.BufferID]
		if !ok {
			return replayError(ev, "release of unknown buffer %d", ev.BufferID)
		}
		if ev.TaskID == 0 {
			if !resource.Presented || resource.Owner != 0 {
				return replayError(ev, "buffer %d is not display-owned", ev.BufferID)
			}
		} else if resource.Owner != ev.TaskID || resource.Presented {
			return replayError(ev, "task %d cannot release buffer %d owned by %d",
				ev.TaskID, ev.BufferID, resource.Owner)
		}
		delete(r.resources, ev.BufferID)
		if r.displayed != nil && r.displayed.BufferID == ev.BufferID {
			r.displayed = nil
		}
	default:
		return replayError(ev, "unknown event kind %q", ev.Kind)
	}
	return nil
}

func (r *replayState) task(ev Event) (*TaskRecord, error) {
	record, ok := r.tasks[ev.TaskID]
	if !ok {
		return nil, replayError(ev, "unknown task %d", ev.TaskID)
	}
	return record, nil
}

func (r *replayState) activeTask(ev Event) (*TaskRecord, error) {
	record, err := r.task(ev)
	if err != nil {
		return nil, err
	}
	if record.State.Terminal() {
		return nil, replayError(ev, "task %d is already %s", ev.TaskID, record.State)
	}
	return record, nil
}

func (r *replayState) abort(ev Event, next TaskState, allowed ...TaskState) error {
	record, err := r.task(ev)
	if err != nil {
		return err
	}
	for _, state := range allowed {
		if record.State == state {
			record.State = next
			record.FinishedAt = ev.At
			record.Reason = ev.Reason
			return nil
		}
	}
	return replayError(ev, "task %d cannot move from %s to %s", ev.TaskID, record.State, next)
}

func (r *replayState) finishFrom(ev Event, from, next TaskState) error {
	record, err := r.task(ev)
	if err != nil {
		return err
	}
	if record.State != from {
		return replayError(ev, "task %d cannot move from %s to %s", ev.TaskID, record.State, next)
	}
	record.State = next
	record.FinishedAt = ev.At
	record.Reason = ev.Reason
	record.Error = ev.Error
	return nil
}

func (r *replayState) snapshot() Snapshot {
	tasks := make([]TaskRecord, 0, len(r.order))
	for _, id := range r.order {
		tasks = append(tasks, *r.tasks[id])
	}
	resources := make([]ResourceInfo, 0, len(r.resources))
	for _, resource := range r.resources {
		resources = append(resources, resource)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	var displayed *DisplayedFrame
	if r.displayed != nil {
		copy := *r.displayed
		copy.Pixels = append([]byte(nil), r.displayed.Pixels...)
		displayed = &copy
	}
	return Snapshot{
		CurrentFrame: r.frame,
		CurrentPose:  r.pose,
		Displayed:    displayed,
		Tasks:        tasks,
		Resources:    resources,
	}
}

func replayError(ev Event, format string, args ...any) error {
	return fmt.Errorf("%w: seq %d (%s): %s", ErrInvalidReplay, ev.Seq, ev.Kind, fmt.Sprintf(format, args...))
}
