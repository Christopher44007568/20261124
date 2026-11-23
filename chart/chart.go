// Package chart builds deterministic raster calibration scenes using only the
// Go standard library. The scenes exercise white edges, colored glyphs, and
// highlights on a black background.
package chart

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

var (
	black = color.RGBA{A: 255}
	white = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	red   = color.RGBA{R: 255, A: 255}
	green = color.RGBA{G: 255, A: 255}
	blue  = color.RGBA{B: 255, A: 255}
)

// RGBTestChart renders the standard fidelity chart. It contains:
//   - white grid lines and a white ring on black,
//   - red, green, and blue text,
//   - small white highlights on black near all four corners.
func RGBTestChart(width, height int) (*image.RGBA, error) {
	if width < 32 || height < 32 {
		return nil, fmt.Errorf("chart must be at least 32x32, got %dx%d", width, height)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0] = black.R
		img.Pix[i+1] = black.G
		img.Pix[i+2] = black.B
		img.Pix[i+3] = black.A
	}

	thickness := maxInt(1, minInt(width, height)/360)
	for _, x := range []int{width / 4, 3 * width / 4} {
		fillRect(img, x-thickness/2, 0, x-thickness/2+thickness, height, white)
	}
	for _, y := range []int{height / 4, 3 * height / 4} {
		fillRect(img, 0, y-thickness/2, width, y-thickness/2+thickness, white)
	}

	centerX := (width - 1) / 2
	centerY := (height - 1) / 2
	ringRadius := 0.29 * float64(minInt(width, height))
	drawRing(img, centerX, centerY, ringRadius, maxFloat(1.5, float64(thickness)), white)

	cross := maxInt(4, minInt(width, height)/14)
	fillRect(img, centerX-cross/2, centerY, centerX+cross/2+1, centerY+1, white)
	fillRect(img, centerX, centerY-cross/2, centerX+1, centerY+cross/2+1, white)

	scale := maxInt(2, minInt(width, height)/120)
	textX := width / 14
	textY := height / 16
	drawColoredString(img, "R", textX, textY, scale, []color.RGBA{red})
	drawColoredString(img, "G", textX+6*scale, textY, scale, []color.RGBA{green})
	drawColoredString(img, "B", textX+12*scale, textY, scale, []color.RGBA{blue})

	yellow := color.RGBA{R: 255, G: 220, A: 255}
	cyan := color.RGBA{G: 255, B: 255, A: 255}
	drawColoredString(img, "CA", 7*width/10, textY, scale, []color.RGBA{yellow, cyan})

	highlightSigma := math.Max(2.5, float64(minInt(width, height))*0.012)
	insetX := maxInt(28, width/7)
	insetY := maxInt(28, height/7)
	for _, p := range [][2]int{
		{insetX, insetY},
		{width - 1 - insetX, insetY},
		{insetX, height - 1 - insetY},
		{width - 1 - insetX, height - 1 - insetY},
	} {
		drawHighlight(img, float64(p[0]), float64(p[1]), highlightSigma)
	}

	return img, nil
}

func drawHighlight(img *image.RGBA, cx, cy, sigma float64) {
	radius := int(math.Ceil(4 * sigma))
	minX := maxInt(0, int(cx)-radius)
	maxX := minInt(img.Rect.Dx()-1, int(cx)+radius)
	minY := maxInt(0, int(cy)-radius)
	maxY := minInt(img.Rect.Dy()-1, int(cy)+radius)
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			value := 255 * math.Exp(-(dx*dx+dy*dy)/(2*sigma*sigma))
			if value < 1 {
				continue
			}
			offset := img.PixOffset(x, y)
			v := uint8(math.Round(value))
			img.Pix[offset+0] = maxByte(img.Pix[offset+0], v)
			img.Pix[offset+1] = maxByte(img.Pix[offset+1], v)
			img.Pix[offset+2] = maxByte(img.Pix[offset+2], v)
		}
	}
}

func drawRing(img *image.RGBA, cx, cy int, radius, thickness float64, c color.RGBA) {
	limit := int(math.Ceil(radius + thickness + 1))
	for y := maxInt(0, cy-limit); y <= minInt(img.Rect.Dy()-1, cy+limit); y++ {
		for x := maxInt(0, cx-limit); x <= minInt(img.Rect.Dx()-1, cx+limit); x++ {
			dx := float64(x - cx)
			dy := float64(y - cy)
			if math.Abs(math.Hypot(dx, dy)-radius) <= thickness*0.5 {
				setPixel(img, x, y, c)
			}
		}
	}
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := maxInt(0, y0); y < minInt(img.Rect.Dy(), y1); y++ {
		for x := maxInt(0, x0); x < minInt(img.Rect.Dx(), x1); x++ {
			setPixel(img, x, y, c)
		}
	}
}

func setPixel(img *image.RGBA, x, y int, c color.RGBA) {
	if !image.Pt(x, y).In(img.Rect) {
		return
	}
	offset := img.PixOffset(x, y)
	img.Pix[offset+0] = c.R
	img.Pix[offset+1] = c.G
	img.Pix[offset+2] = c.B
	img.Pix[offset+3] = c.A
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func maxByte(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
