package foveal

import (
	"math"
	"testing"
)

const testW, testH = 64, 48

func newTestDenoiser(t *testing.T) *Denoiser {
	t.Helper()
	d, err := New(DefaultParams(testW, testH))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func staticScene() *Scene {
	s := NewScene(testW, testH)
	s.Gaze = Vec2{float32(testW) * 0.3, float32(testH) * 0.5}
	s.Object.SpeedX = 0
	s.Object.X = -float32(s.Object.Size) - 2 // parked offscreen
	return s
}

// regionMSE computes mean squared error against clean within one zone.
func regionMSE(t *testing.T, res *Result, clean []float32, zone Zone) (mse float64, count int) {
	t.Helper()
	for p, z := range res.Zones {
		if z != zone {
			continue
		}
		for c := 0; c < 3; c++ {
			diff := float64(res.RGB[p*3+c] - clean[p*3+c])
			mse += diff * diff
		}
		count++
	}
	if count == 0 {
		t.Fatalf("no pixels in zone %v", zone)
	}
	return mse / float64(count*3), count
}

func rawMSE(scene *Scene, f *Frame, clean []float32, zone Zone, zones []Zone) float64 {
	var sum float64
	n := 0
	for p, z := range zones {
		if z != zone {
			continue
		}
		for c := 0; c < 3; c++ {
			diff := float64(f.RGB[p*3+c] - clean[p*3+c])
			sum += diff * diff
		}
		n++
	}
	return sum / float64(max(1, n*3))
}

// TestStaticConvergence: on a still scene both zones converge, and the fovea
// with its long history ends up much cleaner than the short-history periphery.
func TestStaticConvergence(t *testing.T) {
	scene := staticScene()
	d := newTestDenoiser(t)

	var firstRawFov, lastFov, lastPeriph float64
	var finalRes *Result
	for i := 0; i < 48; i++ {
		f, clean, _ := scene.Render()
		res, err := d.Process(f)
		if err != nil {
			t.Fatalf("Process: %v", err)
		}
		if i == 0 {
			firstRawFov = rawMSE(scene, f, clean, ZoneFovea, res.Zones)
		}
		if i == 47 {
			lastFov, _ = regionMSE(t, res, clean, ZoneFovea)
			lastPeriph, _ = regionMSE(t, res, clean, ZonePeriphery)
			finalRes = res
		}
		scene.Step()
	}

	if !(lastFov < firstRawFov/4) {
		t.Fatalf("fovea did not converge: raw=%v final=%v", firstRawFov, lastFov)
	}
	if !(lastFov < lastPeriph/2) {
		t.Fatalf("long foveal history not cleaner than periphery: fov=%v periph=%v",
			lastFov, lastPeriph)
	}

	// Ages must be capped at the two configured history lengths.
	cfg := d.Params()
	for p, st := range d.state {
		limit := float32(cfg.FovealHistory)
		if finalRes.Zones[p] == ZonePeriphery {
			limit = float32(cfg.PeripheralHistory)
		}
		if st.age > limit+1e-6 {
			t.Fatalf("pixel %d age %v exceeds zone cap %v", p, st.age, limit)
		}
	}
}

// TestPeripheryDoesNotSmearFovea: aggressive peripheral accumulation must
// never leak into foveal pixels, even when the gaze disk crosses regions that
// were previously peripheral.
func TestPeripheryDoesNotSmearFovea(t *testing.T) {
	scene := staticScene()
	d := newTestDenoiser(t)

	// Prime the periphery for many frames while gaze sits on the left.
	for i := 0; i < 24; i++ {
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
		scene.Step()
	}

	// Move the gaze slowly (below saccade threshold) so the new fovea region
	// was peripheral history one frame earlier.
	scene.Gaze = Vec2{float32(testW) * 0.65, float32(testH) * 0.5}
	f, clean, _ := scene.Render()
	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}

	// Every foveal pixel must either start fresh or be reconstructed only from
	// foveal-zone taps; the output must stay close to the current clean sample
	// instead of inheriting peripheral state.
	mse, _ := regionMSE(t, res, clean, ZoneFovea)
	if mse > float64(scene.NoiseSigma*scene.NoiseSigma)*1.05 {
		t.Fatalf("fovea contaminated by peripheral history: mse=%v", mse)
	}
}

func mustRender(s *Scene) *Frame {
	f, _, _ := s.Render()
	return f
}

// TestSaccadeInvalidationRange: a fast gaze jump resets the corridor and the
// landing disk, while untouched regions keep their converged history.
func TestSaccadeInvalidationRange(t *testing.T) {
	scene := staticScene()
	d := newTestDenoiser(t)
	for i := 0; i < 24; i++ {
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
		scene.Step()
	}

	from := scene.Gaze
	scene.Gaze = Vec2{float32(testW) * 0.85, float32(testH) * 0.15}
	f, _, _ := scene.Render()
	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}

	corr2 := d.Params().SaccadeCorridor * d.Params().SaccadeCorridor
	landingR := d.Params().FoveaRadius + d.Params().GuardWidth
	var corridorHits, farKept int
	for y := 0; y < testH; y++ {
		for x := 0; x < testW; x++ {
			p := y*testW + x
			px := Vec2{float32(x) + 0.5, float32(y) + 0.5}
			inCorridor := pointSegmentDist2(px, from, scene.Gaze) <= corr2
			dx, dy := px.X-scene.Gaze.X, px.Y-scene.Gaze.Y
			inLanding := dx*dx+dy*dy <= landingR*landingR
			if inCorridor || inLanding {
				if res.Reset[p]&ReasonEye == 0 {
					t.Fatalf("pixel (%d,%d) inside saccade range not reset", x, y)
				}
				corridorHits++
			} else if d.state[p].age > 1 && res.Reset[p] == ReasonNone {
				farKept++
			}
		}
	}
	if corridorHits == 0 {
		t.Fatal("saccade range was empty")
	}
	if farKept == 0 {
		t.Fatal("history outside the saccade range was wrongly discarded")
	}
}

func approxEqual(a, b float32) bool { return math.Abs(float64(a-b)) < 1e-6 }
