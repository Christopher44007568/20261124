package stereo

import (
	"hash/fnv"
)

// Eye 标识一只眼睛。
type Eye uint8

const (
	LeftEye Eye = iota
	RightEye
)

// String 返回眼别的可读名称。
func (e Eye) String() string {
	switch e {
	case LeftEye:
		return "left"
	case RightEye:
		return "right"
	default:
		return "unknown"
	}
}

// ClearColor 是帧缓冲的默认清屏颜色（深蓝灰）。
var ClearColor = [4]uint8{16, 20, 28, 255}

// Framebuffer 是单眼 RGBA8 帧缓冲，像素按行优先存储。
type Framebuffer struct {
	Width  int
	Height int
	Pix    []uint8
}

// NewFramebuffer 创建并清空一个帧缓冲。
func NewFramebuffer(width, height int) *Framebuffer {
	if width <= 0 || height <= 0 {
		panic("stereo: framebuffer dimensions must be positive")
	}
	fb := &Framebuffer{
		Width:  width,
		Height: height,
		Pix:    make([]uint8, width*height*4),
	}
	fb.Clear()
	return fb
}

// Clear 用清屏颜色填充整个帧缓冲。
func (f *Framebuffer) Clear() {
	for i := 0; i < len(f.Pix); i += 4 {
		f.Pix[i] = ClearColor[0]
		f.Pix[i+1] = ClearColor[1]
		f.Pix[i+2] = ClearColor[2]
		f.Pix[i+3] = ClearColor[3]
	}
}

func (f *Framebuffer) offset(x, y int) int {
	return (y*f.Width + x) * 4
}

// At 返回指定像素的 RGBA 值。
func (f *Framebuffer) At(x, y int) [4]uint8 {
	i := f.offset(x, y)
	return [4]uint8{f.Pix[i], f.Pix[i+1], f.Pix[i+2], f.Pix[i+3]}
}

// Set 写入指定像素的 RGBA 值。
func (f *Framebuffer) Set(x, y int, c [4]uint8) {
	i := f.offset(x, y)
	f.Pix[i] = c[0]
	f.Pix[i+1] = c[1]
	f.Pix[i+2] = c[2]
	f.Pix[i+3] = c[3]
}

// Clone 生成帧缓冲的完整拷贝，用于批次快照。
func (f *Framebuffer) Clone() *Framebuffer {
	pix := make([]uint8, len(f.Pix))
	copy(pix, f.Pix)
	return &Framebuffer{Width: f.Width, Height: f.Height, Pix: pix}
}

// Hash 对整幅画面计算 FNV-1a 哈希。
func (f *Framebuffer) Hash() uint64 {
	h := fnv.New64a()
	_, _ = h.Write(f.Pix)
	return h.Sum64()
}

// StencilBuffer 是单眼 8 位模板缓冲。
type StencilBuffer struct {
	Width  int
	Height int
	Pix    []uint8
}

// NewStencilBuffer 创建全零模板缓冲。
func NewStencilBuffer(width, height int) *StencilBuffer {
	return &StencilBuffer{
		Width:  width,
		Height: height,
		Pix:    make([]uint8, width*height),
	}
}

// At 读取模板值。
func (s *StencilBuffer) At(x, y int) uint8 {
	return s.Pix[y*s.Width+x]
}

// Set 写入模板值。
func (s *StencilBuffer) Set(x, y int, v uint8) {
	s.Pix[y*s.Width+x] = v
}

// Clone 生成模板缓冲的完整拷贝。
func (s *StencilBuffer) Clone() *StencilBuffer {
	pix := make([]uint8, len(s.Pix))
	copy(pix, s.Pix)
	return &StencilBuffer{Width: s.Width, Height: s.Height, Pix: pix}
}

// PixelDiff 返回两个等尺寸帧缓冲之间所有不同像素的包围盒与像素数；
// 完全一致时 ok 为 false。
func PixelDiff(a, b *Framebuffer) (region Rect, count int, ok bool) {
	if a.Width != b.Width || a.Height != b.Height {
		panic("stereo: cannot diff framebuffers of different sizes")
	}
	region = Rect{MinX: a.Width, MinY: a.Height, MaxX: -1, MaxY: -1}
	for y := 0; y < a.Height; y++ {
		for x := 0; x < a.Width; x++ {
			if a.At(x, y) != b.At(x, y) {
				count++
				if x < region.MinX {
					region.MinX = x
				}
				if x+1 > region.MaxX {
					region.MaxX = x + 1
				}
				if y < region.MinY {
					region.MinY = y
				}
				if y+1 > region.MaxY {
					region.MaxY = y + 1
				}
			}
		}
	}
	if count == 0 {
		return Rect{}, 0, false
	}
	return region, count, true
}
