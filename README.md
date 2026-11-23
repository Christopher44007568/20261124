# Go 头姿态四坐标系转换与预览

一个无第三方依赖的 Go 实现：把同一份头/设备姿态在 **World、Device、Camera、Screen** 四个坐标系之间任意转换，并对转换结果做逐点往返校验。项目同时生成 SVG 预览，显示世界锚点、三轴箭头、相机轨迹和屏幕像素视图。

## 坐标契约

所有坐标系都是**右手系**。四元数统一使用**标量在前**的 `(w, x, y, z)` 顺序，旋转为主动旋转（active），遵循右手定则。`q` 与 `-q` 表示同一个旋转；`q.Mul(r)` 表示先应用 `r`、再应用 `q`。

| 帧 | 单位 | +X | +Y | +Z | 手性 |
|---|---|---|---|---|---|
| `World` | 米 `m` | 东/右 | 上 | 南/后 | 右手 |
| `Device` | 米 `m` | 设备右方 | 设备上方 | 朝向用户、屏幕外 | 右手 |
| `Camera` | 米 `m` | 画面右方 | 画面上方 | 相机后方（光学视线为 `-Z`） | 右手 |
| `Screen` | 像素 `px` | 像素右方 | 像素下方 | 进入屏幕，深度也用像素表示 | 右手 |

每个轴的完整定义可通过 `headpose.FrameSpecs()` 或 `frame.Spec()` 查询。`World` 的 `+Z = +X × +Y` 因此指向南；`Screen` 的 `+X × +Y` 指向屏幕内，所以“Y 向下、Z 向屏幕内”的组合仍然是右手系。

### 相似变换

`Transform` 表示正缩放的相似变换：

```text
p_target = R * (s * p_source) + t
```

- `R` 是单位四元数，`s > 0` 是目标单位/源单位，`t` 使用目标坐标系单位。
- 只允许正缩放，因此不会通过负缩放引入反射或手性翻转。
- `Rotation()` 暴露旋转；`ApplyDirection()` 只旋转方向，`ApplyPoint()` 同时应用缩放、旋转和平移。
- `Inverse()` 是精确逆变换；`Compose()` 按 A→B、B→C 的顺序组合为 A→C。

## 帧图与屏幕映射

`Scene` 注册三条双向边：

```text
Device <-> World <-> Camera <-> Screen
```

任意两个帧之间使用最短路径组合变换。例如 `Device -> Screen` 实际走：

```text
Device -> World -> Camera -> Screen
```

`Camera` 到 `Screen` 使用 180° 绕 `+X` 的右手旋转、主轴点平移和 `pixelsPerMeter` 缩放：

```text
x_screen = principalX + s * x_camera
y_screen = principalY - s * y_camera
z_screen =            -s * z_camera
```

这会把 Camera 的 `+Y up` 映射到 Screen 的 `+Y down`，把 Camera 的 `+Z backward` 映射到 Screen 的 `+Z into screen`。旋转行列式为 `+1`，缩放为正，因此手性不变。

这里的 Screen 是**可逆的仿射像素帧**，不是透视投影：屏幕 `Z` 保存像素深度，所以任意点可以无损往返。若业务需要透视成像，应在该仿射帧之外单独做投影和反投影，避免把不可逆投影混进坐标帧契约。

## 快速使用

```go
devicePose, _ := headpose.NewRigid(deviceRotation, headpose.Vec3{X: 0.1, Y: 1.6, Z: 0.1})
cameraPose, _ := headpose.NewRigid(cameraRotation, headpose.Vec3{X: 0.0, Y: 1.6, Z: 0.2})

scene, err := headpose.NewScene(devicePose, cameraPose, headpose.ScreenParams{
    PixelsPerMeter: 1200,
    PrincipalX:     640,
    PrincipalY:     360,
})
if err != nil {
    log.Fatal(err)
}

screenPoint, err := scene.ConvertPoint(headpose.Device, headpose.Screen, devicePoint)
worldPoint, err := scene.ConvertPoint(headpose.Screen, headpose.World, screenPoint)

check, err := scene.RoundTripPoint(headpose.Device, headpose.Screen, devicePoint, 1e-9)
if err != nil || !check.Pass {
    log.Fatalf("round trip failed: %+v, %v", check, err)
}
```

`RoundTripPoint` 的误差以**源帧单位**逐分量比较。`CheckAllPairs` 接收每个帧各一个样本点，执行全部 16 个有序帧对的往返检查。

常用 API：

- `FrameSpecs()`：四帧的单位、轴向、手性和四元数顺序。
- `QuatFromAxisAngle()`、`QuatFromBasis()`：显式构造规范旋转。
- `NewScene()`：按 Device→World、Camera→World、Camera→Screen 建图。
- `ConvertPoint()` / `ConvertDirection()`：任意帧间点与方向转换。
- `FrameGraph.Transform()` / `Path()`：取组合变换和实际路径。
- `RoundTripPoint()` / `CheckAllPairs()`：往返校验与误差报告。

## 预览

```bash
go run ./cmd/preview
```

默认输出 `artifacts/coordinate-preview.svg`。左侧为等距世界视图，包含锚点、World/Device/Camera 三轴箭头、17 个相机位姿及光学 `-Z` 方向；右侧为屏幕像素视口，显示同一批锚点的 World→Screen 结果和主轴点。

可用参数：

```bash
go run ./cmd/preview -out artifacts/coordinate-preview.svg -width 1180 -height 720
```

预览由标准库直接写 SVG，不使用绘图库。屏幕里的锚点坐标与转换 API 使用同一条 `World -> Camera -> Screen` 路径。

## 验证

```bash
go test ./...
go vet ./...
```

测试覆盖：

- 四个帧的显式单位、轴向、手性和 `WXYZ` 顺序。
- 90° 轴向错配：Device 的 `+X/+Y/+Z` 与 World 的对应方向逐一断言。
- 负四元数：`q` 与 `-q` 的场景转换一致，`SameRotation` 识别符号等价。
- 非单位缩放：`1200 px/m` 的正向/反向比例，以及 100 次往返累积误差。
- 嵌套转换：`Device→World→Camera→Screen` 与直接组合一致。
- 全部 16 个有序帧对的往返校验。
- 预览 SVG 的 XML 合法性、转义、锚点、箭头和轨迹元素。

命令行会打印帧契约、16 个往返结果的最大误差、一个 Device 样本到 World/Camera/Screen 的实际坐标，然后写入预览。
