# 眼别拆分阴影投射者基准

纯 Go 标准库实现，无外部依赖。程序把阴影投射者按左/右眼分别标记可见性，
在同一场景中比较三种阴影图布局：

- `shared`：一张阴影图，每个 texel 保存两位眼别可见性。
- `independent`：每眼各一张完整阴影图。
- `hybrid`：按宏单元（默认 8×8 texel）存放；两眼结果相同的单元共享，
  结果不同的单元按眼复制。

## 运行

```bash
go test ./...
go run ./cmd/shadowbench
go run ./cmd/shadowbench -out artifacts/report.json
go run ./cmd/shadowbench -json
```

可调参数：`-frames`、`-map`、`-tile`、`-width`、`-height`、`-light-turns`。
命令在任一不变量失败时返回非零状态码。

## 场景

默认场景包含：

- `occluder-wall`：高 2.8 m 的遮光墙，遮挡墙后的细杆。
- `left-eye-rod` / `right-eye-rod`：墙边缘外的细杆，因视差各自只对一只眼可见。
- `cross-eye-near` / `cross-eye-far`：两眼都可见的跨眼实例。
- 遮光墙本身也是两眼共享的投射者。

相机 IPD 为 6.4 cm。转头时眼位围绕颈部中心旋转，而不只是改变视线方向，
因此压力测试同时改变视锥、视差和遮挡关系。

## 指标口径

- **投射者数**：每帧实际驻留的投射者引用数。`shared`/`hybrid` 使用两眼的
  并集，`independent` 按眼累加，所以单眼专属物体会在独立布局中重复计数。
- **阴影图复用率**：两眼都启用时，由共享存储服务的眼别宏单元查询比例。
  `shared` 固定为 100%，`independent` 为 0%，`hybrid` 落在两者之间。
- **像素误差**：同一组相机射线下，与解析逐像素射线参考的阴影像素分类不一致率。
  只统计命中地面的像素；离散阴影图分辨率会带来量化误差。
- **资源峰值**：阴影数据字节数，以及加上投射者引用和眼别掩码后的总模型字节数。
  宏单元布局还计入每个单元的元数据开销。
- **失效探针**：双眼建图完成后，只切换 `Active` 掩码以模拟一眼失效，不重建、
  不删除存储，再比较存活眼的整幅阴影画面。`changed_pixels` 必须为 0，
  同时专属投射者计数或 `eye_split_texels` 必须大于 0，证明共享层仍保留了两眼差异。

## 压力测试

默认 36 帧，光源旋转 1.5 圈；头部偏航以 6 个周期快速摆动，幅度约 ±24°，
俯仰也做小幅摆动。每帧都重新计算眼别投射者集合和三种布局，记录峰值与累计误差。

默认配置的参考输出：

```text
STRATEGY     CASTERS avg/peak  REUSE%  PIXEL ERR%  SHADOW KiB  RESOURCE KiB
shared       4.3/5             100.0   0.4988      9.0         9.1
independent  6.7/8             0.0     0.4988      18.0        18.1
hybrid       4.3/5             96.8    0.4988      11.8        11.9
```

三种布局的像素误差相同，因为共享布局独立保存眼别位，混合布局只在两眼结果
一致时复用宏单元；任何不一致单元都保留各眼副本。因此复用优化没有引入额外
可见性错误。完整数值和探针结果写入 `artifacts/report.json`。

## 代码结构

- `shadow/scene.go`：地面投影几何、光源射线与遮挡测试。
- `shadow/eyes.go`：双眼姿态、逐眼投射者可见性、默认场景。
- `shadow/shadowmap.go`：共享/独立布局与采样。
- `shadow/hybrid.go`：宏单元分组与逐眼副本。
- `shadow/render.go`：解析参考渲染、策略渲染与像素比较。
- `shadow/bench.go`：旋转光源、快速转头、指标累计和失效探针。
- `cmd/shadowbench/main.go`：命令行与表格/JSON 输出。
