package reproject

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Scheduler owns frame/pose versions, task state, and buffer ownership. Public
// methods are safe for concurrent use.
type Scheduler struct {
	mu      sync.Mutex
	work    WorkFunc
	now     func() time.Time
	baseCtx context.Context
	cancel  context.CancelFunc
	queue   chan TaskID
	wg      sync.WaitGroup

	closed   bool
	closeOne sync.Once
	nextTask TaskID
	frame    FrameVersion
	pose     PoseVersion
	display  *buffer
	pool     *bufferPool
	tasks    map[TaskID]*task
	order    []TaskID

	nextEvent uint64
	events    []Event
}

type task struct {
	id          TaskID
	frame       FrameVersion
	pose        PoseVersion
	input       *buffer
	output      *buffer
	state       TaskState
	submittedAt time.Time
	startedAt   time.Time
	completedAt time.Time
	finishedAt  time.Time
	finalUsed   PoseVersion
	reason      string
	err         string
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
	doneClosed  bool
}

type buffer struct {
	id        BufferID
	kind      ResourceKind
	owner     TaskID
	origin    TaskID
	frame     FrameVersion
	pose      PoseVersion
	presented bool
	data      []byte
}

type bufferPool struct {
	next BufferID
	live map[BufferID]*buffer
}

func newBufferPool() *bufferPool {
	return &bufferPool{live: make(map[BufferID]*buffer)}
}

func (p *bufferPool) allocate(owner TaskID, kind ResourceKind, frame FrameVersion, pose PoseVersion, data []byte) *buffer {
	p.next++
	b := &buffer{
		id: p.next, kind: kind, owner: owner, origin: owner,
		frame: frame, pose: pose, data: data,
	}
	p.live[b.id] = b
	return b
}

func (p *bufferPool) release(owner TaskID, id BufferID) (*buffer, error) {
	b, ok := p.live[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if b.owner != owner || b.presented {
		return nil, ErrNotOwner
	}
	delete(p.live, id)
	return b, nil
}

func (p *bufferPool) releaseDisplay(id BufferID) (*buffer, error) {
	b, ok := p.live[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if b.owner != 0 || !b.presented {
		return nil, ErrNotOwner
	}
	delete(p.live, id)
	return b, nil
}

func (p *bufferPool) present(owner TaskID, id BufferID) (*buffer, error) {
	b, ok := p.live[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	if b.owner != owner || b.presented {
		return nil, ErrNotOwner
	}
	b.kind = ResourceDisplay
	b.owner = 0
	b.presented = true
	return b, nil
}

// New creates a Scheduler and starts its worker goroutines.
func New(cfg Config) (*Scheduler, error) {
	if cfg.Work == nil {
		return nil, fmt.Errorf("%w: Work is required", ErrInvalidConfig)
	}
	if cfg.Workers < 0 || cfg.QueueSize < 0 {
		return nil, fmt.Errorf("%w: Workers and QueueSize cannot be negative", ErrInvalidConfig)
	}
	if cfg.Workers == 0 {
		cfg.Workers = 2
	}
	if cfg.QueueSize == 0 {
		cfg.QueueSize = 64
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{
		work:    cfg.Work,
		now:     cfg.Now,
		baseCtx: ctx,
		cancel:  cancel,
		queue:   make(chan TaskID, cfg.QueueSize),
		pool:    newBufferPool(),
		tasks:   make(map[TaskID]*task),
	}
	s.wg.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go s.worker()
	}
	return s, nil
}

// SetPose advances the current pose. Every active task bound to an older pose
// is superseded immediately; later work completion cannot commit it.
func (s *Scheduler) SetPose(version PoseVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	if version <= s.pose {
		return ErrInvalidVersion
	}
	s.pose = version
	at := s.now()
	s.emitLocked(Event{At: at, Kind: EventPoseAdvanced, PoseVersion: version})
	for _, id := range s.order {
		t := s.tasks[id]
		if (t.state == TaskQueued || t.state == TaskRunning) && t.pose < version {
			s.abortLocked(t, TaskSuperseded, "newer pose arrived", at)
		}
	}
	return nil
}

// SubmitFrame queues reprojection of frame at the current pose. A newer frame
// supersedes every older active request before the new task is queued.
func (s *Scheduler) SubmitFrame(frame Frame) (TaskID, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return 0, ErrClosed
	}
	if s.pose == 0 {
		s.mu.Unlock()
		return 0, ErrNoPose
	}
	if frame.Version <= s.frame {
		s.mu.Unlock()
		return 0, ErrInvalidVersion
	}

	s.frame = frame.Version
	at := s.now()
	s.emitLocked(Event{At: at, Kind: EventFrameAdvanced, FrameVersion: frame.Version})
	for _, id := range s.order {
		t := s.tasks[id]
		if t.state == TaskQueued || t.state == TaskRunning {
			s.abortLocked(t, TaskSuperseded, "newer frame arrived", at)
		}
	}

	s.nextTask++
	id := s.nextTask
	pixels := append([]byte(nil), frame.Pixels...)
	input := s.allocateLocked(id, ResourceInput, frame.Version, s.pose, pixels)
	output := s.allocateLocked(id, ResourceOutput, frame.Version, s.pose, make([]byte, len(pixels)))
	taskCtx, taskCancel := context.WithCancel(s.baseCtx)
	t := &task{
		id: id, frame: frame.Version, pose: s.pose,
		input: input, output: output, state: TaskQueued, submittedAt: at,
		ctx: taskCtx, cancel: taskCancel, done: make(chan struct{}),
	}
	s.tasks[id] = t
	s.order = append(s.order, id)
	s.emitLocked(Event{
		At: at, Kind: EventTaskQueued, TaskID: id,
		FrameVersion: frame.Version, PoseVersion: s.pose,
		State: TaskQueued, InputBuffer: input.id, OutputBuffer: output.id,
		Length: len(pixels),
	})
	s.mu.Unlock()

	select {
	case s.queue <- id:
	case <-s.baseCtx.Done():
	}
	return id, nil
}

// Cancel aborts a queued or running task. Queued resources are released
// immediately; running resources are released only after its WorkFunc returns,
// because the worker may still be writing its private staging buffer.
func (s *Scheduler) Cancel(id TaskID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrClosed
	}
	t, ok := s.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}
	if t.state != TaskQueued && t.state != TaskRunning {
		return ErrTaskNotActive
	}
	s.abortLocked(t, TaskCancelled, "cancelled by owner", s.now())
	return nil
}

// Wait blocks until the task reaches a terminal state or ctx is cancelled.
func (s *Scheduler) Wait(ctx context.Context, id TaskID) (TaskRecord, error) {
	s.mu.Lock()
	t, ok := s.tasks[id]
	if !ok {
		s.mu.Unlock()
		return TaskRecord{}, ErrTaskNotFound
	}
	done := t.done
	s.mu.Unlock()

	select {
	case <-done:
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.recordLocked(t), nil
	case <-ctx.Done():
		return TaskRecord{}, ctx.Err()
	}
}

// Snapshot returns an immutable copy of versions, records, ownership, and the
// currently displayed output.
func (s *Scheduler) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// Events returns a copy of the replay journal.
func (s *Scheduler) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := make([]Event, len(s.events))
	for i, ev := range s.events {
		events[i] = ev
		events[i].Output = append([]byte(nil), ev.Output...)
	}
	return events
}

// ReplayJournal replays the current journal into an independent snapshot.
func (s *Scheduler) ReplayJournal() (Snapshot, error) {
	return Replay(s.Events())
}

// Close supersedes all active tasks and waits for cooperative workers to exit.
func (s *Scheduler) Close() {
	s.closeOne.Do(func() {
		s.mu.Lock()
		s.closed = true
		at := s.now()
		for _, id := range s.order {
			t := s.tasks[id]
			if t.state == TaskQueued || t.state == TaskRunning {
				s.abortLocked(t, TaskCancelled, "scheduler closed", at)
			}
		}
		s.cancel()
		s.mu.Unlock()
		s.wg.Wait()
	})
}

func (s *Scheduler) allocateLocked(owner TaskID, kind ResourceKind, frame FrameVersion, pose PoseVersion, data []byte) *buffer {
	return s.pool.allocate(owner, kind, frame, pose, data)
}

func (s *Scheduler) recordLocked(t *task) TaskRecord {
	return TaskRecord{
		ID: t.id, InputFrame: t.frame, PoseVersion: t.pose, State: t.state,
		SubmittedAt: t.submittedAt, StartedAt: t.startedAt,
		CompletedAt: t.completedAt, FinishedAt: t.finishedAt,
		FinalUsedVersion: t.finalUsed,
		InputBuffer:      t.input.id, OutputBuffer: t.output.id,
		Reason: t.reason, Error: t.err,
	}
}

func (s *Scheduler) snapshotLocked() Snapshot {
	tasks := make([]TaskRecord, 0, len(s.order))
	for _, id := range s.order {
		tasks = append(tasks, s.recordLocked(s.tasks[id]))
	}
	resources := make([]ResourceInfo, 0, len(s.pool.live))
	for _, b := range s.pool.live {
		resources = append(resources, ResourceInfo{
			ID: b.id, Kind: b.kind, Owner: b.owner,
			Frame: b.frame, Pose: b.pose, Len: len(b.data),
			Presented: b.presented,
		})
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID < resources[j].ID })
	var displayed *DisplayedFrame
	if s.display != nil {
		displayed = &DisplayedFrame{
			TaskID: s.display.origin, FrameVersion: s.display.frame,
			PoseVersion: s.display.pose, BufferID: s.display.id,
			Pixels: append([]byte(nil), s.display.data...),
		}
	}
	return Snapshot{
		CurrentFrame: s.frame, CurrentPose: s.pose,
		Displayed: displayed, Tasks: tasks, Resources: resources,
	}
}

func (s *Scheduler) emitLocked(ev Event) {
	s.nextEvent++
	ev.Seq = s.nextEvent
	if ev.At.IsZero() {
		ev.At = s.now()
	}
	if ev.Output != nil {
		ev.Output = append([]byte(nil), ev.Output...)
	}
	s.events = append(s.events, ev)
}
