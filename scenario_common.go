package main

// buildBaseOps 构造嵌套面板 + 反向模板 + 空矩形的完整渲染流。
// 末尾的同步点（SYNC_POINT）同时更换 scissor 与模板引用，用于复现 desync。
func buildBaseOps(clearOrderBug bool) []Op {
	ops := []Op{
		{Kind: OpClear, Target: ClearColorTarget, ClearC: colClear, Comment: "clear color"},
		{Kind: OpClear, Target: ClearStencilTarget, ClearV: 0, Comment: "clear stencil to 0"},

		// 外层面板：scissor 裁剪到 outer，模板标记为 1。
		{Kind: OpPushScissor, Rect: rectOuter, Comment: "push scissor outer"},
		{
			Kind: OpStencilRect, Rect: rectOuter,
			State: StencilState{Enabled: true, Func: CmpAlways, Ref: refOuter, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpReplace, WriteMask: 0xff},
			Comment: "mark outer stencil=1",
		},
		{
			Kind: OpDrawRect, Rect: rectOuter, Color: colOuter,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refOuter, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpKeep, WriteMask: 0},
			Comment: "fill outer where stencil==1",
		},

		// 内层面板：嵌套 scissor，模板递增到 2。
		{Kind: OpPushScissor, Rect: rectInner, Comment: "push scissor inner"},
		{
			Kind: OpStencilRect, Rect: rectInner,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refOuter, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpIncr, WriteMask: 0xff},
			Comment: "incr inner stencil 1->2",
		},
		{
			Kind: OpDrawRect, Rect: rectInner, Color: colInner,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refInner, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpKeep, WriteMask: 0},
			Comment: "fill inner where stencil==2",
		},

		// 装饰面板：反向模板操作，把模板值递减回 1。
		{
			Kind: OpStencilRect, Rect: rectDecor,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refInner, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpDecr, WriteMask: 0xff},
			Comment: "reverse op DECR 2->1",
		},
		{
			Kind: OpDrawRect, Rect: rectDecor, Color: colDecor,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refOuter, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpKeep, WriteMask: 0},
			Comment: "fill decor where stencil==1",
		},

		// 空矩形 scissor：正确实现下后续同步点被完全裁掉。
		{Kind: OpPushScissor, Rect: rectEmpty, Comment: "push EMPTY scissor"},

		// 同步点：scissor（空->弹出非空）与模板引用（2->7）同一指令区间变化。
		{Kind: OpPopScissor, Comment: "pop empty scissor (SYNC_POINT)"},
		{
			Kind: OpStencilRect, Rect: rectDecor,
			State: StencilState{Enabled: true, Func: CmpAlways, Ref: refBad, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpReplace, WriteMask: 0xff},
			Comment: "SYNC_POINT",
		},
		{
			Kind: OpDrawRect, Rect: rectDecor, Color: colFooter,
			State: StencilState{Enabled: true, Func: CmpEqual, Ref: refBad, Mask: 0xff,
				FailOp: OpKeep, PassOp: OpKeep, WriteMask: 0},
			Comment: "fill decor where stencil==7",
		},

		{Kind: OpPopScissor, Comment: "pop inner scissor"},
		{Kind: OpPopScissor, Comment: "pop outer scissor"},

		// 页脚：无模板，只受 scissor 控制。
		{Kind: OpPushScissor, Rect: rectFooter, Comment: "push scissor footer"},
		{
			Kind: OpDrawRect, Rect: rectFooter, Color: colFooter,
			State:   StencilState{Enabled: false},
			Comment: "fill footer",
		},
	}

	if clearOrderBug {
		// 缺陷场景：清空指令发生在仍有残留 scissor 的时刻，且漏配对应 pop。
		ops = append([]Op{
			{Kind: OpPushScissor, Rect: rectInner, Comment: "leftover scissor before clear"},
			{Kind: OpClear, Target: ClearColorTarget, ClearC: colBadClear, Comment: "clear under leftover scissor"},
			{Kind: OpClear, Target: ClearStencilTarget, ClearV: 0, Comment: "clear stencil under leftover scissor"},
		}, ops...)
	}
	return ops
}
