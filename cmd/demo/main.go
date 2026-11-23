// Command demo walks the depth/template lifecycle on the host and right
// machines, including the stale-eye guard and a format-version change.
package main

import (
	"fmt"

	"depthlifecycle/depthlife"
)

func main() {
	hostLog := depthlife.NewMemoryLogger()
	rightLog := depthlife.NewMemoryLogger()
	host := depthlife.New(depthlife.Host, hostLog)
	rightMgr := depthlife.New(depthlife.RightMachine, rightLog)

	samples := []uint16{120, 130, 5, 6}
	raw := depthlife.EncodeAttachment(1, 2, 2, samples)

	runEye := func(m *depthlife.Manager, eye depthlife.Eye, name string) {
		att, err := m.Load(eye, raw, 1)
		if err != nil {
			panic(err)
		}
		if _, err := m.Parse(eye); err != nil {
			panic(err)
		}
		mask, err := m.ApplyMask(eye, 100, 0.5)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s eye=%s mask passed=%v coverage=%.2f att=%s\n",
			name, eye, mask.Passed, mask.Coverage, att.ID[:12])
		if mask.Passed {
			_, blob, err := m.Save(eye)
			if err != nil {
				panic(err)
			}
			fmt.Printf("%s eye=%s saved %d bytes\n", name, eye, len(blob))
		}
	}

	runEye(host, depthlife.Left, "host ")
	runEye(host, depthlife.Right, "host ")
	fmt.Printf("host states: %v\n", host.States())

	if _, err := rightMgr.Load(depthlife.Left, raw, 1); err != nil {
		panic(err)
	}
	if _, err := rightMgr.Parse(depthlife.Left); err != nil {
		panic(err)
	}

	// Cross-frame reuse on the same attachment is allowed.
	res, err := host.Reuse(depthlife.Left, mustID(raw), 2)
	fmt.Printf("host reuse: reused=%v state=%s err=%v\n", res != nil && res.Reused,
		safeState(res), err)

	// A format change invalidates every derived artifact.
	v2 := depthlife.EncodeAttachment(2, 2, 2, []uint16{200, 200, 200, 200})
	if _, err := host.Load(depthlife.Left, v2, 3); err != nil {
		panic(err)
	}
	snap, _ := host.Snapshot(depthlife.Left)
	fmt.Printf("after format change: state=%s depth=%v mask=%v tpl=%v\n",
		snap.State, snap.HasDepth, snap.HasMask, snap.HasTemplate)

	fmt.Printf("host stale-eye events: %d\n", hostLog.CountByKind(depthlife.EvStaleEye, 0))
	fmt.Printf("host format-changed events: %d\n", hostLog.CountByKind(depthlife.EvFormatChanged, 0))
	fmt.Printf("right-machine events: %d (independent log)\n", len(rightLog.Events()))
}

func mustID(raw []byte) string {
	att, err := depthlife.DecodeAttachment(raw)
	if err != nil {
		panic(err)
	}
	return att.ID
}

func safeState(r *depthlife.FrameResult) depthlife.State {
	if r == nil {
		return depthlife.Empty
	}
	return r.State
}
