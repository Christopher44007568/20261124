package main

import "testing"

func TestWhiteNormalization(t *testing.T) {
	w, h := 72, 40
	src := newSource(w, h)
	for i := range src.pixels {
		src.pixels[i] = [3]float64{1, 1, 1}
	}
	for _, p := range []*Panel{stripePanel(w, h), pentilePanel(w, h), monoPanel(w, h)} {
		sim := runSimulation(p, src)
		var sum [3]float64
		n := 0
		var maxV float64
		for y := 3; y < h-3; y++ {
			for x := 3; x < w-3; x++ {
				c := sim.left.px[y*w+x]
				for k := range sum {
					sum[k] += c[k]
				}
				for _, v := range c {
					if v > maxV {
						maxV = v
					}
				}
				n++
			}
		}
		for k := range sum {
			mean := sum[k] / float64(n)
			if d := mean - 1.0; d > 0.02 || d < -0.02 {
				t.Errorf("%s ch%d white mean=%.4f want~1", p.name, k, mean)
			}
		}
		t.Logf("%s interior white mean=(%.4f,%.4f,%.4f) max=%.4f",
			p.name, sum[0]/float64(n), sum[1]/float64(n), sum[2]/float64(n), maxV)
	}
}
