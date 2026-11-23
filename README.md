# 20261124

## 10 bit 运动矢量量化对比

浮点运动矢量量化到 10 bit 有符号补码（代码范围 `[-512, 511]`，1 code = 1 px）。
任何越界或 NaN 分量一律返回 `mvquant.ErrOutOfRange` 拒绝编码，不做静默夹紧。

- `quant.go`：代码范围、向量类型、解码与越界检查
- `encoders.go`：截断 `trunc`、四舍五入 `round`、误差扩散 `diffusion`（逐块时间误差反馈，残差恒 < 1 code）
- `field.go`：平移、旋转、缩放、静止四类 16x16 块向量场
- `metrics.go`：最大位移误差、零向量比例、连续帧残影（块轨迹位置漂移）
- `runner.go` / `cmd/mvquant`：12 组“场 x 编码器”实验与报告输出

运行：

```bash
go test ./...
go run ./cmd/mvquant
```

报告写入 `artifacts/quant_report.txt`。30 帧下截断在慢速平移上零向量比例 100% 且残影线性累积；
误差扩散以少量非零代码换取有界残影（漂移 < 1 px 量级）；静止场三种编码均零误差、零残影。
