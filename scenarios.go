package main

type Scenario struct {
	Name string
	Bugs Bugs
	Frame
}

// BuildScenarios 构造五个复现场景，逻辑 UI 完全相同，仅环境与缺陷注入不同。
func BuildScenarios() []Scenario {
	const logW, logH = 12, 10

	mk := func(name string, bugs Bugs, ctx FrameCtx) Scenario {
		return Scenario{Name: name, Bugs: bugs, Frame: Frame{Ctx: ctx, Ops: buildBaseOps(bugs.ClearOrder)}}
	}

	return []Scenario{
		mk("S1_window_move", Bugs{WindowMove: true}, FrameCtx{
			Name: "window moved +3,+2", LogW: logW, LogH: logH, DPI: 1,
			WinX: 27, WinY: 23, PrevWinX: 24, PrevWinY: 21, PrevDPI: 1,
		}),
		mk("S2_dpi_change", Bugs{DPIChange: true}, FrameCtx{
			Name: "dpi 1->2", LogW: logW, LogH: logH, DPI: 2,
			WinX: 10, WinY: 10, PrevWinX: 10, PrevWinY: 10, PrevDPI: 1,
		}),
		mk("S3_empty_rect", Bugs{EmptyRect: true}, FrameCtx{
			Name: "empty scissor ignored", LogW: logW, LogH: logH, DPI: 1,
			WinX: 0, WinY: 0, PrevWinX: 0, PrevWinY: 0, PrevDPI: 1,
		}),
		mk("S4_clear_order", Bugs{ClearOrder: true}, FrameCtx{
			Name: "clear under leftover scissor", LogW: logW, LogH: logH, DPI: 1,
			WinX: 0, WinY: 0, PrevWinX: 0, PrevWinY: 0, PrevDPI: 1,
		}),
		mk("S5_stencil_scissor_desync", Bugs{StencilSync: true}, FrameCtx{
			Name: "scissor+ref change same frame", LogW: logW, LogH: logH, DPI: 1,
			WinX: 0, WinY: 0, PrevWinX: 0, PrevWinY: 0, PrevDPI: 1,
		}),
	}
}

// ReferenceCtx 返回与各场景相同逻辑尺寸的参考帧环境。
func ReferenceCtx(dpi int) FrameCtx {
	return FrameCtx{Name: "reference", LogW: 12, LogH: 10, DPI: dpi, PrevDPI: dpi}
}
