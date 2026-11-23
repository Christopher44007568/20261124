package depthlife

import (
	"sync"
	"testing"
)

func TestHostAndRightMachineIsolation(t *testing.T) {
	hostLog, rightLog := NewMemoryLogger(), NewMemoryLogger()
	host := New(Host, hostLog)
	rightMgr := New(RightMachine, rightLog)

	raw := makeAttachment(t, 1, 2, 2, []uint16{100, 100, 1, 1})
	for _, m := range []*Manager{host, rightMgr} {
		if _, err := m.Load(Left, raw, 1); err != nil {
			t.Fatalf("Load: %v", err)
		}
		if _, err := m.Parse(Left); err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if _, err := m.ApplyMask(Left, 50, 0.5); err != nil {
			t.Fatalf("ApplyMask: %v", err)
		}
	}
	hostTpl, hostBlob, err := host.Save(Left)
	if err != nil {
		t.Fatalf("host Save: %v", err)
	}
	rightTpl, rightBlob, err := rightMgr.Save(Left)
	if err != nil {
		t.Fatalf("right Save: %v", err)
	}
	if hostTpl.Machine != Host || rightTpl.Machine != RightMachine {
		t.Fatalf("machine tags wrong: host=%d right=%d", hostTpl.Machine, rightTpl.Machine)
	}
	if string(hostBlob) == string(rightBlob) {
		t.Fatalf("blobs for different machines must differ in machine tag")
	}

	host.ClearAll()
	if got := host.States()[Left]; got != Empty {
		t.Fatalf("host state = %s, want empty", got)
	}
	if got := rightMgr.States()[Left]; got != Saved {
		t.Fatalf("right machine state = %s, want saved (must be independent)", got)
	}
	if hostLog.CountByKind(EvSaved, Left) != 1 || rightLog.CountByKind(EvSaved, Left) != 1 {
		t.Fatalf("each machine must log its own save event")
	}
}

func TestInvalidEye(t *testing.T) {
	m, _ := newTestManager(t)
	if _, err := m.Snapshot(Eye(9)); err == nil {
		t.Fatalf("invalid eye must error")
	}
	if err := m.Clear(Eye(0)); err == nil {
		t.Fatalf("Clear invalid eye must error")
	}
}

func TestConcurrentEyeLifecycle(t *testing.T) {
	m, _ := newTestManager(t)
	raw := makeAttachment(t, 1, 2, 2, []uint16{100, 100, 100, 100})
	var wg sync.WaitGroup
	for _, eye := range []Eye{Left, Right} {
		wg.Add(1)
		go func(eye Eye) {
			defer wg.Done()
			for frame := 0; frame < 50; frame++ {
				if _, err := m.Load(eye, raw, uint64(frame)); err != nil {
					t.Errorf("Load: %v", err)
					return
				}
				if _, err := m.Parse(eye); err != nil {
					t.Errorf("Parse: %v", err)
					return
				}
				if _, err := m.ApplyMask(eye, 50, 0.99); err != nil {
					t.Errorf("ApplyMask: %v", err)
					return
				}
				if _, _, err := m.Save(eye); err != nil {
					t.Errorf("Save: %v", err)
					return
				}
			}
		}(eye)
	}
	wg.Wait()
	for _, eye := range []Eye{Left, Right} {
		snap, _ := m.Snapshot(eye)
		if snap.State != Saved || !snap.HasDepth || !snap.HasMask || !snap.HasTemplate {
			t.Fatalf("eye %s final snapshot wrong: %+v", eye, snap)
		}
	}
}
