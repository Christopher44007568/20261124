package integration_test

import (
	"image/color"
	"testing"

	"github.com/Christopher44007568/20261124/chart"
	"github.com/Christopher44007568/20261124/chroma"
)

func TestCalibrationChartCenterAndFringes(t *testing.T) {
	const width, height = 321, 241
	src, err := chart.RGBTestChart(width, height)
	if err != nil {
		t.Fatal(err)
	}
	cfg := chroma.Config{
		Aberration: chroma.Aberration{Red: 0.05, Green: 0, Blue: -0.05},
		Edge:       chroma.EdgeBlack,
	}
	out, err := chroma.WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := chroma.WarpImage(src, chroma.Config{})
	if err != nil {
		t.Fatal(err)
	}

	center := out.RGBAAt((width-1)/2, (height-1)/2)
	if center.R != center.G || center.G != center.B || center.R < 200 {
		t.Fatalf("center crosshair split by aberration: %+v", center)
	}

	lineRegion := imageRegion{x0: width/4 - 15, y0: height/3 - 15, x1: width/4 + 15, y1: height/3 + 15}
	if got := colorFringePixels(out, lineRegion, 16); got == 0 {
		t.Fatal("white line region has no composite color fringe")
	}
	if got := colorFringePixels(flat, lineRegion, 1); got != 0 {
		t.Fatalf("undistorted grayscale line gained %d colored pixels", got)
	}

	insetX := max(28, width/7)
	insetY := max(28, height/7)
	highlightRegion := imageRegion{
		x0: width - 1 - insetX - 18,
		y0: height - 1 - insetY - 18,
		x1: width - 1 - insetX + 18,
		y1: height - 1 - insetY + 18,
	}
	if got := colorFringePixels(out, highlightRegion, 8); got == 0 {
		t.Fatal("corner highlight region has no composite color fringe")
	}
	if got := colorFringePixels(flat, highlightRegion, 1); got != 0 {
		t.Fatalf("undistorted grayscale highlight gained %d colored pixels", got)
	}
}

type imageRegion struct {
	x0, y0, x1, y1 int
}

func colorFringePixels(img interface {
	RGBAAt(x, y int) color.RGBA
}, region imageRegion, threshold uint8) int {
	count := 0
	for y := region.y0; y <= region.y1; y++ {
		for x := region.x0; x <= region.x1; x++ {
			c := img.RGBAAt(x, y)
			high := max(c.R, c.G, c.B)
			low := min(c.R, c.G, c.B)
			if high-low > threshold {
				count++
			}
		}
	}
	return count
}
