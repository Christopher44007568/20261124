package chroma

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestWhiteLineChannelsMoveIndependently(t *testing.T) {
	const (
		width  = 201
		height = 201
		lineX  = 150
		lineW  = 5
	)
	src := solidImage(width, height, color.RGBA{R: 0, G: 0, B: 0, A: 255})
	for y := 0; y < height; y++ {
		for x := lineX - lineW/2; x <= lineX+lineW/2; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}

	cfg := Config{Aberration: Aberration{Red: 0.15, Green: 0, Blue: -0.15}}
	out, err := WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}

	redX := channelCentroidX(out, Red, 90, 110, 80, 170, 32)
	greenX := channelCentroidX(out, Green, 90, 110, 80, 170, 32)
	blueX := channelCentroidX(out, Blue, 90, 110, 80, 170, 32)
	if !(redX < greenX && greenX < blueX) {
		t.Fatalf("white-line channel centroids not ordered R<G<B: R=%v G=%v B=%v", redX, greenX, blueX)
	}
	if greenX-redX < 0.5 || blueX-greenX < 0.5 {
		t.Fatalf("channel separation too small: R=%v G=%v B=%v", redX, greenX, blueX)
	}

	fringePixels := 0
	for y := 90; y <= 110; y++ {
		for x := 80; x <= 170; x++ {
			r, g, b, _ := out.RGBAAt(x, y).RGBA()
			high := maxUint32(r, g, b)
			low := minUint32(r, g, b)
			if high-low > 32*257 {
				fringePixels++
			}
		}
	}
	if fringePixels == 0 {
		t.Fatal("white line did not produce a composite color fringe")
	}
}

func TestWarpHasExactZeroAberrationAtOddImageCenter(t *testing.T) {
	const size = 201
	src := solidImage(size, size, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	cfg := Config{Aberration: Aberration{Red: 0.5, Green: -0.2, Blue: -0.5}}
	out, err := WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}
	got := out.RGBAAt(size/2, size/2)
	if got.R != got.G || got.G != got.B {
		t.Fatalf("center pixel has channel split: %+v", got)
	}
}

func TestOutOfBoundsModes(t *testing.T) {
	const size = 16
	src := solidImage(size, size, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	cfg := Config{Aberration: Aberration{Red: 1.2, Green: 1.2, Blue: 1.2}}

	cfg.Edge = EdgeBlack
	black, err := WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := black.RGBAAt(size-1, size-1); got != (color.RGBA{A: 255}) {
		t.Fatalf("black edge corner = %+v, want opaque black", got)
	}

	cfg.Edge = EdgeClamp
	clamped, err := WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := clamped.RGBAAt(size-1, size-1); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("clamped edge corner = %+v, want opaque white", got)
	}

	cfg.Edge = EdgeTransparent
	transparent, err := WarpImage(src, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := transparent.RGBAAt(size-1, size-1); got != (color.RGBA{}) {
		t.Fatalf("transparent edge corner = %+v, want transparent black", got)
	}
}

func TestMaskIsWarpedSeparatelyForEachChannel(t *testing.T) {
	const (
		width  = 201
		height = 201
		lineX  = 150
		lineW  = 5
	)
	src := solidImage(width, height, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	mask := image.NewGray(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := lineX - lineW/2; x <= lineX+lineW/2; x++ {
			mask.SetGray(x, y, color.Gray{Y: 255})
		}
	}

	cfg := Config{Aberration: Aberration{Red: 0.15, Green: 0, Blue: -0.15}}
	result, err := Warp(src, mask, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(result.Masks[Red].Pix, result.Masks[Green].Pix) ||
		bytes.Equal(result.Masks[Green].Pix, result.Masks[Blue].Pix) {
		t.Fatal("misaligned channel masks were reused")
	}

	redX := grayCentroidX(result.Masks[Red], 90, 110, 80, 170, 32)
	greenX := grayCentroidX(result.Masks[Green], 90, 110, 80, 170, 32)
	blueX := grayCentroidX(result.Masks[Blue], 90, 110, 80, 170, 32)
	if !(redX < greenX && greenX < blueX) {
		t.Fatalf("mask centroids not ordered R<G<B: R=%v G=%v B=%v", redX, greenX, blueX)
	}

	mapper, err := NewMapper(width, height, cfg)
	if err != nil {
		t.Fatal(err)
	}
	for ch := Red; ch <= Blue; ch++ {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				point := mapper.MapPixel(x, y, ch)
				want := toByte(sampleGray(mask, point.X, point.Y, cfg.Edge))
				got := result.Masks[ch].GrayAt(x, y).Y
				if got != want {
					t.Fatalf("%s mask at (%d,%d) = %d, want %d", ch, x, y, got, want)
				}
			}
		}
	}
}

func TestWarpRejectsMaskSizeMismatch(t *testing.T) {
	src := solidImage(20, 20, color.RGBA{R: 255, A: 255})
	mask := image.NewGray(image.Rect(0, 0, 19, 20))
	if _, err := Warp(src, mask, Config{}); err == nil {
		t.Fatal("expected mask size mismatch")
	}
}

func TestWarpHandlesNonZeroImageOrigin(t *testing.T) {
	src := image.NewRGBA(image.Rect(10, 20, 20, 30))
	for y := 20; y < 30; y++ {
		for x := 10; x < 20; x++ {
			src.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	mask := image.NewGray(image.Rect(10, 20, 20, 30))
	for y := 20; y < 30; y++ {
		for x := 10; x < 20; x++ {
			mask.SetGray(x, y, color.Gray{Y: 255})
		}
	}
	result, err := Warp(src, mask, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Image.RGBAAt(0, 0); got != (color.RGBA{255, 255, 255, 255}) {
		t.Fatalf("warped top-left = %+v, want white", got)
	}
	if got := result.Masks[Blue].GrayAt(9, 9).Y; got != 255 {
		t.Fatalf("warped mask bottom-right = %d, want 255", got)
	}
}

func solidImage(width, height int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func channelCentroidX(img *image.RGBA, ch Channel, y0, y1, x0, x1 int, threshold uint8) float64 {
	var sum, weight float64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			c := img.RGBAAt(x, y)
			value := c.R
			if ch == Green {
				value = c.G
			}
			if ch == Blue {
				value = c.B
			}
			if value < threshold {
				continue
			}
			sum += float64(x) * float64(value)
			weight += float64(value)
		}
	}
	if weight == 0 {
		return 0
	}
	return sum / weight
}

func grayCentroidX(img *image.Gray, y0, y1, x0, x1 int, threshold uint8) float64 {
	var sum, weight float64
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			value := img.GrayAt(x, y).Y
			if value < threshold {
				continue
			}
			sum += float64(x) * float64(value)
			weight += float64(value)
		}
	}
	if weight == 0 {
		return 0
	}
	return sum / weight
}

func maxUint32(a, b, c uint32) uint32 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

func minUint32(a, b, c uint32) uint32 {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}
