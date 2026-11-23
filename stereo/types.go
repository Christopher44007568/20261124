package stereo

// Vertex 为模型空间顶点，带颜色与世界空间法线。
type Vertex struct {
	Pos    Vec3
	Normal Vec3
	Color  Vec3
}

// Tri 是一个三角形的三个顶点索引。
type Tri struct{ I0, I1, I2 int }

// Mesh 是被多眼 / 多实例共享的几何资源，只应被分配一次。
type Mesh struct {
	Name  string
	Verts []Vertex
	Tris  []Tri
	// Transparent 为 true 时走排序 + alpha 混合通道。
	Transparent bool
	Alpha       float64
}

// Instance 引用共享 Mesh，并给出实例自身的模型矩阵与颜色色调。
type Instance struct {
	Mesh  *Mesh
	Model Mat4
	Tint  Vec3
}

// Eye 标识左右眼。
type Eye int

const (
	Left Eye = iota
	Right
)

func (e Eye) String() string {
	if e == Left {
		return "L"
	}
	return "R"
}

// EyeParams 是单只眼的视图/投影参数。
type EyeParams struct {
	Eye      Eye
	View     Mat4
	Proj     Mat4
	ViewProj Mat4
	Frustum  [6]float64 // l, r, b, t, n, f
}

// Scene 包含共享实例集合与双眼参数。
type Scene struct {
	Instances []Instance
	Eyes      [2]EyeParams
	Width     int
	Height    int
}

// Frame 是一次渲染的输出：颜色缓冲、深度缓冲和透明提交顺序记录。
type Frame struct {
	Width, Height int
	Color         []Vec3
	Depth         []float64
	// TransOrder 按实际混合顺序记录 "mesh:tri:depth" 摘要。
	TransOrder []string
}

func newFrame(w, h int) *Frame {
	n := w * h
	f := &Frame{
		Width:  w,
		Height: h,
		Color:  make([]Vec3, n),
		Depth:  make([]float64, n),
	}
	for i := range f.Depth {
		f.Depth[i] = 1 // NDC 远平面
	}
	return f
}

// Counters 汇总一条提交路径的批次与内存计量。
type Counters struct {
	Submissions      int // 命令缓冲提交次数
	DrawCalls        int // 实际绘制调用
	ViewBindings     int // 视图/视口绑定次数
	UniformUpdates   int // uniform 槽写入次数
	TransparentSorts int // 透明队列重排次数
	ClippedTris      [2]int
	MemoryBytes      int64
	MemoryBreakdown  map[string]int64
}

// PathResult 是一条提交路径渲染双眼后的完整结果。
type PathResult struct {
	Name     string
	Frames   [2]*Frame
	Counters Counters
	// MatrixFP 记录每只眼实际生效的 ViewProj 指纹。
	MatrixFP [2]uint64
}

// FaultMode 描述注入的故障类型。
type FaultMode int

const (
	FaultNone FaultMode = iota
	// FaultSingleEyeMatrix：只更新单眼矩阵，另一只眼沿用旧矩阵。
	FaultSingleEyeMatrix
)
