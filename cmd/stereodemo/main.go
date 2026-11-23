// stereodemo 演示双眼共享状态管线的批次审计与单眼故障定位。
package main

import (
	"fmt"
	"os"

	"stereoaudit/stereo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	registry := stereo.NewDemoRegistry()
	batches := stereo.DemoBatches()
	cfg := stereo.EngineConfig{Width: stereo.FrameWidth, Height: stereo.FrameHeight, KeepSnapshots: true}

	engine := stereo.NewEngine(cfg, registry)
	records, err := engine.SubmitSequential(Clone(batches))
	if err != nil {
		return err
	}

	fmt.Println("== 批次审计记录（顺序提交） ==")
	for _, rec := range records {
		fmt.Printf("seq=%02d id=%-22s kind=%-6s eyes=%-6s stateKey=%016x inputs=%016x left=%016x right=%016x\n",
			rec.Seq, rec.BatchID, rec.Kind, eyeList(rec.Eyes),
			rec.StateKey, rec.InputsDigest, rec.LeftHash, rec.RightHash)
	}

	if err := verifySubmissionEquivalence(cfg, registry, batches); err != nil {
		return err
	}
	fmt.Println("\n== 提交方式一致性 ==")
	fmt.Println("顺序提交与批量提交的记录、双眼画面哈希完全一致")

	materialFault := stereo.Fault{
		Kind:    stereo.FaultMaterial,
		BatchID: "b05-highlight-l",
		Eye:     stereo.LeftEye,
		BadMaterial: stereo.Material{
			Name: "mat.CORRUPT", Color: [4]uint8{255, 0, 255, 255}, Blend: stereo.BlendOverwrite,
		},
	}
	if err := printAudit("单眼材质错误", cfg, registry, batches, materialFault); err != nil {
		return err
	}

	badStencil := stereo.StencilState{
		Enabled: true, Ref: 1, ReadMask: 0xff, Compare: stereo.CmpNotEqual,
		FailOp: stereo.StencilZero, PassOp: stereo.StencilKeep, WriteMask: 0xff,
	}
	stencilFault := stereo.Fault{
		Kind:       stereo.FaultStencil,
		BatchID:    "b06-highlight-r",
		Eye:        stereo.RightEye,
		BadStencil: badStencil,
	}
	if err := printAudit("单眼模板配置错误", cfg, registry, batches, stencilFault); err != nil {
		return err
	}

	if err := printPollutionDemo(cfg, registry, batches); err != nil {
		return err
	}
	return nil
}

func verifySubmissionEquivalence(cfg stereo.EngineConfig, registry *stereo.Registry, batches []stereo.Batch) error {
	cfg.KeepSnapshots = false

	seqEngine := stereo.NewEngine(cfg, registry)
	seqRecords, err := seqEngine.SubmitSequential(Clone(batches))
	if err != nil {
		return err
	}
	bulkEngine := stereo.NewEngine(cfg, registry)
	bulkRecords, err := bulkEngine.SubmitBulk(Clone(batches))
	if err != nil {
		return err
	}
	if len(seqRecords) != len(bulkRecords) {
		return fmt.Errorf("record count mismatch: %d vs %d", len(seqRecords), len(bulkRecords))
	}
	for i := range seqRecords {
		if !seqRecords[i].Equal(bulkRecords[i]) {
			return fmt.Errorf("record %d differs between sequential and bulk submission", i)
		}
	}
	if seqEngine.Framebuffer(stereo.LeftEye).Hash() != bulkEngine.Framebuffer(stereo.LeftEye).Hash() ||
		seqEngine.Framebuffer(stereo.RightEye).Hash() != bulkEngine.Framebuffer(stereo.RightEye).Hash() {
		return fmt.Errorf("final frame hashes differ between submission modes")
	}
	return nil
}

func printAudit(title string, cfg stereo.EngineConfig, registry *stereo.Registry, batches []stereo.Batch, f stereo.Fault) error {
	report, err := stereo.RunAudit(cfg, registry, batches, f)
	if err != nil {
		return err
	}
	fmt.Printf("\n== 故障定位：%s ==\n", title)
	fmt.Printf("注入目标: batch=%s eye=%s kind=%d\n", f.BatchID, f.Eye, f.Kind)
	if report.FirstStateKeyDiff != nil {
		d := report.FirstStateKeyDiff
		fmt.Printf("首个状态键分歧: seq=%02d batch=%s eye=%s\n", d.Seq, d.BatchID, d.Eye)
	}
	if len(report.Impacts) == 0 {
		fmt.Println("未观察到像素差异（错误被后续绘制覆盖或模板完全拒绝）")
	}
	for _, impact := range report.Impacts {
		fmt.Printf("首个受影响眼别=%s seq=%02d batch=%s 像素数=%d 区域=%s\n",
			impact.Eye, impact.Seq, impact.BatchID, impact.PixelCount, rectText(impact.Region))
	}
	for _, eye := range []stereo.Eye{stereo.LeftEye, stereo.RightEye} {
		region, ok := report.FinalRegions[eye]
		if !ok {
			continue
		}
		fmt.Printf("最终残留差异: eye=%s 像素数=%d 区域=%s\n",
			eye, report.FinalPixelCounts[eye], rectText(region))
	}
	return nil
}

func printPollutionDemo(cfg stereo.EngineConfig, registry *stereo.Registry, batches []stereo.Batch) error {
	fmt.Println("\n== 共享状态污染对照 ==")

	isolatedCfg := cfg
	isolatedCfg.DisableStateIsolation = false
	isolated := stereo.NewEngine(isolatedCfg, registry)
	if _, err := isolated.Run(Clone(batches)); err != nil {
		return err
	}

	// 构造事故路径：关闭状态隔离，右眼 b09 把污染材质写入共享状态后不还原，
	// 紧随其后的 b10 以 Inherit 方式沿用该材质并绘制另一区域，
	// 单眼更新就此在同一管线内扩散到额外像素。
	polluted := make([]stereo.Batch, len(batches))
	copy(polluted, batches)
	for i := range polluted {
		if polluted[i].ID == "b09-mono-glow-r" {
			polluted[i].Inherit = false
			polluted[i].State.Material = stereo.PollutedMaterial()
			polluted[i].State.Stencil = stereo.StencilState{}
			polluted[i].State.Scissor = stereo.Scissor{}
			polluted[i].GeometryName = stereo.GeomMarker
		}
		if polluted[i].ID == "b10-inherit-marker-r" {
			polluted[i].Inherit = true
			polluted[i].GeometryName = stereo.GeomMarker
		}
	}
	pollutedCfg := cfg
	pollutedCfg.DisableStateIsolation = true
	pollutedEngine := stereo.NewEngine(pollutedCfg, registry)
	if _, err := pollutedEngine.Run(polluted); err != nil {
		return err
	}

	for _, eye := range []stereo.Eye{stereo.LeftEye, stereo.RightEye} {
		region, count, ok := stereo.PixelDiff(isolated.Framebuffer(eye), pollutedEngine.Framebuffer(eye))
		if !ok {
			fmt.Printf("eye=%s 无污染差异\n", eye)
			continue
		}
		fmt.Printf("eye=%s 污染像素数=%d 区域=%s\n", eye, count, rectText(region))
	}
	return nil
}

func Clone(in []stereo.Batch) []stereo.Batch {
	return stereo.CloneBatches(in)
}

func eyeList(eyes []stereo.Eye) string {
	out := ""
	for i, eye := range eyes {
		if i > 0 {
			out += "+"
		}
		out += eye.String()
	}
	return out
}

func rectText(r stereo.Rect) string {
	return fmt.Sprintf("x=[%d,%d) y=[%d,%d)", r.MinX, r.MaxX, r.MinY, r.MaxY)
}
