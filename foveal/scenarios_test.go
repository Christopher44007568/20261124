package foveal

import "testing"

// TestObjectMotionAndDisocclusion: a moving foreground object invalidates its
// boundary and the revealed background strip, while the uniform object
// interior and the untouched background keep their history.
func TestObjectMotionAndDisocclusion(t *testing.T) {
	scene := NewScene(testW, testH)
	scene.Gaze = Vec2{float32(testW) * 0.5, float32(testH) * 0.5}
	scene.Object.X = float32(testW) * 0.4
	scene.Object.Y = float32(testH) * 0.35
	scene.Object.PrevX = scene.Object.X
	scene.Object.PrevY = scene.Object.Y
	scene.Object.SpeedX = 2.5
	d := newTestDenoiser(t)

	// Converge on the initial layout.
	for i := 0; i < 24; i++ {
		scene.Object.SpeedX = 0
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
	}

	scene.Object.SpeedX = 2.5
	scene.Step()
	f, _, invalid := scene.Render()
	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}

	var disocclusionResets, motionResets, invalidCount int
	for p := range f.RGB[:f.Width*f.Height] {
		if invalid[p] {
			invalidCount++
			if res.Reset[p]&ReasonDisocclusion == 0 {
				t.Fatalf("disocclusion at pixel %d not invalidated", p)
			}
			disocclusionResets++
		}
		if res.Reset[p]&ReasonMotion != 0 {
			motionResets++
		}
	}
	if invalidCount == 0 {
		t.Fatal("scene produced no disoccluded pixels")
	}
	if motionResets == 0 {
		t.Fatal("moving object boundary produced no motion invalidation")
	}

	// Far-away static background must keep converged history.
	corner := 0
	if d.state[corner].age <= 1 || res.Reset[corner] != ReasonNone {
		t.Fatal("untouched static corner lost its history")
	}
}

// TestDynamicHighlight: an appearing highlight invalidates its footprint so no
// ghost of the pre-highlight frame remains; when it disappears the region
// re-converges instead of clamping to a stale bright history.
func TestDynamicHighlight(t *testing.T) {
	scene := staticScene()
	spotCenter := Vec2{float32(testW) * 0.8, float32(testH) * 0.25}
	scene.Spot.X, scene.Spot.Y = spotCenter.X, spotCenter.Y
	d := newTestDenoiser(t)

	for i := 0; i < 24; i++ {
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
		scene.Step()
	}

	scene.Spot.Enabled = true
	f, _, _ := scene.Render()
	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}

	var highlightResets int
	for y := 0; y < testH; y++ {
		for x := 0; x < testW; x++ {
			dx := float32(x) - spotCenter.X
			dy := float32(y) - spotCenter.Y
			if dx*dx+dy*dy <= scene.Spot.Radius*scene.Spot.Radius {
				p := y*testW + x
				if luminance([3]float32{
					f.RGB[p*3], f.RGB[p*3+1], f.RGB[p*3+2],
				}) > d.Params().HighlightThreshold && res.Reset[p]&ReasonHighlight == 0 {
					t.Fatalf("highlight pixel (%d,%d) kept stale history", x, y)
				}
				if res.Reset[p]&ReasonHighlight != 0 {
					highlightResets++
				}
			}
		}
	}
	if highlightResets == 0 {
		t.Fatal("dynamic highlight triggered no invalidation")
	}

	// Remove the spot and confirm the region returns to normal radiance.
	scene.Spot.Enabled = false
	for i := 0; i < 12; i++ {
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
		scene.Step()
	}
	f, clean, _ := scene.Render()
	res, _ = d.Process(f)
	mse, _ := regionMSE(t, res, clean, ZonePeriphery)
	if mse > float64(scene.NoiseSigma*scene.NoiseSigma) {
		t.Fatalf("highlight ghost remained after removal: mse=%v", mse)
	}
}

// TestExposureCompensationPreservesHistory: a moderate exposure step is
// compensated without any reset, while an extreme step forces a full reset.
func TestExposureCompensationPreservesHistory(t *testing.T) {
	scene := staticScene()
	d := newTestDenoiser(t)
	for i := 0; i < 24; i++ {
		if _, err := d.Process(mustRender(scene)); err != nil {
			t.Fatal(err)
		}
		scene.Step()
	}

	// Moderate 1.5x exposure change: ages must survive everywhere.
	scene.SetExposure(1.5)
	f, _, _ := scene.Render()
	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}
	for p := range d.state {
		if res.Reset[p]&ReasonExposure != 0 {
			t.Fatal("moderate exposure change triggered a reset")
		}
		if d.state[p].age <= 1 {
			t.Fatalf("pixel %d lost history during exposure compensation", p)
		}
	}

	// Extreme 8x jump: every pixel resets.
	scene.SetExposure(12)
	f, _, _ = scene.Render()
	res, err = d.Process(f)
	if err != nil {
		t.Fatal(err)
	}
	for p := range d.state {
		if res.Reset[p]&ReasonExposure == 0 {
			t.Fatalf("pixel %d not reset by extreme exposure", p)
		}
	}
}

// TestGlobalMotionNoFalseReset: a uniform head-turn motion vector field has
// zero divergence; after reprojection all static pixels keep converging.
func TestGlobalMotionNoFalseReset(t *testing.T) {
	scene := staticScene()
	d := newTestDenoiser(t)

	// Build a frame with a uniform global motion vector and a shifted but
	// otherwise identical background by wrapping coordinates manually.
	prime, _, _ := scene.Render()
	for i := 0; i < 24; i++ {
		if _, err := d.Process(prime); err != nil {
			t.Fatal(err)
		}
	}

	shift := Vec2{3, 0}
	f, _, _ := scene.Render()
	w, h := f.Width, f.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := y*w + x
			f.MV[p] = shift
			sx := x - int(shift.X)
			if sx < 0 {
				f.MV[p] = Vec2{} // newly entering border pixel
				continue
			}
			sp := y*w + sx
			for c := 0; c < 3; c++ {
				f.RGB[p*3+c] = prime.RGB[sp*3+c]
			}
		}
	}

	res, err := d.Process(f)
	if err != nil {
		t.Fatal(err)
	}
	var interiorKept int
	for y := 1; y < h-1; y++ {
		for x := 4; x < w-1; x++ {
			p := y*w + x
			if res.Reset[p] == ReasonNone && d.state[p].age > 1 {
				interiorKept++
			}
		}
	}
	if interiorKept == 0 {
		t.Fatal("uniform head-turn motion wrongly invalidated static history")
	}
}
