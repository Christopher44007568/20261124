# 共享几何双目软件光栅器

一个只使用 Go 标准库的双目软件光栅器。左右眼共享同一份顶点和索引，
但相机、实例模型矩阵和材质参数始终按眼独立提供。

## 功能

- 索引三角形软件光栅化：六平面裁剪、透视除法、Z-buffer、透视校正插值。
- 共享几何：同一个 `*Geometry` 可以同时出现在左右眼批次中，只上传一次。
- 逐眼状态：每只眼拥有独立的 `Camera`、`DrawItem.Model` 和 `Material`。
- 统计：左右眼批次、共享几何访问、顶点/索引读取、几何上传和变换上传。
- 参考实现：`RenderReconstructed` 为每只眼深拷贝一份几何后渲染。
- 结果比较：逐像素、逐眼批次/读取/矩阵指纹/上传量对比。

## 快速开始

```go
geometry := &binocular.Geometry{
    Vertices: []binocular.Vertex{
        {Position: binocular.Vec3{X: -1, Y: -1}, Color: binocular.Vec3{X: 1, Y: 1, Z: 1}},
        {Position: binocular.Vec3{X: 1, Y: -1}, Color: binocular.Vec3{X: 1, Y: 1, Z: 1}},
        {Position: binocular.Vec3{X: 0, Y: 1}, Color: binocular.Vec3{X: 1, Y: 1, Z: 1}},
    },
    Indices: []uint32{0, 1, 2},
}

left := binocular.EyePass{
    Eye:    binocular.EyeLeft,
    Camera: leftCamera,
    Items: []binocular.DrawItem{{
        Geometry: geometry,
        Model:    binocular.RotateY(0.2),
        Material: binocular.Material{BaseColor: binocular.Vec3{X: 1, Y: 0.7, Z: 0.5}},
    }},
}
right := binocular.EyePass{
    Eye:    binocular.EyeRight,
    Camera: rightCamera,
    Items: []binocular.DrawItem{{
        Geometry: geometry, // 与左眼是同一个指针
        Model:    binocular.RotateY(-0.2),
        Material: binocular.Material{BaseColor: binocular.Vec3{X: 0.5, Y: 0.7, Z: 1}},
    }},
}

rasterizer := binocular.NewRasterizer()
shared, err := rasterizer.RenderShared(left, right)
reference, err := rasterizer.RenderReconstructed(left, right)
report := binocular.CompareResults(shared, reference)
```

`CompareResults(...).ExactMatch` 要求左右图像完全一致、逐眼批次和
顶点/索引读取一致、变换与材质上传一致，以及逐眼变换矩阵指纹一致。

## 统计口径

- `Batches`：一个 `DrawItem` 在一个眼 pass 中算一个批次。
- `SharedGeometryAccesses`：共享模式下几何绑定的总次数，等于两眼底层
  `GeometryAccesses` 之和；参考模式为 0，因为两遍使用各自的副本。
- `VertexReads` / `IndexReads`：实际从顶点/索引数组读取的元素数。
- `GeometryUploadBytes`：按唯一 `*Geometry` 计费，每个顶点 48 字节
  （位置 + 颜色，均为 3 个 `float64`），每个索引 4 字节。
- `TransformUploads` / `TransformUploadBytes`：每批次上传模型、视图、
  投影三个 `Mat4`，共 3 × 16 × 8 = 384 字节。相机矩阵不会跨眼缓存。
- `MaterialUploadBytes`：每批次 3 个 `float64`，共 24 字节。
- `ClippedTriangles`：按眼独立计数，一只眼的近/远或侧平面裁剪不会影响
  另一只眼。

共享路径只复用几何负载；相机、模型矩阵、材质和帧缓冲均按眼创建。
`TransformDigest` 分别记录左右眼实际矩阵上传序列的指纹，可用于确认
复用过程中没有发生矩阵串用。

## 演示

```bash
go run ./cmd/stereo-demo -out artifacts/demo
```

演示包含一个共享立方体和一个近裁剪探针。左眼实例旋转/材质与右眼不同，
左眼近平面裁剪一个三角形，右眼不裁剪。输出包括：

- `shared_left.ppm`、`shared_right.ppm`
- `reference_left.ppm`、`reference_right.ppm`
- `report.json`（完整统计与比较）

示例输出：

```text
shared batches: left=2 right=2 total=4
shared geometry accesses: 4 (left=2 right=2)
clipped triangles: left=1 right=0
geometry upload: shared=1452 B / 2 upload(s), reference=2904 B / 4 upload(s), saved=1452 B
transform upload: shared=1536 B, reference=1536 B
matches reconstructed reference: true
```

## 测试

```bash
go test ./...
go vet ./...
```

测试覆盖共享/重建结果一致、左右相机/材质/旋转隔离、单眼裁剪隔离、
矩阵指纹不串用以及非法索引拒绝。
