# 20261124

## LOD 轮廓误差审计

纯 Go 标准库实现，不依赖外部渲染器或网格库。

运行：

```bash
go run . > artifacts/lod-audit.json
go test ./...
```

程序对同一个程序化位移网格生成三级 LOD：

- `LOD 0`：原始网格，1280 个三角形。
- `LOD 1`：QEM 边折叠，约 62% 三角形。
- `LOD 2`：QEM 边折叠，约 34% 三角形。
- 所有边界顶点禁止折叠，外部边界裂缝保持为零。

审计场景：

- 正面：正交正面观察。
- 掠射角：低入射角放大轮廓误差。
- 快速缩放：提高相机缩放系数。
- 遮挡后重新出现：中央遮挡区域从覆盖掩码中剔除后再比较。

每个相邻 LOD 双向切换均记录：

- `screenSpaceErrorPx`：双向最近顶点几何误差换算成屏幕像素，含 max、p95、rms、mean。
- `normalJumpDegrees`：逐像素插值法线的夹角跳变，含 max、p95、rms、mean。
- `boundaryCrackPx`：锁定边界的双向最近点像素裂缝。
- `missingPixels` / `extraPixels`：参考覆盖掩码的缺失或增加像素。
- `onePixelBreaches`：缺失覆盖像素数量；任意一个缺失即判失败。

当前生成的 `artifacts/lod-audit.json` 会保留实际审计结果。掠射角场景预期可能失败，用来证明一像素轮廓断裂会被捕获；正面、快速缩放和遮挡后重新出现应通过。
