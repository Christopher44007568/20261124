# 透明层时间重投影对比器

这是一个只使用 Go 标准库的 CPU 透明渲染实验。它不支持任何图形 SDK，也不依赖
OpenGL、Vulkan、Metal 或第三方图像库。

场景包含三个独立透明层：

- `fog`：远层全屏雾，带缓慢移动的密度纹理。
- `glass`：中层旋转玻璃板，持续移动并带轻微深度起伏。
- `filament`：近层细丝，仅约一个像素宽。

程序同时运行三条时间路径，并与每帧重新超采样得到的参考结果比较：

1. `per-layer`：每个透明物体单独保存覆盖率、颜色、深度和历史权重，再按深度
   从远到近合成。
2. `whole-block`：先合成整块颜色，再用一个平均运动向量和共享权重重投影。
3. `no-history`：只显示当前帧，不使用历史。

## 关键约束

- 透明层按 ID 分开保存历史，不共享同一张历史纹理。
- 历史权重由层深度和屏幕运动分别计算，例如近层细丝与远层雾的权重不同。
- 每个历史样本都要通过深度容差检查；深度不匹配时丢弃历史。
- 历史颜色/覆盖只允许落在当前图层的像素足迹内。物体移走后，旧位置不会再把
  历史覆盖当成近表面。
- 最终合成严格按深度从远到近执行，顺序错误会改变颜色结果，测试会直接覆盖
  这一行为。

## 指标

`coverage` 指标关注近层（玻璃和细丝），不把全屏雾算作物体覆盖：

- `color_rmse`：线性 RGB 相对超采样参考的均方根误差。
- `alpha_mae`：累计透明 alpha 的平均绝对误差。
- `coverage_iou`：近层覆盖率掩码的交并比。
- `order_agreement`：最近可见透明层（考虑深度顺序）与参考的一致率。
- `stale_coverage`：当前近层足迹之外仍存在明显近层 alpha 的像素比例。逐层路径
  应为 0；整块路径通常大于 0。

## 运行

```bash
go run ./cmd/transparency-lab
```

默认输出到 `artifacts/temporal-transparency/`：

- `reference-color.png`、`per-layer-color.png`、`whole-block-color.png`、
  `no-history-color.png`
- 对应四张 `*-coverage.png` 近层覆盖率图
- `report.json`：每帧指标、层深度、历史权重与汇总

常用参数：

```bash
go run ./cmd/transparency-lab \
  -width 128 -height 80 -frames 30 -spp 6 -scale 4 \
  -out artifacts/temporal-transparency
```

`-spp` 是参考帧每个轴的采样数；默认 `6`，即每像素 36 个样本。

## 测试

```bash
go test ./...
go vet ./...
```

测试覆盖：

- 不同深度、不同运动速度得到不同历史权重；
- 合成器无视输入顺序，始终从远到近绘制；
- 移动图层移走后的旧覆盖被拒绝；
- 深度不匹配的历史被拒绝；
- 逐层路径的颜色误差和覆盖率优于整块路径与无历史路径；
- 逐层路径陈旧覆盖为 0，而整块路径会保留旧近层覆盖；
- 逐层路径的绘制顺序一致率优于整块路径。
