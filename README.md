# 20261124

## posetimeline

纯 Go 的姿态时间线校验器（无图形 SDK 依赖），决定重投影时该选哪条 pose。

### 规则

- 新旧顺序只看经时钟域转换后的时间戳，绝不把数组最后一项当最新。
- 每条样本产出一条 `Decision`：接受状态、参考时间、全局时间、拒绝原因位掩码。
- 拒绝原因：重复时间戳、时钟倒退、过期 pose、未来 pose、左右眼时间不一致、一帧多 pose、未知时钟域、时间线已失效。
- 时钟倒退会使该眼时间线失效；失效期间拒绝新样本并停止运动积分，`Reset` 后用下一条被接受样本重新锚定。
- 重投影选择：时间戳 `<= targetTime` 的最新一条**已接受**样本，可选最大年龄约束；不返回未来/被拒样本，不做外推。

### 用法

```go
clocks := posetimeline.NewClockRegistry()
clocks.Register("imu", 1000) // imu tick + 1000 = 主时钟 tick

v := posetimeline.NewValidator(posetimeline.Config{
	MaxPoseAge:   5_000_000, // 5ms
	EyeTolerance: 100,       // 左右眼允许 100ns 偏差
}, clocks)

decisions := v.Submit(referenceTime, batch)
sel, ok := v.SelectForReprojection(posetimeline.EyeLeft, targetTime, maxAge)
motion := v.IntegratedMotion(posetimeline.EyeLeft)
```

### 验证

```
go vet ./...
go test -race ./...
```
