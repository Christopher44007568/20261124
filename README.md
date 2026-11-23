# lensdistortion

纯 Go 标准库实现的径向镜头畸变映射与显示覆盖检查，不依赖图形 SDK。

## 方向约定

校正渲染必须按**逆变换**查源：

```go
source, ok := mapper.SourceFromDisplay(displayPoint)
```

`SourceFromDisplay` 把无畸变的显示坐标映射到原始图像中应采样的坐标；
`DisplayFromSource` 是其反方向，适合验证和投影。不要把这两个方向互换，
否则桶形畸变会在边缘表现成反向弯曲。

径向模型为:

```text
u   = |p - Center| / ReferenceRadius
rho = u * (1 + K1*u^2 + K2*u^4 + ...)
```

其中 `rho` 是有畸变源半径。`K1 < 0` 表示桶形畸变，`K1 > 0` 表示枕形畸变。
`MaxRadius` 是允许的最大归一化显示半径；矩形图像四角需要至少
`sqrt(2)`。

## 三种映射

```go
coefficients := []float64{-0.08, 0.012}
calibration := lensdistortion.Lens{
    Center:          lensdistortion.Point{X: 480, Y: 540},
    ReferenceRadius: 540,
    Coefficients:    coefficients,
    MaxRadius:       1.5,
}

analytic, _ := lensdistortion.NewAnalyticMap(calibration)
polynomial, _ := lensdistortion.NewPolynomialMap(calibration, coefficients)
lut, _ := lensdistortion.NewUniformLUTFromLens(calibration, 1.5, 128)
```

- `AnalyticMap` 直接计算模型；反向采用区间约束的 Newton/二分求解。
- `PolynomialMap` 直接计算显示半径到源半径的多项式，不依赖
  `Lens.Coefficients`。
- `PiecewiseLUT` 对归一化半径做分段线性插值，最后一个节点之外不做外插。

三种映射都实现 `Mapper` 接口，中心点固定，边缘和四角经过同一套域检查。
超出 `MaxRadius`、LUT 节点或非单调模型时返回 `false` 或显式错误。

## 双眼

每个眼独立持有一个 `Mapper`，因此可以分别使用不同的畸变中心：

```go
rig, _ := lensdistortion.NewStereoRig(leftMapper, rightMapper)
source, ok := rig.SourceFromDisplay(lensdistortion.LeftEye, p)
```

也可通过 `NewStereoRigFromCalibrations` 为左右眼选择解析、多项式或 LUT
映射。仓库不会从另一只眼复制中心。

## 非矩形显示区

`PolygonRegion` 支持任意简单多边形掩膜。`AnalyzeCoverage` 会逐像素区分：

- 掩膜外，不采样；
- 正常映射；
- 模型域外；
- 源边界外；
- 双线性采样足迹越界形成的边界空洞；
- `SampleClamp` 下经边界复制的像素。

```go
report, _ := lensdistortion.AnalyzeCoverage(rig.Mapper(lensdistortion.LeftEye),
    lensdistortion.CoverageOptions{
        Width: 1920, Height: 1080,
        SourceWidth: 1920, SourceHeight: 1080,
        Region: polygon,
        Sampling: lensdistortion.SampleBilinear,
    })
if err := report.Validate(); err != nil {
    // report.HolePixels / SourceOutPixels / ModelOutPixels 可用于定位。
}
```

`SampleClamp` 不会读取源图外部，但仍将发生夹取的像素计入
`SourceOutPixels`，避免把真实越界静默隐藏。

## 坐标约定

像素中心使用整数坐标。宽为 `W` 的图像有效中心是 `0..W-1`；四边形掩膜
顶点的坐标也必须与该坐标系一致。所有半径为归一化系数域，
`ReferenceRadius` 负责像素距离到归一化半径的换算。

运行验证：

```sh
go test -race ./...
go vet ./...
```
