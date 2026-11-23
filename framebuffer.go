package main

// Framebuffer 为物理像素帧缓冲，颜色与模板按像素一一对应。
type Framebuffer struct {
	Width   int
	Height  int
	Color   []RGBA
	Stencil []uint8
}

func NewFramebuffer(w, h int) *Framebuffer {
	return &Framebuffer{
		Width:   w,
		Height:  h,
		Color:   make([]RGBA, w*h),
		Stencil: make([]uint8, w*h),
	}
}

func (f *Framebuffer) index(x, y int) int { return y*f.Width + x }

func (f *Framebuffer) InBounds(x, y int) bool {
	return x >= 0 && y >= 0 && x < f.Width && y < f.Height
}

// ClearColor 仅清空颜色缓冲，不触碰模板值。
func (f *Framebuffer) ClearColor(c RGBA, clip Rect, clearWhole bool) {
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			if !clearWhole && !clip.Contains(x, y) {
				continue
			}
			f.Color[f.index(x, y)] = c
		}
	}
}

// ClearStencil 仅清空模板缓冲。
func (f *Framebuffer) ClearStencil(v uint8, clip Rect, clearWhole bool) {
	for y := 0; y < f.Height; y++ {
		for x := 0; x < f.Width; x++ {
			if !clearWhole && !clip.Contains(x, y) {
				continue
			}
			f.Stencil[f.index(x, y)] = v
		}
	}
}

func (f *Framebuffer) Pixel(x, y int) (RGBA, uint8) {
	i := f.index(x, y)
	return f.Color[i], f.Stencil[i]
}

func (f *Framebuffer) SetColor(x, y int, c RGBA) { f.Color[f.index(x, y)] = c }

func (f *Framebuffer) SetStencil(x, y int, v uint8) { f.Stencil[f.index(x, y)] = v }
