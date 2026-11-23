# Timewarp Audit Simulator

纯 Go 的低帧率重投影（late-latched timewarp）模拟器，不依赖 OpenGL、Vulkan、Metal 或任何图形 SDK。图像以符号 ID 表示；程序真正记录的是姿态版本、历史来源、中间图像和父子链。

## 行为约定

- 渲染帧、姿态采样和 scanout 共用一个单调微秒时间轴。
- 调用方必须按事件时间顺序提交；这使“时钟倒退”可以直接作为不可恢复的管线故障。
- 每次 scanout 使用当时最新的渲染帧，并重投影到当时最新的姿态版本。
- 同一渲染帧可跨越多次 scanout 被连续 warp；每次 warp 生成唯一中间图像，父节点是本次实际读取的历史图像。
- 新渲染帧到达后，来源链从该帧的 source 图像重新开始。
- pose 版本未推进或姿态数值未变化时复用现有图像，不产生无意义的 identity warp。
- 重复 frame ID 只计数一次，不替换当前 source，也不增加帧、warp 或来源链接账。
- 任一时间戳小于已观察高水位，管线永久进入 stopped 状态；之后所有 scanout 都返回错误且不会产生新 warp。
- 每次 warp 的 `TargetPose` 是绝对姿态，而不是对上一帧误差继续累加；因此中间链可以很长，但姿态误差不会随 warp 次数漂移。

## 运行

```bash
go run ./cmd/warpdemo -scenario all
go run ./cmd/warpdemo -scenario constant
go run ./cmd/warpdemo -scenario stop
go run ./cmd/warpdemo -scenario drop
```

输出包括：

- 接受帧、重复帧、scanout、warp、复用帧 warp 的计数。
- 最大、最终、累计绝对和 RMS 偏航误差。
- 最终 scanout 的 `source -> intermediate -> ... -> final` 来源链。
- 本次运行观察到的最长来源链。
- 最近 warp 的姿态版本迁移、历史来源和输出中间图像。
- 重复帧和时钟回退保护结果。

测试：

```bash
go test ./...
go test -race ./...
go vet ./...
```

## 三个检查场景

1. `constant-turn`：90 deg/s 匀速转头，120 Hz scanout、约 30 Hz 渲染、250 Hz pose 采样。检查长中间链下最大误差不超过 pose 采样间隔对应的角度上界（约 0.36°）。
2. `sudden-stop`：120 ms 时突然停止。检查停止后只允许一次收敛 warp，后续姿态版本推进但姿态不变时不再 warp，最终误差归零。
3. `frame-drop-recovery`：100-180 ms 不提交渲染帧。检查旧 source 被重复 warp、姿态继续前进，新帧到达后来源链重置并恢复低误差。

## API 速览

```go
p := timewarp.NewPipeline()

pose := timewarp.Pose{Version: 1, At: 0, Yaw: -45}
_, _ = p.SubmitPose(pose)
_, _ = p.SubmitFrame(timewarp.Frame{
    ID:         1,
    RenderTime: 0,
    Pose:       pose,
})

scanout, _ := p.Scanout(timewarp.Millis(8))
chain, _ := p.SourceChain(scanout.ImageID)
for _, image := range chain {
    // image.ParentID, image.SourceFrameID, image.Pose.Version
}

records := p.Records()       // 每次真实 warp 的审计记录
stats := p.Stats()           // 接受量与重复量分离
```

核心类型：

- `WarpRecord`：`HistorySourceID`、`IntermediateImageID`、`HistoryPoseVersion`、`TargetPoseVersion`、`SourceFramePoseVersion`、`DeltaYaw`、`ChainDepth`。
- `Image`：`Kind`、`ParentID`、`SourceFrameID`、`Pose`、`WarpSequence`、`ChainDepth`。
- `Scanout`：最终 `ImageID`、`Warped`、对应 `WarpRecord` 和展开后的 `Provenance`。
- `ScenarioReport`：误差采样、统计、全部 warp 记录、最终链和最长链。

`ErrClockWentBackwards` 是终止条件；停止后返回的错误同时匹配 `ErrWarpStopped` 和 `ErrClockWentBackwards`。
