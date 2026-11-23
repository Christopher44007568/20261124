package reproject

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type manualClock struct {
	mu sync.Mutex
	at time.Time
}

func newManualClock() *manualClock {
	return &manualClock{at: time.Unix(1_700_000_000, 0).UTC()}
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *manualClock) Advance(d time.Duration) {
	c.mu.Lock()
	c.at = c.at.Add(d)
	c.mu.Unlock()
}

type gate struct {
	once sync.Once
	ch   chan struct{}
}

func newGate() *gate {
	return &gate{ch: make(chan struct{})}
}

func (g *gate) Open() {
	g.once.Do(func() { close(g.ch) })
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker")
		var zero T
		return zero
	}
}

func waitTask(t *testing.T, s *Scheduler, id TaskID) TaskRecord {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	record, err := s.Wait(ctx, id)
	if err != nil {
		t.Fatalf("wait task %d: %v", id, err)
	}
	return record
}

func resourceFor(snapshot Snapshot, id BufferID) (ResourceInfo, bool) {
	for _, resource := range snapshot.Resources {
		if resource.ID == id {
			return resource, true
		}
	}
	return ResourceInfo{}, false
}

func assertReplayMatches(t *testing.T, s *Scheduler) {
	t.Helper()
	live := s.Snapshot()
	replayed, err := s.ReplayJournal()
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if !reflect.DeepEqual(live, replayed) {
		t.Fatalf("replay mismatch\nlive:     %#v\nreplayed: %#v", live, replayed)
	}
}

func TestPoseUpdateAfterTaskStartRejectsLateOutput(t *testing.T) {
	clock := newManualClock()
	started := make(chan ReprojectionInput, 2)
	releaseStale := newGate()
	work := func(ctx context.Context, in ReprojectionInput, dst *StagingBuffer) error {
		started <- in
		if in.FrameVersion == 1 {
			<-releaseStale.ch
			_, err := dst.Write([]byte("stale"))
			return err
		}
		_, err := dst.Write([]byte("fresh"))
		return err
	}
	s, err := New(Config{Workers: 2, Work: work, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer releaseStale.Open()

	if err := s.SetPose(1); err != nil {
		t.Fatal(err)
	}
	oldID, err := s.SubmitFrame(Frame{Version: 1, Pixels: []byte("input")})
	if err != nil {
		t.Fatal(err)
	}
	if in := receive(t, started); in.PoseVersion != 1 || in.FrameVersion != 1 {
		t.Fatalf("unexpected input: %+v", in)
	}

	if err := s.SetPose(2); err != nil {
		t.Fatal(err)
	}
	clock.Advance(5 * time.Second)
	releaseStale.Open()
	old := waitTask(t, s, oldID)
	if old.State != TaskSuperseded {
		t.Fatalf("old task state = %s, want superseded", old.State)
	}
	if old.FinalUsedVersion != 0 {
		t.Fatalf("old task final version = %d, want 0", old.FinalUsedVersion)
	}
	if want := clock.Now(); !old.CompletedAt.Equal(want) {
		t.Fatalf("old task completed at %s, want %s", old.CompletedAt, want)
	}
	if displayed := s.Snapshot().Displayed; displayed != nil {
		t.Fatalf("stale task published output: %+v", displayed)
	}

	newID, err := s.SubmitFrame(Frame{Version: 2, Pixels: []byte("next!")})
	if err != nil {
		t.Fatal(err)
	}
	_ = receive(t, started)
	current := waitTask(t, s, newID)
	if current.State != TaskCommitted || current.FinalUsedVersion != 2 {
		t.Fatalf("new task = state %s, final version %d", current.State, current.FinalUsedVersion)
	}
	displayed := s.Snapshot().Displayed
	if displayed == nil || string(displayed.Pixels) != "fresh" || displayed.TaskID != newID {
		t.Fatalf("displayed frame = %+v", displayed)
	}
	assertReplayMatches(t, s)
}

func TestCancelReportsOnlyActiveTasks(t *testing.T) {
	s, err := New(Config{
		Workers: 1,
		Work: func(context.Context, ReprojectionInput, *StagingBuffer) error {
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetPose(1); err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitFrame(Frame{Version: 1, Pixels: []byte("frame")})
	if err != nil {
		t.Fatal(err)
	}
	waitTask(t, s, id)
	if err := s.Cancel(id); !errors.Is(err, ErrTaskNotActive) {
		t.Fatalf("cancel committed task error = %v", err)
	}
	if err := s.Cancel(999); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("cancel missing task error = %v", err)
	}
}

func TestNextFrameSupersedesOldRequestAndLateTaskCannotPublish(t *testing.T) {
	clock := newManualClock()
	started := make(chan FrameVersion, 2)
	releaseOld := newGate()
	work := func(ctx context.Context, in ReprojectionInput, dst *StagingBuffer) error {
		started <- in.FrameVersion
		switch in.FrameVersion {
		case 1:
			<-releaseOld.ch
			_, err := dst.Write([]byte("OLD00"))
			return err
		default:
			_, err := dst.Write([]byte("NEW00"))
			return err
		}
	}
	s, err := New(Config{Workers: 2, Work: work, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer releaseOld.Open()
	if err := s.SetPose(1); err != nil {
		t.Fatal(err)
	}

	oldID, err := s.SubmitFrame(Frame{Version: 1, Pixels: []byte("old00")})
	if err != nil {
		t.Fatal(err)
	}
	if frame := receive(t, started); frame != 1 {
		t.Fatalf("first worker frame = %d", frame)
	}

	newID, err := s.SubmitFrame(Frame{Version: 2, Pixels: []byte("new00")})
	if err != nil {
		t.Fatal(err)
	}
	if frame := receive(t, started); frame != 2 {
		t.Fatalf("second worker frame = %d", frame)
	}
	newRecord := waitTask(t, s, newID)
	if newRecord.State != TaskCommitted || newRecord.FinalUsedVersion != 1 {
		t.Fatalf("new frame record = %+v", newRecord)
	}
	before := s.Snapshot().Displayed
	if before == nil || string(before.Pixels) != "NEW00" || before.TaskID != newID {
		t.Fatalf("new frame did not publish: %+v", before)
	}

	clock.Advance(time.Second)
	releaseOld.Open()
	oldRecord := waitTask(t, s, oldID)
	if oldRecord.State != TaskSuperseded || oldRecord.FinalUsedVersion != 0 {
		t.Fatalf("old frame record = %+v", oldRecord)
	}
	after := s.Snapshot().Displayed
	if after == nil || after.BufferID != before.BufferID {
		t.Fatalf("late task replaced current buffer: before=%+v after=%+v", before, after)
	}
	if string(after.Pixels) != "NEW00" {
		t.Fatalf("current pixels changed to %q", after.Pixels)
	}
	if _, live := resourceFor(s.Snapshot(), oldRecord.OutputBuffer); live {
		t.Fatalf("late task output %d was not reclaimed", oldRecord.OutputBuffer)
	}
	assertReplayMatches(t, s)
}

func TestCancelReclaimsOnlyTheCancelledTasksResources(t *testing.T) {
	clock := newManualClock()
	started := make(chan FrameVersion, 3)
	releaseSecond := newGate()
	work := func(ctx context.Context, in ReprojectionInput, dst *StagingBuffer) error {
		started <- in.FrameVersion
		switch in.FrameVersion {
		case 1:
			_, err := dst.Write([]byte("BASE"))
			return err
		case 2:
			<-releaseSecond.ch
			_, err := dst.Write([]byte("OLD2"))
			return err
		default:
			<-ctx.Done()
			_, err := dst.Write([]byte("CANC"))
			return err
		}
	}
	s, err := New(Config{Workers: 3, Work: work, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	defer releaseSecond.Open()
	if err := s.SetPose(1); err != nil {
		t.Fatal(err)
	}

	firstID, err := s.SubmitFrame(Frame{Version: 1, Pixels: []byte("base")})
	if err != nil {
		t.Fatal(err)
	}
	_ = receive(t, started)
	if record := waitTask(t, s, firstID); record.State != TaskCommitted {
		t.Fatalf("base task = %+v", record)
	}
	base := s.Snapshot().Displayed
	if base == nil || string(base.Pixels) != "BASE" {
		t.Fatalf("base frame not displayed: %+v", base)
	}

	secondID, err := s.SubmitFrame(Frame{Version: 2, Pixels: []byte("old2")})
	if err != nil {
		t.Fatal(err)
	}
	_ = receive(t, started)
	thirdID, err := s.SubmitFrame(Frame{Version: 3, Pixels: []byte("canc")})
	if err != nil {
		t.Fatal(err)
	}
	_ = receive(t, started)

	second := s.Snapshot().Tasks[1]
	if second.ID != secondID || second.State != TaskSuperseded {
		t.Fatalf("second task before cancel = %+v", second)
	}
	beforeCancel := s.Snapshot()
	if _, live := resourceFor(beforeCancel, second.InputBuffer); !live {
		t.Fatal("superseded but still running task lost its input early")
	}
	if _, live := resourceFor(beforeCancel, second.OutputBuffer); !live {
		t.Fatal("superseded but still running task lost its output early")
	}

	if err := s.Cancel(thirdID); err != nil {
		t.Fatalf("cancel third task: %v", err)
	}
	third := waitTask(t, s, thirdID)
	if third.State != TaskCancelled || third.FinalUsedVersion != 0 || third.CompletedAt.IsZero() {
		t.Fatalf("cancelled task = %+v", third)
	}
	if err := s.Cancel(thirdID); !errors.Is(err, ErrTaskNotActive) {
		t.Fatalf("second cancel error = %v", err)
	}
	if err := s.Cancel(firstID); !errors.Is(err, ErrTaskNotActive) {
		t.Fatalf("cancel committed task error = %v", err)
	}

	afterCancel := s.Snapshot()
	if afterCancel.Displayed == nil || afterCancel.Displayed.BufferID != base.BufferID ||
		string(afterCancel.Displayed.Pixels) != "BASE" {
		t.Fatalf("cancel changed current display: before=%+v after=%+v", base, afterCancel.Displayed)
	}
	if _, live := resourceFor(afterCancel, third.InputBuffer); live {
		t.Fatal("cancelled task input was not reclaimed")
	}
	if _, live := resourceFor(afterCancel, third.OutputBuffer); live {
		t.Fatal("cancelled task output was not reclaimed")
	}
	if _, live := resourceFor(afterCancel, second.InputBuffer); !live {
		t.Fatal("cancelling task three reclaimed task two input")
	}
	if _, live := resourceFor(afterCancel, second.OutputBuffer); !live {
		t.Fatal("cancelling task three reclaimed task two output")
	}

	releaseSecond.Open()
	secondRecord := waitTask(t, s, secondID)
	if secondRecord.State != TaskSuperseded || secondRecord.FinalUsedVersion != 0 {
		t.Fatalf("second task after late completion = %+v", secondRecord)
	}
	final := s.Snapshot()
	if final.Displayed == nil || final.Displayed.BufferID != base.BufferID {
		t.Fatalf("late second task changed display: %+v", final.Displayed)
	}
	for _, resource := range final.Resources {
		if resource.Owner == secondID || resource.Owner == thirdID {
			t.Fatalf("terminal task still owns resource: %+v", resource)
		}
	}
	assertReplayMatches(t, s)
}

func TestCloseWaitsForRunningWorkerAndReplayMatches(t *testing.T) {
	clock := newManualClock()
	started := make(chan struct{})
	work := func(ctx context.Context, in ReprojectionInput, dst *StagingBuffer) error {
		close(started)
		<-ctx.Done()
		_, err := dst.Write([]byte("done"))
		return err
	}
	s, err := New(Config{Workers: 1, Work: work, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.SetPose(1); err != nil {
		t.Fatal(err)
	}
	id, err := s.SubmitFrame(Frame{Version: 1, Pixels: []byte("work")})
	if err != nil {
		t.Fatal(err)
	}
	receive(t, started)

	s.Close()
	s.Close()
	record, err := s.Wait(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if record.State != TaskCancelled || record.FinalUsedVersion != 0 || record.CompletedAt.IsZero() {
		t.Fatalf("closed task = %+v", record)
	}
	snapshot := s.Snapshot()
	if snapshot.Displayed != nil || len(snapshot.Resources) != 0 {
		t.Fatalf("closed scheduler retained resources: %+v", snapshot)
	}
	if err := s.SetPose(2); !errors.Is(err, ErrClosed) {
		t.Fatalf("SetPose after close = %v", err)
	}
	replayed, err := s.ReplayJournal()
	if err != nil {
		t.Fatalf("replay after close: %v", err)
	}
	if !reflect.DeepEqual(snapshot, replayed) {
		t.Fatalf("replay mismatch after close\nlive:     %#v\nreplayed: %#v", snapshot, replayed)
	}
}

func TestReplayRejectsCrossTaskResourceRelease(t *testing.T) {
	at := time.Unix(1_700_000_000, 0).UTC()
	events := []Event{
		{Seq: 1, At: at, Kind: EventFrameAdvanced, FrameVersion: 1},
		{Seq: 2, At: at, Kind: EventPoseAdvanced, PoseVersion: 1},
		{Seq: 3, At: at, Kind: EventTaskQueued, TaskID: 1, FrameVersion: 1,
			PoseVersion: 1, InputBuffer: 1, OutputBuffer: 2, Length: 4},
		{Seq: 4, At: at, Kind: EventTaskStarted, TaskID: 1},
		{Seq: 5, At: at, Kind: EventTaskCancelled, TaskID: 1, Reason: "cancelled"},
		{Seq: 6, At: at, Kind: EventResourceReleased, TaskID: 1, BufferID: 1},
		{Seq: 7, At: at, Kind: EventResourceReleased, TaskID: 99, BufferID: 2},
	}
	if _, err := Replay(events); !errors.Is(err, ErrInvalidReplay) {
		t.Fatalf("cross-owner replay error = %v", err)
	}
}
