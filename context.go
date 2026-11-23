package main

// FrameCtx 描述一帧渲染时的窗口环境。
type FrameCtx struct {
	Name     string
	LogW     int // 逻辑宽度
	LogH     int // 逻辑高度
	DPI      int // 物理像素 = 逻辑坐标 * DPI
	WinX     int // 窗口左上角屏幕坐标（逻辑单位）
	WinY     int
	PrevWinX int // 上一帧窗口位置，供缺陷建模使用
	PrevWinY int
	PrevDPI  int
}

func (f FrameCtx) PhysW() int { return f.LogW * f.DPI }
func (f FrameCtx) PhysH() int { return f.LogH * f.DPI }

func (f FrameCtx) FullLogical() Rect { return Rect{X: 0, Y: 0, W: f.LogW, H: f.LogH} }

// Frame 为一帧测试用例：环境 + 指令流。
type Frame struct {
	Ctx FrameCtx
	Ops []Op
}
