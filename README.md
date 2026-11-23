# 20261124

相机(头动)与物体运动分离的屏幕空间速度 / 深度差 / 历史权重计算,仅用 Go 标准库。

- `vec.go` — Vec2/Vec3/Mat4、LookAt、Perspective。
- `motion.go` — `Tracker` 把运动拆成 `CameraSource` / `ObjectSource` 两个独立来源,
  各自记录速度、深度差、历史权重与失效范围;`Combined` 每帧只应用一次物体矩阵。
- `scenarios_test.go` — 移动物体、快速转头、反向运动、静止目标四组对照,
  以及"物体矩阵不重复叠加"的契约测试。

```sh
go test ./...   # 运行对照测试
go run ./cmd/demo
```
