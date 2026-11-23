# fidelitycheck

一个只依赖 Go 标准库的合成图像修复评测器。它不把全图平均误差当作唯一结论，而是同时测量中心细节、外围补全、彩色边缘保留和连续帧稳定性。

## 快速运行

```bash
go run ./cmd/fidelitycheck
```

默认会扫描 3 个场景、3 个掩码半径和 4 种重建算法，共 36 次运行。完整 JSON 可写到文件：

```bash
go run ./cmd/fidelitycheck \
  -radii 25,40,55 \
  -algorithms mean,nearest,blur,diffusion \
  -json report.json
```

退出码：

- `0`：运行完成且所有质量门禁通过。
- `1`：运行完成，但至少一个算法触发了质量门禁。
- `2`：参数或运行错误。

使用 `-gate=false` 可在门禁失败时仍返回 `0`，适合先收集报告。

## 指标与场景

| 场景 | 合成内容 | 主要测量 |
| --- | --- | --- |
| `frequency-gradient` | 中心约 0.21 cycles/px、向外降到约 0.018 cycles/px 的径向频率渐变 | `center_mae`、`center_p95`，以及掩码外圈环带的 `periphery_mae` |
| `colored-edges` | 高饱和红、绿、蓝、黄等彩色扇区和环带边界 | `edge_mae`、`edge_retention` |
| `slow-motion` | 每帧约移动 2 px 的彩色条纹目标 | `temporal_mae`、`flicker_mean/p95/max` |

半径参数是“较短边一半”的百分比。例如 `30` 表示直径为较短边 30% 的圆形掩码。评测会改变半径，避免只针对一种缺损尺寸调参。

每个序列至少需要 3 帧。`center_p95`、`center_max` 和闪烁指标覆盖全部帧/帧对，而不是取第一张或最后一张截图。`flicker` 使用重建结果与真值的时间二阶差分残差，专门捕获匀速运动目标上的逐帧抖动。

## 失败规则

同一场景、同一半径下，每个算法都会和 `-baseline` 指定的参考算法比较。若全图 MAE 有改善，但出现以下任一情况，该次运行判为 `FAIL`：

- 中心平均误差或中心 P95 变差（`center_regression`）。
- 彩色边缘场景中边缘保留率明显下降（`edge_retention_regression`）。
- 运动场景中帧间闪烁 P95 变差（`temporal_flicker_regression`）。

因此“外围补全得更好、平均误差更低，但中心细节或连续帧稳定性退化”不能通过门禁。默认容差在 `internal/eval/eval.go` 中定义。

## JSON 报告

报告包含：

- 扫描配置和总运行数、失败数。
- 每个场景/半径的掩码、中心区、外围环带像素数。
- 每个算法的全局、掩码内、中心、外围、边缘误差。
- `center_p95`、`center_max`、逐帧指标、逐帧时间误差和闪烁序列。
- 相对 baseline 的增益/退化量及具体违规原因。

## 零外部依赖

模块只使用 `fmt`、`image`、`math`、`encoding/json`、`flag` 等标准库，`go.mod` 没有 `require` 项。实现不使用随机数，相同参数会生成相同序列和报告。

```bash
go test ./...
```
