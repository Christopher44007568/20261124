package stereo

// FrameWidth/FrameHeight 是演示与测试共用的每眼帧缓冲尺寸。
const (
	FrameWidth  = 40
	FrameHeight = 24
)

// 资源名称常量。
const (
	GeomSky        = "geom.sky"
	GeomPanelL     = "geom.panel-left"
	GeomPanelR     = "geom.panel-right"
	GeomBadgeL     = "geom.badge-left"
	GeomBadgeR     = "geom.badge-right"
	GeomHighlight  = "geom.highlight"
	GeomHighlightR = "geom.highlight-right"
	GeomMarker     = "geom.marker"

	TexBrick = "tex.brick"
	TexGlow  = "tex.glow"
)

// NewDemoRegistry 构造演示场景的全部几何与纹理输入资源。
func NewDemoRegistry() *Registry {
	r := NewRegistry()
	must(r.AddGeometry(Geometry{Name: GeomSky, Bounds: Rect{MaxX: 40, MaxY: 24}}))
	must(r.AddGeometry(Geometry{Name: GeomPanelL, Bounds: Rect{MinX: 2, MinY: 4, MaxX: 19, MaxY: 20}}))
	must(r.AddGeometry(Geometry{Name: GeomPanelR, Bounds: Rect{MinX: 21, MinY: 4, MaxX: 38, MaxY: 20}}))
	must(r.AddGeometry(Geometry{Name: GeomBadgeL, Bounds: Rect{MinX: 4, MinY: 6, MaxX: 12, MaxY: 11}}))
	must(r.AddGeometry(Geometry{Name: GeomBadgeR, Bounds: Rect{MinX: 23, MinY: 6, MaxX: 31, MaxY: 11}}))
	must(r.AddGeometry(Geometry{Name: GeomHighlight, Bounds: Rect{MinX: 6, MinY: 8, MaxX: 14, MaxY: 13}}))
	must(r.AddGeometry(Geometry{Name: GeomHighlightR, Bounds: Rect{MinX: 25, MinY: 8, MaxX: 33, MaxY: 13}}))
	must(r.AddGeometry(Geometry{Name: GeomMarker, Bounds: Rect{MinX: 30, MinY: 18, MaxX: 37, MaxY: 23}}))

	must(r.AddSwatch(Swatch{Name: TexBrick, Color: [4]uint8{220, 180, 140, 255}}))
	must(r.AddSwatch(Swatch{Name: TexGlow, Color: [4]uint8{180, 220, 255, 255}}))
	return r
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// 材质与状态工厂：集中定义保证场景可读、故障可对比。
func skyMaterial() Material {
	return Material{Name: "mat.sky", Color: [4]uint8{40, 60, 96, 255}, Blend: BlendOverwrite}
}

func panelMaterial() Material {
	return Material{Name: "mat.panel", TextureName: TexBrick, Color: [4]uint8{200, 200, 200, 255}, Blend: BlendOverwrite}
}

func badgeMaterial() Material {
	return Material{Name: "mat.badge", Color: [4]uint8{210, 80, 80, 255}, Blend: BlendAlphaOver}
}

func highlightMaterial() Material {
	return Material{Name: "mat.highlight", TextureName: TexGlow, Color: [4]uint8{255, 255, 255, 200}, Blend: BlendAlphaOver}
}

func markerMaterial() Material {
	return Material{Name: "mat.marker", Color: [4]uint8{120, 255, 170, 255}, Blend: BlendOverwrite}
}

// PollutedMaterial 是污染复现场景中被单眼批次错误写入共享状态的材质。
func PollutedMaterial() Material {
	return Material{Name: "mat.polluted", Color: [4]uint8{255, 0, 255, 255}, Blend: BlendOverwrite}
}

// DemoBatches 返回演示场景的固定批次序列：
// 共享几何 2 个、双眼批次 3 个、单眼批次 4 个、污染复现批次 1 个。
func DemoBatches() []Batch {
	scissorPanel := Scissor{Enabled: true, MinX: 1, MinY: 3, MaxX: 39, MaxY: 21}
	stencilMask := StencilState{
		Enabled: true, Ref: 1, ReadMask: 0xff, Compare: CmpAlways,
		FailOp: StencilKeep, PassOp: StencilReplace, WriteMask: 0xff,
	}
	stencilTest := StencilState{
		Enabled: true, Ref: 1, ReadMask: 0xff, Compare: CmpEqual,
		FailOp: StencilKeep, PassOp: StencilReplace, WriteMask: 0xff,
	}
	stencilOff := StencilState{}

	return []Batch{
		{
			ID: "b01-sky", Kind: SharedBatch,
			State:        RenderState{Material: skyMaterial(), Stencil: stencilOff},
			GeometryName: GeomSky,
		},
		{
			ID: "b02-panels", Kind: StereoBatch,
			State: RenderState{
				Material: panelMaterial(), Stencil: stencilOff, Scissor: scissorPanel,
			},
			GeometryName: GeomPanelL,
		},
		{
			ID: "b03-badge-mask-l", Kind: MonoBatch, Eye: MaskLeft,
			State: RenderState{
				Material: badgeMaterial(), Stencil: stencilMask, Scissor: scissorPanel,
			},
			GeometryName: GeomBadgeL,
		},
		{
			ID: "b04-badge-mask-r", Kind: MonoBatch, Eye: MaskRight,
			State: RenderState{
				Material: badgeMaterial(), Stencil: stencilMask, Scissor: scissorPanel,
			},
			GeometryName: GeomBadgeR,
		},
		{
			ID: "b05-highlight-l", Kind: MonoBatch, Eye: MaskLeft,
			State: RenderState{
				Material: highlightMaterial(), Stencil: stencilTest, Scissor: scissorPanel,
			},
			GeometryName: GeomHighlight,
		},
		{
			ID: "b06-highlight-r", Kind: MonoBatch, Eye: MaskRight,
			State: RenderState{
				Material: highlightMaterial(), Stencil: stencilTest, Scissor: scissorPanel,
			},
			GeometryName: GeomHighlightR,
		},
		{
			ID: "b07-both-frame", Kind: StereoBatch,
			State: RenderState{
				Material: markerMaterial(), Stencil: stencilOff,
				Scissor: Scissor{Enabled: true, MinX: 0, MinY: 0, MaxX: 40, MaxY: 24},
			},
			GeometryName: GeomMarker,
		},
		{
			ID: "b08-shared-footer", Kind: SharedBatch,
			State: RenderState{
				Material: markerMaterial(), Stencil: stencilOff,
				Scissor: Scissor{Enabled: true, MinX: 0, MinY: 22, MaxX: 40, MaxY: 24},
			},
			GeometryName: GeomSky,
		},
		{
			// 在右眼无模板地再绘制一次：当 b06 的模板配置被错误注入时，
			// 基线会在此批次画出高光，故障管线则因错误模板状态被拒绝，
			// 从而留下可定位的首个像素差异。
			ID: "b09-mono-glow-r", Kind: MonoBatch, Eye: MaskRight,
			State: RenderState{
				Material: highlightMaterial(), Stencil: stencilOff, Scissor: scissorPanel,
			},
			GeometryName: GeomHighlightR,
		},
		{
			// 故意沿用当前共享状态：前面的单眼批次若未做状态隔离，
			// 这个右眼批次会继承被污染的材质并把污染显式画到另一只眼。
			ID: "b10-inherit-marker-r", Kind: MonoBatch, Eye: MaskRight,
			Inherit:      true,
			GeometryName: GeomMarker,
		},
	}
}
