package main

import "testing"

func panelsUnderTest() []*Panel {
	return []*Panel{
		stripePanel(width, height),
		pentilePanel(width, height),
		monoPanel(width, height),
	}
}

func TestStructuralLayouts(t *testing.T) {
	for _, p := range panelsUnderTest() {
		for _, e := range p.emitters {
			if e.filter == whiteFilter {
				continue // mono white emitter is a single physical emitter
			}
			active := 0
			for _, f := range e.filter {
				if f > 0 {
					active++
				}
			}
			if active != 1 {
				t.Fatalf("%s: emitter with %d active channels, want exactly 1", p.name, active)
			}
		}
	}

	stripe := stripePanel(width, height)
	if got := len(stripe.emitters); got != 3*width*height {
		t.Errorf("stripe emitters=%d want %d", got, 3*width*height)
	}

	pentile := pentilePanel(width, height)
	d := filterDensity(pentile)
	if !approxEq(d[1], 0.5) || !approxEq(d[0], 0.25) || !approxEq(d[2], 0.25) {
		t.Errorf("pentile density R=%.3f G=%.3f B=%.3f want .25/.5/.25", d[0], d[1], d[2])
	}
	if got := float64(len(pentile.emitters)) / float64(width*height); !approxEq(got, 2.0) {
		t.Errorf("pentile emitters/pixel=%.3f want 2", got)
	}

	mono := monoPanel(width, height)
	if len(mono.emitters) != width*height {
		t.Errorf("mono emitters=%d want %d", len(mono.emitters), width*height)
	}
}

func TestRedLineCrosstalkAndPeak(t *testing.T) {
	src := redLineScene(width, height)
	for _, p := range panelsUnderTest() {
		sim := runSimulation(p, src)
		m := measure(p, sim, "redline", src)
		if isMono(p) {
			if m.Crosstalk != 0 {
				t.Errorf("%s mono crosstalk=%.4f want 0", p.name, m.Crosstalk)
			}
			continue
		}
		if m.Crosstalk <= 0 || m.Crosstalk >= 1 {
			t.Errorf("%s redline crosstalk=%.4f want in (0,1)", p.name, m.Crosstalk)
		}
		if m.PanelPeakChannel[0] < 0.99 {
			t.Errorf("%s red peak=%.3f want ~1", p.name, m.PanelPeakChannel[0])
		}
		if m.PanelPeakChannel[1] != 0 || m.PanelPeakChannel[2] != 0 {
			t.Errorf("%s non-red panel channels must stay dark at drive level", p.name)
		}
	}
}

func TestBinocularDifference(t *testing.T) {
	for _, sc := range scenes {
		src := sc.make(width, height)
		for _, p := range panelsUnderTest() {
			sim := runSimulation(p, src)
			mad, _ := binocularDiff(sim.left, sim.right)
			if mad <= 0 {
				t.Errorf("%s/%s binocular MAD=%.6f want > 0", sc.name, p.name, mad)
			}
		}
	}
}

// TestEdgePhaseNotBlurred verifies the eye shift moves the perceived edge
// crossing instead of being erased by a symmetric global blur.
func TestEdgePhaseNotBlurred(t *testing.T) {
	src := edgeScene(width, height)
	for _, p := range panelsUnderTest() {
		sim := runSimulation(p, src)
		col := width / 2
		var diff float64
		for y := 0; y < height; y++ {
			a := sim.left.px[y*width+col]
			b := sim.right.px[y*width+col]
			for k := 0; k < 3; k++ {
				d := a[k] - b[k]
				if d < 0 {
					d = -d
				}
				diff += d
			}
		}
		if diff/float64(3*height) <= 0 {
			t.Errorf("%s edge transition identical between eyes; phase was blurred away", p.name)
		}
	}
}

func TestDiagonalHasPartialCoverage(t *testing.T) {
	src := diagonalScene(width, height)
	for _, p := range panelsUnderTest() {
		sim := runSimulation(p, src)
		if n := countPartialEmitters(sim); n == 0 {
			t.Errorf("%s diagonal produced no partial emitters", p.name)
		}
	}
}

func approxEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-6
}
