# fovealtemporal

零外部依赖的 Go 实现：注视点感知的时域累积降噪器（foveated temporal
accumulation）。中心（fovea）使用长历史让静帧充分收敛，外围
（periphery）使用短历史快速丢弃旧样本；眼动、物体运动、曝光变化各自生成
独立的失效范围，避免“一刀切”重置。

## 设计要点

### 两套历史长度

- 每个像素按当前注视点划分 `ZoneFovea` / `ZonePeriphery`，带守卫带宽
  （`GuardWidth`）抑制边界抖动导致的归属翻转。
- 累积采用等效样本数 EMA：`alpha = 1 / min(age+1, zoneHistory)`。
  默认中心 32 帧、外围 4 帧（`DefaultParams`），`age` 严格封顶在各自
  历史长度上。

### 三类失效范围

- 眼动：注视速度超过 `SaccadeSpeed` 判定为扫视，重置“上一注视点 → 当前
  注视点”的线段走廊（半宽 `SaccadeCorridor`）与落点圆盘；慢速追踪只移动
  圆盘，新覆盖区域通过区域变更（`ReasonZone`）重建，不做大面积重置。
- 物体运动：仅当运动矢量模长超过 `MotionThreshold` 且与四邻域的散度超过
  `DivergenceThreshold` 时才重置；全局一致的转头运动散度为零，历史保留。
  遮挡揭示同时支持调用方的 `Invalid` 提示与深度差检测
  （`DepthDisocclusionThreshold`）。
- 曝光变化：中等曝光跳变对历史颜色乘以 `ExposureScale` 就地补偿，静帧区域
  不丢历史；`|log2(scale)|` 超过 `ExposureResetThreshold` 才全帧重置。
- 动态高光：亮度超过 `HighlightThreshold` 且与重投影历史的相对偏差超过
  `HighlightTolerance` 时仅重置高光盘内像素（`ReasonHighlight`），高光消失后
  区域重新收敛，无拖影。

### 两条硬约束如何保证

- 外围降噪不拖糊中心：重投影使用分区双线性采样，属于另一区域的采样点权重
  清零并重新归一化；邻域 min/max 钳制（`ClampExpand`）同样只取同区域像素；
  采样点全部属于另一区域时按 `ReasonZone` 用当前样本重建。
- 重置不误删有效区域：所有失效原因以位掩码逐像素记录（`ResetReason`），
  扫视走廊、运动散度、高光、深度揭示都是局部范围；均匀运动、温和曝光、
  未受影响的静帧区域保持 `ReasonNone` 并继续累积。

## 代码结构

- `foveal/types.go`：帧、参数、区域、失效位掩码、统计类型。
- `foveal/params.go`：默认参数与归一化校验。
- `foveal/denoise.go`：降噪器主体与逐帧流水线。
- `foveal/invalidation.go`：扫视走廊、运动散度、高光与曝光判定。
- `foveal/accumulate.go`：分区双线性重投影、深度遮挡、邻域钳制、EMA 累积。
- `foveal/scene.go`、`foveal/scene_rng.go`：零依赖合成场景（静态背景、移动
  物体与真实揭示区、游走高光、曝光、确定性高斯噪声）。
- `foveal/*_test.go`：四类主场景与两条硬约束的测试。
- `cmd/demo/main.go`：串联四个场景，打印统计并输出 PPM 帧。

## 使用方式

```go
d, _ := foveal.New(foveal.DefaultParams(width, height))
frame := &foveal.Frame{
    Width: width, Height: height,
    RGB: rgb, MV: motionVectors, Gaze: gaze,
    Depth: depth, Invalid: disocclusionHints,
    ExposureScale: 1,
}
result, err := d.Process(frame)
// result.RGB 为降噪输出；result.Reset 记录每个像素的失效原因。
```

运行测试与演示：

```sh
go test ./...
go run ./cmd/demo [输出目录]   # 默认写入 artifacts/frames，P6 PPM 格式
```

测试覆盖：静帧收敛（中心误差显著低于外围且 age 封顶）、扫视失效范围与
走廊外历史保留、外围历史不污染中心、物体运动/遮挡揭示、动态高光出现与
消失、曝光补偿与极端重置、全局转头无误重置。
