package xr

import "fmt"

// S3ClockJump injects a discontinuity into the render clock while tracking is
// healthy and verifies the machine does not mistake it for a tracking loss.
func S3ClockJump() string {
	cfg := DefaultConfig()
	clock := NewClock(0)
	tk := NewTracker(clock)
	sm := NewMachine(cfg, PolicyInertial)

	var out string
	out += "================ 场景 3：时钟跳变 ================\n"
	out += fmt.Sprintf("跳变检测阈值 |Δt| > %.0f ms, 丢失超时 %.0f ms\n",
		cfg.ClockJumpMs, cfg.LostTimeoutMs)

	const jumpAt = 1000.0
	const jumpMs = 500.0 // positive jump; negative jumps are handled identically
	jumped := false
	spuriousLoss := false
	printed := 0
	for clock.Now() <= jumpAt+600 {
		if !jumped && clock.Now() >= jumpAt-stepMs {
			clock.Jump(jumpMs)
			jumped = true
		} else {
			clock.Advance(stepMs)
		}
		s, ev := tk.Poll()
		r := sm.Observe(clock.Now(), s, ev, nil)
		if r.State == StateLost {
			spuriousLoss = true
		}
		if jumped && printed < 8 {
			out += fmt.Sprintf("t=%8.0f  状态=%-13s 样本年龄=%6.1f ms 跳变标记=%v 透明度=%.2f\n",
				clock.Now(), r.State, r.AgeMs, r.ClockJumped, r.Alpha)
			printed++
		}
	}
	if spuriousLoss {
		out += "结果: 时钟跳变被误判为追踪丢失 (异常)。\n"
	} else {
		out += "结果: 时钟跳变仅重置样本年龄基准, 未触发 LOST/黑帧, 渲染保持 NORMAL。\n"
	}

	// Negative jump (clock goes backwards).
	clock2 := NewClock(0)
	tk2 := NewTracker(clock2)
	sm2 := NewMachine(cfg, PolicyFade)
	jumped = false
	printed = 0
	for clock2.Now() <= 1700 {
		if !jumped && clock2.Now() >= 1000-stepMs {
			clock2.Jump(-400)
			jumped = true
		} else {
			clock2.Advance(stepMs)
		}
		s, ev := tk2.Poll()
		r := sm2.Observe(clock2.Now(), s, ev, nil)
		if jumped && printed < 6 {
			out += fmt.Sprintf("[负跳变] t=%8.0f 状态=%-13s 跳变标记=%v\n",
				clock2.Now(), r.State, r.ClockJumped)
			printed++
		}
	}
	return out
}
