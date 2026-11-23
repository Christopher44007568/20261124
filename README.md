# 逐眼近裁剪对照台

纯 Go 标准库实现的立体近裁剪演示。左右眼各有自己的位置、会聚方向、非对称视锥、
近裁剪多边形、屏幕覆盖和可见三角形计数；不会平均双眼近面，也不会把一只眼的结果
复制给另一只眼。

## 运行

```bash
go run ./cmd/stereonearclip
```

打开 `http://127.0.0.1:8080`。页面提供左右双画布、对象级表格、双眼差值和
`/api/report?scene=<id>` JSON 报告。

直接输出某个场景的报告：

```bash
go run ./cmd/stereonearclip -json -scene large-plane
go run ./cmd/stereonearclip -json -scene long-rod
go run ./cmd/stereonearclip -json -scene near-sheet
```

## 场景

- `large-plane`：大倾斜平面。左眼完全在近面前，右眼因会聚视角裁掉大部分。
- `long-rod`：从相机后方延伸到深处的十二边长杆。双眼裁剪环的位置和可见边缘不同。
- `near-sheet`：半入近面的透明片。透明只影响绘制透明度，不影响裁剪与面积。

## 指标

- `sourceArea3D`：按源三角形求和的三维表面积。
- `clippedArea3D`：近面裁剪后凸多边形的三维表面积。
- `rejectedTriangles`：完全在近面后方的源三角形数。
- `rejectedObjects`：所有三角形都被拒绝的对象数。
- `nearClippedTriangles`：近面裁剪后三角化得到的三角形数。
- `visibleSourceTriangles`：裁剪和视口裁剪后仍有屏幕面积的源三角形数。
- `visibleScreenTriangles`：最终屏幕多边形的三角化计数。
- `screenAreaPx` / `coveragePct`：视口内多边形面积与占比。
- `nearEdgeLengthPx`：近面截边在视口内的像素长度，画布中以红色标出。

## 验证

```bash
go test ./...
go vet ./...
```

测试覆盖单顶点/双顶点穿近面、完全拒绝、近面面积缩减、非对称投影、视口覆盖，
以及三个场景的逐眼差异。`large-plane` 还专门断言左眼近面边为零、右眼大于零，
且右眼裁剪后面积严格小于源面积。
