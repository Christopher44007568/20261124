package main

// Minimal 5x7 bitmap font (one bit per column, MSB on the left). It contains
// exactly the glyphs rendered by the text scene and has no external font
// dependency.
const (
	glyphCols = 5
	glyphRows = 7
)

var bitmapFont = map[rune][glyphRows]byte{
	'R': {
		0b01110,
		0b10001,
		0b10001,
		0b11110,
		0b10100,
		0b10010,
		0b10001,
	},
	'G': {
		0b01110,
		0b10001,
		0b10000,
		0b10111,
		0b10001,
		0b10001,
		0b01110,
	},
	'B': {
		0b11110,
		0b10001,
		0b10001,
		0b11110,
		0b10001,
		0b10001,
		0b11110,
	},
	'0': {
		0b01110,
		0b10001,
		0b10011,
		0b10101,
		0b11001,
		0b10001,
		0b01110,
	},
	'1': {
		0b00100,
		0b01100,
		0b00100,
		0b00100,
		0b00100,
		0b00100,
		0b01110,
	},
	' ': {},
}
