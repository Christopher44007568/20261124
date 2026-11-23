package main

import "math"

// Source is the idealized logical image: linear sRGB (0..1) per logical pixel.
type Source struct {
	w, h   int
	pixels [][3]float64
}

func newSource(w, h int) *Source {
	return &Source{w: w, h: h, pixels: make([][3]float64, w*h)}
}

func (s *Source) at(x, y int) [3]float64 {
	if x < 0 || y < 0 || x >= s.w || y >= s.h {
		return [3]float64{}
	}
	return s.pixels[y*s.w+x]
}

func (s *Source) set(x, y int, c [3]float64) {
	if x >= 0 && y >= 0 && x < s.w && y < s.h {
		s.pixels[y*s.w+x] = c
	}
}

func redLineScene(w, h int) *Source {
	s := newSource(w, h)
	col := w / 2
	for y := 0; y < h; y++ {
		s.set(col, y, [3]float64{1, 0, 0})
	}
	return s
}

// diagonalScene paints a 1-px-wide white 45-degree line with coverage
// anti-aliasing, so subpixel phase along both axes is exercised.
func diagonalScene(w, h int) *Source {
	s := newSource(w, h)
	const halfWidth = 0.5
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			const samples = 8
			var cover float64
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := float64(x) + (float64(sx)+0.5)/samples
					py := float64(y) + (float64(sy)+0.5)/samples
					if math.Abs(py-px)/math.Sqrt2 <= halfWidth {
						cover++
					}
				}
			}
			cover /= samples * samples
			if cover > 0 {
				s.set(x, y, [3]float64{cover, cover, cover})
			}
		}
	}
	return s
}

func textScene(w, h int) *Source {
	s := newSource(w, h)
	text := "RGB01"
	advance := glyphCols + 1
	textW := len(text)*advance - 1
	x0 := (w - textW) / 2
	y0 := (h - glyphRows) / 2
	for i, r := range text {
		glyph, ok := bitmapFont[r]
		if !ok {
			continue
		}
		for gy := 0; gy < glyphRows; gy++ {
			for gx := 0; gx < glyphCols; gx++ {
				if glyph[gy]&(1<<(glyphCols-1-gx)) != 0 {
					s.set(x0+i*advance+gx, y0+gy, [3]float64{1, 1, 1})
				}
			}
		}
	}
	return s
}

// edgeScene paints a vertical black-to-white transition through the middle
// column, keeping a sharp two-level contrast.
func edgeScene(w, h int) *Source {
	s := newSource(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x >= w/2 {
				s.set(x, y, [3]float64{1, 1, 1})
			}
		}
	}
	return s
}

type sceneDef struct {
	name string
	make func(w, h int) *Source
}

var scenes = []sceneDef{
	{"redline", redLineScene},
	{"diagonal45", diagonalScene},
	{"text", textScene},
	{"edge", edgeScene},
}
