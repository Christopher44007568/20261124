package xr

import (
	"fmt"
	"math"
	"strings"
)

const stepMs = 10.0

// S1Outage runs a 300 ms tracking outage under each policy and reports what
// the renderer showed: inertial error growth, freeze behaviour and black
// frames.
func S1Outage() string {
	const outageMs = 300.0
	type row struct {
		tAfter        float64
		state         State
		inertialErr   float64
		freezeErr     float64
		extrapolating bool
		alpha         float64
	}
	var out string
	out += "================ 场景 1：300 ms 追踪中断 ================\n"

	var inertialPeak, freezePeak float64
	var blackFrames int
	for _, policy := range []Policy{PolicyInertial, PolicyFreeze, PolicyFade} {
		clock := NewClock(0)
		tk := NewTracker(clock)
		sm := NewMachine(DefaultConfig(), policy)
		var rows []row
		blackFrames = 0
		inertialPeak, freezePeak = 0, 0
		var rawJump, smoothedBias float64
		recoverySeen := false

		outageStart, outageEnd := 1000.0, 1000.0+outageMs
		tk.ScheduleOutage(outageStart, outageEnd)

		for clock.Now() <= outageEnd+350 {
			clock.Advance(stepMs)
			t := clock.Now()
			s, ev := tk.Poll()
			if s != nil && !recoverySeen && t >= outageEnd {
				// Raw discontinuity the renderer would have to absorb.
				if sm.last != nil {
					rawJump = PositionError(sm.last.Pose, s.Pose)
				}
				recoverySeen = true
				smoothedBias = 0
			}
			r := sm.Observe(clock.Now(), s, ev, nil)
			if recoverySeen && r.State == StateRecovery {
				smoothedBias = math.Max(smoothedBias, PositionError(r.RenderPose, s.Pose))
			}
			inOutage := t > outageStart && t < outageEnd
			if inOutage && r.Black {
				blackFrames++
			}
			if inOutage {
				gt := tk.GroundTruth(t)
				// Compare against what each policy would render at this tick.
				ie := math.NaN()
				fe := math.NaN()
				if sm.last != nil {
					age := t - sm.last.TMs
					ep := Extrapolate(sm.last.Pose, sm.linVel, sm.angVel, age/1000)
					ie = PositionError(ep, gt)
					fe = PositionError(sm.last.Pose, gt)
				}
				if r.Extrapolated {
					inertialPeak = math.Max(inertialPeak, ie)
				}
				freezePeak = math.Max(freezePeak, fe)
				rows = append(rows, row{t - outageStart, r.State, ie, fe, r.Extrapolated, r.Alpha})
			}
		}

		out += fmt.Sprintf("\n--- 策略: %s ---\n", policy)
		out += "中断后(ms)  状态            外推误差(m)  冻结误差(m)  外推中  内容透明度\n"
		for _, r := range rows {
			flag := ""
			if r.extrapolating {
				flag = "是"
			}
			out += fmt.Sprintf("%9.0f   %-13s  %10.4f  %10.4f  %-4s   %8.3f\n",
				r.tAfter, r.state, r.inertialErr, r.freezeErr, flag, r.alpha)
		}
		if policy == PolicyInertial {
			out += fmt.Sprintf("峰值: 限时外推误差(预算内) %.4f m, 冻结误差 %.4f m, 黑帧 %d 帧 (每帧 %.0f ms)\n",
				inertialPeak, freezePeak, blackFrames, stepMs)
		} else {
			out += fmt.Sprintf("峰值: 冻结误差 %.4f m, 黑帧 %d 帧 (每帧 %.0f ms, 该策略不做惯性外推)\n",
				freezePeak, blackFrames, stepMs)
		}
		out += fmt.Sprintf("恢复: 样本原始跳变 %.4f m, RECOVERY 平滑期最大首帧偏差 %.4f m\n",
			rawJump, smoothedBias)
	}

	// Standalone answer to "how long can the old pose be extrapolated?".
	clock := NewClock(0)
	tk := NewTracker(clock)
	for clock.Now() < 1000 {
		clock.Advance(stepMs)
		tk.Poll()
	}
	base := tk.GroundTruth(1000)
	lin, ang := tk.Velocities(1000)
	out += "\n旧姿态限时外推可用窗口 (阈值 3 cm / 10 cm):\n"
	out += "外推时长(ms)  位置误差(m)   角度误差(rad)\n"
	for _, dtMs := range []float64{20, 50, 80, 100, 120, 150, 200, 300} {
		ep := Extrapolate(base, lin, ang, dtMs/1000)
		gt := tk.GroundTruth(1000 + dtMs)
		out += fmt.Sprintf("%9.0f    %10.4f   %11.4f\n",
			dtMs, PositionError(ep, gt), AngleError(ep.Q, gt.Q))
	}
	return out
}

// S2CrossRoom relocates the tracker into a different room (new world origin)
// and checks whether the old trail and anchors survive the origin swap.
func S2CrossRoom() string {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyInertial)

	// Two anchors placed in room A before the move.
	var anchors []Anchor
	anchorGT := map[string]Pose{}
	for t := 0.0; t <= 800; t += stepMs {
		clock.Advance(stepMs)
		s, ev := tk.Poll()
		sm.Observe(clock.Now(), s, ev, anchors)
	}
	a1 := tk.SpawnAnchor("desk-anchor", "blue")
	a2 := tk.SpawnAnchor("lamp-anchor", "yellow")
	anchors = append(anchors, a1, a2)
	for id, a := range map[string]Anchor{"desk-anchor": a1, "lamp-anchor": a2} {
		anchorGT[id] = a.Pose.Transform(tk.originFrom.Invert())
	}

	// Loss across the room, then relocalization: room B origin is offset by
	// 6 m and rotated 90 degrees relative to the canonical room frame.
	relAt := 1000.0
	tk.ScheduleOutage(relAt, relAt+250)
	roomB := RigidTransform{
		Q: QuatFromAxisAngle(Vec3{0, 1, 0}, math.Pi/2),
		T: Vec3{X: 6, Y: 0, Z: -2},
	}

	var out string
	out += "================ 场景 2：跨房间重定位 ================\n"
	out += fmt.Sprintf("重定位前原点: room-A, 轨迹长度 %d 个渲染位姿, 锚点 %d 个\n",
		len(sm.Trail()), len(anchors))

	var preRelocTrail, projectedTrail []Pose
	firstFrameBias := math.NaN()
	rawSampleJump := math.NaN()
	relocDone := false
	var transitions []string
	prevState := sm.State()
	for clock.Now() <= relAt+900 {
		clock.Advance(stepMs)
		now := clock.Now()
		if now == relAt+250 {
			tk.Relocate(now, "room-B", roomB)
		}
		s, ev := tk.Poll()
		if ev != nil {
			// Snapshot the old trail; the machine re-projects it in-place
			// through Delta when it consumes the reloc event below.
			preRelocTrail = append(preRelocTrail, sm.Trail()...)
			for i := range anchors {
				anchors[i] = tk.ReprojectAnchor(anchors[i], *ev)
			}
		}
		r := sm.Observe(now, s, ev, anchors)
		if r.State != prevState {
			transitions = append(transitions,
				fmt.Sprintf("  t=%7.0f ms: %s -> %s", now, prevState, r.State))
			prevState = r.State
		}
		if ev != nil {
			projectedTrail = append(projectedTrail, sm.Trail()...)
		}
		if ev != nil {
			relocDone = true
			if s != nil {
				// The recovery span the smoothstep has to bridge: the frozen
				// frame (recoverFrom) vs the first sample in the new origin
				// (recoverTo). The renderer eases across it instead of snapping.
				rawSampleJump = PositionError(sm.recoverFromPose(), s.Pose)
			}
		}
		if relocDone && r.State == StateRecovery && math.IsNaN(firstFrameBias) && s != nil {
			// First RECOVERY frame: frozen pose eased toward the fresh sample.
			firstFrameBias = PositionError(r.RenderPose, s.Pose)
		}
	}

	out += fmt.Sprintf("重定位瞬间原始采样跳变: %.4f m, RECOVERY 首帧残余偏差: %.4f m\n",
		rawSampleJump, firstFrameBias)
	out += "状态迁移序列:\n" + strings.Join(transitions, "\n") + "\n"

	// Did the old trail survive? Every legacy point must map through Delta to
	// within rounding error of the trail re-projected by the state machine.
	ev := sm.LastReloc()
	maxTrailErr := 0.0
	for i, p := range preRelocTrail {
		want := p.Transform(ev.Delta)
		if i < len(projectedTrail) {
			maxTrailErr = math.Max(maxTrailErr, PositionError(want, projectedTrail[i]))
		}
	}
	out += fmt.Sprintf("旧轨迹经 Delta 变换后与新原点轨迹最大偏差: %.6f m (0 表示可继续使用)\n",
		maxTrailErr)

	out += "锚点重投影后的世界误差 (应始终贴在真实世界位置):\n"
	for _, a := range anchors {
		err := tk.AnchorWorldError(a, anchorGT[a.ID])
		out += fmt.Sprintf("  %-12s 原点=%s 世界误差=%.6f m\n", a.ID, a.Origin, err)
	}
	out += "结论: 旧轨迹/锚点在旧原点下的原始坐标不可直接使用, 经\n"
	out += "      p_new = Delta * p_old 重投影后可无缝继续使用。\n"
	return out
}
