package panelsampling

import "testing"

func TestClampPreservesThinEdgeAndCornerHighlight(t *testing.T) {
	texture := scenarioTexture(t)
	sampler, err := NewSampler(PolicyClamp, NewOOBLog())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		source    Coord
		wantColor Color
		wantDepth float32
	}{
		{
			name:      "thin left line",
			source:    Coord{X: -0.25, Y: 0.4},
			wantColor: Color{R: 1, G: 1, B: 1, A: 1},
			wantDepth: 0.2,
		},
		{
			name:      "top right corner highlight",
			source:    Coord{X: 1.25, Y: -0.5},
			wantColor: Color{R: 1, G: 1, A: 1},
			wantDepth: 0.1,
		},
		{
			name:      "single texel left line",
			source:    Coord{X: -0.001, Y: -0.5},
			wantColor: Color{R: 1, G: 1, B: 1, A: 1},
			wantDepth: 0.2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sample, err := sampler.Sample(texture, SampleRequest{
				Eye: EyeLeft, Stage: StageDistortion, Channel: ChannelRGB, UV: test.source,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertColorNear(t, sample.Color, test.wantColor)
			if !sample.Valid || !sample.HasDepth {
				t.Fatalf("sample Valid=%v HasDepth=%v, want true/true", sample.Valid, sample.HasDepth)
			}
			if !near(float64(sample.Depth), float64(test.wantDepth), 1e-6) {
				t.Fatalf("Depth = %v, want %v", sample.Depth, test.wantDepth)
			}
		})
	}
}

func TestTransparentUIDoesNotInventDepth(t *testing.T) {
	texture := scenarioTexture(t)
	depth, err := NewDepthBuffer(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	for _, policy := range []Policy{PolicyClamp, PolicyRepeat, PolicyTransparentBlack} {
		t.Run(policy.String(), func(t *testing.T) {
			sampler, err := NewSampler(policy, NewOOBLog())
			if err != nil {
				t.Fatal(err)
			}
			ui, err := sampler.Sample(texture, SampleRequest{
				Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelRGB,
				UV: Coord{X: 0.5, Y: 0.75},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !ui.Valid {
				t.Fatal("in-panel transparent UI should be a valid color sample")
			}
			if ui.HasDepth {
				t.Fatal("transparent UI sample has depth")
			}
			if ui.Color.A != 0 {
				t.Fatalf("transparent UI alpha = %v, want 0", ui.Color.A)
			}
			if depth.Store(0, 0, ui) {
				t.Fatal("depth buffer accepted a transparent UI sample")
			}
			if _, ok := depth.Lookup(0, 0); ok {
				t.Fatal("transparent UI left a false depth")
			}

			straddle, err := sampler.Sample(texture, SampleRequest{
				Eye: EyeLeft, Stage: StageChromaticAberration, Channel: ChannelG,
				UV: Coord{X: 0.3, Y: 0.67},
			})
			if err != nil {
				t.Fatal(err)
			}
			if straddle.HasDepth {
				t.Fatal("mixed opaque/transparent footprint has depth")
			}
		})
	}

	transparent, err := NewSampler(PolicyTransparentBlack, NewOOBLog())
	if err != nil {
		t.Fatal(err)
	}
	outside, err := transparent.Sample(texture, SampleRequest{
		Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelB,
		UV: Coord{X: -0.2, Y: 0.75},
	})
	if err != nil {
		t.Fatal(err)
	}
	if outside.Valid || outside.HasDepth {
		t.Fatalf("transparent-black fallback Valid=%v HasDepth=%v, want false/false",
			outside.Valid, outside.HasDepth)
	}
	if depth.Store(0, 0, outside) {
		t.Fatal("depth buffer accepted transparent-black fallback")
	}
	if _, ok := depth.Lookup(0, 0); ok {
		t.Fatal("transparent-black fallback left a false depth")
	}
}

func TestInvalidSamplesCannotOverwriteValidDepth(t *testing.T) {
	texture := solidTexture(t, EyeLeft, 1, 1, Texel{
		Color:    Color{R: 1, A: 1},
		Depth:    0.75,
		HasDepth: true,
	})
	sampler, err := NewSampler(PolicyTransparentBlack, NewOOBLog())
	if err != nil {
		t.Fatal(err)
	}
	valid, err := sampler.Sample(texture, SampleRequest{
		Eye: EyeLeft, Stage: StageDirect, Channel: ChannelRGB, UV: Coord{X: 0.5, Y: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}

	depth, err := NewDepthBuffer(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !depth.Store(0, 0, valid) {
		t.Fatal("valid depth was not stored")
	}

	invalidSamples := []Sample{
		{Depth: 0.01, HasDepth: true, Valid: false},
		{Color: Transparent, Valid: true, HasDepth: false},
	}
	fallback, err := sampler.Sample(texture, SampleRequest{
		Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelRGB, UV: Coord{X: 1.01, Y: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	invalidSamples = append(invalidSamples, fallback)

	for index, sample := range invalidSamples {
		if depth.Store(0, 0, sample) {
			t.Fatalf("invalid sample %d was stored", index)
		}
		stored, ok := depth.Lookup(0, 0)
		if !ok || !near(float64(stored), 0.75, 1e-6) {
			t.Fatalf("invalid sample %d changed depth to %v (valid=%v)", index, stored, ok)
		}
	}
}

func scenarioTexture(t *testing.T) *Texture {
	t.Helper()
	const width, height = 4, 4
	texels := make([]Texel, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			texel := Texel{
				Color:    Color{R: 0.05, G: 0.05, B: 0.08, A: 1},
				Depth:    0.9,
				HasDepth: true,
			}
			switch {
			case x == 0:
				texel.Color = Color{R: 1, G: 1, B: 1, A: 1}
				texel.Depth = 0.2
			case x >= 1 && x <= 2 && y >= 2 && y <= 3:
				texel = Texel{Color: Transparent}
			case x == 3 && y == 0:
				texel.Color = Color{R: 1, G: 1, A: 1}
				texel.Depth = 0.1
			}
			texels[y*width+x] = texel
		}
	}
	texture, err := NewTexture(EyeLeft, width, height, texels)
	if err != nil {
		t.Fatal(err)
	}
	return texture
}
