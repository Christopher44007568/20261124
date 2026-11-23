# Panel Boundary Sampling

Go 实现的单眼面板边界采样与验证包，不依赖图形 SDK。用于在畸变、色差和重投影之后，对每个源采样坐标执行明确的越界策略，并防止回退跨眼或污染深度。

## 语义

- 采样坐标是归一化 UV，面板内为 `[0, 1]`，像素中心对应 `0` 和 `1`。
- 颜色是预乘 alpha 的 RGBA，透明区域不会在双线性过滤时引入颜色。
- `PolicyClamp`：双线性采样时使用最近的边缘纹素。
- `PolicyRepeat`：只在当前眼纹理内按 `[0, 1)` 环绕；眼与纹理不匹配会返回 `ErrEyeMismatch`。
- `PolicyTransparentBlack`：返回预乘透明黑、`Valid=false`、`HasDepth=false`。
- `NaN`/`Inf` 等非有限坐标始终记录，并安全退化为透明黑且无深度。

每次越界请求都会写入 `OOBLog`，不做去重。记录包含序号、眼、阶段、通道、原始源坐标、解析后坐标、是否成功回退以及深度是否有效。

## 深度规则

`Sample.Valid` 表示颜色采样可用；`Sample.HasDepth` 单独表示深度可用。只有四邻域所有有效权重的纹素都带深度时，采样才带深度。`DepthBuffer.Store` 同时要求 `Valid && HasDepth`，因此透明黑、透明 UI 和无效采样既不会创建新深度，也不会覆盖已有深度。

## 示例

```go
log := panelsampling.NewOOBLog()
sampler, _ := panelsampling.NewSampler(panelsampling.PolicyClamp, log)

sample, err := sampler.Sample(leftTexture, panelsampling.SampleRequest{
    Eye:     panelsampling.EyeLeft,
    Stage:   panelsampling.StageChromaticAberration,
    Channel: panelsampling.ChannelR,
    UV:      panelsampling.Coord{X: -0.02, Y: 0.5},
})
if err != nil {
    // 眼不匹配、无效通道或策略错误。
}
if sample.Valid && sample.HasDepth {
    // 可安全写入深度。
}
```

## 验证

```sh
go test -race ./...
go vet ./...
```

测试覆盖三种回退策略、逐通道与重复坐标审计、双眼隔离、边缘细线、角落高光、透明 UI、混合透明度深度，以及无效采样不覆盖有效深度。
