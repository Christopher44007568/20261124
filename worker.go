package reproject

import (
	"context"
	"fmt"
	"time"
)

func (s *Scheduler) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.baseCtx.Done():
			return
		case id := <-s.queue:
			s.run(id)
		}
	}
}

func (s *Scheduler) run(id TaskID) {
	s.mu.Lock()
	t := s.tasks[id]
	if t == nil || t.state != TaskQueued {
		s.mu.Unlock()
		return
	}
	t.state = TaskRunning
	t.startedAt = s.now()
	s.emitLocked(Event{
		At: t.startedAt, Kind: EventTaskStarted, TaskID: id,
		FrameVersion: t.frame, PoseVersion: t.pose, State: TaskRunning,
	})
	in := ReprojectionInput{
		TaskID:       id,
		FrameVersion: t.frame,
		PoseVersion:  t.pose,
		Pixels:       append([]byte(nil), t.input.data...),
	}
	dst := &StagingBuffer{id: t.output.id, owner: id, data: t.output.data}
	s.mu.Unlock()

	err := callWork(s.work, t.ctx, in, dst)
	completedAt := s.now()
	t.cancel()

	s.mu.Lock()
	s.completeLocked(t, completedAt, err)
	s.mu.Unlock()
}

func callWork(work WorkFunc, ctx context.Context, in ReprojectionInput, dst *StagingBuffer) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("reproject: work panicked: %v", recovered)
		}
	}()
	return work(ctx, in, dst)
}

func (s *Scheduler) completeLocked(t *task, completedAt time.Time, workErr error) {
	if !t.completedAt.IsZero() {
		return
	}
	t.completedAt = completedAt
	errText := ""
	if workErr != nil {
		errText = workErr.Error()
	}
	s.emitLocked(Event{
		At: completedAt, Kind: EventTaskCompleted, TaskID: t.id,
		FrameVersion: t.frame, PoseVersion: t.pose, State: t.state,
		Output: append([]byte(nil), t.output.data...), Error: errText,
	})

	if t.state != TaskRunning {
		t.err = errText
		s.releaseInputLocked(t)
		s.releaseOutputLocked(t)
		s.closeDoneLocked(t)
		return
	}

	if workErr != nil {
		t.state = TaskFailed
		t.err = errText
		t.finishedAt = completedAt
		s.emitLocked(Event{
			At: completedAt, Kind: EventTaskFailed, TaskID: t.id,
			FrameVersion: t.frame, PoseVersion: t.pose,
			State: TaskFailed, Reason: "work failed", Error: errText,
		})
		s.releaseInputLocked(t)
		s.releaseOutputLocked(t)
		s.closeDoneLocked(t)
		return
	}

	t.state = TaskComputed
	if t.frame != s.frame || t.pose != s.pose {
		t.state = TaskDiscarded
		t.finishedAt = completedAt
		t.reason = "stale frame or pose at commit"
		s.emitLocked(Event{
			At: completedAt, Kind: EventTaskDiscarded, TaskID: t.id,
			FrameVersion: t.frame, PoseVersion: t.pose,
			State: TaskDiscarded, Reason: t.reason,
		})
		s.releaseInputLocked(t)
		s.releaseOutputLocked(t)
		s.closeDoneLocked(t)
		return
	}

	s.commitLocked(t, completedAt)
}

func (s *Scheduler) commitLocked(t *task, at time.Time) {
	if _, err := s.pool.present(t.id, t.output.id); err != nil {
		t.state = TaskFailed
		t.err = err.Error()
		t.finishedAt = at
		s.emitLocked(Event{
			At: at, Kind: EventTaskFailed, TaskID: t.id,
			FrameVersion: t.frame, PoseVersion: t.pose,
			State: TaskFailed, Reason: "output ownership lost", Error: t.err,
		})
		s.releaseInputLocked(t)
		s.releaseOutputLocked(t)
		s.closeDoneLocked(t)
		return
	}
	if s.display != nil {
		old := s.display
		if _, err := s.pool.releaseDisplay(old.id); err != nil {
			t.err = err.Error()
		}
		s.emitLocked(Event{
			At: at, Kind: EventResourceReleased, TaskID: 0,
			BufferID: old.id, State: TaskCommitted,
		})
	}
	s.releaseInputLocked(t)
	s.display = t.output
	t.state = TaskCommitted
	t.finalUsed = t.pose
	t.finishedAt = at
	s.emitLocked(Event{
		At: at, Kind: EventTaskCommitted, TaskID: t.id,
		FrameVersion: t.frame, PoseVersion: t.pose,
		State: TaskCommitted, FinalUsedVersion: t.pose,
	})
	s.closeDoneLocked(t)
}

func (s *Scheduler) abortLocked(t *task, next TaskState, reason string, at time.Time) {
	if t.state != TaskQueued && t.state != TaskRunning {
		return
	}
	wasQueued := t.state == TaskQueued
	t.state = next
	t.reason = reason
	t.finishedAt = at
	if t.cancel != nil {
		t.cancel()
	}
	kind := EventTaskSuperseded
	if next == TaskCancelled {
		kind = EventTaskCancelled
	}
	s.emitLocked(Event{
		At: at, Kind: kind, TaskID: t.id,
		FrameVersion: t.frame, PoseVersion: t.pose,
		State: next, Reason: reason,
	})
	if wasQueued {
		s.releaseInputLocked(t)
		s.releaseOutputLocked(t)
		s.closeDoneLocked(t)
	}
}

func (s *Scheduler) releaseInputLocked(t *task) {
	s.releaseResourceLocked(t, t.input)
}

func (s *Scheduler) releaseOutputLocked(t *task) {
	s.releaseResourceLocked(t, t.output)
}

func (s *Scheduler) releaseResourceLocked(t *task, b *buffer) {
	if b == nil {
		return
	}
	released, err := s.pool.release(t.id, b.id)
	if err != nil {
		if t.err == "" {
			t.err = err.Error()
		}
		return
	}
	s.emitLocked(Event{
		At: s.now(), Kind: EventResourceReleased, TaskID: t.id,
		BufferID: released.id, State: t.state,
	})
}

func (s *Scheduler) closeDoneLocked(t *task) {
	if !t.doneClosed {
		close(t.done)
		t.doneClosed = true
	}
}
