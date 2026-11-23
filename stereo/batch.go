package stereo

import "fmt"

// BatchKind 区分共享几何、单眼与双眼三类批次。
type BatchKind uint8

const (
	// SharedBatch 只光栅化一次，结果同时写入左右眼同一区域。
	SharedBatch BatchKind = iota
	// MonoBatch 只绘制到 EyeMask 指定的单眼。
	MonoBatch
	// StereoBatch 用同一状态分别绘制左右眼。
	StereoBatch
)

// String 返回批次类型的可读名称。
func (k BatchKind) String() string {
	switch k {
	case SharedBatch:
		return "shared"
	case MonoBatch:
		return "mono"
	case StereoBatch:
		return "stereo"
	default:
		return "unknown"
	}
}

// EyeMask 是单眼批次的眼别位掩码。
type EyeMask uint8

const (
	MaskLeft  EyeMask = 1 << LeftEye
	MaskRight EyeMask = 1 << RightEye
)

// Eye 把单眼位掩码转换为眼别；非单眼掩码会报错。
func (m EyeMask) Eye() (Eye, error) {
	switch m {
	case MaskLeft:
		return LeftEye, nil
	case MaskRight:
		return RightEye, nil
	default:
		return 0, fmt.Errorf("stereo: eye mask %d is not exactly one eye", m)
	}
}

// Batch 描述一次绘制调用。
type Batch struct {
	ID    string
	Kind  BatchKind
	Eye   EyeMask // 仅 MonoBatch 使用
	State RenderState

	// GeometryName 与 TextureName 是输入资源句柄。
	GeometryName string
	TextureName  string

	// Inherit 为 true 时跳过状态隔离，直接沿用当前共享状态。
	// 它用于复现“单眼更新污染另一眼”的真实事故现场。
	Inherit bool
}

// Eyes 返回该批次应当触达的眼别列表（保持左眼在前的稳定顺序）。
func (b Batch) Eyes() ([]Eye, error) {
	switch b.Kind {
	case SharedBatch, StereoBatch:
		return []Eye{LeftEye, RightEye}, nil
	case MonoBatch:
		eye, err := b.Eye.Eye()
		if err != nil {
			return nil, err
		}
		return []Eye{eye}, nil
	default:
		return nil, fmt.Errorf("stereo: unknown batch kind %d", b.Kind)
	}
}

// Record 是批次执行后固化下来的审计记录。
type Record struct {
	Seq          int
	BatchID      string
	Kind         BatchKind
	Eyes         []Eye
	StateKey     uint64
	InputsDigest uint64
	LeftHash     uint64
	RightHash    uint64
}

// FrameHashes 返回左右眼画面哈希，便于按眼别比对。
func (r Record) FrameHashes() (left, right uint64) {
	return r.LeftHash, r.RightHash
}

// Equal 比较两条审计记录是否逐字段一致。
func (r Record) Equal(o Record) bool {
	if r.Seq != o.Seq || r.BatchID != o.BatchID || r.Kind != o.Kind ||
		r.StateKey != o.StateKey || r.InputsDigest != o.InputsDigest ||
		r.LeftHash != o.LeftHash || r.RightHash != o.RightHash ||
		len(r.Eyes) != len(o.Eyes) {
		return false
	}
	for i := range r.Eyes {
		if r.Eyes[i] != o.Eyes[i] {
			return false
		}
	}
	return true
}
