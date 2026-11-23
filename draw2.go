package main

// draw 处理 STENCIL_RECT 与 DRAW_RECT：逐像素做 scissor 判定、几何判定与模板测试。
func (e *engine) draw(step int, op Op) {
	ctx := e.frame.Ctx
	logClip := e.top()
	physClip := e.toPhysical(logClip)
	geom := e.toPhysical(op.Rect)

	// 缺陷 StencilSync：本指令同时更换 scissor 与模板引用时，引用沿用缓存旧值。
	effState := op.State
	if e.bugs.StencilSync && op.State.Enabled {
		if op.Comment == "SYNC_POINT" {
			effState.Ref = e.lastRef
		}
		e.lastRef = op.State.Ref
	}

	for y := 0; y < ctx.PhysH(); y++ {
		for x := 0; x < ctx.PhysW(); x++ {
			cBefore, sBefore := e.fb.Pixel(x, y)
			scissorPass := physClip.Contains(x, y)
			geomPass := geom.Contains(x, y)
			stencilPass := true
			sAfter := sBefore
			cAfter := cBefore
			wroteColor := false

			if scissorPass && geomPass {
				stencilPass = effState.Compare(sBefore)
				sAfter = effState.Apply(sBefore, stencilPass)
				e.fb.SetStencil(x, y, sAfter)
				if stencilPass && op.Kind == OpDrawRect {
					cAfter = op.Color
					e.fb.SetColor(x, y, cAfter)
					wroteColor = true
				}
			}

			e.tr.At(x, y).Recs = append(e.tr.At(x, y).Recs, StepRecord{
				Step:           step,
				OpKind:         op.Kind,
				Comment:        op.Comment,
				ClipLogical:    logClip,
				ClipPhysical:   physClip,
				GeomPhysical:   geom,
				ScissorPass:    scissorPass,
				StencilEnabled: effState.Enabled,
				StencilFunc:    effState.Func,
				StencilRef:     effState.Ref,
				StencilPass:    stencilPass,
				StencilBefore:  sBefore,
				StencilAfter:   sAfter,
				ColorBefore:    cBefore,
				ColorAfter:     cAfter,
				WroteColor:     wroteColor,
			})
		}
	}
}
