package main

// Bugs 汇总可注入的缺陷，零值即参考（正确）实现。
type Bugs struct {
	WindowMove  bool // 窗口移动后 scissor/几何仍套用旧窗口偏移
	DPIChange   bool // DPI 改变后仍按旧比例把逻辑矩形映射为物理矩形
	EmptyRect   bool // 空 scissor 矩形被错误忽略（未真正推入裁剪栈）
	ClearOrder  bool // 清空前残留的 scissor 未解除，清空被局部裁剪
	StencilSync bool // scissor 与模板引用同帧变化时，模板引用沿用缓存旧值
}

func (b Bugs) Label() string {
	switch {
	case b.WindowMove:
		return "BUG_WINDOW_MOVE"
	case b.DPIChange:
		return "BUG_DPI_CHANGE"
	case b.EmptyRect:
		return "BUG_EMPTY_RECT"
	case b.ClearOrder:
		return "BUG_CLEAR_ORDER"
	case b.StencilSync:
		return "BUG_STENCIL_SCISSOR_DESYNC"
	default:
		return "REFERENCE"
	}
}

type engine struct {
	bugs  Bugs
	frame Frame
	fb    *Framebuffer
	tr    *TraceGrid
	stack []Rect
	// 上一条带模板状态指令的引用值，用于缺陷 StencilSync。
	lastRef uint8
}

// Render 执行整帧指令流并返回帧缓冲与逐像素追踪。
func Render(f Frame, bugs Bugs) (*Framebuffer, *TraceGrid) {
	e := &engine{
		bugs:  bugs,
		frame: f,
		fb:    NewFramebuffer(f.Ctx.PhysW(), f.Ctx.PhysH()),
		tr:    NewTraceGrid(f.Ctx.PhysW(), f.Ctx.PhysH()),
		stack: []Rect{f.Ctx.FullLogical()},
	}
	for step, op := range f.Ops {
		e.dispatch(step, op)
	}
	return e.fb, e.tr
}

func (e *engine) top() Rect { return e.stack[len(e.stack)-1] }

// toPhysical 把逻辑矩形映射为物理矩形，缺陷在此注入。
func (e *engine) toPhysical(r Rect) Rect {
	ctx := e.frame.Ctx
	switch {
	case e.bugs.WindowMove:
		p := r.Scale(ctx.DPI)
		p.X += (ctx.WinX - ctx.PrevWinX) * ctx.DPI
		p.Y += (ctx.WinY - ctx.PrevWinY) * ctx.DPI
		return p
	case e.bugs.DPIChange:
		return r.Scale(ctx.PrevDPI)
	default:
		return r.Scale(ctx.DPI)
	}
}

func (e *engine) dispatch(step int, op Op) {
	switch op.Kind {
	case OpPushScissor:
		e.pushScissor(step, op)
	case OpPopScissor:
		e.popScissor(step, op)
	case OpClear:
		e.clear(step, op)
	default:
		e.draw(step, op)
	}
}

func (e *engine) pushScissor(step int, op Op) {
	// 正确语义：空矩形同样入栈，栈顶变为不可通过任何像素的空裁剪。
	if e.bugs.EmptyRect && op.Rect.Empty() {
		// 缺陷：直接忽略空矩形，裁剪区域维持上一层。
	} else {
		e.stack = append(e.stack, Intersect(e.top(), op.Rect))
	}
	e.recordStackStep(step, op)
}

func (e *engine) popScissor(step int, op Op) {
	if e.bugs.ClearOrder {
		// 缺陷：与残留裁剪配对的 pop 被遗漏，局部 scissor 泄漏到后续绘制。
		if op.Comment == "pop outer scissor" {
			e.recordStackStep(step, op)
			return
		}
	}
	if len(e.stack) > 1 {
		e.stack = e.stack[:len(e.stack)-1]
	}
	e.recordStackStep(step, op)
}
