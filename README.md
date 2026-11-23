# 注视边界降噪审计

纯 Go 标准库实现。没有 `require`，不需要 CGO，也不读取外部图像。

程序沿椭圆注视掩码构造法线、深度、亮度同时发生阶跃的合成场景，并比较三种策略：

- `raw`：未降噪输入，保留缺失样本。
- `partitioned`：只平滑外围有效像素；中心像素保持原样。
- `full`：全图平滑，故意忽略注视分区，仅作为越界光晕对照基线。

## 约束

`partitioned` 的每个滤波 tap 必须满足：

```text
Mask[source] == Mask[target]
```

中心样本不会进入外围权重，也不会进入分母。补洞使用同样的分区约束。每个空洞输出都记录：

- 目标像素索引与坐标；
- 所有供体像素索引、坐标和归一化权重；
- 是否跨区；
- 可用记录供体和原始颜色逐项重建结果的校验。

全图模式允许跨区，因此其 `cross_partition_taps` 非零时表示基线确实借用了另一分区样本。

## 场景

- `boundary-step`：沿注视边界直接生成法线、深度、亮度突变。
- `thin-rod`：约两像素宽的细杆穿过边界。
- `translucent-edge`：半透明玻璃边跨过边界，法线和深度被部分混合。
- `exposure-jump`：几何不变、仅亮度曝光发生硬跳变，用于检查亮度边缘停止项。
- `combined`：叠加以上三种压力结构。

输入还包含确定性孤立空洞、2x2 空洞块，以及注视边界内外两侧的空洞。

## 滤波

权重为空间项、法线项、深度项和亮度项的乘积：

```text
w = ws(dx,dy) * wn(dot(n0,n1)) * wd(d0-d1) * wl(l0-l1)
```

每一项目均为高斯型边缘停止权重。中心区在分区模式下完全不参与平滑，但其中的空洞仍会用中心区自身样本补齐。

## 指标

- 中心、外围和边界带颜色误差；
- 内外侧有符号亮度偏置及 `halo_bias`；
- 边缘能量保持率；
- 每个压力特征的 MAE 与边缘能量保持率；
- 跨区 tap 数和归一化跨区权重；
- 空洞供体完整性、权重和、跨区情况与可重建性。

## 运行

```bash
go run ./cmd/gazedenoise -out artifacts/report
```

常用参数：

```bash
go run ./cmd/gazedenoise \
  -out artifacts/report \
  -width 256 -height 192 \
  -noise 0.035 -seed 20261124 \
  -scenes combined,thin-rod \
  -radius 3 -hole-radius 5 \
  -sigma-luma 0.045
```

输出结构：

```text
artifacts/report/
  report.json
  report.md
  combined/
    truth.png
    input.png
    mask.png
    normal.png
    depth.png
    luma.png
    raw.png
    raw-diff.png
    provenance-raw.json
    partitioned.png
    partitioned-diff.png
    provenance-partitioned.json
    provenance-partitioned.png
    full.png
    full-diff.png
    provenance-full.json
```

差异图中红色表示输出比真值亮，蓝色表示比真值暗，青色为注视边界。来源图中目标空洞为洋红色，实际供体为绿色。

## 验证

```bash
go test ./...
go vet ./...
```

测试覆盖场景确定性、边界三项阶跃、三类压力结构跨区、分区零借用、全图基线跨区、补洞来源重建、PNG/JSON/Markdown 输出以及零外部依赖。
