package main

// Mismatch 描述首个错误像素及两侧追踪。
type Mismatch struct {
	X, Y       int
	ColorRef   RGBA
	ColorBug   RGBA
	StencilRef uint8
	StencilBug uint8
	RefTrace   *PixelTrace
	BugTrace   *PixelTrace
}

// Compare 按行优先扫描，返回第一个颜色或模板不一致的像素。
func Compare(refFB, bugFB *Framebuffer, refTr, bugTr *TraceGrid) *Mismatch {
	for y := 0; y < refFB.Height; y++ {
		for x := 0; x < refFB.Width; x++ {
			rc, rs := refFB.Pixel(x, y)
			bc, bs := bugFB.Pixel(x, y)
			if rc != bc || rs != bs {
				return &Mismatch{
					X: x, Y: y,
					ColorRef:   rc,
					ColorBug:   bc,
					StencilRef: rs,
					StencilBug: bs,
					RefTrace:   refTr.At(x, y),
					BugTrace:   bugTr.At(x, y),
				}
			}
		}
	}
	return nil
}

// CountMismatches 统计错误像素总数。
func CountMismatches(refFB, bugFB *Framebuffer) int {
	n := 0
	for i := range refFB.Color {
		if refFB.Color[i] != bugFB.Color[i] || refFB.Stencil[i] != bugFB.Stencil[i] {
			n++
		}
	}
	return n
}
