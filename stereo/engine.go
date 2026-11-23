package stereo

import "fmt"

// EngineConfig 控制引擎的审计行为。
type EngineConfig struct {
	// Width/Height 是每只眼睛帧缓冲的像素尺寸。
	Width  int
	Height int
	// KeepSnapshots 为 true 时保存每个批次之后的双眼画面快照，
	// 故障定位器依赖它找出“首个”受影响批次。
	KeepSnapshots bool
	// DisableStateIsolation 为 true 时关闭共享状态的保存/还原，
	// 用于复现单眼更新污染另一眼的事故路径；默认零值即安全隔离。
	DisableStateIsolation bool
}

// Snapshot 保存一个批次执行后的双眼画面。
type Snapshot struct {
	Seq   int
	Left  *Framebuffer
	Right *Framebuffer
}

// Engine 是双眼软件渲染管线。
type Engine struct {
	cfg       EngineConfig
	registry  *Registry
	current   RenderState
	fbs       [2]*Framebuffer
	stencils  [2]*StencilBuffer
	records   []Record
	snapshots []Snapshot
}

// NewEngine 创建引擎并初始化双眼缓冲。
func NewEngine(cfg EngineConfig, registry *Registry) *Engine {
	e := &Engine{cfg: cfg, registry: registry}
	e.fbs[LeftEye] = NewFramebuffer(cfg.Width, cfg.Height)
	e.fbs[RightEye] = NewFramebuffer(cfg.Width, cfg.Height)
	e.stencils[LeftEye] = NewStencilBuffer(cfg.Width, cfg.Height)
	e.stencils[RightEye] = NewStencilBuffer(cfg.Width, cfg.Height)
	return e
}

// CurrentState 返回当前共享渲染状态。
func (e *Engine) CurrentState() RenderState {
	return e.current
}

// Records 返回全部已执行批次的审计记录。
func (e *Engine) Records() []Record {
	return e.records
}

// Snapshots 返回全部批次后的画面快照；未开启快照时返回 nil。
func (e *Engine) Snapshots() []Snapshot {
	return e.snapshots
}

// Framebuffer 返回指定眼别的当前帧缓冲。
func (e *Engine) Framebuffer(eye Eye) *Framebuffer {
	return e.fbs[eye]
}

// StencilBuffer 返回指定眼别的当前模板缓冲。
func (e *Engine) StencilBuffer(eye Eye) *StencilBuffer {
	return e.stencils[eye]
}

func (e *Engine) validate(b Batch) error {
	if b.ID == "" {
		return fmt.Errorf("stereo: batch id must not be empty")
	}
	if _, err := b.Eyes(); err != nil {
		return err
	}
	if b.Kind != MonoBatch && b.Eye != 0 {
		return fmt.Errorf("stereo: batch %q eye mask only valid for mono batches", b.ID)
	}
	if _, err := e.registry.Geometry(b.GeometryName); err != nil {
		return fmt.Errorf("stereo: batch %q: %w", b.ID, err)
	}
	if _, err := e.registry.Swatch(b.TextureName); err != nil {
		return fmt.Errorf("stereo: batch %q: %w", b.ID, err)
	}
	return nil
}

// Run 按切片顺序逐个执行批次。SubmitSequential 与 SubmitBulk
// 都通过这同一个执行入口，因此两种提交方式的结果必然一致。
func (e *Engine) Run(batches []Batch) ([]Record, error) {
	for _, b := range batches {
		if err := e.validate(b); err != nil {
			return e.records, err
		}
		if err := e.execute(b); err != nil {
			return e.records, err
		}
	}
	return e.records, nil
}

// SubmitSequential 逐批提交。
func (e *Engine) SubmitSequential(batches []Batch) ([]Record, error) {
	return e.Run(batches)
}

// SubmitBulk 一次性批量提交；内部保持与顺序提交完全相同的执行顺序。
func (e *Engine) SubmitBulk(batches []Batch) ([]Record, error) {
	return e.Run(batches)
}

func (e *Engine) execute(b Batch) error {
	eyes, err := b.Eyes()
	if err != nil {
		return err
	}

	// 默认启用状态隔离：单眼批次对共享状态的修改在批次结束后还原，
	// 防止污染另一眼。显式关闭隔离或 Inherit 批次则模拟事故路径。
	saved := e.current
	switch {
	case b.Inherit:
		// 继承批次沿用共享槽中的当前状态，不做任何切换。
	case e.cfg.DisableStateIsolation:
		// 隔离关闭：批次状态直接写入共享槽且不还原，污染会持续传播。
		e.current = b.State
	default:
		// 安全默认：批次结束后还原共享状态。
		e.current = b.State
	}

	if b.Kind == SharedBatch {
		// 共享几何只光栅化一次，再把同一像素结果复制到另一只眼，
		// 保证双眼严格一致，也避免重复扫描带来的歧义。
		if err := e.draw(LeftEye, b); err != nil {
			return err
		}
		e.copySharedResult(b)
	} else {
		for _, eye := range eyes {
			if err := e.draw(eye, b); err != nil {
				return err
			}
		}
	}

	if !b.Inherit && !e.cfg.DisableStateIsolation {
		e.current = saved
	}

	seq := len(e.records)
	rec := Record{
		Seq:          seq,
		BatchID:      b.ID,
		Kind:         b.Kind,
		Eyes:         append([]Eye(nil), eyes...),
		StateKey:     e.effectiveState(b).StateKey(),
		InputsDigest: ResourceDigest(b.GeometryName, b.TextureName),
		LeftHash:     e.fbs[LeftEye].Hash(),
		RightHash:    e.fbs[RightEye].Hash(),
	}
	e.records = append(e.records, rec)

	if e.cfg.KeepSnapshots {
		e.snapshots = append(e.snapshots, Snapshot{
			Seq:   seq,
			Left:  e.fbs[LeftEye].Clone(),
			Right: e.fbs[RightEye].Clone(),
		})
	}
	return nil
}

func (e *Engine) effectiveState(b Batch) RenderState {
	if b.Inherit {
		return e.current
	}
	return b.State
}

func (e *Engine) copySharedResult(b Batch) {
	geom, err := e.registry.Geometry(b.GeometryName)
	if err != nil {
		return
	}
	state := e.effectiveState(b)
	target := geom.Bounds
	if state.Scissor.Enabled {
		target = target.Intersect(state.Scissor.rect())
	}
	target = target.Intersect(Rect{MaxX: e.cfg.Width, MaxY: e.cfg.Height})
	if target.Empty() {
		return
	}
	src := e.fbs[LeftEye]
	dst := e.fbs[RightEye]
	for y := target.MinY; y < target.MaxY; y++ {
		for x := target.MinX; x < target.MaxX; x++ {
			dst.Set(x, y, src.At(x, y))
		}
	}
	// 模板副作用同样共享。
	srcStencil := e.stencils[LeftEye]
	dstStencil := e.stencils[RightEye]
	for y := target.MinY; y < target.MaxY; y++ {
		for x := target.MinX; x < target.MaxX; x++ {
			dstStencil.Set(x, y, srcStencil.At(x, y))
		}
	}
}

func (e *Engine) draw(eye Eye, b Batch) error {
	geom, err := e.registry.Geometry(b.GeometryName)
	if err != nil {
		return err
	}
	swatch, err := e.registry.Swatch(b.TextureName)
	if err != nil {
		return err
	}
	state := e.effectiveState(b)

	target := geom.Bounds
	if state.Scissor.Enabled {
		target = target.Intersect(state.Scissor.rect())
		if target.Empty() {
			return nil
		}
	}
	target = target.Intersect(Rect{MaxX: e.cfg.Width, MaxY: e.cfg.Height})
	if target.Empty() {
		return nil
	}

	fb := e.fbs[eye]
	stencil := e.stencils[eye]
	src := modulate(state.Material.Color, swatch.Color)

	for y := target.MinY; y < target.MaxY; y++ {
		for x := target.MinX; x < target.MaxX; x++ {
			if state.Stencil.Enabled {
				stored := stencil.At(x, y) & state.Stencil.ReadMask
				ref := state.Stencil.Ref & state.Stencil.ReadMask
				if !state.Stencil.Compare.pass(ref, stored) {
					next := state.Stencil.FailOp.apply(state.Stencil.Ref, stored)
					stencil.Set(x, y, next&state.Stencil.WriteMask)
					continue
				}
			}
			fb.Set(x, y, blendPixel(fb.At(x, y), src, state.Material.Blend))
			if state.Stencil.Enabled {
				stored := stencil.At(x, y)
				next := state.Stencil.PassOp.apply(state.Stencil.Ref, stored)
				stencil.Set(x, y, next&state.Stencil.WriteMask)
			}
		}
	}
	return nil
}

func modulate(a, b [4]uint8) [4]uint8 {
	return [4]uint8{
		uint8(uint16(a[0]) * uint16(b[0]) / 255),
		uint8(uint16(a[1]) * uint16(b[1]) / 255),
		uint8(uint16(a[2]) * uint16(b[2]) / 255),
		uint8(uint16(a[3]) * uint16(b[3]) / 255),
	}
}

func blendPixel(dst, src [4]uint8, mode BlendMode) [4]uint8 {
	switch mode {
	case BlendOverwrite:
		return src
	case BlendAlphaOver:
		return alphaOver(dst, src)
	default:
		return src
	}
}

func alphaOver(dst, src [4]uint8) [4]uint8 {
	sa := uint32(src[3])
	if sa == 255 {
		return src
	}
	if sa == 0 {
		return dst
	}
	da := uint32(dst[3])
	outA := sa + da*(255-sa)/255
	if outA == 0 {
		return [4]uint8{}
	}
	var out [4]uint8
	for i := 0; i < 3; i++ {
		sc := uint32(src[i]) * sa
		dc := uint32(dst[i]) * da * (255 - sa) / 255
		out[i] = uint8((sc + dc) / outA)
	}
	out[3] = uint8(outA)
	return out
}
