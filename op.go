package main

// 模板比较函数，对应 D3D11/OpenGL/Vulkan 的常用比较语义。
type CmpFunc int

const (
	CmpAlways CmpFunc = iota
	CmpNever
	CmpEqual
	CmpNotEqual
	CmpLess
	CmpLessEqual
	CmpGreater
	CmpGreaterEqual
)

func (c CmpFunc) String() string {
	switch c {
	case CmpAlways:
		return "ALWAYS"
	case CmpNever:
		return "NEVER"
	case CmpEqual:
		return "EQUAL"
	case CmpNotEqual:
		return "NOT_EQUAL"
	case CmpLess:
		return "LESS"
	case CmpLessEqual:
		return "LESS_EQUAL"
	case CmpGreater:
		return "GREATER"
	case CmpGreaterEqual:
		return "GREATER_EQUAL"
	}
	return "?"
}

// 模板操作：保持、替换、递增（钳制）、递减（钳制）、清零。
type StencilOp int

const (
	OpKeep StencilOp = iota
	OpZero
	OpReplace
	OpIncr
	OpDecr
)

func (s StencilOp) String() string {
	switch s {
	case OpKeep:
		return "KEEP"
	case OpZero:
		return "ZERO"
	case OpReplace:
		return "REPLACE"
	case OpIncr:
		return "INCR_CLAMP"
	case OpDecr:
		return "DECR_CLAMP"
	}
	return "?"
}

// StencilState 描述一次绘制时的模板测试与写入规则。
type StencilState struct {
	Enabled   bool
	Func      CmpFunc
	Ref       uint8
	Mask      uint8
	FailOp    StencilOp // 模板测试失败
	DepthOp   StencilOp // 本模拟器无深度，该字段保留并视同 PassOp
	PassOp    StencilOp // 模板测试通过
	WriteMask uint8
}

// Compare 执行模板比较（Mask 同时作用于引用与缓冲值）。
func (s StencilState) Compare(stored uint8) bool {
	if !s.Enabled {
		return true
	}
	a := s.Ref & s.Mask
	b := stored & s.Mask
	switch s.Func {
	case CmpAlways:
		return true
	case CmpNever:
		return false
	case CmpEqual:
		return a == b
	case CmpNotEqual:
		return a != b
	case CmpLess:
		return a < b
	case CmpLessEqual:
		return a <= b
	case CmpGreater:
		return a > b
	case CmpGreaterEqual:
		return a >= b
	}
	return false
}

// Apply 按测试结果计算新模板值，写入时受 WriteMask 限制。
func (s StencilState) Apply(stored uint8, passed bool) uint8 {
	if !s.Enabled {
		return stored
	}
	op := s.PassOp
	if !passed {
		op = s.FailOp
	}
	var next uint8
	switch op {
	case OpKeep:
		next = stored
	case OpZero:
		next = 0
	case OpReplace:
		next = s.Ref
	case OpIncr:
		if stored < 255 {
			next = stored + 1
		} else {
			next = 255
		}
	case OpDecr:
		if stored > 0 {
			next = stored - 1
		} else {
			next = 0
		}
	}
	return (stored &^ s.WriteMask) | (next & s.WriteMask)
}

// OpType 标识渲染指令种类。
type OpType int

const (
	OpClear OpType = iota
	OpPushScissor
	OpPopScissor
	OpStencilRect
	OpDrawRect
)

func (t OpType) String() string {
	switch t {
	case OpClear:
		return "CLEAR"
	case OpPushScissor:
		return "PUSH_SCISSOR"
	case OpPopScissor:
		return "POP_SCISSOR"
	case OpStencilRect:
		return "STENCIL_RECT"
	case OpDrawRect:
		return "DRAW_RECT"
	}
	return "?"
}

// ClearTarget 区分颜色清空与模板清空，用于复现清空顺序问题。
type ClearTarget int

const (
	ClearColorTarget ClearTarget = iota
	ClearStencilTarget
)

func (t ClearTarget) String() string {
	if t == ClearColorTarget {
		return "COLOR"
	}
	return "STENCIL"
}

// Op 为渲染流中的一条逻辑指令。Rect 一律为逻辑坐标，渲染时按 DPI 缩放。
type Op struct {
	Kind    OpType
	Rect    Rect
	Target  ClearTarget
	ClearV  uint8
	ClearC  RGBA
	State   StencilState
	Color   RGBA
	Comment string
}
