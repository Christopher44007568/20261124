package stereo

// RenderMultiview 模拟单趟多视图：一次提交，几何只遍历一次，
// 每个 draw 携带 viewMask 同时作用于双眼。
func RenderMultiview(sc *Scene, fault FaultMode, prev [2]Mat4) PathResult {
	r := PathResult{Name: "multiview-single-pass"}
	r.Counters.MemoryBreakdown = map[string]int64{}

	// 共享资源：几何一份、双视图帧缓冲一份、命令缓冲一份、UBO 双槽。
	geo := geometryBytes(sc)
	fb := framebufferBytes(sc)
	cmd := int64(drawCount(sc)) * bytesCmdRecord
	ubo := int64(2) * bytesUBOSlot
	state := int64(2) * bytesViewState
	put(r.Counters.MemoryBreakdown, "shared-geometry", geo)
	put(r.Counters.MemoryBreakdown, "view-framebuffers", fb)
	put(r.Counters.MemoryBreakdown, "command-buffer", cmd)
	put(r.Counters.MemoryBreakdown, "ubo-view-matrices", ubo)
	put(r.Counters.MemoryBreakdown, "view-state", state)

	// 单趟：一次提交、一次视口/视图绑定（viewMask=both）。
	r.Counters.Submissions = 1
	r.Counters.ViewBindings = 1
	r.Counters.DrawCalls = drawCount(sc)

	// 多视图 UBO 一次性写入双眼矩阵；单眼矩阵更新故障无法在此路径发生
	//（双眼槽在同一次提交内一起刷新），因此两次写入都被记录。
	r.Counters.UniformUpdates = 2
	for e := Left; e <= Right; e++ {
		vp := effectiveVP(sc, prev, e, FaultNone)
		r.MatrixFP[e] = vp.Fingerprint()
		ep := sc.Eyes[e]
		ep.ViewProj = vp
		r.Frames[e] = renderEye(sc, ep, &r.Counters)
	}
	sum(&r.Counters)
	return r
}

// RenderTwoPass 模拟两次独立绘制提交：每只眼一条命令缓冲、一次提交。
func RenderTwoPass(sc *Scene, fault FaultMode, prev [2]Mat4) PathResult {
	r := PathResult{Name: "two-independent-passes"}
	r.Counters.MemoryBreakdown = map[string]int64{}

	// 几何仍共享，但每趟各持一份命令缓冲、帧缓冲元数据、UBO 槽与视图状态。
	geo := geometryBytes(sc)
	fb := framebufferBytes(sc)
	cmd := int64(2*drawCount(sc)) * bytesCmdRecord
	ubo := int64(2) * bytesUBOSlot
	state := int64(2) * bytesViewState
	put(r.Counters.MemoryBreakdown, "shared-geometry", geo)
	put(r.Counters.MemoryBreakdown, "view-framebuffers", fb)
	put(r.Counters.MemoryBreakdown, "command-buffer", cmd)
	put(r.Counters.MemoryBreakdown, "ubo-view-matrices", ubo)
	put(r.Counters.MemoryBreakdown, "view-state", state)

	r.Counters.Submissions = 2
	r.Counters.ViewBindings = 2
	r.Counters.DrawCalls = 2 * drawCount(sc)

	for e := Left; e <= Right; e++ {
		vp := effectiveVP(sc, prev, e, fault)
		// 每趟独立写一次 UBO 槽；故障时右眼槽没有被刷新，却仍被绑定。
		if !(fault == FaultSingleEyeMatrix && e == Right) {
			r.Counters.UniformUpdates++
		}
		r.MatrixFP[e] = vp.Fingerprint()
		ep := sc.Eyes[e]
		ep.ViewProj = vp
		r.Frames[e] = renderEye(sc, ep, &r.Counters)
	}
	sum(&r.Counters)
	return r
}

// RenderReference 是无故障真值，供诊断模块判断错误归属哪条路径。
func RenderReference(sc *Scene) PathResult {
	return RenderMultiview(sc, FaultNone, [2]Mat4{})
}

func put(m map[string]int64, k string, v int64) {
	m[k] = v
}

func sum(c *Counters) {
	c.MemoryBytes = 0
	for _, v := range c.MemoryBreakdown {
		c.MemoryBytes += v
	}
}
