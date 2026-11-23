package chart

import "testing"

func TestRGBTestChartContainsRequiredTargets(t *testing.T) {
	const width, height = 641, 481
	img, err := RGBTestChart(width, height)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.Bounds().Dx(); got != width {
		t.Fatalf("width = %d, want %d", got, width)
	}
	if got := img.Bounds().Dy(); got != height {
		t.Fatalf("height = %d, want %d", got, height)
	}

	if got := img.RGBAAt(1, 1); got.R != 0 || got.G != 0 || got.B != 0 {
		t.Fatalf("background pixel = %+v, want black", got)
	}

	line := img.RGBAAt(width/4, height/2)
	if line.R < 250 || line.G < 250 || line.B < 250 {
		t.Fatalf("white grid line pixel = %+v", line)
	}

	center := img.RGBAAt((width-1)/2, (height-1)/2)
	if center.R < 250 || center.G < 250 || center.B < 250 {
		t.Fatalf("center crosshair pixel = %+v", center)
	}

	var redPixels, greenPixels, bluePixels int
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.RGBAAt(x, y)
			switch {
			case c.R > 200 && c.G < 20 && c.B < 20:
				redPixels++
			case c.G > 200 && c.R < 20 && c.B < 20:
				greenPixels++
			case c.B > 200 && c.R < 20 && c.G < 20:
				bluePixels++
			}
		}
	}
	if redPixels == 0 || greenPixels == 0 || bluePixels == 0 {
		t.Fatalf("missing colored text: red=%d green=%d blue=%d", redPixels, greenPixels, bluePixels)
	}

	insetX := maxInt(28, width/7)
	insetY := maxInt(28, height/7)
	highlight := img.RGBAAt(width-1-insetX, height-1-insetY)
	if highlight.R < 200 || highlight.G < 200 || highlight.B < 200 {
		t.Fatalf("corner highlight center = %+v", highlight)
	}
}

func TestRGBTestChartRejectsTinyImages(t *testing.T) {
	if _, err := RGBTestChart(31, 100); err == nil {
		t.Fatal("expected tiny chart to be rejected")
	}
	if _, err := RGBTestChart(100, 31); err == nil {
		t.Fatal("expected tiny chart to be rejected")
	}
}
