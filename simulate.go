package main

const (
	coverageSamples = 6 // per axis inside each emitter aperture
	// psfHalfWidth is the eye point-spread-function half-width in logical
	// pixels. The horizontal lobe is wider to model optical cross-talk
	// between neighboring subpixel columns; the vertical lobe stays one
	// pixel wide. Both eyes share these widths and differ only in phase.
	psfHalfWidthX = 1.25
	psfHalfWidthY = 1.0
	// leakage is the fraction of an emitter's light that the PSF carries into
	// a subpixel whose color filter selects a different channel.
	leakage = 0.12
	rec709R = 0.2126
	rec709G = 0.7152
	rec709B = 0.0722
)

type EyeImage struct {
	px [][3]float64
}

type Simulation struct {
	panel       *Panel
	drive       []float64
	channel     []uint8
	panelLuma   []float64
	left, right EyeImage
}

func luma(c [3]float64) float64 {
	return rec709R*c[0] + rec709G*c[1] + rec709B*c[2]
}

func isMono(p *Panel) bool {
	return p.emitters[0].filter == whiteFilter
}

// runSimulation drives every emitter from the logical source using geometric
// coverage sampling, then reconstructs the two retinal images. The eye PSF is
// the same fixed-width kernel for both eyes; only the sampling phase shifts.
func runSimulation(p *Panel, src *Source) *Simulation {
	mono := isMono(p)
	sim := &Simulation{
		panel:     p,
		drive:     make([]float64, len(p.emitters)),
		channel:   make([]uint8, len(p.emitters)),
		panelLuma: make([]float64, len(p.emitters)),
	}
	for i := range p.emitters {
		e := &p.emitters[i]
		var channel uint8
		var weight float64
		for k := 0; k < 3; k++ {
			if e.filter[k] > 0 {
				channel = uint8(k)
				weight = e.filter[k]
				break
			}
		}
		var total float64
		for sy := 0; sy < coverageSamples; sy++ {
			for sx := 0; sx < coverageSamples; sx++ {
				px := e.ap.min.x + (float64(sx)+0.5)/coverageSamples*(e.ap.max.x-e.ap.min.x)
				py := e.ap.min.y + (float64(sy)+0.5)/coverageSamples*(e.ap.max.y-e.ap.min.y)
				c := src.at(int(px), int(py))
				if mono {
					total += luma(c)
				} else {
					total += c[channel] * weight
				}
			}
		}
		sim.drive[i] = total / (coverageSamples * coverageSamples)
		sim.channel[i] = channel
		sim.panelLuma[i] = sim.drive[i] * luma(e.filter)
	}
	sim.left = reconstruct(p, sim, -0.5)
	sim.right = reconstruct(p, sim, 0.5)
	return sim
}

// reconstruct evaluates the continuous emitter field at every logical pixel
// center (shifted for the eye) using an analytic triangle-kernel overlap.
// The dominant term is the emitter's own filter color; light spreading through
// the local PSF onto neighboring subpixels also leaks into other channels at
// the leakage fraction, modeling real optical cross-talk without global blur.
// Each sample is divided by the full-weight mass it would receive from an
// all-on field (a local partition of unity), so uniform white reconstructs to
// 1 while genuine subpixel phase variation is retained, including the
// aperiodic mass ripple of a PenTile panel.
func reconstruct(p *Panel, sim *Simulation, eyeShiftX float64) EyeImage {
	img := EyeImage{px: make([][3]float64, p.w*p.h)}
	reach := int(p.psfX) + 2
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			center := vec2{float64(x) + 0.5 + eyeShiftX, float64(y) + 0.5}
			var acc, mass [3]float64
			for _, idx := range p.candidates(x, y, reach) {
				e := &p.emitters[idx]
				wgt := overlapWeight(center, p.psfX, p.psfY, e.ap)
				if wgt == 0 {
					continue
				}
				if isMono(p) {
					acc[0] += sim.drive[idx] * wgt
					acc[1] += sim.drive[idx] * wgt
					acc[2] += sim.drive[idx] * wgt
					mass[0] += wgt
					mass[1] += wgt
					mass[2] += wgt
					continue
				}
				for k := 0; k < 3; k++ {
					if e.filter[k] > 0 {
						acc[k] += sim.drive[idx] * wgt
						mass[k] += wgt
					} else {
						acc[k] += sim.drive[idx] * wgt * leakage
						mass[k] += wgt * leakage
					}
				}
			}
			for k := 0; k < 3; k++ {
				if mass[k] > 1e-9 {
					acc[k] /= mass[k]
				}
			}
			img.px[y*p.w+x] = acc
		}
	}
	return img
}
