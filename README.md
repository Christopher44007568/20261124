# eye-buffer atlas 布局与采样验证器

纯 Go 实现，只使用标准库。它把左右眼分配到独立 viewport，按眼生成完整或关闭 mip 的层级，并在真实 RGBA8 atlas 上做布局与双线性采样验证。

重点不是只比较 `width`，而是同时验证：

- atlas 全局 row stride：`align(width*4, alignment)`。
- 每个层级自己的 packed stride、4 字节对齐和 byte offset。
- 双眼 slot、base level、各级 mip、gutter 之间的越界与重叠。
- UV 边缘采样和滤波足迹是否跨出当前 mip，或落入 gutter/另一只眼。
- 实际 RGBA 颜色是否发生串色；错误会指出眼、mip level、UV 和 atlas tap。

## 快速运行

```bash
go test ./...
go run ./cmd/eyebuffer-verify
```

默认示例为：

- 左眼 `257x129`，开启完整 mip。
- 右眼 `191x97`，关闭 mip，因此只有 base level。
- `gutter=3`、`alignment=4`，使用双线性边缘采样。

输出图集为 `artifacts/eye-atlas.png`，通过时终端类似：

```text
PASS atlas=460x287 stride=1840 alignment=4 gutter=3 scale=1
  left  logical=257x129 physical=257x129 levels=9 base-row=1028 slot=(0,0)-(263,287)
  right logical=191x97 physical=191x97 levels=1 base-row=764 slot=(263,0)-(460,103)
texels=62603 samples=570 issues=0
```

常用参数：

```bash
# 高 DPI：逻辑尺寸不变，物理 viewport 按 2x 生成
go run ./cmd/eyebuffer-verify -dpi 2 -left 257x129 -right 191x97

# 改变 gutter、对齐和输出路径
go run ./cmd/eyebuffer-verify -gutter 4 -align 8 -out /tmp/eye-atlas.png

# 只测 nearest，或主动使用 atlas 级 clamp 复现串色
go run ./cmd/eyebuffer-verify -filter nearest
go run ./cmd/eyebuffer-verify -address atlas
```

`-align` 必须是 4 的倍数；RGBA8 的 packed row span 不会接受非四字节对齐配置。

## 故障演示

CLI 可注入故障，失败时以退出码 `1` 结束并直接打印定位信息：

```bash
go run ./cmd/eyebuffer-verify -fault stride
go run ./cmd/eyebuffer-verify -fault bleed
go run ./cmd/eyebuffer-verify -fault overlap
go run ./cmd/eyebuffer-verify -fault gutter
go run ./cmd/eyebuffer-verify -fault cross
```

诊断类型：

| Kind | 含义 |
| --- | --- |
| `stride` | 全局 stride、packed stride 或 byte offset 不匹配 |
| `alignment` | row span、stride 或 offset 未按 4 字节对齐 |
| `overlap` | 双眼或 mip 矩形相交 |
| `mip_bounds` | mip/槽位超出 atlas 或所属 slot |
| `mip_policy` | 某眼关闭 mip，但布局中仍存在额外层级或尺寸错误 |
| `gutter` | 双线性滤波足迹进入 gutter、row padding 或邻接层级 |
| `cross_region` | 采样 tap 跨入另一只眼或另一 mip |
| `color_bleed` | 颜色与应有 texel/blend 不一致 |

例如 `-fault stride` 会先给出：

```text
[stride] atlas: atlas width 460 with 4-byte alignment: expected 1840 bytes/row, got 1839 bytes/row
[alignment] atlas: atlas row stride: expected multiple of 4, got 1839 bytes
```

## 包接口

```go
eyes := []eyebuffer.EyeSpec{
    {Name: "left", Width: 257, Height: 129, Mipmaps: true},
    {Name: "right", Width: 191, Height: 97, Mipmaps: false},
}
atlas, err := eyebuffer.Generate(eyes, eyebuffer.LayoutOptions{
    Gutter: 3, Alignment: 4, Scale: 1,
})
if err != nil {
    panic(err)
}
report := eyebuffer.Validate(atlas, eyebuffer.DefaultValidationOptions())
if !report.OK() {
    for _, issue := range report.Issues {
        fmt.Println(issue)
    }
}
```

`Generate` 会写入稳定的眼/level/坐标纹理指纹，方便定位行错位和串色。若要把真实源图通过 `Atlas.CopyLevel` 写入，可在 `ValidationOptions.Expected` 提供对应的 `TexelFunc`；`CopyLevel` 会先拒绝过小或非 4 字节对齐的源 stride。

每个 `Tap` 都保留 `Raw`、`Local`、`Atlas` 坐标和权重。`ClampToLevel` 是正确语义；`ClampToAtlas` 用于复现“把子区域当整张 atlas 做 clamp”的常见串色 bug。

## 测试覆盖

- 左右眼长度/高度不同且 slot 不重叠。
- 单眼开启 mip、另一眼关闭 mip。
- `2x` 高 DPI 下逻辑尺寸、物理尺寸和 stride 对齐。
- base/mip 的归一化边缘与双线性 tap 不越界。
- gutter 为 0 时的滤波足迹风险。
- 错误 stride、非四字节对齐、mip 重叠。
- 被污染的 texel、跨眼颜色和 atlas 级 clamp 串色。
- PNG 输出尺寸与可见像素一致性。
