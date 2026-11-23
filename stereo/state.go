// Package stereo 模拟一个双眼（VR 风格）软件渲染管线：
// 材质、模板测试与裁剪状态由左右眼共享，批次按眼别分组提交，
// 每次提交都会固化状态键、输入资源摘要与画面哈希，便于审计与故障定位。
package stereo

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
)

// CmpFunc 是模板测试使用的比较函数。
type CmpFunc uint8

const (
	CmpAlways CmpFunc = iota
	CmpEqual
	CmpNotEqual
	CmpLess
	CmpLEqual
	CmpGreater
	CmpGEqual
	CmpNever
)

func (c CmpFunc) pass(ref, stored uint8) bool {
	switch c {
	case CmpAlways:
		return true
	case CmpEqual:
		return ref == stored
	case CmpNotEqual:
		return ref != stored
	case CmpLess:
		return ref < stored
	case CmpLEqual:
		return ref <= stored
	case CmpGreater:
		return ref > stored
	case CmpGEqual:
		return ref >= stored
	case CmpNever:
		return false
	default:
		return false
	}
}

// StencilOp 描述模板测试通过/失败后对模板缓冲区的动作。
type StencilOp uint8

const (
	StencilKeep StencilOp = iota
	StencilZero
	StencilReplace
	StencilIncr
	StencilDecr
)

func (op StencilOp) apply(ref, stored uint8) uint8 {
	switch op {
	case StencilKeep:
		return stored
	case StencilZero:
		return 0
	case StencilReplace:
		return ref
	case StencilIncr:
		if stored < 255 {
			return stored + 1
		}
		return stored
	case StencilDecr:
		if stored > 0 {
			return stored - 1
		}
		return stored
	default:
		return stored
	}
}

// StencilState 是双眼共享的模板测试配置。
type StencilState struct {
	Enabled   bool
	Ref       uint8
	ReadMask  uint8
	Compare   CmpFunc
	FailOp    StencilOp
	DepthFail StencilOp // 软件管线没有深度，保留字段以贴近真实 API
	PassOp    StencilOp
	WriteMask uint8
}

// Equal 比较两个模板配置是否逐字段一致。
func (s StencilState) Equal(o StencilState) bool {
	return s == o
}

// BlendMode 决定材质颜色如何与帧缓冲区混合。
type BlendMode uint8

const (
	BlendOverwrite BlendMode = iota
	BlendAlphaOver
)

// Material 是双眼共享的材质状态。
type Material struct {
	Name        string
	TextureName string // 空字符串表示内置白色纹理
	Color       [4]uint8
	Blend       BlendMode
}

// Equal 比较两个材质是否逐字段一致。
func (m Material) Equal(o Material) bool {
	return m == o
}

// Scissor 是双眼共享的裁剪矩形；采用帧缓冲像素坐标，
// Min 为包含边界、Max 为不包含边界，与 image.Rectangle 的约定一致。
type Scissor struct {
	Enabled bool
	MinX    int
	MinY    int
	MaxX    int
	MaxY    int
}

// Equal 比较两个裁剪状态是否逐字段一致。
func (s Scissor) Equal(o Scissor) bool {
	return s == o
}

func (s Scissor) rect() Rect {
	return Rect{MinX: s.MinX, MinY: s.MinY, MaxX: s.MaxX, MaxY: s.MaxY}
}

// RenderState 聚合一个批次绘制时刻的全部共享状态。
type RenderState struct {
	Material Material
	Stencil  StencilState
	Scissor  Scissor
}

// Equal 比较两个渲染状态是否逐字段一致。
func (r RenderState) Equal(o RenderState) bool {
	return r.Material.Equal(o.Material) &&
		r.Stencil.Equal(o.Stencil) &&
		r.Scissor.Equal(o.Scissor)
}

func writeUint64(h hash.Hash64, v uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], v)
	_, _ = h.Write(buf[:])
}

func writeUint32(h hash.Hash64, v uint32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	_, _ = h.Write(buf[:])
}

func writeUint8(h hash.Hash64, v uint8) {
	_, _ = h.Write([]byte{v})
}

func writeString(h hash.Hash64, s string) {
	writeUint64(h, uint64(len(s)))
	_, _ = h.Write([]byte(s))
}

// StateKey 对渲染状态做稳定哈希，任何单眼配置错误都会改变键值。
func (r RenderState) StateKey() uint64 {
	h := fnv.New64a()
	writeString(h, r.Material.Name)
	writeString(h, r.Material.TextureName)
	for _, c := range r.Material.Color {
		writeUint8(h, c)
	}
	writeUint8(h, uint8(r.Material.Blend))

	writeUint8(h, boolByte(r.Stencil.Enabled))
	writeUint8(h, r.Stencil.Ref)
	writeUint8(h, r.Stencil.ReadMask)
	writeUint8(h, uint8(r.Stencil.Compare))
	writeUint8(h, uint8(r.Stencil.FailOp))
	writeUint8(h, uint8(r.Stencil.DepthFail))
	writeUint8(h, uint8(r.Stencil.PassOp))
	writeUint8(h, r.Stencil.WriteMask)

	writeUint8(h, boolByte(r.Scissor.Enabled))
	writeUint32(h, uint32(r.Scissor.MinX))
	writeUint32(h, uint32(r.Scissor.MinY))
	writeUint32(h, uint32(r.Scissor.MaxX))
	writeUint32(h, uint32(r.Scissor.MaxY))
	return h.Sum64()
}

func boolByte(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
