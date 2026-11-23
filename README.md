# stereogaze

双目注视采样原型。每只眼睛独立生成自己的采样图，共享掩码只用于减少重复计算，不能改变任何单眼质量边界。仅使用 Go 标准库，无外部依赖。

## 模型

- `internal/gaze`：单眼采样图、质量边界、重合区、共享掩码与工作量计划。
- `internal/scenario`：三个必测场景——反向扫视、短暂遮挡、完全同视点。
- `internal/render`：五联画面，依次为左眼采样图、右眼采样图、重合区、共享掩码、立体画面。
- `internal/record`：逐帧 PNG、每场景循环 GIF 和 `records.json` 结构化记录。
- `cmd/stereogaze`：命令行入口。

质量层级按注视点半径分为中央凹、旁中央凹和外围。遮挡眼的所有单元格回退到可配置的外围层级，且该帧禁止任何共享。

## 共享规则

1. 两眼各自独立调用 `BuildMask`，共享逻辑拿不到生成阶段的控制权。
2. 重合区是两眼同时采样的单元格；共享掩码是其子集。
3. 只有两眼都可见、层级相同且都不位于质量边界上的单元格才能共享。
4. `ApplySharing` 重构后的两张图必须与独立生成的图逐格相等，测试与录制都会校验该不变量。

## 运行

```bash
go test ./... -count=1
go run ./cmd/stereogaze -out out -grid 32 -frames 16
```

输出：

- `out/<scenario>/frame_XXX.png`：每帧五联采样画面。
- `out/<scenario>.gif`：每场景的立体画面序列。
- `out/records.json`：每帧的注视点、遮挡状态、重合/共享单元格数、工作量和不变量结果。
