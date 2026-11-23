package rollingshutter

import (
	"testing"
	"time"
)

func TestRowTimingSeparatesStartDirectionAndPoseSample(t *testing.T) {
	base := RowTimingConfig{
		FirstExposureStart: 2 * time.Millisecond,
		RowPeriod:          10 * time.Microsecond,
		ExposureDuration:   100 * time.Microsecond,
		PoseSampleOffset:   25 * time.Microsecond,
		Direction:          TopToBottom,
	}
	forward, err := NewRowTiming(base)
	if err != nil {
		t.Fatal(err)
	}
	reversedConfig := base
	reversedConfig.Direction = BottomToTop
	reversed, err := NewRowTiming(reversedConfig)
	if err != nil {
		t.Fatal(err)
	}

	forwardTop, _ := forward.Row(0, 480)
	forwardBottom, _ := forward.Row(479, 480)
	reversedTop, _ := reversed.Row(0, 480)
	reversedBottom, _ := reversed.Row(479, 480)

	if forwardTop.ExposureStart != 2*time.Millisecond {
		t.Fatalf("forward top exposure start = %s", forwardTop.ExposureStart)
	}
	if forwardTop.PoseSampleTime != 2025*time.Microsecond {
		t.Fatalf("forward top pose sample = %s", forwardTop.PoseSampleTime)
	}
	if forwardBottom.ExposureStart != 6790*time.Microsecond {
		t.Fatalf("forward bottom exposure start = %s", forwardBottom.ExposureStart)
	}
	if forwardTop.PoseSampleTime == forwardBottom.PoseSampleTime {
		t.Fatal("top and bottom rows must not share a pose sample time")
	}
	if forwardTop.DirectionReversed || !reversedTop.DirectionReversed {
		t.Fatal("reversed direction was not preserved in row windows")
	}
	if reversedTop.ReadoutOrder != 479 || reversedBottom.ReadoutOrder != 0 {
		t.Fatalf("reversed readout order: top=%d bottom=%d",
			reversedTop.ReadoutOrder, reversedBottom.ReadoutOrder)
	}
	if reversedTop.ExposureStart != forwardBottom.ExposureStart {
		t.Fatal("bottom-up readout should expose the physical top row last")
	}
}

func TestRowTimingRejectsInvalidConfiguration(t *testing.T) {
	valid := RowTimingConfig{
		RowPeriod:        time.Microsecond,
		ExposureDuration: time.Millisecond,
		Direction:        TopToBottom,
	}
	cases := []struct {
		name   string
		mutate func(*RowTimingConfig)
	}{
		{"zero row period", func(c *RowTimingConfig) { c.RowPeriod = 0 }},
		{"sample after exposure", func(c *RowTimingConfig) { c.PoseSampleOffset = 2 * time.Millisecond }},
		{"unknown direction", func(c *RowTimingConfig) { c.Direction = 0 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			cfg := valid
			test.mutate(&cfg)
			if _, err := NewRowTiming(cfg); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestRowTimingTimeErrorAndSpan(t *testing.T) {
	timing, err := NewRowTiming(RowTimingConfig{
		FirstExposureStart: time.Millisecond,
		RowPeriod:          10 * time.Microsecond,
		ExposureDuration:   100 * time.Microsecond,
		PoseSampleOffset:   50 * time.Microsecond,
		Direction:          TopToBottom,
	})
	if err != nil {
		t.Fatal(err)
	}
	row, _ := timing.Row(10, 100)
	if got, want := timing.SignedTimeError(row), 100*time.Microsecond; got != want {
		t.Fatalf("time error = %s, want %s", got, want)
	}
	if got, want := mustFrameSpan(t, timing, 100), 1090*time.Microsecond; got != want {
		t.Fatalf("frame span = %s, want %s", got, want)
	}
}

func mustFrameSpan(t *testing.T, timing RowTiming, height int) time.Duration {
	t.Helper()
	span, err := timing.FrameSpan(height)
	if err != nil {
		t.Fatal(err)
	}
	return span
}
