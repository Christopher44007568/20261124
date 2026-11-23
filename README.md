# 20261124

## 纯 Go 重投影拒绝掩码

本模块只使用 Go 标准库，不依赖图形 SDK。输入为 CPU 侧深度、视图空间法线、像素置信度、相机轴与焦距，输出逐像素的历史坐标、接受/拒绝状态和明确拒绝原因。

### 判定结果

- `no_history`：重投影点落在历史帧之外，或位于相机后方。
- `low_history_confidence`：历史深度无效，或历史像素置信度不足，可用于透明边缘。
- `current_invalid`：当前像素没有可靠表面，不能申请历史混合。
- `disocclusion`：历史采样点比预测位置更远，说明新前景露出了旧背景。
- `depth_mismatch`：双向深度差超过保守阈值，但不满足明确遮挡方向。
- `normal_mismatch`：将历史法线转换到当前视图后，与当前表面法线不一致。

深度判定同时包含固定偏差和相对深度比例，因此近墙、薄杆和远处表面不会共用同一个绝对误差。所有拒绝都写入 `PixelResult.Reason`；调用方不能把 `Valid=false` 的像素与历史颜色悄悄混合。

重投影历史坐标使用最近邻像素采样。掩码允许保守拒绝，但测试会保证存在稳定表面时不会整屏丢弃；错误前景、透明边缘和出屏历史都有独立原因，不会被当成可混合样本。

### 覆盖的合成场景

`go test ./...` 运行以下场景：

- 近墙与薄杆：区分 `disocclusion` 和普通 `depth_mismatch`。
- 透明边缘：低置信历史像素被明确标记为 `low_history_confidence`。
- 相机前移：几何预测深度保持稳定墙面有效，只拒绝露出的区域。
- 相机旋转：出屏区域标记为 `no_history`，重叠区域仍可接受，保证不会整屏丢弃。
- 物体切换：错误前景触发深度拒绝，错误表面方向触发法线拒绝。

### 最小用法

```go
validator, err := reproject.NewValidator(reproject.DefaultLimits())
mask, stats, err := validator.Evaluate(currentFrame, currentCamera, historyFrame, historyCamera)
for _, pixel := range mask.Pixels {
    if pixel.Rejected() {
        // 重建或显式清零历史，而不是混合错误表面。
        continue
    }
}
```
