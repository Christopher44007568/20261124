# posebench

纯 Go 的逐行扫描（rolling shutter）头部姿态采样基准。它不依赖图形 SDK，
只使用 Go 标准库；不渲染人脸图像，而是用三维人脸特征点、针孔相机和解析运动轨迹
直接测量几何误差。

项目回答三个问题：

1. 同一帧内，顶行和底行看到的头姿态究竟差多少？
2. 统一时间戳、逐行姿态、分段姿态分别带来多大的姿态误差？
3. 分段边界会不会产生可见跳变？

## 快速运行

```bash
go test ./...
go run ./cmd/posebench
go run ./cmd/posebench -json
go run ./cmd/posebench -scenario fast-turn -frames 240 -segments 8
```

常用参数：

| 参数 | 默认值 | 含义 |
| --- | --- | --- |
| `-scenario` | `all` | `fast-turn`、`side-tilt`、`combined` 或逗号分隔列表 |
| `-frames` | `120` | 在一个运动周期内采样的帧相位数量 |
| `-segments` | `8` | 分段策略的段数 |
| `-fps` | `60` | 帧率 |
| `-readout-ms` | `0` | 逐行读出时长；`0` 表示等于一帧周期 |
| `-direction` | `top-to-bottom` | 扫描方向，也可用 `bottom-to-top` |
| `-unified-at` | `0.5` | 统一时间戳在读出窗口中的位置，取值 `(0, 1]` |
| `-boundary-budget-deg` | `0.5` | 边界不连续角度预算 |
| `-boundary-budget-px` | `0.5` | 边界不连续像素预算 |
| `-json` | `false` | 输出 JSON |
| `-out` | 空 | 写入文件而不是标准输出 |
| `-list` | `false` | 列出运动场景 |

## 模型

### 逐行时间

设帧起始时间为 `t0`，图像高度为 `H`，逐行读出时长为 `T`。从上到下扫描时：

```text
t(row) = t0 + (row / (H - 1)) * T
```

从下到上时用 `1 - row/(H-1)`。`-readout-ms` 小于帧周期时，帧与帧之间留有空白；
基准会在一整个运动周期内均匀抽取帧相位，避免只测到某个幸运时刻。

### 头部运动

姿态是四元数旋转加毫米平移。运动由若干正弦项叠加：

```text
angle(t) = sum(A_i * sin(2*pi*f_i*t + phi_i))
translation(t) = base + sum(...)
```

默认包含三个场景：

| 场景 | 内容 | 峰值角速度 |
| --- | --- | --- |
| `fast-turn` | 2 Hz、±30° 偏航，带少量俯仰和滚转 | 约 378°/s |
| `side-tilt` | 2 Hz、±22° 侧倾，叠加偏航和横向平移 | 约 280°/s |
| `combined` | 偏航、侧倾、点头和侧移组合 | 约 483°/s |

### 相机和特征点

默认 `640x480` 针孔相机，`fx=fy=1000`，主点在图像中心。头部中心位于
`z=550 mm`。14 个特征点覆盖眉眼、鼻梁、鼻尖、面颊、嘴角、下颌和下巴。
这些几何量足以计算剪切和姿态误差，又不需要任何图形库。

## 三种部署策略

| 策略 | 每帧姿态数 | 说明 |
| --- | ---: | --- |
| `uniform @ 50% readout` | 1 | 帧中间时刻的单一姿态；平均误差最小，但仍不能表示整个面板 |
| `per-row` | H | 每个行时间独立姿态；作为精度上界 |
| `segmented smooth` | N | 每段中心一个锚点，段间用四元数球面插值 |

此外保留了两个**仅用于诊断、不可部署**的基线：

| 诊断基线 | 为什么不可部署 |
| --- | --- |
| `uniform @ frame start` | 用同一帧起始姿态覆盖整个面板，正是需求禁止的行为 |
| `segmented hard` | 每段内姿态恒定，段边界直接跳变 |

代码中的 `Deployable` 字段和测试会阻止把这两个基线当作正式方案。

### 分段间为什么没有可见跳变

设第 `i` 段中心行为 `c_i`，锚点姿态为 `q_i`。相邻两个中心锚点之间：

```text
u = (row - c_i) / (c_(i+1) - c_i)
s = u*u*(3 - 2*u)                  // smoothstep
q(row) = slerp(q_i, q_(i+1), s)    // 平移分量同时线性插值
```

`smoothstep` 在锚点处导数为零，锚点之间是一条连续可导的四元数曲线。
段边界落在相邻段中心之间，因此不是“两段各用不同公式拼在一起”，而是同一条曲线
穿过边界，边界本身不会出现位置或速度跳变。段中心直接采样真实姿态，每个锚点处的
姿态也是准确的。

硬分段作为反例保留：它的边界残差会明显超过预算，JSON 中
`boundary_within_budget` 为 `false`。

## 测量定义

所有统计按“场景内所有帧 × 所有行”聚合，并给出均值、RMS、95 分位和最大值。

### 姿态误差

对每一行 `row`，把策略给出的姿态与真实姿态 `pose(t(row))` 比较：

```text
orientation_error = 2 * acos(|dot(q_est, q_true)|)
translation_error = ||t_est - t_true||
```

四元数误差取最短弧，因此不会出现 `+q` 和 `-q` 表示同一旋转时的假误差。

### 图像剪切

同一帧中，真实 rolling shutter 投影与帧中间时刻的 global shutter 投影比较：

1. 用帧中间姿态投影 14 个特征点，得到参考行。
2. 根据每个特征点的参考行找到真实采样时间，再投影；
   对行坐标迭代 4 次，处理“运动中的点落到另一行”的隐式关系。
3. 计算特征点位移的 RMS、最大值，以及参考点到实际点的最小二乘仿射变换。
4. 对面板顶行和底行时间分别投影脸部中心，报告两者的像素漂移和剪切角。

仿射形式为：

```text
[x']   [a b] [x]   [tx]
[y'] = [c d] [y] + [ty]
```

其中 `b` 是水平剪切系数（x 随 y 变化），`c` 是垂直剪切系数。
`top_bottom_shift_px` 更直观，仿射系数更适合做回归和阈值判断。

### 分段边界

对每个段边界行 `b`：

```text
raw_step_deg    = angle(pose_est[b-1], pose_est[b])
true_step_deg   = angle(pose_true[b-1], pose_true[b])
residual_deg    = max(0, raw_step_deg - true_step_deg)
```

像素残差同理，用脸部中心投影后的欧氏距离计算。减去真实相邻行运动，
是为了区分“头部本来就在动”和“算法引入的跳变”。

`boundary_within_budget` 同时要求角度残差和像素残差小于预算。
`max_adjacent_row_step_deg` 则给出整帧任意相邻行的最大阶跃，便于发现非边界位置的异常。

## 输出

表格模式每个场景输出一行剪切统计和五行策略统计。默认参数下（120 帧、8 段）
`fast-turn` 的代表性结果：

```text
image shear: top-bottom 2.84 px avg / 5.33 px max, angle 0.34/0.64 deg,
             landmark RMS 0.41/0.67 px, affine |b|=0.0032 |c|=0.0009
uniform @ frame start   1   2.021/2.573/5.120/6.289 ... FORBIDDEN
uniform @ 50% readout   1   1.013/1.290/2.568/3.145 ... ok
per-row               480   0.000/0.000/0.000/0.000 ... ok
segmented smooth        8   0.044/0.068/0.140/0.394 ... ok
segmented hard          8   0.127/0.161/0.321/0.394 ... JUMP/DIAG
```

这说明：

- 只用帧起始姿态会让平均角度误差翻倍，且最大误差约 6.3°。
- 帧中间统一时间戳优于起始姿态，但仍无法消除面板内 0.3° 量级的剪切。
- 8 段平滑分段的平均误差约 0.04°，边界残差约 0.007° / 0.006 px，
  远低于 0.5° / 0.5 px 预算。
- 硬分段在边界产生约 0.79° 和 0.67 px 的可见跳变。

`-json` 输出 `schema: posebench/v1` 的完整报告。主要字段：

| 字段 | 含义 |
| --- | --- |
| `image_shear` | 面板顶底漂移、剪切角、特征点位移、仿射剪切 |
| `strategies[].orientation_error_deg` | 角度误差分布 |
| `strategies[].translation_error_mm` | 平移误差分布（毫米） |
| `strategies[].boundary_rows` | 分段边界所在行号 |
| `strategies[].boundary_step_deg` | 分段边界原始阶跃 |
| `strategies[].max_boundary_discontinuity_deg` | 扣除真实运动后的角度残差 |
| `strategies[].max_boundary_discontinuity_px` | 扣除真实运动后的像素残差 |
| `strategies[].boundary_within_budget` | 是否满足边界预算 |
| `strategies[].deployable` | 是否允许进入生产路径 |

## 验证

```bash
go test ./...
go vet ./...
```

关键测试：

- 行时间端点、上下扫描方向和逆映射。
- 四元数旋转、最短弧球面插值。
- `per-row` 姿态与真实行时间一致。
- 帧起始统一姿态被标记为不可部署，且误差大于帧中间姿态。
- 三种运动场景下，平滑分段边界均满足预算，硬分段均被拒绝。
- 静态头部剪切为零，快速转头产生非零剪切。

## 边界和限制

- 这是几何基准，不是真实相机标定工具；没有镜头畸变、曝光、噪声、运动模糊、遮挡和光照。
- 人脸用稀疏三维特征点表示，未包含纹理和稠密网格。
- `per-row` 是理想精度上界，真实估计器还会有检测噪声和帧间抖动。
- 图像剪切用脸部中心顶底投影和特征点仿射拟合来定义；它衡量 rolling shutter 几何扭曲，
  不等同于材料力学中的剪切应变。
- 没有接入任何图形 SDK，也不生成或读取图片；如需可视化，可把 JSON 交给外部工具。
