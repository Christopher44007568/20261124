package main

import (
	"fmt"
	"io"
)

// DumpASCII 输出帧缓冲的可视化地图：大写字母为颜色块，数字为最终模板值。
func DumpASCII(w io.Writer, fb *Framebuffer) {
	colorGlyph := map[RGBA]byte{
		colClear:    '.',
		colOuter:    'O',
		colInner:    'I',
		colDecor:    'D',
		colFooter:   'F',
		colBadClear: 'X',
	}
	fmt.Fprintln(w, "颜色地图: .=背景 O=外层 I=内层 D=装饰 F=页脚 X=错误清空 #=未初始化")
	for y := 0; y < fb.Height; y++ {
		for x := 0; x < fb.Width; x++ {
			c, _ := fb.Pixel(x, y)
			g, ok := colorGlyph[c]
			if !ok {
				g = '#'
			}
			fmt.Fprintf(w, "%c", g)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "模板地图:")
	for y := 0; y < fb.Height; y++ {
		for x := 0; x < fb.Width; x++ {
			_, s := fb.Pixel(x, y)
			if s == 0 {
				fmt.Fprint(w, ".")
			} else {
				fmt.Fprintf(w, "%d", s)
			}
		}
		fmt.Fprintln(w)
	}
}
