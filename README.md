# 滚动快门逐行曝光模型

本工程用 Go 标准库实现逐行曝光的滚动快门分析。每一行都有独立的曝光窗口、读出顺序和姿态采样时刻，画面顶部与底部不会复用同一个姿态。

## 行时间模型

物理图像行号 `row` 从上到下递增，`H` 为总行数：

```text
top-to-bottom: order = row
bottom-to-top: order = H - 1 - row

exposureStart(row) = T0 + order * rowPeriod
exposureEnd(row)   = exposureStart(row) + exposureDuration
poseSample(row)    = exposureStart(row) + poseSampleOffset
timeError(row)     = poseSample(row) - (T0 + poseSampleOffset)
```

`T0` 是读出顺序中第一行的曝光起点，不一定是物理顶行。曝光起点、读出方向和姿态采样偏移分别由 `RowTimingConfig` 管理；`RowTiming.Row` 返回独立的 `ExposureStart`、`ReadoutOrder` 和 `PoseSampleTime`。时间误差以第一行的姿态采样时刻为全局快门参考，并保留符号。

## 三段轨迹与逐行投影

`ThreeSegmentTrajectory` 依次执行：

1. 平移：相机位置从原点到 `Translation`。
2. 俯仰：绕相机 `x` 轴旋转 `Pitch`。
3. 突然转向：在较短时间内绕 `y` 轴旋转 `Turn`（yaw）。

每个图像行按自己的 `PoseSampleTime` 调用 `PoseSource.PoseAt`，然后用该姿态将世界空间直线投影到针孔相机。`ProjectLineAtRow` 二分求解直线上投影纵坐标等于目标行中心的世界点，因此同一世界直线在不同行会得到不同投影像素。

直线弯曲量定义为：所有有效行投影点相对“首行投影点—末行投影点”弦的最大垂直距离，单位为像素；同时输出 RMS 弯曲量和最大弯曲所在行。

## 强制标记

- 方向反转：`bottom-to-top` 会在汇总中设置 `directionReversed: true`，并加入 `readout_direction_reversed` 旗标；每一行也带有 `directionReversed`。
- 姿态缺样：`PoseSource` 返回 `ErrPoseMissing` 时，该行保留时间和误差数据，但设置 `poseMissing: true`、`poseAvailable: false`，不填入相邻姿态，也不生成投影；汇总加入 `pose_samples_missing` 旗标。
- 采样异常或直线未覆盖某行时，分别加入 `pose_sampling_errors` 或 `line_projection_incomplete`。

`KeyframePoseSource` 会在关键帧范围外或关键帧间隔超过 `maxGap` 参数时返回 `ErrPoseMissing`。`DropoutPoseSource` 可显式屏蔽一段时间，用于回放真实缺样。

## 运行

```bash
# 默认 480 行、三段轨迹、完整 JSON（包含每行投影）
go run ./cmd/rollingshutter

# 只看汇总
go run ./cmd/rollingshutter -summary-only

# 反向读出，并在 2ms 到 2.5ms 之间制造姿态缺样
go run ./cmd/rollingshutter -summary-only \
  -direction bottom-to-top \
  -drop-pose-start 2ms -drop-pose-end 2500us

go test ./...
```

常用参数：`-rows`、`-row-period`、`-exposure`、`-pose-offset`、`-frame-start`、`-translation-x`、`-pitch`、`-turn`。JSON 中时间字段以纳秒整数输出，世界坐标以米为单位，图像坐标以像素为单位。

默认输出的汇总示例如下：

```json
{
  "summary": {
    "readoutDirection": "top-to-bottom",
    "directionReversed": false,
    "rowCount": 480,
    "projectedRows": 480,
    "poseMissingRows": 0,
    "topRowSampleNs": 500000,
    "bottomRowSampleNs": 4332000,
    "topBottomSampleDeltaNs": 3832000,
    "maxAbsTimeErrorNs": 3832000,
    "maxAbsTimeErrorRow": 479,
    "straightLineCurvature": {
      "computed": true,
      "maxPixels": 67.84420475248413,
      "rmsPixels": 48.66673701536505,
      "atRow": 312,
      "chordLengthPixels": 493.00636589301337
    },
    "flags": []
  }
}
```
