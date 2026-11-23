package chroma

import (
	"math"
	"testing"
)

func TestCenterHasZeroAberration(t *testing.T) {
	cfg := Config{
		Aberration: Aberration{Red: 0.08, Green: -0.03, Blue: -0.11},
		Calibration: Calibration{
			Left: EyeCalibration{
				CenterOffset: Point{X: 0.12, Y: -0.07},
				RadialBias:   Aberration{Red: 0.02, Green: 0.01, Blue: -0.015},
			},
		},
	}
	center := cfg.OpticalCenter()
	for ch := Red; ch <= Blue; ch++ {
		if got := RadialScale(cfg, center, ch); got != 1 {
			t.Fatalf("%s scale at optical center = %v, want 1", ch, got)
		}
		if got := MapPoint(cfg, center, ch); got != center {
			t.Fatalf("%s mapped optical center = %+v, want %+v", ch, got, center)
		}
	}
}

func TestCornersCarryMaximumAberration(t *testing.T) {
	cfg := Config{Aberration: Aberration{Red: 0.07, Green: -0.02, Blue: -0.09}}
	corners := [...]Point{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}, {X: 1, Y: 1}}

	for ch := Red; ch <= Blue; ch++ {
		wantScale := 1 + cfg.EffectiveAberration().For(ch)
		for _, corner := range corners {
			if got := RadialScale(cfg, corner, ch); math.Abs(got-wantScale) > 1e-12 {
				t.Fatalf("%s scale at corner %+v = %v, want %v", ch, corner, got, wantScale)
			}
		}

		cornerDisplacement := math.Abs(RadialScale(cfg, Point{X: 1, Y: 1}, ch) - 1)
		for _, p := range []Point{
			{X: 0, Y: 0},
			{X: 0.5, Y: 0},
			{X: 0.5, Y: 0.5},
			{X: 0.8, Y: -0.4},
			{X: 0, Y: 1},
		} {
			if displacement := math.Abs(RadialScale(cfg, p, ch) - 1); displacement > cornerDisplacement+1e-12 {
				t.Fatalf("%s displacement at %+v (%v) exceeds corner (%v)", ch, p, displacement, cornerDisplacement)
			}
		}
	}
}

func TestMonocularCalibrationIsIsolated(t *testing.T) {
	cfg := Config{
		Aberration: Aberration{Red: 0.01, Green: 0.02, Blue: 0.03},
		Eye:        LeftEye,
		Calibration: Calibration{
			Left: EyeCalibration{
				CenterOffset: Point{X: 0.2, Y: -0.1},
				RadialBias:   Aberration{Red: 0.04, Green: 0.05, Blue: 0.06},
			},
			Right: EyeCalibration{
				CenterOffset: Point{X: -0.15, Y: 0.08},
				RadialBias:   Aberration{Red: -0.04, Green: -0.05, Blue: -0.06},
			},
		},
	}

	left := cfg.EffectiveAberration()
	if !aberrationClose(left, Aberration{Red: 0.05, Green: 0.07, Blue: 0.09}) {
		t.Fatalf("left effective aberration = %+v", left)
	}
	if got := cfg.OpticalCenter(); got != (Point{X: 0.2, Y: -0.1}) {
		t.Fatalf("left optical center = %+v", got)
	}

	right := cfg
	right.Eye = RightEye
	if got := right.EffectiveAberration(); !aberrationClose(got, Aberration{Red: -0.03, Green: -0.03, Blue: -0.03}) {
		t.Fatalf("right effective aberration = %+v", got)
	}
	if got := right.OpticalCenter(); got != (Point{X: -0.15, Y: 0.08}) {
		t.Fatalf("right optical center = %+v", got)
	}

	probe := Point{X: 0.8, Y: 0.7}
	if MapPoint(cfg, probe, Red) == MapPoint(right, probe, Red) {
		t.Fatal("left/right eye mappings unexpectedly identical")
	}
}

func aberrationClose(a, b Aberration) bool {
	return math.Abs(a.Red-b.Red) < 1e-12 &&
		math.Abs(a.Green-b.Green) < 1e-12 &&
		math.Abs(a.Blue-b.Blue) < 1e-12
}

func TestConfigValidationRejectsNonFiniteValues(t *testing.T) {
	if err := (Config{Aberration: Aberration{Red: math.NaN()}}).Validate(); err == nil {
		t.Fatal("expected NaN aberration to be rejected")
	}
	if err := (Config{Calibration: Calibration{
		Left: EyeCalibration{CenterOffset: Point{X: math.Inf(1)}},
	}}).Validate(); err == nil {
		t.Fatal("expected infinite calibration offset to be rejected")
	}
}

func TestMapperPixelCenterIsExactForOddSize(t *testing.T) {
	cfg := Config{Aberration: Aberration{Red: 0.2, Green: -0.1, Blue: -0.3}}
	mapper, err := NewMapper(101, 101, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for ch := Red; ch <= Blue; ch++ {
		if got := mapper.MapPixel(50, 50, ch); got != (Point{X: 50, Y: 50}) {
			t.Fatalf("%s center pixel maps to %+v, want (50,50)", ch, got)
		}
	}
}
