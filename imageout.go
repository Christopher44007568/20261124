package main

import (
	"math"
	"os"
	"path/filepath"
)

const panelScale = 4

func toSRGB(linear float64) uint8 {
	v := clamp01(linear)
	if v <= 0.0031308 {
		v *= 12.92
	} else {
		v = 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return uint8(clamp01(v)*255 + 0.5)
}

type ppmImage struct {
	w, h int
	rgb  []uint8
}

func newPPM(w, h int) *ppmImage {
	return &ppmImage{w: w, h: h, rgb: make([]uint8, 3*w*h)}
}

func (im *ppmImage) set(x, y int, c [3]float64) {
	if x < 0 || y < 0 || x >= im.w || y >= im.h {
		return
	}
	o := 3 * (y*im.w + x)
	im.rgb[o] = toSRGB(c[0])
	im.rgb[o+1] = toSRGB(c[1])
	im.rgb[o+2] = toSRGB(c[2])
}

func (im *ppmImage) write(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	header := []byte("P6\n")
	header = append(header, itoaBytes(im.w)...)
	header = append(header, ' ')
	header = append(header, itoaBytes(im.h)...)
	header = append(header, '\n', '2', '5', '5', '\n')
	if _, err := f.Write(header); err != nil {
		return err
	}
	_, err = f.Write(im.rgb)
	return err
}

func itoaBytes(v int) []byte {
	if v == 0 {
		return []byte{'0'}
	}
	var buf [16]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return buf[i:]
}

// renderPanel rasterizes the physical emitters at a magnified scale using
// point sampling per output cell, preserving individual aperture geometry
// instead of smoothing subpixels together.
func renderPanel(p *Panel, sim *Simulation) *ppmImage {
	im := newPPM(p.w*panelScale, p.h*panelScale)
	for oy := 0; oy < im.h; oy++ {
		for ox := 0; ox < im.w; ox++ {
			fx := (float64(ox) + 0.5) / panelScale
			fy := (float64(oy) + 0.5) / panelScale
			cx, cy := int(fx), int(fy)
			var c [3]float64
			if cx >= 0 && cx < p.w && cy >= 0 && cy < p.h {
				for _, id := range p.cells[cx*p.cellH+cy] {
					e := &p.emitters[id]
					if !e.ap.contains(vec2{fx, fy}) {
						continue
					}
					d := sim.drive[id]
					for k := 0; k < 3; k++ {
						c[k] += d * e.filter[k]
					}
				}
			}
			im.set(ox, oy, c)
		}
	}
	return im
}

func renderEye(p *Panel, img EyeImage) *ppmImage {
	im := newPPM(p.w, p.h)
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			im.set(x, y, img.px[y*p.w+x])
		}
	}
	return im
}
