package main

// recordStackStep 为 scissor 入栈/出栈指令记录每个像素的裁剪判定。
func (e *engine) recordStackStep(step int, op Op) {
	ctx := e.frame.Ctx
	logClip := e.top()
	physClip := e.toPhysical(logClip)
	for y := 0; y < ctx.PhysH(); y++ {
		for x := 0; x < ctx.PhysW(); x++ {
			c, s := e.fb.Pixel(x, y)
			e.tr.At(x, y).Recs = append(e.tr.At(x, y).Recs, StepRecord{
				Step:          step,
				OpKind:        op.Kind,
				Comment:       op.Comment,
				ClipLogical:   logClip,
				ClipPhysical:  physClip,
				ScissorPass:   physClip.Contains(x, y),
				StencilBefore: s,
				StencilAfter:  s,
				ColorBefore:   c,
				ColorAfter:    c,
			})
		}
	}
}

// clear 执行颜色或模板清空。参考实现不受 scissor 限制；ClearOrder 缺陷下被残留裁剪限制。
func (e *engine) clear(step int, op Op) {
	ctx := e.frame.Ctx
	fullLog := ctx.FullLogical()
	logClip := fullLog
	if e.bugs.ClearOrder {
		logClip = e.top()
	}
	physClip := e.toPhysical(logClip)
	clearWhole := !e.bugs.ClearOrder

	for y := 0; y < ctx.PhysH(); y++ {
		for x := 0; x < ctx.PhysW(); x++ {
			cBefore, sBefore := e.fb.Pixel(x, y)
			pass := clearWhole || physClip.Contains(x, y)
			cAfter, sAfter := cBefore, sBefore
			if pass {
				if op.Target == ClearColorTarget {
					cAfter = op.ClearC
					e.fb.SetColor(x, y, cAfter)
				} else {
					sAfter = op.ClearV
					e.fb.SetStencil(x, y, sAfter)
				}
			}
			e.tr.At(x, y).Recs = append(e.tr.At(x, y).Recs, StepRecord{
				Step:          step,
				OpKind:        op.Kind,
				Comment:       op.Comment,
				ClipLogical:   logClip,
				ClipPhysical:  physClip,
				ScissorPass:   pass,
				StencilBefore: sBefore,
				StencilAfter:  sAfter,
				ColorBefore:   cBefore,
				ColorAfter:    cAfter,
				WroteColor:    pass && op.Target == ClearColorTarget,
			})
		}
	}
}
