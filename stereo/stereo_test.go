package stereo

import "testing"

func testScene() *Scene { return BuildScene(96, 72) }

func TestHealthyPathsBitIdentical(t *testing.T) {
	sc := testScene()
	ref := RenderReference(sc)
	mv := RenderMultiview(sc, FaultNone, [2]Mat4{})
	tp := RenderTwoPass(sc, FaultNone, [2]Mat4{})
	for e := Left; e <= Right; e++ {
		if d := CompareFrames(ref.Frames[e], mv.Frames[e], e); !d.BitIdentical {
			t.Fatalf("multiview %v 与参考不一致: %+v", e, d)
		}
		if d := CompareFrames(mv.Frames[e], tp.Frames[e], e); !d.BitIdentical {
			t.Fatalf("两条路径 %v 输出不一致: %+v", e, d)
		}
	}
}

func TestTransparentOrderFlippedAcrossEyes(t *testing.T) {
	sc := testScene()
	ref := RenderReference(sc)
	l, r := ref.Frames[Left].TransOrder, ref.Frames[Right].TransOrder
	if len(l) < 2 || len(r) < 2 {
		t.Fatalf("透明片数量不足: L=%v R=%v", l, r)
	}
	if equalStrings(l, r) {
		t.Fatalf("双眼透明顺序未发生翻转: L=%v R=%v", l, r)
	}
}

func TestCrossEyeClippingDiffers(t *testing.T) {
	sc := testScene()
	ref := RenderReference(sc)
	if ref.Counters.ClippedTris[Left] == ref.Counters.ClippedTris[Right] {
		t.Fatalf("期望双眼裁剪结果不同，实际 L=%d R=%d",
			ref.Counters.ClippedTris[Left], ref.Counters.ClippedTris[Right])
	}
}

func TestAsymmetricFrustum(t *testing.T) {
	sc := testScene()
	l, r := sc.Eyes[Left].Frustum, sc.Eyes[Right].Frustum
	if l[0] == r[0] && l[1] == r[1] {
		t.Fatalf("左右 frustum 未体现非对称投影: L=%v R=%v", l, r)
	}
	if center(l) == center(r) {
		t.Fatal("左右视锥中心应不同")
	}
}

func center(f [6]float64) float64 { return (f[0] + f[1]) / 2 }

func TestSharedInstances(t *testing.T) {
	sc := testScene()
	cov := CollectCoverage(sc, RenderReference(sc))
	if cov.SharedMeshCount != len(uniqueMeshes(sc)) || cov.SharedMeshCount < 5 {
		t.Fatalf("共享 mesh 计数异常: %d", cov.SharedMeshCount)
	}
	mv := RenderMultiview(sc, FaultNone, [2]Mat4{})
	tp := RenderTwoPass(sc, FaultNone, [2]Mat4{})
	if got := mv.Counters.MemoryBreakdown["shared-geometry"]; got != tp.Counters.MemoryBreakdown["shared-geometry"] {
		t.Fatal("两条路径几何内存应共享且相等")
	}
}

func TestSingleEyeMatrixFaultIsLocatedToTwoPass(t *testing.T) {
	sc := testScene()
	prev := StaleMatrices(sc)
	ref := RenderReference(sc)
	mv := RenderMultiview(sc, FaultSingleEyeMatrix, prev)
	tp := RenderTwoPass(sc, FaultSingleEyeMatrix, prev)
	dg := Diagnose(ref, mv, tp, FaultSingleEyeMatrix)
	if dg.Healthy {
		t.Fatal("故障场景不应判定为健康")
	}
	if dg.FaultPath != tp.Name {
		t.Fatalf("错误归属错误: %s", dg.FaultPath)
	}
	// 多视图路径双眼仍与参考逐位一致。
	for e := Left; e <= Right; e++ {
		if d := CompareFrames(ref.Frames[e], mv.Frames[e], e); !d.BitIdentical {
			t.Fatalf("多视图路径不应受单眼矩阵故障影响: %+v", d)
		}
	}
	// 双趟路径右眼出错、左眼保持正确。
	if d := CompareFrames(ref.Frames[Left], tp.Frames[Left], Left); !d.BitIdentical {
		t.Fatal("双趟路径左眼不应出错")
	}
	if d := CompareFrames(ref.Frames[Right], tp.Frames[Right], Right); d.DiffPixels == 0 {
		t.Fatal("双趟路径右眼应出现颜色差异")
	}
	if tp.MatrixFP[Right] == ref.MatrixFP[Right] {
		t.Fatal("右眼 VP 指纹应与参考不同")
	}
	if tp.Counters.UniformUpdates != 1 {
		t.Fatalf("故障下双趟路径只应刷新 1 次 UBO，实际 %d", tp.Counters.UniformUpdates)
	}
}

func TestBatchAndMemoryCounters(t *testing.T) {
	sc := testScene()
	mv := RenderMultiview(sc, FaultNone, [2]Mat4{})
	tp := RenderTwoPass(sc, FaultNone, [2]Mat4{})
	if mv.Counters.Submissions != 1 || tp.Counters.Submissions != 2 {
		t.Fatal("提交次数不符合路径语义")
	}
	if tp.Counters.DrawCalls != 2*mv.Counters.DrawCalls {
		t.Fatal("双趟 draw call 应为多视图的两倍")
	}
	if tp.Counters.MemoryBytes <= mv.Counters.MemoryBytes {
		t.Fatal("双趟路径因重复命令缓冲应占用更多内存")
	}
}
