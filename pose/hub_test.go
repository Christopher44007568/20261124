package pose

import (
	"math"
	"testing"
)

func TestClockDomainsAreIsolated(t *testing.T) {
	hub := NewHub(Config{Capacity: 8, Extrapolate: ms, MaxGap: 100 * ms})
	cam, imu := Domain("camera"), Domain("imu")

	hub.Insert(cam, sampleAt(10*ms, 10))
	hub.Insert(cam, sampleAt(20*ms, 20))
	hub.Insert(imu, sampleAt(5*ms, 5))

	if got := hub.Buffer(cam).Len(); got != 2 {
		t.Fatalf("camera Len = %d, want 2", got)
	}
	if got := hub.Buffer(imu).Len(); got != 1 {
		t.Fatalf("imu Len = %d, want 1", got)
	}

	// An IMU-domain query must never see camera samples even though the same
	// timestamp range is covered.
	r := hub.Buffer(imu).Query(15 * ms)
	if r.Status != StatusAfterWindow {
		t.Fatalf("cross-domain leak: imu query status = %d, want AfterWindow", r.Status)
	}

	// Unknown domain starts empty rather than borrowing another clock's data.
	if r := hub.Buffer(Domain("other")).Query(10 * ms); r.Status != StatusEmpty {
		t.Fatalf("unknown domain status = %d, want Empty", r.Status)
	}
}

func TestSlerpShortArcAndNormalization(t *testing.T) {
	a := RotAngleZ(0.1)
	b := RotAngleZ(2*math.Pi - 0.1) // nearly a full turn, the short arc is -0.2 rad
	mid := Slerp(a, b, 0.5)
	if n := mid.norm(); math.Abs(n-1) > 1e-12 {
		t.Fatalf("slerp output not unit: %v", n)
	}
	want := Identity()
	if err := AngleBetween(mid, want); err > 1e-9 {
		t.Fatalf("short-arc slerp angle = %v, want ~0", err)
	}
}
