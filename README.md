# 双眼共享状态渲染审计（stereoaudit）

纯 Go 标准库实现的双眼（VR 风格）软件渲染管线模拟器，用于复现并定位
“材质 / 模板测试 / 裁剪状态由双眼共享时，单眼状态更新污染另一眼”的问题。

## 模型

- 每只眼睛拥有独立的 RGBA8 帧缓冲与 8 位模板缓冲；材质、模板、裁剪状态
  保存在唯一的共享状态槽 `Engine.current` 中。
- 批次分三类，均在 `stereo/batch.go` 中定义：
  - `SharedBatch`：共享几何，只光栅化一次，结果复制到双眼同一区域；
  - `MonoBatch`：单眼批次，记录唯一眼别（`MaskLeft` / `MaskRight`）；
  - `StereoBatch`：双眼批次，用同一状态分别绘制左右眼。
- 每条批次执行后固化一条 `Record`：眼别、状态键（FNV-1a）、
  输入资源摘要（几何 + 纹理名称哈希）、左右眼画面哈希。
- 默认开启状态隔离：单眼批次对共享状态的修改在批次结束后还原；
  `EngineConfig.DisableStateIsolation` 可关闭隔离以复现事故路径，
  `Batch.Inherit` 让批次直接沿用当前共享状态。

## 故障注入与定位

`stereo.RunAudit` 在同一场景上分别运行基线与故障管线，并逐批次保存双眼快照：

1. `InjectFault` 只改写指定眼别的单眼批次（材质或模板），深拷贝不污染原场景；
2. 比对每条记录的状态键，报告首个状态键分歧的眼别与批次；
3. 逐批次做像素 diff，报告每只眼睛首个可见分歧的序号、批次、像素数与包围盒；
4. 汇总最终画面上的残留差异区域。

顺序提交 `SubmitSequential` 与批量提交 `SubmitBulk` 走同一个执行入口，
因此记录与双眼画面哈希逐字段一致。

## 运行

```bash
go run ./cmd/stereodemo
go test ./...
go test -race -cover ./...
```

演示程序会输出：全部批次审计记录、顺序/批量一致性、单眼材质错误定位、
单眼模板错误定位，以及关闭隔离后单眼更新在共享状态槽中扩散的污染对照。

## 目录

- `stereo/state.go`：材质、模板、裁剪状态与状态键；
- `stereo/resources.go`：几何、纹理样本与资源摘要；
- `stereo/framebuffer.go`：双眼帧缓冲、模板缓冲、像素 diff；
- `stereo/batch.go`：三类批次与审计记录；
- `stereo/engine.go`：状态隔离、光栅化、模板/裁剪、快照与哈希；
- `stereo/fault.go`：故障注入与首个受影响眼别/区域定位；
- `stereo/scenario.go`：演示场景的资源与固定批次序列；
- `cmd/stereodemo/main.go`：可运行演示。
