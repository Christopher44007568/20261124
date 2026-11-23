# 20261124

## foveq — 联合质量档控制器与双眼预算调度器

纯 Go（仅标准库，零外部依赖）实现的注视点渲染质量治理。核心约束：**eye-buffer
尺寸、注视点质量、帧预算三个旋钮永不独立调整**，它们被绑定为不可变的
`Profile`（一个质量档），控制器只在整档之间升降。

### 设计如何对应需求

- **联合档位**：`profile.go` 的 `Profile` 同时持有每眼分辨率、`FoveaScale` 和
  `FrameBudgetMicros`；`NewLadder` 强制三个维度随档位严格单调，任何“只调一个
  旋钮”的配置都会被拒绝。
- **多信号驱动升降档**：`decide.go` 的规则引擎同时消费平滑后的 GPU 时间、
  中心误差、外围误差、注视速度与负载突增标记（`metrics.go` 的 EMA 滤波）。
  升档需要误差诉求 + 足够 headroom；降档由持续 GPU 超预算驱动。
- **迟滞与冻结**：升档需连续 `UpshiftHoldFrames` 帧合格；负载突增（含额外
  `SpikeCooldownFrames`）与快速注视（`FastGazeDps`）都会清零升档计数并记录
  冻结事件；全局 `ShiftCooldownFrames` 抑制抖动。
- **降档不越界**：所有档位变化经过 `Ladder.clampTier`，最低档继续超预算时
  产生 `boundary_clamp` 事件而不会出现负档位。
- **失败回到上一可用配置**：升档进入试用期（`TrialFrames`），期内失败或持续
  超预算回滚到 `lastGood`，失败档进入 `FailureCooldownFrames` 冷却，防止被
  立刻重新选中；`lastGood` 只在干净、未超预算的非试用帧更新。
- **抗饥饿调度**：`scheduler.go` 用加权 DRR 在左右眼间分配 GPU 预算，并显式
  把等待超过 `StarvationFrames` 的眼睛提到队首。相比平均分配，长期份额收敛到
  权重，且任何一只眼都不会长期零分配。
- **完整遥测**：`telemetry.go` 用有界环形缓冲记录三类流——每帧决策输入
  （原始 + 平滑信号 + 触发原因）、分辨率轨迹、画质事件，可 `Dump` 为 JSON。

### 包结构

- `profile.go`：联合质量档与档位阶梯（含默认 5 档）。
- `metrics.go`：每帧 `Sample`、EMA 平滑、`Decision` 审计记录。
- `controller.go` / `decide.go`：状态机、迟滞规则、试用与回退。
- `scheduler.go`：加权 DRR 双眼调度与饥饿保护。
- `telemetry.go`：线程安全环形缓冲与 JSON 导出。
- `cmd/foveqdemo`：脚本化负载演示，输出 `telemetry.json`。

### 运行

```sh
go test ./... -race        # 单元测试（含竞态检测）
go run ./cmd/foveqdemo     # 演示，生成 telemetry.json
```

### 最小用法

```go
ladder := foveq.DefaultLadder()
tel := foveq.NewTelemetry(1024)
cfg := foveq.DefaultConfig(ladder)
cfg.Telemetry = tel
c, _ := foveq.NewController(cfg)

profile, decision := c.Observe(foveq.Sample{
    Frame: 1, GPUMicros: 4100,
    CenterError: 0.25, PeripheralError: 0.08, GazeVelocity: 40,
})
// profile 是下一帧应使用的完整联合档位，三个旋钮一起生效。
```
