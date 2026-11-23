package main

import (
	"fmt"
	"math"
)

// Emitter is one physical light-emitting aperture plus its color filter.
// A mono panel uses the white filter (1,1,1); a color panel never has more
// than one active channel per emitter, so no logical pixel is assumed to own
// a complete RGB triplet.
type Emitter struct {
	ap     rect
	filter [3]float64
	cellX  int
	cellY  int
}

type Panel struct {
	name     string
	w, h     int
	psfX     float64
	psfY     float64
	emitters []Emitter
	cells    map[int][]int
	cellH    int
}

var (
	redFilter   = [3]float64{1, 0, 0}
	greenFilter = [3]float64{0, 1, 0}
	blueFilter  = [3]float64{0, 0, 1}
	whiteFilter = [3]float64{1, 1, 1}
)

func newPanel(name string, w, h int, psfX, psfY float64, emitters []Emitter) *Panel {
	p := &Panel{
		name:     name,
		w:        w,
		h:        h,
		psfX:     psfX,
		psfY:     psfY,
		emitters: emitters,
		cells:    make(map[int][]int),
		cellH:    h + 2,
	}
	for i := range emitters {
		e := &emitters[i]
		key := e.cellX*p.cellH + e.cellY
		p.cells[key] = append(p.cells[key], i)
	}
	return p
}

func (p *Panel) candidates(cx, cy, reach int) []int {
	var out []int
	for ix := cx - reach; ix <= cx+reach; ix++ {
		if ix < 0 || ix >= p.w {
			continue
		}
		for iy := cy - reach; iy <= cy+reach; iy++ {
			if iy < 0 || iy >= p.h {
				continue
			}
			out = append(out, p.cells[ix*p.cellH+iy]...)
		}
	}
	return out
}

// stripePanel builds a classic RGB vertical stripe: every logical pixel owns
// three tall vertical subpixels, one per channel.
func stripePanel(w, h int) *Panel {
	const (
		subW   = 0.26
		gap    = 0.06
		left   = 0.05
		top    = 0.04
		height = 0.92
	)
	filters := [3][3]float64{redFilter, greenFilter, blueFilter}
	var emitters []Emitter
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			for i := 0; i < 3; i++ {
				x0 := float64(x) + left + float64(i)*(subW+gap)
				ap := rect{
					min: vec2{x0, float64(y) + top},
					max: vec2{x0 + subW, float64(y) + top + height},
				}
				emitters = append(emitters, Emitter{ap: ap, filter: filters[i], cellX: x, cellY: y})
			}
		}
	}
	return newPanel(fmt.Sprintf("rgb_stripe_%dx%d", w, h), w, h, psfHalfWidthX, psfHalfWidthY, emitters)
}

// pentilePanel builds an RGBG PenTile-style checkerboard. Green occupies half
// the emitter sites; red and blue each occupy one quarter. With two emitters
// per logical pixel this yields one green, half a red and half a blue per
// pixel on average, and no emitter carries more than one channel.
func pentilePanel(w, h int) *Panel {
	const (
		spacing = 1.0 / math.Sqrt2
		size    = spacing * 0.84
		half    = size / 2
	)
	var emitters []Emitter
	cols, rows := w, 2*h
	for k := 0; k < rows; k++ {
		for j := 0; j < cols; j++ {
			cx := (float64(j) + 0.5)
			cy := (float64(k) + 0.5) * spacing
			var filter [3]float64
			switch {
			case k%2 == 0:
				// Even rows: alternating red / green.
				if j%2 == 0 {
					filter = redFilter
				} else {
					filter = greenFilter
				}
			default:
				// Odd rows: alternating green / blue.
				if j%2 == 0 {
					filter = greenFilter
				} else {
					filter = blueFilter
				}
			}
			ap := rect{min: vec2{cx - half, cy - half}, max: vec2{cx + half, cy + half}}
			emitters = append(emitters, Emitter{
				ap:     ap,
				filter: filter,
				cellX:  int(cx),
				cellY:  int(cy),
			})
		}
	}
	return newPanel(fmt.Sprintf("pentile_rgbg_%dx%d", w, h), w, h, psfHalfWidthX, psfHalfWidthY, emitters)
}

// monoPanel builds a black-and-white panel: a single white emitter per
// logical pixel, driven by the luma of the source color.
func monoPanel(w, h int) *Panel {
	const (
		inset = 0.05
		size  = 0.9
	)
	var emitters []Emitter
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ap := rect{
				min: vec2{float64(x) + inset, float64(y) + inset},
				max: vec2{float64(x) + inset + size, float64(y) + inset + size},
			}
			emitters = append(emitters, Emitter{ap: ap, filter: whiteFilter, cellX: x, cellY: y})
		}
	}
	return newPanel(fmt.Sprintf("mono_bw_%dx%d", w, h), w, h, psfHalfWidthX, psfHalfWidthY, emitters)
}
