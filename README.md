# RGB 径向色差校准

白线边缘出现红/绿/蓝彩边时，场景本身可能仍然是灰阶的。根因通常是 RGB
三个通道使用了不同的径向缩放，而不是场景里有彩色内容。本仓库用纯 Go
实现逐通道径向重采样、逐通道掩码和可复现的校准图，不依赖 OpenGL、
Vulkan 或其他图形 SDK。

## 模型

输出像素先转换为归一化视场坐标：

```text
p = (2x/(W-1)-1, 2y/(H-1)-1)
```

对通道 `c`：

```text
center = selected-eye calibration center
r      = |p - center| / sqrt(2)
scale  = 1 + (base[c] + eyeBias[eye][c]) * r^2
source = center + (p - center) * scale
```

因此 `r=0` 时缩放恒为 1，中心像素没有色差；无光心偏移时四角 `r=1`，
径向位移最大。正系数表示向更外侧采样，画面内容会向中心收；负系数相反。

## 单眼校准

`chroma.Calibration` 分别保存左眼和右眼的 `CenterOffset` 与 `RadialBias`。
渲染时由 `Config.Eye` 选择一只眼，基础通道系数与该眼的偏置相加。另一只眼
的偏置不会泄漏进来，也不会被三通道共享。

## 掩码

`chroma.Warp` 在每个通道自己的 `source` 坐标采样颜色和覆盖率，并在
`Result.Masks` 中返回三张独立灰度掩码。红色通道对齐的掩码对绿色、蓝色
通常已经错位，因此实现不会把一张掩码复用到三个通道。

## 越界采样

`Config.Edge` 支持：

- `EdgeBlack`：界外按不透明黑处理，适合观察扩张通道的黑边；
- `EdgeClamp`：钳制到最近边缘像素；
- `EdgeTransparent`：界外按透明黑处理。

三个通道和掩码使用同一越界策略，但各自使用自己的采样坐标。

## 校准图

`chart.RGBTestChart` 生成确定性的黑底校准图，包含：

- 白网格、白圆环和中心十字，用于检查白线两侧的合成彩边；
- 红、绿、蓝文字与黄/青 `CA` 文字，用于检查彩色内容不会被错误串色；
- 四角黑底白色高光渐变，用于放大最大径向色差并检查越界采样。

## 运行

```bash
go test ./...
go run ./cmd/chromacal -out out \
  -red 0.020 -green 0 -blue -0.020 \
  -eye left -bias-red 0.005 -edge black
```

输出包括 `source.png`、`warped.png` 以及 `channel-red.png`、
`channel-green.png`、`channel-blue.png`。主要参数：

- `-red/-green/-blue`：三通道基础径向系数；
- `-eye left|right`：选择单眼校准；
- `-bias-red/-bias-green/-bias-blue`：所选眼睛的通道偏置；
- `-center-x/-center-y`：所选眼睛的归一化光心偏移；
- `-edge black|clamp|transparent`：越界策略。

## 验证覆盖

- 连续坐标和奇数尺寸像素网格的中心零色差；
- 四角最大径向缩放，以及 R/G/B 独立位移；
- 白线通道质心顺序与合成彩边；
- 黑白高光在色差开启时出现彩边，关闭时保持灰阶；
- 左/右眼校准偏置隔离；
- 三张掩码分别按对应通道坐标采样，禁止复用；
- 黑色、钳制、透明三种越界采样；
- 非零图像原点和掩码尺寸不匹配错误。
