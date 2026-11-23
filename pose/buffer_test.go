package pose

import (
	"math"
	"testing"
)

const ms = Timestamp(1_000_000)

func sampleAt(t Timestamp, x float64) Sample {
	return Sample{
		Time: t,
		Pos:  Vec3{X: x, Y: x * 2, Z: -x},
		Rot:  RotAngleZ(float64(t) / float64(ms) * 0.1),
	}
}

func TestOutOfOrderInsertKeepsTimeOrder(t *testing.T) {
	buf := New(Config{Capacity: 8, Extrapolate: ms, MaxGap: 10 * ms})
	for _, t := range []Timestamp{3 * ms, 1 * ms, 5 * ms, 2 * ms, 4 * ms} {
		buf.Insert(sampleAt(t, float64(t/ms)))
	}
	if got := buf.Len(); got != 5 {
		t.Fatalf("Len = %d, want 5", got)
	}
	for i := 1; i < 5; i++ {
		prev := buf.at(i - 1).Time
		cur := buf.at(i).Time
		if cur <= prev {
			t.Fatalf("ring not sorted at %d: %d !< %d", i, prev, cur)
		}
	}
}

func TestDuplicateTimestampOverwrites(t *testing.T) {
	buf := New(Config{Capacity: 4, Extrapolate: ms, MaxGap: 10 * ms})
	buf.Insert(sampleAt(ms, 1))
	dup := sampleAt(ms, 99)
	buf.Insert(dup)
	if got := buf.Len(); got != 1 {
		t.Fatalf("Len = %d, want 1 (duplicate must not grow ring)", got)
	}
	r := buf.Query(ms)
	if r.Status != StatusOK || r.Sample.Pos.X != 99 {
		t.Fatalf("duplicate not overwritten: status=%d x=%v", r.Status, r.Sample.Pos.X)
	}
	if buf.Stats().Duplicates != 1 {
		t.Fatalf("Duplicates = %d, want 1", buf.Stats().Duplicates)
	}
}

func TestClockRollbackEvictsNewestWhenFull(t *testing.T) {
	buf := New(Config{Capacity: 3, Extrapolate: ms, MaxGap: 100 * ms})
	var rolled Timestamp
	buf.cfg.OnClockRollback = func(d Timestamp) { rolled = d }
	for _, t := range []Timestamp{2 * ms, 4 * ms, 6 * ms} {
		buf.Insert(sampleAt(t, float64(t/ms)))
	}
	buf.Insert(sampleAt(1*ms, 1)) // older than everything; ring already full

	st := buf.Stats()
	if st.ClockRollbacks != 1 || rolled != 5*ms {
		t.Fatalf("rollback not recorded: stats=%+v delta=%d", st, rolled)
	}
	if st.Evictions != 1 {
		t.Fatalf("Evictions = %d, want 1", st.Evictions)
	}
	oldest, ok := buf.Oldest()
	if !ok || oldest.Time != 1*ms {
		t.Fatalf("oldest = %v ok=%v, want 1ms", oldest.Time, ok)
	}
	newest, _ := buf.Newest()
	if newest.Time != 4*ms {
		t.Fatalf("newest = %d, want 4ms (rolled-back insert evicts newest)", newest.Time)
	}
}

func TestResampleTranslationAndQuaternion(t *testing.T) {
	buf := New(Config{Capacity: 8, Extrapolate: ms, MaxGap: 10 * ms})
	s0 := Sample{Time: 0, Pos: Vec3{X: 0}, Rot: RotAngleZ(0)}
	s1 := Sample{Time: 10 * ms, Pos: Vec3{X: 10}, Rot: RotAngleZ(math.Pi / 2)}
	buf.Insert(s1)
	buf.Insert(s0)

	r := buf.Query(5 * ms)
	if r.Status != StatusOK {
		t.Fatalf("Status = %d, want OK", r.Status)
	}
	if math.Abs(r.Sample.Pos.X-5) > 1e-12 {
		t.Fatalf("position lerp = %v, want 5", r.Sample.Pos.X)
	}
	want := RotAngleZ(math.Pi / 4)
	if AngleBetween(r.Sample.Rot, want) > 1e-12 {
		t.Fatalf("slerp angle error = %v", AngleBetween(r.Sample.Rot, want))
	}
}

func TestGapAndWindowStatuses(t *testing.T) {
	buf := New(Config{Capacity: 8, Extrapolate: ms, MaxGap: 5 * ms})
	buf.Insert(sampleAt(0, 0))
	buf.Insert(sampleAt(10*ms, 10))

	r := buf.Query(5 * ms)
	if r.Status != StatusGapTooLarge {
		t.Fatalf("Status = %d, want GapTooLarge", r.Status)
	}
	r = buf.Query(11 * ms) // 1 ms beyond newest, inside the 1 ms extrapolation budget
	if r.Status != StatusExtrapolated {
		t.Fatalf("Status = %d, want Extrapolated", r.Status)
	}
	r = buf.Query(100 * ms)
	if r.Status != StatusAfterWindow {
		t.Fatalf("Status = %d, want AfterWindow", r.Status)
	}
	r = buf.Query(-ms)
	if r.Status != StatusBeforeWindow {
		t.Fatalf("Status = %d, want BeforeWindow", r.Status)
	}
}

func TestNearestNeighbor(t *testing.T) {
	buf := New(Config{Capacity: 8, Extrapolate: 2 * ms, MaxGap: 100 * ms})
	buf.Insert(sampleAt(0, 0))
	buf.Insert(sampleAt(10*ms, 10))

	r := buf.QueryNearest(4 * ms)
	if r.Sample.Time != 0 || r.Status != StatusOK {
		t.Fatalf("nearest = %d status=%d, want 0ms", r.Sample.Time, r.Status)
	}
	r = buf.QueryNearest(5 * ms) // exact tie: earlier sample wins
	if r.Sample.Time != 0 {
		t.Fatalf("tie broken to %d, want 0ms", r.Sample.Time)
	}
	r = buf.QueryNearest(11 * ms)
	if r.Status != StatusExtrapolated || r.Sample.Time != 10*ms {
		t.Fatalf("future nearest status=%d time=%d", r.Status, r.Sample.Time)
	}
	r = buf.QueryNearest(50 * ms)
	if r.Status != StatusAfterWindow {
		t.Fatalf("Status = %d, want AfterWindow", r.Status)
	}
}

func TestEvictionOfOldestOnMonotonicFill(t *testing.T) {
	buf := New(Config{Capacity: 3, Extrapolate: ms, MaxGap: 100 * ms})
	for _, t := range []Timestamp{1 * ms, 2 * ms, 3 * ms, 4 * ms} {
		buf.Insert(sampleAt(t, float64(t/ms)))
	}
	oldest, _ := buf.Oldest()
	if oldest.Time != 2*ms {
		t.Fatalf("oldest = %d, want 2ms", oldest.Time)
	}
	if buf.Stats().Evictions != 1 {
		t.Fatalf("Evictions = %d, want 1", buf.Stats().Evictions)
	}
}

func TestLateArrivalRevealsGapAnywhereInWindow(t *testing.T) {
	buf := New(Config{Capacity: 16, Extrapolate: ms, MaxGap: 5 * ms})
	// Ordered arrivals except the 2 ms sample, which is delivered last.
	for _, t := range []Timestamp{1 * ms, 3 * ms, 4 * ms, 5 * ms, 2 * ms} {
		buf.Insert(sampleAt(t, float64(t/ms)))
	}
	st := buf.Stats()
	// The largest physical gap is 1 ms here; the point is that scanning the
	// full window yields a correct value regardless of arrival order.
	if st.LargestGap != ms {
		t.Fatalf("LargestGap = %d, want 1ms", st.LargestGap)
	}

	// Now create a real over-limit gap in the middle via a late old sample.
	buf2 := New(Config{Capacity: 16, Extrapolate: ms, MaxGap: 5 * ms})
	for _, t := range []Timestamp{1 * ms, 2 * ms, 10 * ms, 11 * ms} {
		buf2.Insert(sampleAt(t, float64(t/ms)))
	}
	if buf2.Stats().GapsOverLimit != 1 {
		t.Fatalf("GapsOverLimit = %d, want 1", buf2.Stats().GapsOverLimit)
	}
}

func TestEmptyAndSingleSampleQueries(t *testing.T) {
	buf := New(Config{Capacity: 4, Extrapolate: ms, MaxGap: 100 * ms})
	if r := buf.Query(ms); r.Status != StatusEmpty {
		t.Fatalf("empty buffer status = %d, want Empty", r.Status)
	}
	buf.Insert(sampleAt(5*ms, 5))
	if r := buf.Query(5 * ms); r.Status != StatusOK {
		t.Fatalf("exact single-sample status = %d", r.Status)
	}
	if r := buf.Query(6 * ms); r.Status != StatusExtrapolated {
		t.Fatalf("single-sample future status = %d, want Extrapolated", r.Status)
	}
	if r := buf.Query(7 * ms); r.Status != StatusAfterWindow {
		t.Fatalf("beyond budget status = %d, want AfterWindow", r.Status)
	}
}
