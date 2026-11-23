package main

// StepRecord 记录单个像素在单条渲染指令上的完整判定与状态变化。
type StepRecord struct {
	Step           int
	OpKind         OpType
	Comment        string
	ClipLogical    Rect
	ClipPhysical   Rect
	GeomPhysical   Rect
	ScissorPass    bool
	StencilEnabled bool
	StencilFunc    CmpFunc
	StencilRef     uint8
	StencilPass    bool
	StencilBefore  uint8
	StencilAfter   uint8
	ColorBefore    RGBA
	ColorAfter     RGBA
	WroteColor     bool
}

// PixelTrace 汇聚一个像素在整帧内的状态序列。
type PixelTrace struct {
	X    int
	Y    int
	Recs []StepRecord
}

// TraceGrid 按像素索引保存全帧追踪。
type TraceGrid struct {
	W  int
	H  int
	Px []*PixelTrace
}

func NewTraceGrid(w, h int) *TraceGrid {
	g := &TraceGrid{W: w, H: h, Px: make([]*PixelTrace, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			g.Px[y*w+x] = &PixelTrace{X: x, Y: y}
		}
	}
	return g
}

func (g *TraceGrid) At(x, y int) *PixelTrace { return g.Px[y*g.W+x] }
