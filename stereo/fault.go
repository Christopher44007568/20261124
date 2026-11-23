package stereo

import "fmt"

// FaultKind 标识注入的单眼错误类型。
type FaultKind uint8

const (
	// FaultMaterial 替换目标批次的材质。
	FaultMaterial FaultKind = iota
	// FaultStencil 替换目标批次的模板配置。
	FaultStencil
)

// Fault 描述一次只影响单眼的状态配置错误。
type Fault struct {
	Kind        FaultKind
	BatchID     string
	Eye         Eye
	BadMaterial Material     // FaultMaterial 时使用
	BadStencil  StencilState // FaultStencil 时使用
}

// CloneBatches 深拷贝批次切片，避免注入过程改写原始场景。
func CloneBatches(in []Batch) []Batch {
	out := make([]Batch, len(in))
	copy(out, in)
	return out
}

// InjectFault 在批次副本上注入单眼状态错误。
// 只有指定眼别的单眼批次会被改写；目标缺失或眼别不匹配都会报错。
func InjectFault(batches []Batch, f Fault) ([]Batch, error) {
	out := CloneBatches(batches)
	hit := false
	for i := range out {
		b := &out[i]
		if b.ID != f.BatchID || b.Kind != MonoBatch {
			continue
		}
		eye, err := b.Eye.Eye()
		if err != nil {
			return nil, err
		}
		if eye != f.Eye {
			continue
		}
		switch f.Kind {
		case FaultMaterial:
			b.State.Material = f.BadMaterial
		case FaultStencil:
			b.State.Stencil = f.BadStencil
		default:
			return nil, unknownFault(f.Kind)
		}
		hit = true
	}
	if !hit {
		return nil, faultMiss(f)
	}
	return out, nil
}

// EyeImpact 是单只眼睛受到的首个影响。
type EyeImpact struct {
	Eye        Eye
	Seq        int
	BatchID    string
	Region     Rect
	PixelCount int
}

// FaultReport 汇总一次故障注入的定位结果。
type FaultReport struct {
	Fault             Fault
	FirstStateKeyDiff *EyeImpact   // 状态键首次分歧的位置（可能不可见）
	Impacts           []EyeImpact  // 按眼别、序号排序的首个像素分歧
	FinalRegions      map[Eye]Rect // 最终画面上的残留差异区域
	FinalPixelCounts  map[Eye]int
}

// RunAudit 在相同场景上分别执行基线与故障管线，并逐批次比对快照。
func RunAudit(cfg EngineConfig, registry *Registry, batches []Batch, f Fault) (*FaultReport, error) {
	cfg.KeepSnapshots = true

	baseline := NewEngine(cfg, registry)
	if _, err := baseline.Run(CloneBatches(batches)); err != nil {
		return nil, err
	}

	faultyBatches, err := InjectFault(batches, f)
	if err != nil {
		return nil, err
	}
	faulty := NewEngine(cfg, registry)
	if _, err := faulty.Run(faultyBatches); err != nil {
		return nil, err
	}

	report := &FaultReport{
		Fault:            f,
		FinalRegions:     make(map[Eye]Rect),
		FinalPixelCounts: make(map[Eye]int),
	}

	baseRecords := baseline.Records()
	faultRecords := faulty.Records()
	for i := range baseRecords {
		if baseRecords[i].StateKey == faultRecords[i].StateKey {
			continue
		}
		eye := LeftEye
		if len(faultRecords[i].Eyes) == 1 {
			eye = faultRecords[i].Eyes[0]
		}
		report.FirstStateKeyDiff = &EyeImpact{
			Eye:     eye,
			Seq:     faultRecords[i].Seq,
			BatchID: faultRecords[i].BatchID,
		}
		break
	}

	seen := map[Eye]bool{}
	baseSnaps := baseline.Snapshots()
	faultSnaps := faulty.Snapshots()
	for i := range baseSnaps {
		for _, eye := range []Eye{LeftEye, RightEye} {
			if seen[eye] {
				continue
			}
			baseFB := baseSnaps[i].frame(eye)
			faultFB := faultSnaps[i].frame(eye)
			region, count, ok := PixelDiff(baseFB, faultFB)
			if !ok {
				continue
			}
			seen[eye] = true
			report.Impacts = append(report.Impacts, EyeImpact{
				Eye:        eye,
				Seq:        faultSnaps[i].Seq,
				BatchID:    faultRecords[faultSnaps[i].Seq].BatchID,
				Region:     region,
				PixelCount: count,
			})
		}
	}

	for _, eye := range []Eye{LeftEye, RightEye} {
		region, count, ok := PixelDiff(baseline.Framebuffer(eye), faulty.Framebuffer(eye))
		if ok {
			report.FinalRegions[eye] = region
			report.FinalPixelCounts[eye] = count
		}
	}
	return report, nil
}

func (s Snapshot) frame(eye Eye) *Framebuffer {
	if eye == LeftEye {
		return s.Left
	}
	return s.Right
}

func faultMiss(f Fault) error {
	return fmt.Errorf("stereo: no mono batch %q for eye %s matched the fault target",
		f.BatchID, f.Eye)
}

func unknownFault(k FaultKind) error {
	return fmt.Errorf("stereo: unknown fault kind %d", k)
}
