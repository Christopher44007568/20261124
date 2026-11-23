# stereocompare

纯 Go 标准库实现的立体渲染提交路径对比实验：同一场景分别走
**单趟多视图（multiview-single-pass）** 与 **两次独立提交（two-independent-passes）**，
比较颜色、深度、透明顺序、提交批次与内存，并注入一次“只更新单眼矩阵”的故障，
定位错误来自哪条提交路径。无任何第三方包。

## 运行

```bash
go test ./...          # 单元测试：等价性、故障定位、裁剪/非对称/透明/共享覆盖
go run . artifacts/out # 生成两轮对比的 PNG 与 Markdown 报告
```

报告：`artifacts/out/REPORT.md`；每轮输出参考/多视图/双趟的左右眼颜色、深度与差异图。

## 两条路径

- `stereo/paths.go` 的 `RenderMultiview`：1 次提交、1 次视图绑定、UBO 双眼槽同批刷新。
- `stereo/paths.go` 的 `RenderTwoPass`：2 次提交、2 次视图绑定、每趟独立 UBO 槽，
  故障下右眼槽不刷新而沿用上一帧矩阵。
- 两条路径调用同一个确定性光栅化核心 `renderEye`，因此健康时输出逐位一致。

## 故障注入与定位

- `FaultSingleEyeMatrix` 仅作用于双趟路径：右眼 `ViewProj` 被旧矩阵替换。
- 诊断 `stereo/compare.go` 以无故障参考渲染为真值，结合每眼 VP 矩阵 FNV 指纹、
  逐像素颜色/深度差异与透明顺序，输出错误归属与证据。

## 覆盖场景

- 跨眼不同裁剪：非对称视锥使左眼多出侧面裁剪（45 vs 51 个裁剪后三角形）。
- 非对称投影：左右 `Frustum` 的 l/r 与中心偏移不同（`stereo/scene.go`）。
- 共享实例：6 个 `*Mesh` 在两条路径间零复制，几何内存在两条路径完全相等。
- 透明顺序：两个透明片在左右眼的画家算法次序发生翻转并被记录比对。

## 软件光栅化

`stereo/raster.go` 实现 Sutherland–Hodgman 六平面裁剪、透视除法、
透视校正重心插值、深度测试与 alpha 混合；深度为 NDC z，透明片不写深度。
