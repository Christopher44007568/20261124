package panelsampling

import (
	"errors"
	"math"
	"testing"
)

func TestBoundaryPolicies(t *testing.T) {
	left := boundaryFixture(t, EyeLeft)
	right := solidTexture(t, EyeRight, 3, 2, Texel{
		Color:    Color{B: 1, A: 1},
		Depth:    0.1,
		HasDepth: true,
	})

	tests := []struct {
		name           string
		policy         Policy
		wantColor      Color
		wantDepth      float32
		wantValid      bool
		wantDepthValid bool
		wantResolved   Coord
	}{
		{
			name:           "clamped uses near edge",
			policy:         PolicyClamp,
			wantColor:      Color{R: 1, G: 1, B: 1, A: 1},
			wantDepth:      0.25,
			wantValid:      true,
			wantDepthValid: true,
			wantResolved:   Coord{X: 0, Y: 0},
		},
		{
			name:           "repeat wraps within same eye",
			policy:         PolicyRepeat,
			wantColor:      Color{R: 0.8, A: 1},
			wantDepth:      0.55,
			wantValid:      true,
			wantDepthValid: true,
			wantResolved:   Coord{X: 0.9, Y: 0},
		},
		{
			name:           "transparent black has no depth",
			policy:         PolicyTransparentBlack,
			wantColor:      Transparent,
			wantValid:      false,
			wantDepthValid: false,
			wantResolved:   Coord{X: -0.1, Y: 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := NewOOBLog()
			sampler, err := NewSampler(test.policy, log)
			if err != nil {
				t.Fatal(err)
			}
			sample, err := sampler.Sample(left, SampleRequest{
				Eye:     EyeLeft,
				Stage:   StageDistortion,
				Channel: ChannelRGB,
				UV:      Coord{X: -0.1, Y: 0},
			})
			if err != nil {
				t.Fatal(err)
			}

			assertColorNear(t, sample.Color, test.wantColor)
			if sample.Valid != test.wantValid {
				t.Fatalf("Valid = %v, want %v", sample.Valid, test.wantValid)
			}
			if sample.HasDepth != test.wantDepthValid {
				t.Fatalf("HasDepth = %v, want %v", sample.HasDepth, test.wantDepthValid)
			}
			if test.wantDepthValid && !near(float64(sample.Depth), float64(test.wantDepth), 1e-6) {
				t.Fatalf("Depth = %v, want %v", sample.Depth, test.wantDepth)
			}
			if !near(sample.Resolved.X, test.wantResolved.X, 1e-12) ||
				!near(sample.Resolved.Y, test.wantResolved.Y, 1e-12) {
				t.Fatalf("Resolved = %+v, want %+v", sample.Resolved, test.wantResolved)
			}
			if !sample.OutOfBounds {
				t.Fatal("OutOfBounds = false, want true")
			}

			records := log.Snapshot()
			if len(records) != 1 {
				t.Fatalf("got %d records, want 1", len(records))
			}
			record := records[0]
			if record.Sequence != 1 ||
				record.Policy != test.policy ||
				record.Eye != EyeLeft ||
				record.Stage != StageDistortion ||
				record.Channel != ChannelRGB ||
				record.Source != (Coord{X: -0.1, Y: 0}) {
				t.Fatalf("unexpected record: %+v", record)
			}
			if record.Recovered != test.wantValid {
				t.Fatalf("record Recovered = %v, want %v", record.Recovered, test.wantValid)
			}
			if record.DepthValid != test.wantDepthValid {
				t.Fatalf("record DepthValid = %v, want %v", record.DepthValid, test.wantDepthValid)
			}
		})
	}

	log := NewOOBLog()
	repeat, err := NewSampler(PolicyRepeat, log)
	if err != nil {
		t.Fatal(err)
	}
	leftSample, err := repeat.Sample(left, SampleRequest{
		Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelR, UV: Coord{X: 1.1, Y: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	if near(leftSample.Color.B, 1, 1e-12) {
		t.Fatal("repeat escaped into the right eye texture")
	}

	_, err = repeat.Sample(left, SampleRequest{
		Eye: EyeRight, Stage: StageReprojection, Channel: ChannelR, UV: Coord{X: 1.1, Y: 0},
	})
	if !errors.Is(err, ErrEyeMismatch) {
		t.Fatalf("cross-eye request error = %v, want ErrEyeMismatch", err)
	}
	if log.Len() != 1 {
		t.Fatalf("cross-eye error must not record a sample; got %d records", log.Len())
	}

	_, err = repeat.Sample(right, SampleRequest{
		Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelR, UV: Coord{X: -0.1, Y: 0},
	})
	if !errors.Is(err, ErrEyeMismatch) {
		t.Fatalf("cross-eye request error = %v, want ErrEyeMismatch", err)
	}
}

func TestEveryOutOfBoundsCoordinateIsRecorded(t *testing.T) {
	texture := solidTexture(t, EyeLeft, 2, 2, Texel{
		Color:    Color{R: 1, A: 1},
		Depth:    0.5,
		HasDepth: true,
	})
	log := NewOOBLog()
	sampler, err := NewSampler(PolicyClamp, log)
	if err != nil {
		t.Fatal(err)
	}

	requests := []SampleRequest{
		{Eye: EyeLeft, Stage: StageDistortion, Channel: ChannelRGB, UV: Coord{X: -0.125, Y: 0.5}},
		{Eye: EyeLeft, Stage: StageChromaticAberration, Channel: ChannelR, UV: Coord{X: 0.25, Y: -0.125}},
		{Eye: EyeLeft, Stage: StageChromaticAberration, Channel: ChannelG, UV: Coord{X: 0.5, Y: 1.125}},
		{Eye: EyeLeft, Stage: StageChromaticAberration, Channel: ChannelB, UV: Coord{X: 1.125, Y: 0.5}},
		{Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelRGB, UV: Coord{X: -0.5, Y: -0.25}},
		{Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelRGB, UV: Coord{X: -0.5, Y: -0.25}},
		{Eye: EyeLeft, Stage: StageReprojection, Channel: ChannelRGB, UV: Coord{X: math.Inf(1), Y: 0.5}},
	}
	for _, request := range requests {
		if _, err := sampler.Sample(texture, request); err != nil {
			t.Fatal(err)
		}
	}

	records := log.Snapshot()
	if len(records) != len(requests) {
		t.Fatalf("got %d records, want %d", len(records), len(requests))
	}
	for index, record := range records {
		want := requests[index]
		if record.Sequence != uint64(index+1) {
			t.Fatalf("record %d sequence = %d", index, record.Sequence)
		}
		if record.Source != want.UV ||
			record.Eye != want.Eye ||
			record.Stage != want.Stage ||
			record.Channel != want.Channel ||
			record.Policy != PolicyClamp {
			t.Fatalf("record %d = %+v, want source=%+v stage=%s channel=%s",
				index, record, want.UV, want.Stage, want.Channel)
		}
	}
	if !records[4].Recovered || !records[5].Recovered {
		t.Fatal("duplicate finite coordinates should both be recovered and recorded")
	}
	if records[6].Recovered {
		t.Fatal("non-finite coordinate reported recovered")
	}

	invalid, err := sampler.Sample(texture, requests[6])
	if err != nil {
		t.Fatal(err)
	}
	if invalid.Valid || invalid.HasDepth {
		t.Fatalf("non-finite sample Valid=%v HasDepth=%v, want false/false", invalid.Valid, invalid.HasDepth)
	}
}

func TestRepeatNeverCrossesEyeTextures(t *testing.T) {
	left := solidTexture(t, EyeLeft, 4, 1, Texel{
		Color:    Color{R: 1, A: 1},
		Depth:    0.2,
		HasDepth: true,
	})
	right := solidTexture(t, EyeRight, 4, 1, Texel{
		Color:    Color{B: 1, A: 1},
		Depth:    0.3,
		HasDepth: true,
	})
	log := NewOOBLog()
	sampler, err := NewSampler(PolicyRepeat, log)
	if err != nil {
		t.Fatal(err)
	}

	leftSample, err := sampler.Sample(left, SampleRequest{
		Eye: EyeLeft, Stage: StageDistortion, Channel: ChannelRGB, UV: Coord{X: -0.1, Y: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertColorNear(t, leftSample.Color, Color{R: 1, A: 1})

	rightSample, err := sampler.Sample(right, SampleRequest{
		Eye: EyeRight, Stage: StageReprojection, Channel: ChannelRGB, UV: Coord{X: 1.1, Y: 0.5},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertColorNear(t, rightSample.Color, Color{B: 1, A: 1})

	records := log.Snapshot()
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].Eye != EyeLeft || records[1].Eye != EyeRight {
		t.Fatalf("record eyes = %s/%s, want left/right", records[0].Eye, records[1].Eye)
	}
}

func boundaryFixture(t *testing.T, eye Eye) *Texture {
	t.Helper()
	texture, err := NewTexture(eye, 3, 2, []Texel{
		{Color: Color{R: 1, G: 1, B: 1, A: 1}, Depth: 0.25, HasDepth: true},
		{Color: Color{A: 1}, Depth: 0.75, HasDepth: true},
		{Color: Color{R: 1, A: 1}, Depth: 0.5, HasDepth: true},
		{Color: Color{R: 1, G: 1, B: 1, A: 1}, Depth: 0.25, HasDepth: true},
		{Color: Transparent},
		{Color: Color{R: 1, A: 1}, Depth: 0.5, HasDepth: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return texture
}

func solidTexture(t *testing.T, eye Eye, width, height int, texel Texel) *Texture {
	t.Helper()
	texels := make([]Texel, width*height)
	for index := range texels {
		texels[index] = texel
	}
	texture, err := NewTexture(eye, width, height, texels)
	if err != nil {
		t.Fatal(err)
	}
	return texture
}

func assertColorNear(t *testing.T, got, want Color) {
	t.Helper()
	components := []struct {
		name string
		got  float64
		want float64
	}{
		{"R", got.R, want.R},
		{"G", got.G, want.G},
		{"B", got.B, want.B},
		{"A", got.A, want.A},
	}
	for _, component := range components {
		if !near(component.got, component.want, 1e-12) {
			t.Fatalf("Color.%s = %v, want %v (got %+v, want %+v)",
				component.name, component.got, component.want, got, want)
		}
	}
}

func near(got, want, tolerance float64) bool {
	if math.IsNaN(got) || math.IsNaN(want) {
		return false
	}
	return math.Abs(got-want) <= tolerance
}
