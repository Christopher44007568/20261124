package main

import "testing"

func TestRectEmptyAndIntersect(t *testing.T) {
	if (Rect{X: 1, Y: 1, W: 0, H: 3}).Contains(1, 1) {
		t.Fatal("空矩形不应包含任何像素")
	}
	inter := Intersect(Rect{X: 0, Y: 0, W: 5, H: 5}, Rect{X: 3, Y: 3, W: 4, H: 4})
	if inter != (Rect{X: 3, Y: 3, W: 2, H: 2}) {
		t.Fatalf("交集错误: %+v", inter)
	}
	if got := Intersect(Rect{X: 0, Y: 0, W: 1, H: 1}, Rect{X: 5, Y: 5, W: 1, H: 1}); !got.Empty() {
		t.Fatalf("不相交矩形交集应为空: %+v", got)
	}
}

func TestDPIScale(t *testing.T) {
	r := Rect{X: 3, Y: 2, W: 5, H: 4}
	got := r.Scale(2)
	if got != (Rect{X: 6, Y: 4, W: 10, H: 8}) {
		t.Fatalf("DPI 缩放错误: %+v", got)
	}
}

func TestStencilCompareAndReverseOp(t *testing.T) {
	mark := StencilState{Enabled: true, Func: CmpAlways, Ref: 2, Mask: 0xff,
		FailOp: OpKeep, PassOp: OpReplace, WriteMask: 0xff}
	if !mark.Compare(0) || mark.Apply(0, true) != 2 {
		t.Fatal("REPLACE 语义错误")
	}
	decr := StencilState{Enabled: true, Func: CmpEqual, Ref: 2, Mask: 0xff,
		FailOp: OpKeep, PassOp: OpDecr, WriteMask: 0xff}
	if !decr.Compare(2) || decr.Apply(2, true) != 1 {
		t.Fatal("反向 DECR 语义错误")
	}
	if decr.Apply(0, true) != 0 {
		t.Fatal("DECR 应钳制在 0")
	}
	incr := StencilState{Enabled: true, PassOp: OpIncr, WriteMask: 0xff}
	if incr.Apply(255, true) != 255 {
		t.Fatal("INCR 应钳制在 255")
	}
}

// TestReferenceSane 验证参考实现本身无缺陷：与自身对比应为零差异。
func TestReferenceSane(t *testing.T) {
	for _, sc := range BuildScenarios() {
		refFB, refTr := Render(Frame{Ctx: ReferenceCtx(sc.Ctx.DPI), Ops: sc.Ops}, Bugs{})
		againFB, againTr := Render(Frame{Ctx: ReferenceCtx(sc.Ctx.DPI), Ops: sc.Ops}, Bugs{})
		if m := Compare(refFB, againFB, refTr, againTr); m != nil {
			t.Fatalf("场景 %s 参考实现不确定", sc.Name)
		}
	}
}

// TestEveryBugFires 验证每个注入缺陷都能稳定产生错误像素，且首个错误可追踪。
func TestEveryBugFires(t *testing.T) {
	for _, sc := range BuildScenarios() {
		refFB, refTr := Render(Frame{Ctx: ReferenceCtx(sc.Ctx.DPI), Ops: sc.Ops}, Bugs{})
		bugFB, bugTr := Render(sc.Frame, sc.Bugs)
		m := Compare(refFB, bugFB, refTr, bugTr)
		if m == nil {
			t.Fatalf("场景 %s 未复现错误", sc.Name)
		}
		if len(m.BugTrace.Recs) != len(m.RefTrace.Recs) {
			t.Fatalf("场景 %s 状态序列长度不一致", sc.Name)
		}
		if CountMismatches(refFB, bugFB) == 0 {
			t.Fatalf("场景 %s 错误计数为 0", sc.Name)
		}
	}
}
