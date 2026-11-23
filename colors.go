package main

var (
	colClear    = RGBA{0x10, 0x12, 0x18, 0xff}
	colOuter    = RGBA{0x2e, 0x5c, 0x9e, 0xff}
	colInner    = RGBA{0x3f, 0xa0, 0x6b, 0xff}
	colDecor    = RGBA{0xd0, 0x8a, 0x2e, 0xff}
	colFooter   = RGBA{0x7a, 0x4a, 0x9e, 0xff}
	colBadClear = RGBA{0xff, 0x00, 0xff, 0xff}
)

// 嵌套面板布局（逻辑坐标）。
var (
	rectOuter  = Rect{X: 1, Y: 1, W: 10, H: 8}
	rectInner  = Rect{X: 3, Y: 2, W: 5, H: 5}
	rectDecor  = Rect{X: 7, Y: 3, W: 2, H: 2}
	rectFooter = Rect{X: 1, Y: 9, W: 10, H: 1}
	rectEmpty  = Rect{X: 5, Y: 4, W: 0, H: 3}
)

const (
	refOuter = uint8(1)
	refInner = uint8(2)
	refBad   = uint8(7)
)
